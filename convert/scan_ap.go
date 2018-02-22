package main

import (
        "sync"
        "net"
        "fmt"
        "runtime"
        "image_burner/util"
        "image_burner/spinner"
)

type Subnet struct {
    Net             string
    holes           []net.IP                        // skip those ip-addr
    Oak_dev_list    []*oakUtility.Oakridge_Device
    UBNT_ap_list    []*oakUtility.UBNT_AP
    batch           sync.WaitGroup                   // this to wait all host finish before exit
}

func New_Subnet (cidr string) Subnet {
    return Subnet { Net: cidr }
}
func (s *Subnet) Holes (h []net.IP) {
    s.holes = h
}

func (s *Subnet) Scan () {
    hosts,err := oakUtility.Net2hosts_exclude (s.Net, s.holes)
    if err != nil {
        fmt.Println(s.Net,err)
        return
    }

    // spinner
    if runtime.GOOS != "windows" {          // spinner not working for windows
        p := spinner.StartNew(s.Net)
        defer func () {
            p.Stop ()
        } ()
    }

    // do all hosts in one subnet in a parallel
    for _, h := range hosts {
        s.batch.Add(1)
        go s.scan_one (h)
    }
    s.batch.Wait()
}
func (s *Subnet) scan_one (host string) {

    defer s.batch.Done()

    c := oakUtility.New_SSHClient (host)

    if dev := c.Is_oakridge_dev(); dev != nil {
            s.Oak_dev_list = append(s.Oak_dev_list, dev)
    } else if dev := c.Is_ubnt_ap(); dev != nil {
            s.UBNT_ap_list = append(s.UBNT_ap_list, dev)
    }
}

func (s *Subnet) OneLineSummary () {
    fmt.Printf("✓ %s: Completed, %d Oakridge, %d UBNT devices\n",s.Net,len(s.Oak_dev_list),len(s.UBNT_ap_list))
}
func scan_local_subnet () {
    nets, selfs, err := oakUtility.Get_local_subnets()
    if err != nil {
        log.Error.Fatalln(err)
    }

    println("Scanning local networks ...\n")

    // scan each subnet
    for _, net := range nets {
        n := New_Subnet (net)
        n.Holes (selfs)
        n.Scan ()
        n.OneLineSummary()
        netlist =  append(netlist, n)
    }
}
