package oakUtility

import (
    "time"
    "fmt"
    "net"
    "os"
    "io"
    "path"
    "golang.org/x/crypto/ssh"
)

const (
    AC_LITE = "AC-LITE"
    AC_LR   = "AC-LR"
    AC_PRO  = "AC-PRO"
)

type SSHClient struct {
    IPv4        string
    Port        string
    User        string
    Pass        string
    client      *ssh.Client
}
func New_SSHClient (host string) SSHClient {
    return SSHClient {
        IPv4: host,
        Port: "22",     // default port to 22
    }
}

func Get_local_subnets() ([]string, []net.IP, error) {
    var subnets []string
    var selfs []net.IP
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        return nil,nil,err
    }
    for _, a := range addrs {
        ip, net, err := net.ParseCIDR(a.String())
        if err != nil {
            continue
        }
        if ip.To4() == nil {
            continue
        }
        if ip.IsLoopback() {
            continue
        }
        subnets = append(subnets, net.String())
        selfs = append(selfs, ip)
    }
    return subnets, selfs, err
}
//  http://play.golang.org/p/m8TNTtygK0
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
/*
 * give a network string <a.b.c.d/x>, return a string array of all host except thos in <ex> array
 */
func Net2hosts_exclude (cidr string, ex []net.IP) ([]string, error) {
    var ips []string

    ip, ipnet, err := net.ParseCIDR(cidr)
    if err != nil {
        return nil, err
    }

    for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
        skip := false
        for _, e := range ex {
            if e.Equal(ip) {
                skip = true
                break;
            }
        }
        if skip == true {
            continue
        }
        ips = append(ips, ip.String())
    }
    // remove network address and broadcast address
    return ips[1 : len(ips)-1], nil
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

    buf,err := s.CombinedOutput (cmd)
    if err != nil {
        return nil, err
    }
    return buf, nil
}

func (c *SSHClient) Open (user string, pass string) error {
    c.User = user
    c.Pass = pass
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
func (c *SSHClient) Scp (local string, remote string, permission string) (int64,error) {
    f,err := os.Open(local)
    if err != nil {
        return 0,err
    }
    defer f.Close()

    stat,err := f.Stat()
    if err != nil {
        return 0,err
    }

    s, err := c.client.NewSession()
    if err != nil {
        return 0, err
    }
    defer s.Close()

    filename := path.Base(remote)
    directory := path.Dir(remote)

    go func () {
        w, err := s.StdinPipe()
        if err != nil {
            println (err)
            return
        }
        defer w.Close()

        fmt.Fprintln(w, "C"+permission, stat.Size(), filename)
        io.Copy(w, f)
        fmt.Fprintln(w, "\x00")
    } ()

    s.Run("/usr/bin/scp -t " + directory)

    return stat.Size(), nil
}
