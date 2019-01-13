package gossh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// This code is based on: https://web.archive.org/web/20170215184048/https://blogs.oracle.com/janp/entry/how_the_scp_protocol_works

const (
	msgCopyFile = "C"
	msgStartDir = "D"
	msgEndDir   = "E"
	msgTime     = "T"

	// reply or send to end tranfer
	msgOK       = '\x00'
	msgErr      = '\x01'
	msgFatalErr = '\x02'
)

const (
	// SCPFILE file type
	SCPFILE = iota
	// SCPDIR directory type, so recursive
	SCPDIR
)

type scpSession struct {
	session *ssh.Session
	in      io.WriteCloser
	out     io.Reader
	err     io.Reader
	timeout time.Duration
}

func newSCPSession(session *ssh.Session) (*scpSession, error) {

	in, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}

	out, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	e, err := session.StderrPipe()
	if err != nil {
		return nil, err
	}

	s := &scpSession{
		session: session,
		in:      in,
		out:     out,
		err:     e,
		timeout: time.Minute * 15,
	}

	return s, nil
}

// SendBytes creates a file with given name and the byte content
// as content on remote host
func (s *scpSession) SendBytes(content []byte, remoteFile string) error {
	reader := bytes.NewReader(content)

	return s.SendFile("0777", int64(reader.Len()), remoteFile, ioutil.NopCloser(reader))
}

// SendFile creates a file and writes its content to send in console scpSession//.
// remoteFile must be the absolute path.
func (s *scpSession) SendFile(mode string, length int64, remoteFile string, content io.ReadCloser) error {

	return s.execSCPSession(SCPFILE, remoteFile, func() error {
		filename := filepath.Base(remoteFile)

		_, err := fmt.Fprintf(s.in, "%s%s %d %s\n", msgCopyFile, mode, length, filename)
		if err != nil {
			return fmt.Errorf("failed to create a new file: err=%s", err)
		}

		err = s.readReply()
		if err != nil {
			return err
		}

		_, err = io.Copy(s.in, content)
		defer content.Close()
		if err != nil {
			return fmt.Errorf("error while writing content file: err=%s", err)
		}

		err = s.readReply()
		if err != nil {
			return err
		}

		_, err = s.in.Write([]byte{msgOK})
		if err != nil {
			return fmt.Errorf("error while ending transfer: err=%s", err)
		}

		err = s.readReply()
		if err != nil {
			return err
		}

		return err
	})
}

// StartDirectory starts a recursive directory
func (s *scpSession) StartDirectory(mode string, remoteDir string) error {
	dirname := filepath.Base(remoteDir)

	_, err := fmt.Fprintf(s.in, "%s%s %d %s\n", msgStartDir, mode, 0, dirname)
	if err != nil {
		return fmt.Errorf("error while starting a recursive directory: err=%s", err)
	}

	//return s.readReply()
	return nil
}

// EndDirectory ends a recursive directory
func (s *scpSession) EndDirectory() error {
	_, err := fmt.Fprintf(s.in, "%s\n", msgEndDir)
	if err != nil {
		return fmt.Errorf("error while ending a recursive directory: err=%s", err)
	}

	//return s.readReply()
	return nil
}

///////// INTERNAL FUNCTIONS ////////////////////////////

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})

	// Wait for all wait groups are closed
	// and then, close channel
	go func() {
		defer close(c)
		wg.Wait()
	}()

	// Select either channel closed or timeout
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

func (s *scpSession) execSCPSession(kind int, dest string, fn func() error) error {
	var opt string

	// Check type to use correct scp's options
	if kind == SCPFILE {
		opt = "-qt"
	} else if kind == SCPDIR {
		opt = "-rt"
	} else {
		return fmt.Errorf("scp type unknown. Only file or dir is supported")
	}

	defer s.session.Close()

	// Create group to wait 2 routines: start cmd & transfer
	wg := sync.WaitGroup{}
	wg.Add(2)

	// Create a channel for 2 errors:
	// - Start cmd scp
	// - Exec handler scp command
	errCh := make(chan error, 2)

	// Start command
	go func() {
		defer wg.Done()

		cmd := "scp " + opt + " " + dest

		err := s.session.Start(cmd)
		if err != nil {
			errCh <- err
			return
		}
	}()

	// Exec scp handler command
	go func() {
		defer wg.Done()
		defer s.in.Close()

		err := fn()
		if err != nil {
			errCh <- err
			return
		}

	}()

	// Wait for timeout or all wait groupsare done
	if waitTimeout(&wg, s.timeout) {
		return fmt.Errorf("timeout when upload files")
	}

	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return s.session.Wait()
}

func (s *scpSession) readReply() error {
	reader := bufio.NewReader(s.out)
	buffer := make([]byte, 1024)

	n, err := reader.Read(buffer)
	if err != nil {
		return fmt.Errorf("error while reading reply: err=%s", err)
	}

	//fmt.Println("n", n)
	//fmt.Printf("buffer %q\n", string(buffer[:n]))

	b1 := buffer[0]
	b2 := buffer[1]

	// if 1st byte == ok
	if b1 == msgOK {
		// Check if 2nd is error or not.
		// In case of folder does not exist,
		// returned reply: \x00\x01scp: /tmp/titi/toto: No such file or directory
		if b2 != msgErr && b2 != msgFatalErr {
			return nil
		}

		if n > 2 {
			return fmt.Errorf(string(buffer[2:n]))
		}

		return fmt.Errorf("scp: unknown error")
	}

	// 1st byte != ok
	// byte error unknown
	if b1 != msgErr && b1 != msgFatalErr {
		return fmt.Errorf("unexpected reply error type: %v", b1)
	}

	if n > 1 {
		return fmt.Errorf(string(buffer[1:n]))
	}

	if b1 == msgErr {
		return fmt.Errorf("scp: error")
	}

	if b1 == msgFatalErr {
		return fmt.Errorf("scp: fatal error")
	}

	return nil
}
