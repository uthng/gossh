package gossh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cast"
	"golang.org/x/crypto/ssh"
)

// This code is based on: https://web.archive.org/web/20170215184048/https://blogs.oracle.com/janp/entry/how_the_scp_protocol_works

const (
	msgCopyFile = "C"
	msgStartDir = "D"
	msgEndDir   = "E"
	//msgTime     = "T"

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
	// SCPGETFILE download a remote file
	SCPGETFILE
	// SCPGETDIR download a remote direcotry
	SCPGETDIR
)

const (
	bufferSizeDataFile = 1024
)

type scpSession struct {
	session *ssh.Session
	in      io.WriteCloser
	out     io.Reader
	err     io.Reader
	timeout time.Duration

	myClient *Client
}

func newSCPSession(client *Client, session *ssh.Session) (*scpSession, error) {
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
		session:  session,
		in:       in,
		out:      out,
		err:      e,
		timeout:  time.Minute * 15,
		myClient: client,
	}

	return s, nil
}

// SendBytes creates a file with given name and the byte content
// as content on remote host
func (s *scpSession) SendBytes(content []byte, remoteFile, mode string) error {
	reader := bytes.NewReader(content)

	if mode == "" {
		mode = "0755"
	}

	return s.execSCPSession(SCPFILE, remoteFile, func() error {
		return s.sendFile(mode, int64(reader.Len()), remoteFile, ioutil.NopCloser(reader))
	})
}

// SendFile checks and reads content of a local file
// and send it to remote machine
func (s *scpSession) SendFile(localFile, remoteFile, mode string) error {
	localFile = filepath.Clean(localFile)
	remoteFile = filepath.Clean(remoteFile)

	fileInfo, err := os.Stat(localFile)
	if err != nil {
		return fmt.Errorf("failed to stat local file: err=%s", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("local file must a regular file, not a directory")
	}

	file, err := os.Open(localFile)
	defer file.Close()

	if err != nil {
		return fmt.Errorf("failed to open local file: err=%s", err)
	}

	// If mode isnot specified, use localFile's mode instead
	if mode == "" {
		mode = fmt.Sprintf("%#4o", fileInfo.Mode()&os.ModePerm)
	}

	return s.execSCPSession(SCPFILE, remoteFile, func() error {
		return s.sendFile(mode, fileInfo.Size(), remoteFile, file)
	})
}

// SendDir checks and reads recursively content of the given
// directory, then sends it to remote machine.
//
// mode is only applied for the directory. All files/subfolders will
// preserve the same mode on local
func (s *scpSession) SendDir(localDir, remoteDir, mode string) error {
	return s.execSCPSession(SCPDIR, remoteDir, func() error {
		return s.sendDir(localDir, remoteDir, mode)
	})
}

// GetFile gets remote file and save it to the local file.
// remoteFile must be the path to a regular file to download.
// localFile must be the path to the local folder in which remoteFile's will be saved.
// If localFile does not exist, it will be created.
func (s *scpSession) GetFile(remoteFile, localFile string) error {
	localFile = filepath.Clean(localFile)
	remoteFile = filepath.Clean(remoteFile)

	localFolder := path.Dir(localFile)
	remoteFilename := path.Base(remoteFile)

	// Check whether the localFile exists.
	// If not, we create systematically all parent directories.
	// If yes, we check if localFile is a directory or not.
	// If it is a directory then, the localFile will be a
	// concatenation of directory + remoteFile's filename.
	// If not, localFile remains as it is.
	fileInfo, err := os.Stat(localFile)
	if os.IsNotExist(err) {
		err := os.MkdirAll(localFolder, 0755)
		if err != nil {
			return err
		}
	} else {
		if fileInfo.IsDir() {
			localFile = localFolder + "/" + remoteFilename
		}
	}

	return s.execSCPSession(SCPGETFILE, remoteFile, func() error {
		return s.getFile(localFile)
	})
}

// GetDir gets remote folder's contents and save them to the local folder.
// remoteDir must be the path to a the folder to download.
// localDir must be the path to the local folder in which all subfolders and files
// in remoteDir will be downloaded and stored recursively.
// If localDir does not exist, it will be created.
func (s *scpSession) GetDir(remoteDir, localDir string) error {
	localDir = filepath.Clean(localDir)
	remoteDir = filepath.Clean(remoteDir)

	return s.execSCPSession(SCPGETDIR, remoteDir, func() error {
		return s.getDir(localDir)
	})
}

///////// INTERNAL FUNCTIONS ////////////////////////////

// sendFile creates a file and writes its content to send in console scpSession
// remoteFile must be the absolute path.
func (s *scpSession) sendFile(mode string, length int64, remoteFile string, content io.ReadCloser) error {
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
	//defer content.Close()
	if err != nil {
		return fmt.Errorf("error while writing content file: err=%s", err)
	}

	_, err = s.in.Write([]byte{msgOK})
	if err != nil {
		return fmt.Errorf("error while ending transfer: err=%s", err)
	}

	err = s.readReply()
	if err != nil {
		return err
	}

	return nil
}

// sendDir checks and reads recursively content of the given
// directory, then sends it to remote machine.
//
// mode is only applied for the directory. All files/subfolders will
// preserve the same mode on local
func (s *scpSession) sendDir(localDir, remoteDir, mode string) error {
	localDir = filepath.Clean(localDir)
	remoteDir = filepath.Clean(remoteDir)
	dirName := filepath.Base(localDir)

	// Read & check if localDir is a directory
	files, err := ioutil.ReadDir(localDir)
	if err != nil {
		return err
	}

	// new remote dir
	newRemoteDir := remoteDir + "/" + dirName
	// Create a new directory inside remoteDir
	err = s.startDirectory(mode, newRemoteDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			mode := fmt.Sprintf("%#4o", file.Mode()&os.ModePerm)

			err := s.sendDir(localDir+"/"+file.Name(), newRemoteDir, mode)
			if err != nil {
				return err
			}
		}

		if file.Mode().IsRegular() {
			localFile := localDir + "/" + file.Name()
			remoteFile := newRemoteDir + "/" + file.Name()

			fileLocal, err := os.Open(localFile)
			defer fileLocal.Close()

			if err != nil {
				return fmt.Errorf("failed to open local file: err=%s", err)
			} // If mode isnot specified, use localFile's mode instead

			mode := fmt.Sprintf("%#4o", file.Mode()&os.ModePerm)

			err = s.sendFile(mode, file.Size(), remoteFile, fileLocal)
			if err != nil {
				return err
			}
		}
	}

	// end directory creation
	err = s.endDirectory()
	if err != nil {
		return err
	}

	return nil
}

