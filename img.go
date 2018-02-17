package main

import (
        "time"
        "sync"
        "net"
	"log"
        "io/ioutil"
	"os"
	"bytes"
	"golang.org/x/crypto/ssh"
)

/*
 * global vars
*/
var wg sync.WaitGroup       // this to wait all worker before exit
var (
    log_dbg     *log.Logger
    log_info    *log.Logger
    log_wrn     *log.Logger
    log_err     *log.Logger
)


func main() {

    init_log ()

    // get all subnet
    nets, selfs, err := get_local_subnets()
    if err != nil {
        log_err.Fatalln(err)
    }

    // loop each subnet
    for _, net := range nets {
        log_info.Println ("Scanning subnet", net)
        hosts,err := net2hosts_exclude (net, selfs)
        if err != nil {
            log_err.Println(err)
            continue
        }
        num := len(hosts)
        log_info.Println ("Total ", num, "possible hosts")
        if num == 0 {                                    // just be cautious, should not happen
            log_info.Println (net, "has 0 host count?", hosts)
            continue
        }
        // do all hosts in a subnet in a parallel
        wg.Add(num)
        for _, h := range hosts {
            go worker (h)
        }
        wg.Wait()
        log_info.Println ("Done subnet", net)
    }
}

func init_log() {
    log_dbg = log.New(ioutil.Discard, "DEBUG: ",    log.Ldate|log.Ltime|log.Lshortfile)
    log_info = log.New(os.Stdout, "INFO: ",         log.Ldate|log.Ltime|log.Lshortfile)
    log_wrn = log.New(os.Stdout, "WARNING: ",       log.Ldate|log.Ltime|log.Lshortfile)
    log_err = log.New(os.Stderr, "ERROR: ",         log.Ldate|log.Ltime|log.Lshortfile)
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
func net2hosts_exclude (cidr string, ex []net.IP) ([]string, error) {
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
            log_info.Println ("Skip self ip", ip)
            continue
        }
        ips = append(ips, ip.String())
    }
    // remove network address and broadcast address
    return ips[1 : len(ips)-1], nil
}

func get_local_subnets() ([]string, []net.IP, error) {
    var subnets []string
    var selfs []net.IP
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        log_err.Println(err)
        return nil,nil,err
    }
    for _, a := range addrs {
        ip, net, err := net.ParseCIDR(a.String())
        if err != nil {
            log_err.Println(err)
            continue
        }
        if ip.To4() == nil {
            log_dbg.Println("ignore addr not IPv4", a)
            continue
        }
        if ip.IsLoopback() {
            log_dbg.Println("ignore Loopback addr", a)
            continue
        }
        log_dbg.Println("IPv4 Network", net)
        subnets = append(subnets, net.String())
        selfs = append(selfs, ip)
    }
    return subnets, selfs, err
}


func worker (host string) {
    log_dbg.Println ("connecting ", host)
    worker1 (host)
    log_dbg.Println("finished", host)
    defer wg.Done()
}

func worker1 (host string) {
    var dst bytes.Buffer
    sshConfig := &ssh.ClientConfig{
        User: "root",
        Auth: []ssh.AuthMethod{ssh.Password("oakridge")},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout: time.Second*5,
    }

    dst.WriteString(host)
    dst.WriteString(":22")
    client, err := ssh.Dial("tcp", dst.String(), sshConfig)

    if err != nil {
        log_dbg.Println (err)
        return
    }
    defer client.Close()

    session, err := client.NewSession()
    if err != nil {
        log_err.Println (err)
        return
    }

    out, err := session.CombinedOutput("cat /etc/issue")
    if err != nil {
        log_err.Println (err)
        return
    }
    log_info.Println(string(out))
}
