package oakUtility

import (
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