// startDirectory starts a recursive directory
func (s *scpSession) startDirectory(mode string, remoteDir string) error {
	dirname := filepath.Base(remoteDir)

	_, err := fmt.Fprintf(s.in, "%s%s %d %s\n", msgStartDir, mode, 0, dirname)
	if err != nil {
		return fmt.Errorf("error while starting a recursive directory: err=%s", err)
	}

	return s.readReply()
}

// endDirectory ends a recursive directory
func (s *scpSession) endDirectory() error {
	_, err := fmt.Fprintf(s.in, "%s\n", msgEndDir)
	if err != nil {
		return fmt.Errorf("error while ending a recursive directory: err=%s", err)
	}

	return s.readReply()
}

// getFile gets a remote file and writes its content to the given local file
func (s *scpSession) getFile(localFile string) error {
	//var err error
	var msg string
	var fields []string

	reader := bufio.NewReader(s.out)

	buffer, n, err := s.readMessage(reader)
	if err != nil {
		return err
	}

	msgType := string(buffer[0])

	if msgType == msgCopyFile {
		msg = string(buffer[1 : n-1])
		fields = strings.Split(msg, " ")

		return s.readFileData(reader, localFile, os.FileMode(cast.ToUint32(fields[0])), cast.ToInt(fields[1]))
	} else if buffer[0] == msgErr || buffer[0] == msgFatalErr {
		return fmt.Errorf("%s", string(buffer[1:n]))
	}

	return fmt.Errorf("expected message type '%s', received '%s'", msgCopyFile, msgType)
}

