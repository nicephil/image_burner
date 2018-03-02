package main

import (
    "fmt"
    "os"
    "net"
    "time"
    "strings"
    "bufio"
    "strconv"
    "sync"
    "image_burner/util"
    "image_burner/ping"
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
const UBNT_ERX = oakUtility.UBNT_ERX


type UBNT_AP struct {
    Mac             string
    IPv4            string
    HWmodel         string
    SWver           string
}
func (d *UBNT_AP) OneLineSummary () string {
    return fmt.Sprintf ("%-12s%-16s%-18s%-16s%s", "Ubiquiti", d.HWmodel, d.Mac, d.IPv4, d.SWver)
}
type Oakridge_Device struct {
    Mac             string
    HWmodel         string
    IPv4            string
    Firmware        string  // this is bootloader version
}
func (d *Oakridge_Device) OneLineSummary () string {
    return fmt.Sprintf ("%-12s%-16s%-18s%-16s%s", "Oakridge", d.HWmodel, d.Mac, d.IPv4, d.Firmware)
}
func Oakdev_PrintHeader () {
    fmt.Printf ("\n%-4s %-12s%-16s%-18s%-16s%s\n", "No.", "SW", "HW", "Mac", "IPv4", "Firmware")
    fmt.Printf ("%s\n", strings.Repeat("=",96))
}

type Subnet struct {
    Net             string
    holes           []net.IP                        // skip those ip-addr
    Oak_dev_list    []*Oakridge_Device
    UBNT_ap_list    []*UBNT_AP
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
    } else if dev := Is_ubnt_ap(c); dev != nil {
            s.UBNT_ap_list = append(s.UBNT_ap_list, dev)
    } else if dev := Is_ubnt_erx(c); dev != nil {
            s.UBNT_ap_list = append(s.UBNT_ap_list, dev)
    }
}

func (s *Subnet) OneLineSummary () {
    fmt.Printf("✓ %s: %d Oakridge, %d UBNT devices\n",s.Net,len(s.Oak_dev_list),len(s.UBNT_ap_list))
}
func Is_ubnt_erx (c oakUtility.SSHClient) (*UBNT_AP) {

    if err := c.Open("ubnt", "ubnt"); err != nil {
        return nil
    }
    defer c.Close()

    var dev UBNT_AP

    buf, err := c.One_cmd ("/opt/vyatta/bin/vyatta-op-cmd-wrapper show version")
    if err != nil {
        log.Debug.Printf ("%s %s: %s\n",c.IPv4, "show version", err.Error())
        return nil
    }
    // pass output string to get mac and hwmodel
    tvs := strings.Split (strings.TrimSpace(string(buf)), "\n")
    for _, t:=range tvs {
        kv := strings.Split (t, ":")
        k := strings.TrimSpace(kv[0])
        v := strings.TrimSpace(kv[1])
        log.Debug.Printf("%s %s:%s\n", c.IPv4, k, v)
        switch k {
        case "HW S/N":
            dev.Mac= v[:2]+":"+v[2:4]+":"+v[4:6]+":"+v[6:8]+":"+v[8:10]+":"+v[10:]
        case "HW model":
            dev.HWmodel=v
            if v == "EdgeRouter X 5-Port" {
                dev.HWmodel=UBNT_ERX
            }
        case "Version":
            dev.SWver= v
        }
    }
    if dev.HWmodel != UBNT_ERX {
        log.Debug.Printf ("unsupport erx hw %v\n",tvs)
        return nil
    }
    dev.IPv4 = c.IPv4
    return &dev
}

func Is_oakridge_dev (c oakUtility.SSHClient) (*Oakridge_Device) {

    if err := c.Open("root", "oakridge"); err != nil {
        log.Debug.Printf ("fail login as root to %s: %s\n",c.IPv4, err.Error())
        return nil
    }
    defer c.Close()

    var dev Oakridge_Device

    // mac-addr
    buf, err := c.One_cmd ("uci get productinfo.productinfo.mac")
    if err != nil {
        log.Debug.Printf ("%s: %s\n",c.IPv4, err.Error())
        return nil
    }
    dev.Mac = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.production")
    if err != nil {
        log.Debug.Printf ("%s: %s\n",c.IPv4, err.Error())
        return nil
    }
    dev.HWmodel = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.swversion")
    if err == nil {
        dev.Firmware = strings.TrimSpace(string(buf))
    }

    // only try get bootversion if can't get swversion
    if dev.Firmware == "" {
        buf, err = c.One_cmd ("uci get productinfo.productinfo.bootversion")
        if err != nil {
            log.Debug.Printf ("%s: %s\n",c.IPv4, err.Error())
            return nil
        }
    }
    dev.IPv4 = c.IPv4
    return &dev
}
func Is_ubnt_ap (c oakUtility.SSHClient) (*UBNT_AP) {

    if err := c.Open("ubnt", "ubnt"); err != nil {
        return nil
    }
    defer c.Close()

    var dev UBNT_AP

    buf, err := c.One_cmd ("cat /proc/ubnthal/system.info")
    if err != nil {
        log.Debug.Printf ("%s: %s\n",c.IPv4, err.Error())
        return nil
    }
    // pass output string to get mac and hwmodel
    tvs := strings.Split (strings.TrimSpace(string(buf)), "\n")
    for _, t:=range tvs {
        switch v := strings.Split (t, "="); v[0] {
        case "eth0.macaddr":
            dev.Mac= v[1]
        case "systemid":
            switch v[1] {
            case "e517":
                dev.HWmodel=AC_LITE
            case "e527":
                dev.HWmodel=AC_LR
            case "e537":
                dev.HWmodel=AC_PRO
            default:
                // only support model above
                log.Debug.Printf ("%s: %s not support\n",c.IPv4, v[1])
                return nil
            }
        }
    }

    // sw ver
    buf, err = c.One_cmd ("cat /etc/version")
    if err != nil {
        log.Debug.Printf ("%s: %s\n",c.IPv4, err.Error())
        return nil
    }
    dev.SWver= strings.TrimSpace(string(buf))
    dev.IPv4 = c.IPv4
    return &dev
}


