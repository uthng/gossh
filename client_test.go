package gossh

import (
	"fmt"
	"io/ioutil"
	//"os"
	"strings"
	"testing"

	"github.com/gliderlabs/ssh"

	"github.com/stretchr/testify/assert"
)

func TestClientUserPass(t *testing.T) {
	s := &ssh.Server{
		Addr: ":2222",
		Handler: func(s ssh.Session) {
			fmt.Fprintf(s, "%s", strings.Join(s.Command(), " "))
		},
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return ctx.User() == "user" && password == "pass"
		},
	}

	defer s.Close()
	go s.ListenAndServe()

	config, err := NewClientConfigWithUserPass("user", "pass", "localhost", 2222, false)
	if !assert.Nil(t, err) {
		return
	}

	client, err := NewClient(config)
	if !assert.Nil(t, err) {
		return
	}

	assert.NotNil(t, client)

	cmd := "echo HelloWorld"
	res, err := client.ExecCommand(cmd)
	assert.Nil(t, err)
	assert.Equal(t, string(res), cmd)
}

func TestExecCommand(t *testing.T) {
	s := &ssh.Server{
		Addr: ":2222",
		Handler: func(s ssh.Session) {
			fmt.Fprintf(s, "%s", strings.Join(s.Command(), " "))
		},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			data, _ := ioutil.ReadFile("./data/id_rsa.pub")
			allowed, _, _, _, _ := ssh.ParseAuthorizedKey(data)
			return ctx.User() == "user" && ssh.KeysEqual(key, allowed)
		},
	}

	defer s.Close()
	go s.ListenAndServe()

	config, err := NewClientConfigWithKeyFile("user", "./data/id_rsa", "localhost", 2222, false)
	if !assert.Nil(t, err) {
		return
	}

	client, err := NewClient(config)
	if !assert.Nil(t, err) {
		return
	}

	assert.NotNil(t, client)

	cmd := "echo HelloWorld"
	res, err := client.ExecCommand(cmd)
	assert.Nil(t, err)
	assert.Equal(t, string(res), cmd)
}
