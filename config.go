package gossh

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Config englobes ssh client configuration with host/port
type Config struct {
	Host         string
	Port         int
	ClientConfig *ssh.ClientConfig
}

// NewClientConfigWithKeyFile returns a configuration
// corresponding to a simple configuration with private key
func NewClientConfigWithKeyFile(username string, sshKey string, host string, port int, checkHostKey bool) (*Config, error) {
	var hostKey ssh.PublicKey

	c := &Config{
		Host: host,
		Port: port,
	}

	// Read private key
	key, err := ioutil.ReadFile(sshKey)
	if err != nil {
		return nil, err
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	if checkHostKey {
		//arr := strings.Split(host, ":")
		hostKey, err = getHostKey(host, strconv.Itoa(port))
		if err != nil {
			return nil, err
		}
	}

	c.ClientConfig = &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			//ssh.Password("chrYsal1s-adm1n"),
			//ssh.PublicKeyFile("/home/uthng/.ssh/ssh_servers"),
			ssh.PublicKeys(signer),
		},
		//HostKeyCallback: ssh.FixedHostKey(hostKey),
		//HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		//HostKeyCallback: nil,
	}

	if checkHostKey {
		c.ClientConfig.HostKeyCallback = ssh.FixedHostKey(hostKey)
	} else {
		c.ClientConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	return c, nil
}

// NewClientConfigWithSignedPubKeyFile returns a configuration
// corresponding to the configuration using private key along with
// a signed public key.
func NewClientConfigWithSignedPubKeyFile(username, sshKey, signedPubKey, host string, port int, checkHostKey bool) (*Config, error) {
	var hostKey ssh.PublicKey

	c := &Config{
		Host: host,
		Port: port,
	}

	// Read private key
	key, err := ioutil.ReadFile(sshKey)
	if err != nil {
		return nil, err
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	// Load the certificate
	cert, err := ioutil.ReadFile(signedPubKey)
	if err != nil {
		return nil, err
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	if err != nil {
		return nil, err
	}

	certSigner, err := ssh.NewCertSigner(pubKey.(*ssh.Certificate), signer)
	if err != nil {
		return nil, err
	}

	if checkHostKey {
		//arr := strings.Split(host, ":")
		hostKey, err = getHostKey(host, strconv.Itoa(port))
		if err != nil {
			return nil, err
		}
	}

	c.ClientConfig = &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			//ssh.Password("chrYsal1s-adm1n"),
			//ssh.PublicKeyFile("/home/uthng/.ssh/ssh_servers"),
			ssh.PublicKeys(certSigner),
		},
		//HostKeyCallback: ssh.FixedHostKey(hostKey),
		//HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		//HostKeyCallback: nil,
	}

	if checkHostKey {
		c.ClientConfig.HostKeyCallback = ssh.FixedHostKey(hostKey)
	} else {
		c.ClientConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	return c, nil
}

// NewClientConfigWithUserPass returns a configuration
// with given parameters
func NewClientConfigWithUserPass(username string, password string, host string, port int, checkHostKey bool) (*Config, error) {
	var hostKey ssh.PublicKey
	var err error

	c := &Config{
		Host: host,
		Port: port,
	}

	if checkHostKey {
		//arr := strings.Split(host, ":")
		hostKey, err = getHostKey(host, strconv.Itoa(port))
		if err != nil {
			return nil, err
		}
	}

	c.ClientConfig = &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
	}

	if checkHostKey {
		c.ClientConfig.HostKeyCallback = ssh.FixedHostKey(hostKey)
	} else {
		c.ClientConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	return c, nil
}

/////////// PRIVATE FUNCTIONS ////////////////////////////
func getHostKey(host, port string) (ssh.PublicKey, error) {
	// $HOME/.ssh/known_hosts
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hostport string
	if port == "22" {
		// standard port assumes 22
		// 192.168.10.53 ssh-rsa AAAAB3Nza...vguvx+81N1xaw==
		hostport = host
	} else {
		// non-standard port(s)
		// [ssh.example.com]:1999,[93.184.216.34]:1999 ssh-rsa AAAAB3Nza...vguvx+81N1xaw==
		hostport = "[" + host + "]:" + port
	}

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		if strings.Contains(fields[0], hostport) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				return nil, err
			}
			break // scanning line by line, first occurrence will be returned
		}
	}

	if hostKey == nil {
		return nil, fmt.Errorf("No hostkey for %s", host+":"+port)
	}

	return hostKey, nil
}
