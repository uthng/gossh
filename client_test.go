package gossh

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/gliderlabs/ssh"

	"github.com/stretchr/testify/require"
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
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithUserPass("user", "pass", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	cmd := "echo HelloWorld"
	res, err := client.ExecCommand(cmd)
	require.Nil(t, err)
	require.Equal(t, string(res), cmd)
}

func TestExecCommandWithKeyFile(t *testing.T) {
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
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithKeyFile("user", "./data/id_rsa", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	cmd := "echo HelloWorld"
	res, err := client.ExecCommand(cmd)
	require.Nil(t, err)
	require.Equal(t, string(res), cmd)
}

func TestExecCommandWithSignedPubKey(t *testing.T) {
	s := &ssh.Server{
		Addr: ":2222",
		Handler: func(s ssh.Session) {
			fmt.Fprintf(s, "%s", strings.Join(s.Command(), " "))
		},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			data, _ := ioutil.ReadFile("./data/ca.pub")
			allowed, _, _, _, _ := ssh.ParseAuthorizedKey(data)

			// Public key signed by a certificate authority is not
			// a simple key but a certificate with a little more fields.
			// Among them, the SignatureKey field is the ca.pub.
			cert := key.(*gossh.Certificate)

			return ctx.User() == "user" && ssh.KeysEqual(cert.SignatureKey, allowed)
		},
	}
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithSignedPubKeyFile("user", "./data/id_rsa", "./data/id_rsa-cert.pub", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	cmd := "echo HelloWorld"
	res, err := client.ExecCommand(cmd)
	require.Nil(t, err)

	require.Equal(t, string(res), cmd)
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
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithUserPass("user", "pass", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client.ExecCommand("rm -rf " + path.Dir(tc.dest))

			err = client.SCPBytes(content, tc.dest, "0777")
			if err != nil {
				require.Equal(t, tc.output, err.Error())
				return
			}

			content, err := ioutil.ReadFile(tc.dest)
			require.Nil(t, err)
			require.Equal(t, tc.output, string(content))
		})
	}
}

func TestSCPFile(t *testing.T) {
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
			"SCP single file transfer test\n",
		},
	}

	s := &ssh.Server{
		Addr:    ":2222",
		Handler: sessionHandler,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return ctx.User() == "user" && password == "pass"
		},
	}
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithUserPass("user", "pass", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client.ExecCommand("rm -rf " + path.Dir(tc.dest))

			err = client.SCPFile("./data/scp_single_file", tc.dest, "0777")
			if err != nil {
				require.Equal(t, tc.output, err.Error())
				return
			}

			content, err := ioutil.ReadFile(tc.dest)
			require.Nil(t, err)
			require.Equal(t, tc.output, string(content))
		})
	}
}

func TestSCPDir(t *testing.T) {
	testCases := []struct {
		name   string
		dest   string
		output interface{}
	}{
		{
			"DirOK",
			"/tmp/scp",
			[]string{
				"/tmp/scp",
				"/tmp/scp/bin",
				"/tmp/scp/bin/mac",
				"/tmp/scp/bin/mac/gobin",
				"/tmp/scp/ca",
				"/tmp/scp/ca.pub",
				"/tmp/scp/folder1",
				"/tmp/scp/folder1/test1",
				"/tmp/scp/folder1/test2",
				"/tmp/scp/folder2",
				"/tmp/scp/folder2/test1",
				"/tmp/scp/folder2/test2",
				"/tmp/scp/id_rsa",
				"/tmp/scp/id_rsa-cert.pub",
				"/tmp/scp/id_rsa.pub",
				"/tmp/scp/lorem.txt",
				"/tmp/scp/scp_single_file",
			},
		},
	}

	s := &ssh.Server{
		Addr:    ":2222",
		Handler: sessionHandler,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return ctx.User() == "user" && password == "pass"
		},
	}
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithUserPass("user", "pass", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client.ExecCommand("rm -rf " + tc.dest)

			err = client.SCPDir("./data", tc.dest, "0777")
			if err != nil {
				require.Equal(t, tc.output, err.Error())
				return
			}

			res, err := client.ExecCommand("find " + tc.dest)
			require.Nil(t, err)
			arr := strings.Split(string(res), "\n")

			arr = arr[:len(arr)-1]

			sort.Strings(arr)
			require.Equal(t, tc.output, arr)

			client.ExecCommand("rm -rf " + tc.dest)
		})
	}
}

