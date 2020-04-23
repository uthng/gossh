GOSSH
----

A small Go utility package to handle easier SSH operations such as different kinds of SSH connections (user/pass, private key or signed certificate), command execution, file transfer with SCP protocol.

Currently, it supports:
- Connection with user & password
- Connection with SSH key pair
- Connection with signed SSH certificate
- SCP content, files or directories recursively from local to remote hosts
- SCP files or directories recursively from remote hosts to local

### Usage

#### Connect with a user & password

```golang
  config, err := NewClientConfigWithUserPass("user", "pass", "myremotemachine.com", 22, false)
  if err != nil {
    return err
  }

  client, err := NewClient(config)
  if err != nil {
    return err
  }
```

#### Connect with a SSH key pair

```golang
  config, err := NewClientConfigWithUserPass("user", "/home/user/.ssh/id_rsa", "myremotemachine.com", 22, false)
  if err != nil {
    return err
  }

  client, err := NewClient(config)
  if err != nil {
    return err
  }
```

#### Connect with signed SSH certificate

```golang
  config, err := NewClientConfigWithUserPass("user", "/home/user/.ssh/id_rsa", "/home/user/.ssh/id_rsa-cert.pub", "myremotemachine.com", 22, false)
  if err != nil {
    return err
  }

  client, err := NewClient(config)
  if err != nil {
    return err
  }
```

#### Execute a command

```golang
  res, err := client.ExecCommand("ls -la")
```

#### Transfer to remote machine

##### Content

```golang
  err = client.SCPSendBytes([]byte(`SCP single file transfer test`), "/tmp/scp_single_file", "0777")
```

##### File

```golang
  err = client.SCPSendFile("./data/scp_single_file", "/tmp/scp_single_file", "0777")
```

##### Folder and its subfolders

```golang
  err = client.SCPSendDir("./data", "/tmp/scp", "0777")
```

#### Transfer from remote machine

##### File

```golang
  err = client.SCPGetFile("/tmp/data/scp_single_file", "/tmp/remote/scp_single_file")
```

##### Folder and its subfolders

```golang
  err = client.SCPGetDir("/tmp/data", "/tmp/remote")
```

#### Enable logging

By default, log is disabled but it can be enabled to debug easily using either function or environment variables:

- Set log level: `client.SetVerbosity(5)` or `GOSSH_VERBOSITY=5`
- Disable log colors: `client.DisableLogVerbosity()` or GOSSH_DISABLE_COLOR=1`

`
### Documentation

#### Gossh
Cf. [Godoc](https://godoc.org/github.com/uthng/gossh) or test files in this repository.

#### SCP protocol

Cf.[SCP protocol](https://web.archive.org/web/20170215184048/https://blogs.oracle.com/janp/entry/how_the_scp_protocol_works) or its copy [here](scp_protocol.md)
