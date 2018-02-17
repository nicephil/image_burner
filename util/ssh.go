package oakUtility

import (
        "time"
	"golang.org/x/crypto/ssh"
)


func One_cmd (c *ssh.Client, cmd string) ([]byte, error) {

    s, err := c.NewSession()
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

func Connect (host, port, user, pass string) (*ssh.Client, error) {
    sshConfig := &ssh.ClientConfig{
        User: user,
        Auth: []ssh.AuthMethod{ssh.Password(pass)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout: time.Second*3,
    }

    c, e := ssh.Dial("tcp", host+":"+port, sshConfig)
    if e != nil {
        return nil, e
    }
    return c, nil
}
