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
const Banner_start = "\nFirmware Restore Utility, Ver 1.01, (c) Oakridge Networks, Inc. 2018\n"
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


type Oakridge_Device struct {
    Mac             string
    Model           string
    IPv4            string
    Firmware        string
}
func (d *Oakridge_Device) OneLineSummary () string {
    return fmt.Sprintf ("%-16s%-8s%-18s%-16s%s", "Oakridge", d.Model, d.Mac, d.IPv4, d.Firmware)
}
func Oakdev_PrintHeader () {
    fmt.Printf ("\n%-4s%-16s%-8s%-18s%-16s%s\n", "No.", "SW", "HW", "Mac", "IPv4", "Firmware")
    fmt.Printf ("%s\n", strings.Repeat("=",96))
}

type Subnet struct {
    Net             string
    holes           []net.IP                        // skip those ip-addr
    Oak_dev_list    []*Oakridge_Device
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

    if dev := Is_oakridge_dev(c); dev != nil {
            s.Oak_dev_list = append(s.Oak_dev_list, dev)
    }
}

func (s *Subnet) OneLineSummary () {
    fmt.Printf("✓ %s: Completed, %d Oakridge devices\n",s.Net,len(s.Oak_dev_list))
}

func Is_oakridge_dev (c oakUtility.SSHClient) (*Oakridge_Device) {

    if err := c.Open("root", "oakridge"); err != nil {
        return nil
    }
    defer c.Close()

    var dev Oakridge_Device

    // mac-addr
    buf, err := c.One_cmd ("uci get productinfo.productinfo.mac")
    if err != nil {
        return nil
    }
    dev.Mac = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.production")
    if err != nil {
        return nil
    }
    dev.Model = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.swversion")
    if err != nil {
        return nil
    }
    dev.Firmware = strings.TrimSpace(string(buf))
    dev.IPv4 = c.IPv4
    return &dev
}


func list_scan_result () {
    cnt := 0

    Oakdev_PrintHeader ()
    for _,n:=range netlist {
        for _,o :=range n.Oak_dev_list {
            cnt++
            fmt.Printf("%-3d %s\n", cnt, o.OneLineSummary())
        }
    }
}

type Target struct {
    host        string
    mac         string
    user        string
    pass        string
    Model     string
    result      string
}

func on_demand_download (t *Target) error {
    if local_imgfile[t.Model] == "" {
        local := img[t.Model][0]
        url := img[t.Model][1]
        if _, err := os.Stat(local); os.IsNotExist(err) {
            if err := oakUtility.DownloadFile (local, url, true, "Downloading "+t.Model+" img ... "); err != nil {
                println ("Download fail:", err)
                return err
            }
            fmt.Println("File download done")
        } else {
            fmt.Printf("%s exist, skip download(TODO:check file checksum)\n", local)
        }
        local_imgfile[t.Model] = local
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
    _,err := c.Scp (local_imgfile[t.Model], remotefile, "0644")
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

    //list_oakdev_csv ()

    println(Banner_end)
}
