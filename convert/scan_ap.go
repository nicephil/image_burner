package main

import (
        "sync"
        "net"
        "fmt"
        "os"
        "time"
        "image_burner/util"
        "github.com/briandowns/spinner"
)

type Subnet struct {
    Net             string
    holes           []net.IP                        // skip those ip-addr
    Oak_dev_list    []*oakUtility.Oakridge_Device
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
    p := spinner.New(spinner.CharSets[10], 200*time.Millisecond)
    p.Prefix = fmt.Sprintf("%s ", s.Net)
    //p.Suffix = "  This is suffix"
    p.Writer = os.Stderr
    p.Color("red")
    p.Start()
    defer p.Stop ()

    // do all hosts in one subnet in a parallel
    for _, h := range hosts {
        go s.scan_one (h)
    }
    s.batch.Wait()
}
func (s *Subnet) scan_one (host string) {

    s.batch.Add(1)
    defer s.batch.Done()

    c := oakUtility.New_SSHClient (host)

    if dev := c.Is_oakridge_dev(); dev != nil {
            s.Oak_dev_list = append(s.Oak_dev_list, dev)
    }
}

func (s *Subnet) OneLineSummary () {
    fmt.Println (s.Net, "... found", len(s.Oak_dev_list), "Oakridge device")
}
func scan_local_subnet () {
    nets, selfs, err := oakUtility.Get_local_subnets()
    if err != nil {
        log.Error.Fatalln(err)
    }

    println("Scanning local networks ...")

    // scan each subnet
    for _, net := range nets {
        n := New_Subnet (net)
        n.Holes (selfs)
        n.Scan ()
        n.OneLineSummary()
        netlist =  append(netlist, n)
    }
}
