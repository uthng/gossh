package ssh

import (
	"bytes"
	//"fmt"
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
