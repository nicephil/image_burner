package oakUtility

import (
    "time"
    "fmt"
    "golang.org/x/crypto/ssh"
)


type SSHClient struct {
    IPv4        string
    Port        string
    User        string
    Pass        string
    client      *ssh.Client
}

func (c *SSHClient) One_cmd (cmd string) ([]byte, error) {

    if c.client == nil {
        return nil, fmt.Errorf("%s@%s:%s NOT connected", c.User, c.IPv4, c.Port)
    }
    s, err := c.client.NewSession()
    if err != nil {
        return nil, err
    }
    defer s.Close()

    buf,err := s.Output (cmd)
    if err != nil {
        return nil, err
    }
    return buf, nil
}

func (c *SSHClient) Open () error {
    sshConfig := &ssh.ClientConfig {
        User: c.User,
        Auth: []ssh.AuthMethod{ssh.Password(c.Pass)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout: time.Second*3,
    }

    sc, e := ssh.Dial("tcp", c.IPv4+":"+c.Port, sshConfig)
    if e == nil {
        c.client = sc
    }
    return e
}
func (c *SSHClient) Close () {
    if c.client != nil {
        c.client.Close ()
        c.client = nil
    }
}

func New_SSHClient (host string, port string, username string, passwd string) SSHClient {
    return SSHClient {
        IPv4: host,
        Port: port,
        User: username,
        Pass: passwd,
    }
}

