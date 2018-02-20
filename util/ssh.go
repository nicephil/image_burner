package oakUtility

import (
    "time"
    "fmt"
    "net"
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
func New_SSHClient (host string) SSHClient {
    return SSHClient {
        IPv4: host,
        Port: "22",     // default port to 22
    }
}

type Oakridge_Device struct {
    Mac             string
    HWvendor        string
    HWmodel         string
    IPv4            string
    Firmware        string  // this is bootloader version
}
func Oakdev_PrintHeader () {
    fmt.Printf ("%-4s%-16s%-8s%-18s%-16s%s\n", "No.", "HWvendor", "Model", "Mac", "IPv4", "Firmware")
    fmt.Printf ("%s\n", strings.Repeat("=",96))
}
func (d *Oakridge_Device) OneLineSummary () string {
    return fmt.Sprintf ("%-16s%-8s%-18s%-16s%s", d.HWvendor, d.HWmodel, d.Mac, d.IPv4, d.Firmware)
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

    buf,err := s.Output (cmd)
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
func (c *SSHClient) Is_oakridge_dev () (*Oakridge_Device) {

    if err := c.Open("root", "oakridge"); err != nil {
        return nil
    }
    defer c.Close()

    buf, err := c.One_cmd ("uci show okcfg")
    if err != nil {
        return nil
    }

    var dev Oakridge_Device

    // mac-addr
    buf, err = c.One_cmd ("uci get productinfo.productinfo.mac")
    if err != nil {
        return nil
    }
    dev.Mac = strings.TrimSpace(string(buf))

    // HW vendor
    buf, err = c.One_cmd ("uci get productinfo.productinfo.model")
    if err != nil {
        return nil
    }
    dev.HWvendor = strings.Split(strings.TrimSpace(string(buf)), "_")[0]

    buf, err = c.One_cmd ("uci get productinfo.productinfo.production")
    if err != nil {
        return nil
    }
    dev.HWmodel = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.bootversion")
    if err != nil {
        return nil
    }
    dev.Firmware = strings.TrimSpace(string(buf))
    dev.IPv4 = c.IPv4
    return &dev
}
