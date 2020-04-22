package gossh

import (
	//"fmt"
	"os"
	"strconv"

	log "github.com/uthng/golog"

	"golang.org/x/crypto/ssh"
)

// Client encapsulates ssh client
type Client struct {
	client *ssh.Client
	logger *log.Logger
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

	c.logger = log.NewLogger()
	c.logger.SetVerbosity(log.NONE)

	return c, nil
}

// SetVerbosity sets log level
func (c *Client) SetVerbosity(level int) {
	c.logger.SetVerbosity(level)
}

// DisableLogColor disables log colors
func (c *Client) DisableLogColor(level int) {
	c.logger.DisableColor()
}

// ExecCommand executes a shell command on remote machine
func (c *Client) ExecCommand(cmd string) ([]byte, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	return session.CombinedOutput(cmd)
}

// SCPBytes sends content in bytes to remote machine and save it
// in a file with the given path
func (c *Client) SCPBytes(content []byte, destFile, mode string) error {
	c.checkLogEnvVars()

	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(c, session)
	if err != nil {
		return err
	}

	return scpSession.SendBytes(content, destFile, mode)
}

// SCPFile sends a file to remote machine
func (c *Client) SCPFile(srcFile, destFile, mode string) error {
	c.checkLogEnvVars()

	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(c, session)
	if err != nil {
		return err
	}

	return scpSession.SendFile(srcFile, destFile, mode)
}

// SCPDir sends recursively a directory to remote machine.
// Mode is only applied for the 1st directory. All files/folders
// inside the srcDir will preserve the same mode on remote machine
func (c *Client) SCPDir(srcDir, destDir, mode string) error {
	c.checkLogEnvVars()

	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(c, session)
	if err != nil {
		return err
	}

	return scpSession.SendDir(srcDir, destDir, mode)
}

// SCPGetFile gets srcFile from remote machine and save in destDir.
// srcFile must be a regular file.
// destFile is the local regular in which srcFile's content will be stored;.
// If destFile does not exists, it will be created.
func (c *Client) SCPGetFile(srcFile, destFile string) error {
	c.checkLogEnvVars()

	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(c, session)
	if err != nil {
		return err
	}

	return scpSession.GetFile(srcFile, destFile)
}

// SCPGetDir gets srcDir from remote machine and save in destDir.
// srcDir must be a folder. destDir is the local folder in which
// all files or subfolders inside srcDir will be stored.
// If destDir does not exists, it will be created.
func (c *Client) SCPGetDir(srcDir, destDir string) error {
	c.checkLogEnvVars()

	session, err := c.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	scpSession, err := newSCPSession(c, session)
	if err != nil {
		return err
	}

	return scpSession.GetDir(srcDir, destDir)
}

/////////////// INTERNAL FUNCTIONS //////////////////////////

func (c *Client) checkLogEnvVars() {
	verbosity := os.Getenv("GOSSH_VERBOSITY")
	if s, err := strconv.Atoi(verbosity); err == nil {
		c.logger.SetVerbosity(s)
	}

	disabled := os.Getenv("GOSSH_DISABLE_COLOR")
	if disabled != "" {
		c.logger.DisableColor()
	}
}
