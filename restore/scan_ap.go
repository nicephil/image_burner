package main

import (
        "time"
        "sync"
        "net"
	"bytes"
        "fmt"
        "os"
        "strings"
        "strconv"
	"golang.org/x/crypto/ssh"
        "gopkg.in/cheggaaa/pb.v1"
        "image_burner/util"
)

/*
 * global vars
*/
var scan_sync sync.WaitGroup       // this to wait all worker before exit

type Device struct {
    ipv4        string
    model       string
    hostname    string
    os          string
}

var dev_list []Device

func dump_dev_list () {
    var choice string
    fmt.Println ("Found", len(dev_list), "devices")
    if len(dev_list) == 0 {
        os.Exit(0)
    }
    for {
        fmt.Printf("\nPlease choose devices to restore image(Q to quit):\n")
        fmt.Printf("[0] All devices\n")
        for i, d := range dev_list {
            fmt.Printf("[%d] %v\n", i+1, d)
        }
        fmt.Printf("Your choice:")
        fmt.Scanf("%s\n", &choice)
        log_dbg.Println ("user input:", choice)
        if strings.ToUpper(choice) == "Q" {
            fmt.Printf("Quit\n")
            os.Exit(0)
        }
        c,err := strconv.Atoi(choice)
        if err != nil {
            fmt.Printf ("\n[%s] is invalid\n", choice)
            continue
        }
        if c < 0 || c > len(dev_list) {
            fmt.Printf ("\n[%s] is out-of-range\n", choice)
            continue
        }
        fmt.Printf ("You choose: [%d]\n", c)
        break
    }
}

func progress_bar (total int, p chan string, q chan int) {
    bar := pb.StartNew (total)
    bar.ShowCounters = false
    bar.ShowTimeLeft = false
    bar.ShowSpeed = false
    for w := range p {
        log_dbg.Println("progress:", w)
        bar.Increment()
    }
    bar.FinishPrint("Scan done")
    close (q)
}
func scan_local_subnet () {

    // get all subnet
    nets, selfs, err := get_local_subnets()
    if err != nil {
        log_err.Fatalln(err)
    }

    // scan each subnet in batch mode
    for _, net := range nets {
        fmt.Println ("Scanning subnet", net)
        hosts,err := net2hosts_exclude (net, selfs)
        if err != nil {
            log_err.Println(err)
            continue
        }
        num := len(hosts)
        log_dbg.Println ("Trying ", num, "possible hosts")
        // just be cautious, should not happen
        if num == 0 {
            log_info.Println (net, "has 0 host count?", hosts)
            continue
        }

        p := make(chan string, num)
        q := make(chan int)
        go progress_bar (num, p, q)

        // do all hosts in one subnet in a parallel
        scan_sync.Add(num)
        for _, h := range hosts {
            go scan_one_ap (h, p)
        }
        scan_sync.Wait()
        close (p)
        dummy :=  <-q
        log_dbg.Println ("Done subnet %d", net, dummy)
    }
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
            log_dbg.Println ("Skip self ip", ip)
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


func scan_one_ap (host string, progress chan string) {
    var dst bytes.Buffer
    var dev Device

    defer scan_sync.Done()
    defer func () {
        progress <- dst.String()
    }()

    sshConfig := &ssh.ClientConfig{
        User: "root",
        Auth: []ssh.AuthMethod{ssh.Password("oakridge")},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout: time.Second*3,
    }

    dst.WriteString(host)
    dst.WriteString(":22")

    client, err := ssh.Dial("tcp", dst.String(), sshConfig)
    if err != nil {
        log_dbg.Println (err)
        return
    }
    defer client.Close()

    buf, err := oakUtility.One_cmd (client, "uci get productinfo.productinfo.model")
    if err != nil {
        log_dbg.Println (host, "model", err)
        return
    }
    dev.model = strings.TrimSpace(string(buf))

    buf, err = oakUtility.One_cmd (client, "uci get system.@system[0].hostname")
    if err != nil {
        log_dbg.Println (host, "hostname", err)
        return
    }
    dev.hostname= strings.TrimSpace(string(buf))

    dev.ipv4 = host
    dev_list = append(dev_list, dev)
}
