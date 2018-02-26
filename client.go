package ssh

import (
    //"fmt"
    "bytes"
    "strconv"

    "golang.org/x/crypto/ssh"
)

type Client struct {
    client      *ssh.Client
}

// NewDockerClient initializes a docker client to remote cluster
// following authentication configuration
func NewClient(config *Config) (*Client, error) {
    c := &Client{}

    client, err := ssh.Dial("tcp", config.Host + ":" + strconv.Itoa(config.Port), config.ClientConfig)
    if err != nil {
        return nil, err
    }

    c.client = client
    return c, nil
}

func (c *Client) ExecCommand(cmd string) ([]byte, error) {
    var buf bytes.Buffer

    session, err := c.client.NewSession()
    if err != nil {
        return nil, err
    }

    session.Stdout = &buf

    err = session.Run(cmd)
    if err != nil {
        return nil, err
    }

    return buf.Bytes(), nil
}