func TestSCPGetFile(t *testing.T) {
	testCases := []struct {
		name   string
		src    string
		dest   string
		output interface{}
	}{
		{
			"ErrFileNotfound",
			"/tmp/lorem.txt",
			"./data/remote",
			"scp: /tmp/lorem.txt: No such file or directory\n",
		},
		{
			"ErrFileOKInFoler",
			"/tmp/data/lorem.txt",
			"./data/remote/lorem.txt",
			nil,
		},
	}

	s := &ssh.Server{
		Addr:    ":2222",
		Handler: sessionHandler,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return ctx.User() == "user" && password == "pass"
		},
	}
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithUserPass("user", "pass", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up data before executing tests
			cmd := exec.Command("bash", "-c", "rm -rf /tmp/data; rm -rf ./data/remote; cp -r ./data /tmp/")
			_, err := cmd.CombinedOutput()
			require.Nil(t, err)

			err = client.SCPGetFile(tc.src, tc.dest)
			if err != nil {
				require.Equal(t, tc.output, err.Error())
				return
			}

			require.FileExists(t, tc.dest)

			gotten, err := ioutil.ReadFile(tc.dest)
			require.Nil(t, err)

			expected, err := ioutil.ReadFile(tc.src)
			require.Nil(t, err)

			require.Equal(t, expected, gotten)

			// Clean up data after tests
			cmd = exec.Command("bash", "-c", "rm -rf ./data/remote")
			_, err = cmd.CombinedOutput()
			require.Nil(t, err)
		})
	}
}

func TestSCPGetDir(t *testing.T) {
	testCases := []struct {
		name   string
		src    string
		dest   string
		output interface{}
	}{
		//{
		//"ErrFileNotfound",
		//"/tmp/lorem.txt",
		//"./data/remote",
		//"scp: /tmp/lorem.txt: No such file or directory\n",
		//},
		{
			"OKDir",
			"/tmp/data",
			"/tmp/remote",
			nil,
		},
	}

	s := &ssh.Server{
		Addr:    ":2222",
		Handler: sessionHandler,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return ctx.User() == "user" && password == "pass"
		},
	}
	go s.ListenAndServe()

	defer s.Close()

	time.Sleep(3 * time.Second)

	config, err := NewClientConfigWithUserPass("user", "pass", "localhost", 2222, false)
	require.Nil(t, err)

	client, err := NewClient(config)
	require.Nil(t, err)

	require.NotNil(t, client)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up data before executing tests
			cmd := exec.Command("bash", "-c", "rm -rf "+tc.dest+"; rm -rf "+tc.src+"; cp -r ./data /tmp/")
			_, err := cmd.CombinedOutput()
			require.Nil(t, err)

			err = client.SCPGetDir(tc.src, tc.dest)
			if err != nil {
				require.Equal(t, tc.output, err.Error())
				return
			}

			cmd = exec.Command("bash", "-c", "diff -r "+tc.src+" "+tc.dest+"/"+path.Base(tc.src))
			output, err := cmd.CombinedOutput()
			require.Nil(t, err)
			require.Empty(t, output)

			//Execution of gobin to test if the transfer is correct
			cmd = exec.Command("bash", "-c", tc.dest+"/"+path.Base(tc.src)+"/bin/mac/gobin -h")
			_, err = cmd.CombinedOutput()
			require.Nil(t, err)

			//Clean up data after tests
			cmd = exec.Command("bash", "-c", "rm -rf "+tc.dest)
			_, err = cmd.CombinedOutput()
			require.Nil(t, err)
		})
	}
}
