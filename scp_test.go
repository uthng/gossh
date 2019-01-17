package gossh

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	//"strings"
	//"bytes"
	"testing"

	"github.com/gliderlabs/ssh"

	"github.com/stretchr/testify/assert"
)

func sessionHandler(s ssh.Session) {
	args := s.Command()

	cmd := exec.Command(args[0], args[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		//fmt.Println("err", err)
		fmt.Fprintln(s, "\x01failed to create stdout pipe")
		return
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		//fmt.Printf("err %s\n", err)
		fmt.Fprintln(s, "\x01Cannot create stdout pipe")
		return
	}

	if err := cmd.Start(); err != nil {
		//fmt.Println("err", err.Error())
		fmt.Fprintln(s, "\x01failed to start command")
		return
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, s)
	}()

	go func() {
		io.Copy(s, stdout)
	}()

	if err := cmd.Wait(); err != nil {
		fmt.Fprintln(s, "\x01failed to wait command execution")
		return
	}
}

func TestSCPBytes(t *testing.T) {
	var content = []byte(`SCP single file transfer test`)

	testCases := []struct {
		name   string
		dest   string
		output interface{}
	}{
		{
			"DirNotExist",
			"/tmp/scp/scp_single_file",
			"scp: /tmp/scp/scp_single_file: No such file or directory\n",
		},
		{
			"FileOK",
			"/tmp/scp_single_file",
			"SCP single file transfer test",
		},
	}

	s := &ssh.Server{
		Addr:    ":2222",
		Handler: sessionHandler,
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
		fmt.Println("err", err.Error())
		return
	}

	assert.NotNil(t, client)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client.ExecCommand("rm -rf " + tc.dest)
			err = client.SCPBytes(content, tc.dest)
			if err != nil {
				assert.Equal(t, tc.output, err.Error())
				return
			}

			content, err := ioutil.ReadFile(tc.dest)
			assert.Nil(t, err)
			assert.Equal(t, tc.output, string(content))
		})
	}
}
