GOSSH
----

A small Go utility package to handle easier SSH operations such as different kind of connection, command execution, SCP etc.

Currently, it supports:
- Connection with user & password
- Connection with SSH key pair
- Connection with signed SSH certificate
- SCP content, files or directories from local to remote hosts

### Usage

#### Connect with a user & password

```
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

```
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

```
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

```
  res, err := client.ExecCommand(cmd)
```

#### Transfer to remote machine

##### Content

```
  err = client.SCPBytes([]byte(`SCP single file transfer test`), "/tmp/scp_single_file", "0777")
```

##### File

```
  err = client.SCPFile("./data/scp_single_file", "/tmp/scp_single_file", "0777")
```

##### Folder and its subfolders

```
  err = client.SCPDir("./data", "/tmp/scp", "0777")
```

### Documentation

Cf. [Godoc](https://godoc.org/github.com/uthng/gossh) or test files in this repository.
