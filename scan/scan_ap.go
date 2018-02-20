package main

import (
        "sync"
        "net"
        "fmt"
        "gopkg.in/cheggaaa/pb.v1"
        "image_burner/util"
)

/*
 * global vars
*/
var scan_sync sync.WaitGroup       // this to wait all worker before exit

var oakap_list []*oakUtility.Oakridge_AP

func dump_dev_list () {
    var choice string
    fmt.Println ("Found", len(oakap_list), "Oakridge devices")
    if len(oakap_list) > 0 {
        for _, d := range oakap_list {
            fmt.Printf("%s\n", d.CSV())
        }
        fmt.Printf ("press ENTER to quit\n")
        fmt.Scanf("%s\n", &choice)
    }
}
func progress_bar (total int, p chan string, q chan int) {
    bar := pb.StartNew (total)
    bar.ShowCounters = false
    bar.ShowTimeLeft = false
    bar.ShowSpeed = false
    for w := range p {
        log.Debug.Println("progress:", w)
        bar.Increment()
    }
    bar.FinishPrint("")
    close (q)
}
func scan_local_subnet () {

    // get all subnet
    nets, selfs, err := get_local_subnets()
    if err != nil {
        log.Error.Fatalln(err)
    }

    // scan each subnet in batch mode
    for _, net := range nets {
        fmt.Println ("Scanning subnet", net)
        hosts,err := net2hosts_exclude (net, selfs)
        if err != nil {
            log.Error.Println(err)
            continue
        }
        num := len(hosts)
        log.Debug.Println ("Trying ", num, "possible hosts")
        // just be cautious, should not happen
        if num == 0 {
            log.Info.Println (net, "has 0 host count?", hosts)
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
        log.Debug.Println ("Done subnet %d", net, dummy)
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
            log.Debug.Println ("Skip self ip", ip)
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
        log.Error.Println(err)
        return nil,nil,err
    }
    for _, a := range addrs {
        ip, net, err := net.ParseCIDR(a.String())
        if err != nil {
            log.Error.Println(err)
            continue
        }
        if ip.To4() == nil {
            log.Debug.Println("ignore addr not IPv4", a)
            continue
        }
        if ip.IsLoopback() {
            log.Debug.Println("ignore Loopback addr", a)
            continue
        }
        log.Debug.Println("IPv4 Network", net)
        subnets = append(subnets, net.String())
        selfs = append(selfs, ip)
    }
    return subnets, selfs, err
}


func scan_one_ap (host string, progress chan string) {

    defer scan_sync.Done()

    c := oakUtility.New_SSHClient (host, "22", "root", "oakridge")
    defer func () {
        progress <- c.User+"@"+c.IPv4+":"+c.Port
    }()
    if dev := c.Is_oakridge_AP(); dev != nil {
        oakap_list = append(oakap_list, dev)
    }
}
