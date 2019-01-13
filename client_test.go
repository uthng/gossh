package gossh

import (
	//"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecCommand(t *testing.T) {
	sshUser := os.Getenv("SSH_USER")
	sshPrivKey := os.Getenv("SSH_PRIVKEY")
	sshHost := os.Getenv("SSH_HOST")

	config, err := NewClientConfigWithKeyFile(sshUser, sshPrivKey, sshHost, 22, false)
	if !assert.Nil(t, err) {
		return
	}

	client, err := NewClient(config)
	if !assert.Nil(t, err) {
		return
	}

	assert.NotNil(t, client)

	_, err = client.ExecCommand("ls")
	assert.Nil(t, err)

	//fmt.Println(string(res))
}

func TestSCPBytes(t *testing.T) {
	var content = []byte(`123455ototo totititititi`)

	sshUser := os.Getenv("SSH_USER")
	sshPrivKey := os.Getenv("SSH_PRIVKEY")
	sshHost := os.Getenv("SSH_HOST")

	config, err := NewClientConfigWithKeyFile(sshUser, sshPrivKey, sshHost, 22, false)
	if !assert.Nil(t, err) {
		return
	}

	client, err := NewClient(config)
	if !assert.Nil(t, err) {
		return
	}

	assert.NotNil(t, client)

	err = client.SCPBytes(content, "/tmp/titi/toto")
	assert.Nil(t, err)

	//fmt.Println(string(res))
}
