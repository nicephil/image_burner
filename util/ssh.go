package oakUtility

import (
    "time"
    "fmt"
    "strings"
    "golang.org/x/crypto/ssh"
)


type SSHClient struct {
    IPv4        string
    Port        string
    User        string
    Pass        string
    client      *ssh.Client
}
func New_SSHClient (host string, port string, username string, passwd string) SSHClient {
    return SSHClient {
        IPv4: host,
        Port: port,
        User: username,
        Pass: passwd,
    }
}

type Oakridge_AP struct {
    os              string
    ipv4            string
    model           string
    mac             string
    sw_version      string
    boot_version    string
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
func (c *SSHClient) Is_oakridge_AP () (*Oakridge_AP) {

    if err := c.Open(); err != nil {
        return nil
    }
    defer c.Close()

    buf, err := c.One_cmd ("uci show okcfg")
    if err != nil {
        return nil
    }

    var dev Oakridge_AP

    buf, err = c.One_cmd ("uci get productinfo.productinfo.mac")
    if err != nil {
        return nil
    }
    dev.mac = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.model")
    if err != nil {
        return nil
    }
    dev.model = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.bootversion")
    if err != nil {
        return nil
    }
    dev.boot_version = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.swversion")
    if err != nil {
        return nil
    }
    dev.sw_version = strings.TrimSpace(string(buf))
    dev.ipv4 = c.IPv4
    dev.os = "Oakridge"
    return &dev
}
func (d *Oakridge_AP) CSV() (string) {
    return d.os+", "+d.mac+", "+d.ipv4+", "+d.model+", "+d.sw_version+", "+d.boot_version
}