// getDir gets a remote folder and writes its contents to the given local folder
func (s *scpSession) getDir(localDir string) error {
	//var err error
	var msg string
	var fields []string

	reader := bufio.NewReader(s.out)

	currentDir := localDir

	for {
		buffer, n, err := s.readMessage(reader)
		if err == io.EOF {
			//fmt.Println("EEEEEEOFFFFFFFFFF")
			return nil
		} else if err != nil {
			return err
		}

		if buffer[0] == msgOK {
			buffer = buffer[1:]
			n = len(buffer)
		} else if buffer[0] == msgErr || buffer[0] == msgFatalErr {
			return fmt.Errorf("%s", string(buffer[1:n]))
		}

		msgType := string(buffer[0])

		if msgType == msgStartDir {
			msg = string(buffer[1 : n-1])
			fields = strings.Split(msg, " ")

			//fmt.Printf("fields %q\n", fields)
			currentDir = currentDir + "/" + fields[2]
			s.myClient.logger.Infow("D message", "fields", fields, "dir", currentDir)
			//fmt.Println("creating dir", currentDir)
			err := createLocalDir(currentDir, os.FileMode(cast.ToUint32(fields[0])))
			if err != nil {
				return err
			}
		} else if msgType == msgCopyFile {
			msg = string(buffer[1 : n-1])
			fields = strings.Split(msg, " ")

			//fmt.Printf("fields %q\n", fields)
			//fmt.Println("creating file", currentDir+"/"+fields[2])
			newFile := currentDir + "/" + fields[2]
			s.myClient.logger.Infow("C message", "fields", fields, "file", newFile)
			err := s.readFileData(reader, newFile, os.FileMode(cast.ToUint32(fields[0])), cast.ToInt(fields[1]))
			if err != nil {
				return err
			}
		} else if msgType == msgEndDir {
			s.myClient.logger.Infow("E message", "olddir", currentDir, "newdir", path.Dir(currentDir))
			currentDir = path.Dir(currentDir)
		}
	}
}

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
		opt = "-t"
	} else if kind == SCPDIR {
		opt = "-rt"
	} else if kind == SCPGETFILE {
		opt = "-f"
	} else if kind == SCPGETDIR {
		opt = "-rf"
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

func (s *scpSession) readMessage(reader *bufio.Reader) ([]byte, int, error) {
	// Send msgOK in order to receive data sent from remote machine
	_, err := s.in.Write([]byte{msgOK})
	if err != nil {
		return nil, 0, err
	}

	buffer, err := reader.ReadBytes('\n')

	s.myClient.logger.Infow("Procol message", "buffer", string(buffer), "len", len(buffer))
	//fmt.Println("n", len(buffer))
	//fmt.Printf("buffer %q\n", string(buffer))

	return buffer, len(buffer), err
}

func (s *scpSession) readFileData(reader *bufio.Reader, file string, mode os.FileMode, length int) error {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	cpt := 0

	_, err = s.in.Write([]byte{msgOK})
	if err != nil {
		return err
	}

	//fmt.Println("length", length)
	buf := make([]byte, bufferSizeDataFile)

	for {
		reachEnd := false

		n, _ := reader.Read(buf)

		s.myClient.logger.Debugw("Data file read", "buf", string(buf[:n]), "len", n)

		nbRead := n

		cpt = cpt + n
		if cpt >= length {
			reachEnd = true
			// For some unknown reasons, when we have buffer's size greater
			// than the data length, in some cases, the byte msgOK '\x00'
			// is not tramsitted in the same message data, but in the next one:
			// protocol message. We need to be sure that, in this case, we dont
			// remove the last byte which is '\n' instead of '\x00'.
			// For ex:
			// length 6
			// raw data "test1\n", 6
			// ##########################################
			// raw data length 6, nbRead 5
			// n 15
			// buffer "\x00C0644 6 test2\n"
			if buf[n-1] == msgOK {
				nbRead = nbRead - 1
			}

			//fmt.Printf("raw data length %d, nbRead %d, cpt %d\n", n, nbRead, cpt)
			s.myClient.logger.Debugw("End data file", "buf", string(buf[:nbRead]), "nbRead", nbRead, "cpt", cpt)
		}

		nbWrite, err := f.Write(buf[:nbRead])
		if err != nil {
			return err
		}

		if nbWrite != nbRead {
			return fmt.Errorf("bytes (%d) written to the file is not the same as bytes read (%d)", nbWrite, nbRead)
		}

		if reachEnd {
			return f.Sync()
		}
	}
}

//////// INTERNAL FUNCTIONS //////////

func createLocalDir(dir string, mode os.FileMode) error {
	// Check whether dir exists.
	// If not, we create it with all parent directories.
	// If yes, we check if dir is a directory or not.
	// If yes, dir remains as it is. Otherwise an error will be returned.
	fileInfo, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, mode)
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("%s already exists but not a directory", dir)
	}

	return nil
}
