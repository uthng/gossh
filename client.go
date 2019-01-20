package gossh

import (
	"bytes"
	//"fmt"
	//"io/ioutil"
	"strconv"

	"golang.org/x/crypto/ssh"
)

// Client encapsulates ssh client
type Client struct {
	client *ssh.Client
}

// NewClient initializes a ssh client following
// authentication configuration
func NewClient(config *Config) (*Client, error) {
	c := &Client{}

	client, err := ssh.Dial("tcp", config.Host+":"+strconv.Itoa(config.Port), config.ClientConfig)
	if err != nil {
		return nil, err
	}

	c.client = client
	return c, nil
}

// ExecCommand executes a shell command on remote machine
func (c *Client) ExecCommand(cmd string) ([]byte, error) {
	var buf bytes.Buffer

	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	session.Stdout = &buf

	err = session.Run(cmd)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// SCPBytes sends content in bytes to remote machine and save it
// in a file with the given path
func (c *Client) SCPBytes(content []byte, destFile, mode string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(session)
	if err != nil {
		return err
	}

	return scpSession.SendBytes(content, destFile, mode)
}

// SCPFile sends a file to remote machine
func (c *Client) SCPFile(srcFile, destFile, mode string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(session)
	if err != nil {
		return err
	}

	return scpSession.SendFile(srcFile, destFile, mode)
}

// SCPDir sends recursively a directory to remote machine.
// Mode is only applied for the 1st directory. All files/folders
// inside the srcDir will preserve the same mode on remote machine
func (c *Client) SCPDir(srcDir, destDir, mode string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(session)
	if err != nil {
		return err
	}

	return scpSession.SendDir(srcDir, destDir, mode)
}

/////////////// INTERNAL FUNCTIONS //////////////////////////
