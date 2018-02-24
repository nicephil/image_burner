package main

import (
    "fmt"
    "os"
    "net"
    "strings"
    "sync"
    "image_burner/util"
    "image_burner/spinner"
)

/*
 * global vars
 */
var netlist []Subnet
const Banner_start = "\nOakridge Firmware Update Utility, Ver 1.01, (c) Oakridge Networks, Inc. 2018\n"
const Banner_end = "\nThanks for choose Oakridge Networks Inc.\n"
var log oakUtility.OakLogger

// NOTE:  ac-lite/ac-lr/ac-pro share the same img, for handy program, just list them all
const AC_LITE = oakUtility.AC_LITE
const AC_LR   = oakUtility.AC_LR
const AC_PRO  = oakUtility.AC_PRO
var local_imgfile = map[string]string {
    AC_LITE: "",
    AC_LR:   "",
    AC_PRO:  "",
}
var img = map[string][]string {
    AC_LITE: {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
    AC_LR:   {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
    AC_PRO:  {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
}



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
    p := spinner.StartNew(s.Net)
    defer func () {
        p.Stop ()
    } ()

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

func list_scan_result () {
    cnt := 0

    oakUtility.Oakdev_PrintHeader ()
    for _,n:=range netlist {
        for _,o :=range n.Oak_dev_list {
            cnt++
            fmt.Printf("%-3d %s\n", cnt, o.OneLineSummary())
        }
        for _,u :=range n.UBNT_ap_list {
            cnt++
            fmt.Printf("%-3d %s\n", cnt, u.OneLineSummary())
        }
    }
}

type Target struct {
    host        string
    mac         string
    user        string
    pass        string
    HWmodel     string
    result      string
}

func on_demand_download (t *Target) error {
    if local_imgfile[t.HWmodel] == "" {
        local := img[t.HWmodel][0]
        url := img[t.HWmodel][1]
        if _, err := os.Stat(local); os.IsNotExist(err) {
            if err := oakUtility.DownloadFile (local, url, true, "Downloading "+t.HWmodel+" img ... "); err != nil {
                println ("Download fail:", err)
                return err
            }
            fmt.Println("File download done")
        } else {
            fmt.Printf("%s exist, skip download(TODO:check file checksum)\n", local)
        }
        local_imgfile[t.HWmodel] = local
    }
    return nil
}
func Install_img (t Target, s *sync.WaitGroup) {
    defer s.Done()

    p := spinner.StartNew("Install "+t.host+" ...")

    c := oakUtility.New_SSHClient (t.host)
    if err := c.Open(t.user, t.pass); err != nil {
        println (err)
        return
    }
    defer c.Close()

    remotefile := "oakridge.tar.gz"
    _,err := c.Scp (local_imgfile[t.HWmodel], remotefile, "0644")
    if err != nil {
        println (err)
        return
    }

    fmt.Printf ("Upgrade %s image, MUST NOT POWER OFF DEVICE ...\n", t.host)

    var cmds = []string {
    "tar xzf "+remotefile,
    "rm -rvf "+remotefile,
    "dd if=openwrt-ar71xx-generic-ubnt-unifi-squashfs-sysupgrade.bin of=kernel0.bin bs=7929856 count=1",
    "dd if=openwrt-ar71xx-generic-ubnt-unifi-squashfs-sysupgrade.bin of=kernel1.bin bs=7929856 count=1 skip=1",
    "mtd write kernel0.bin kernel0",
    "mtd write kernel1.bin kernel1",
    "reboot",
    }
    for _, cmd := range cmds {
        _, err := c.One_cmd (cmd)
        if err != nil {
            println (err)
            return
        }
    }
    p.Stop ()
    fmt.Printf ("%s Image upgraded, rebooting ...\n", t.host)
}
func install_oak_firmwire () {
    var choice string
    var targets []Target

    // prepare the list
    for _,n:=range netlist {
        for _,u :=range n.UBNT_ap_list {
            switch u.HWmodel {
            case AC_LITE, AC_LR, AC_PRO:
                t := Target { host: u.IPv4, mac: u.Mac, user: "ubnt", pass: "ubnt", HWmodel: u.HWmodel}
                if err := on_demand_download (&t); err != nil {
                    continue
                }
                targets = append(targets, t)
            }
        }
    }

    if len(targets) == 0 {
        return
    }

    println("\nInstall Oakridge firmware to all 3rd party HW?(Y/N):")
    fmt.Scanf("%s\n", &choice)
    fmt.Printf("\rYou choose: %s\n", choice)
    oakUtility.ClearLine ()
    if strings.Compare(strings.ToUpper(choice), "Y") != 0 {
        return
    }

    // burn img in parallel
    var s sync.WaitGroup
    for _, t:= range targets {
        s.Add (1)
        go Install_img(t, &s)
    }
    s.Wait()
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

func init () {
    log = oakUtility.New_OakLogger()
    log.Set_level ("info")
}

func main() {

    println(Banner_start)

    scan_local_subnet ()

    list_scan_result ()

    install_oak_firmwire ()

    //list_oakdev_csv ()

    println(Banner_end)
}