func list_scan_result () {
    cnt := 0

    Oakdev_PrintHeader ()
    for _,n:=range netlist {
        for _,o :=range n.Oak_dev_list {
            cnt++
            fmt.Printf(" %-3d %s\n", cnt, o.OneLineSummary())
        }
        for _,u :=range n.UBNT_ap_list {
            cnt++
            fmt.Printf("✓%-3d %s\n", cnt, u.OneLineSummary())
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

var erx_imgs = map[string][]string {
    "factory":      {"erx_factory.bin.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubnterx/factory.bin.tar.gz"},
    "oakridge":     {"oakridge_sysupgrade.bin.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubnterx/sysloader/latest-sysupgrade.bin.tar.gz"},
}
func erx_factory_img (host string) error {

    p := spinner.StartNew("Install factory img ...")
    defer p.Stop ()

    c := oakUtility.New_SSHClient (host)
    if err := c.Open("ubnt", "ubnt"); err != nil {
        return err
    }
    defer c.Close()

    file := erx_imgs["factory"][0]
    if _,err := c.Scp (file, "/tmp/"+file, "0644"); err != nil {
        log.Error.Println(err.Error())
        return err
    }
    log.Debug.Printf ("done scp %s to %s:%s\n", file, host, "/tmp/"+file)

    if _, err := c.One_cmd ("tar xzf /tmp/"+file+" -C /tmp"); err != nil {
        log.Error.Println(err.Error())
        return err
    }
    log.Debug.Printf ("done untar %s:%s\n", host, "/tmp/"+file)

    if buf, err := c.One_cmd ("/opt/vyatta/bin/vyatta-op-cmd-wrapper add system image /tmp/lede-ramips-mt7621-ubnt-erx-initramfs-factory.tar"); err != nil {
        log.Error.Println(string(buf), err.Error())
        return err
    }

    if _, err := c.One_cmd ("/opt/vyatta/bin/vyatta-op-cmd-wrapper reboot now"); err != nil {
        log.Error.Println(err.Error())
        return err
    }
    return nil
}
func install_ubnt_erx_img (host string) {
    for _, img := range erx_imgs {
        if err := oakUtility.On_demand_download (img[0],img[1]); err != nil {
            log.Error.Println (err.Error())
            return
        }
    }
    if err := erx_factory_img (host); err != nil {
            return
    }

    p := spinner.StartNew("Wait device bootup ...")
    pinger, err := ping.NewPinger(host)
    if err != nil {
        panic(err)
    }
    pinger.SetStopAfter (5)
    pinger.OnRecv = func(pkt *ping.Packet) {
        fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v\n", pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
    }
    pinger.Run()
    p.Stop ()

    p = spinner.StartNew("Install Oakridge img ...")
    defer p.Stop ()
    c := oakUtility.New_SSHClient (host)                                    // ssh back to device again
    for {
        time.Sleep(1*time.Second)
        err := c.Open("root", "oakridge")
        if err == nil {
            log.Debug.Printf ("ssh connected to %s\n", host)
            break;
        }
        log.Debug.Println (err.Error())
    }
    defer c.Close()

    file := erx_imgs["oakridge"][0]
    if _,err := c.Scp (file, "/tmp/"+file, "0644"); err != nil {
        log.Error.Println(err.Error())
        return
    }
    if _, err := c.One_cmd ("tar xzf /tmp/"+file+" -C /tmp"); err != nil {
        log.Error.Printf ("%s\n", err.Error())
        return
    }
    c.One_cmd ("sysupgrade -n lede-ramips-mt7621-ubnt-erx-squashfs-sysupgrade.bin")
}
func install_one_device (t Target, s *sync.WaitGroup) {
    if s != nil {
        defer s.Done()
    }

    switch t.HWmodel {
    case AC_LITE, AC_LR, AC_PRO:
        install_unifi_ap_img (t)
    case UBNT_ERX:
        install_ubnt_erx_img (t.host)
    default:
        fmt.Printf ("unsupport model %s\n", t.HWmodel)
        return
    }
}
var unifi_ap_imgs = map[string][]string {
    AC_LITE: {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
    AC_LR:   {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
    AC_PRO:  {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
}
func install_unifi_ap_img (t Target) {

    localfile := unifi_ap_imgs[t.HWmodel][0]
    url := unifi_ap_imgs[t.HWmodel][1]

    if err := oakUtility.On_demand_download (localfile, url); err != nil {
        log.Error.Printf ("on-demand-download %s fail: %s\n", url, err.Error())
        return
    }

    p := spinner.StartNew("Install "+t.host+" ...")

    c := oakUtility.New_SSHClient (t.host)
    if err := c.Open(t.user, t.pass); err != nil {
        log.Error.Printf ("ssh %s: %s\n", t.host, err.Error())
        return
    }
    defer c.Close()

    remotefile := "/tmp/oakridge.tar.gz"
    if _,err := c.Scp (localfile, remotefile, "0644"); err != nil {
        log.Error.Println (err.Error())
        return
    }

    fmt.Printf ("\nWriting flash, MUST NOT POWER OFF, it might take several minutes!\n")

    var cmds = []string {
    "tar xzf "+remotefile+" -C /tmp",
    "rm -rvf "+remotefile,
    "dd if=/tmp/openwrt-ar71xx-generic-ubnt-unifi-squashfs-sysupgrade.bin of=/tmp/kernel0.bin bs=7929856 count=1",
    "dd if=/tmp/openwrt-ar71xx-generic-ubnt-unifi-squashfs-sysupgrade.bin of=/tmp/kernel1.bin bs=7929856 count=1 skip=1",
    "mtd write /tmp/kernel0.bin kernel0",
    "mtd write /tmp/kernel1.bin kernel1",
    "reboot",
    }
    for _, cmd := range cmds {
        _, err := c.One_cmd (cmd)
        if err != nil {
            log.Error.Printf ("%s: %s\n", cmd,err.Error())
            return
        }
    }
    p.Stop ()
    fmt.Printf ("\n%s upgraded to Oakridge OS, please power cycle device\n", t.host)
}
func install_oak_firmwire () {
    var targets []Target

    // prepare the list
    for _,n:=range netlist {
        for _,u :=range n.UBNT_ap_list {
            switch u.HWmodel {
            case AC_LITE, AC_LR, AC_PRO, UBNT_ERX:
                t := Target { host: u.IPv4, mac: u.Mac, user: "ubnt", pass: "ubnt", HWmodel: u.HWmodel}
                targets = append(targets, t)
            }
        }
    }

    if len(targets) == 0 {
        println("\nNo supported 3rd-party device found")
        return
    }

    var choice int
    for {
        println("\nChoose which device to restore(ctrl-C to exist):")
        println("[0]. All devices")
        for i,d := range targets {
            fmt.Printf("[%d]. %s %s %s\n", i+1, d.host, d.mac, d.HWmodel)
        }

        fmt.Printf("Please choose: [0~%d]\n", len(targets))
        r := bufio.NewReader (os.Stdin)
        input,err := r.ReadString ('\n')
        if err != nil {
            println (err.Error())
            continue
        }

        if choice, err = strconv.Atoi(strings.TrimSpace(input)); err != nil {
            println (err.Error())
            continue
        }

        if choice >= 0 && choice <= len(targets) {
            oakUtility.ClearLine ()
            fmt.Printf("You choose: %d\n", choice)
            break
        }

        fmt.Printf("Invalid choicse: %d\n", choice)
    }

    if choice == 0 {
        var s sync.WaitGroup
        for _, t:= range targets {
            s.Add (1)
            go install_one_device (t, &s)
        }
        s.Wait()
    } else {
        install_one_device (targets[choice-1], nil)
    }
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

func scan_input_subnet (args []string) {
    var nets []string
    for _,arg := range args {
        net,err := oakUtility.String2netstring (arg)
        if err != nil {
            log.Error.Fatalln(err)
        }
        nets = append (nets,net)
    }

    println("Scan user input networks ...\n")

    // scan each subnet
    for _, net := range nets {
        n := New_Subnet (net)
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

    if len(os.Args) > 1 {
        scan_input_subnet (os.Args[1:])
    } else {
        scan_local_subnet ()
    }

    list_scan_result ()

    install_oak_firmwire ()

    //list_oakdev_csv ()

    println(Banner_end)
}
