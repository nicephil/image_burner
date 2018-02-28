package main

import (
    "fmt"
    "os"
    "bufio"
    "strconv"
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
var img = map[string][]string { //NOTE these 3 are use same img
    AC_LITE: {"ubnt.unifi.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/origin/firmware.bin.tar.gz"},
    AC_LR:   {"ubnt.unifi.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/origin/firmware.bin.tar.gz"},
    AC_PRO:  {"ubnt.unifi.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/origin/firmware.bin.tar.gz"},
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
        log.Debug.Printf("uci get productinfo.productinfo.mac: %s %s\n", c.IPv4, err.Error())
        return nil
    }
    dev.Mac = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.production")
    if err != nil {
        log.Debug.Printf("uci get productinfo.productinfo.production: %s\n", err.Error())
        return nil
    }
    dev.Model = strings.TrimSpace(string(buf))

    buf, err = c.One_cmd ("uci get productinfo.productinfo.swversion")
    if err != nil {
        log.Debug.Printf("uci get productinfo.productinfo.swversion: %s\n", err.Error())
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
    Model       string
    SWver       string
}

func on_demand_download (t *Target) error {
    if local_imgfile[t.Model] == "" {
        local := img[t.Model][0]
        url := img[t.Model][1]
        if _, err := os.Stat(local); os.IsNotExist(err) {
            if err := oakUtility.DownloadFile (local, url, true, "Downloading "+t.Model+" img ... "); err != nil {
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
func write_mtd (t Target, s *sync.WaitGroup) {
    if s != nil {
        defer s.Done()
    }

    switch t.Model {
    case AC_LITE, AC_LR, AC_PRO:
         if err := on_demand_download (&t); err != nil {
            println (err.Error())
            return
         }
    default:
        fmt.Printf ("unsupport model %s\n", t.Model)
        return
    }

    p := spinner.StartNew("Restore "+t.host+" ...")
    defer p.Stop ()

    c := oakUtility.New_SSHClient (t.host)
    if err := c.Open("root", "oakridge"); err != nil {
        println (err)
        return
    }
    defer c.Close()

    remotefile := "/tmp/oak.tar.gz"
    _,err := c.Scp (local_imgfile[t.Model], remotefile, "0644")
    if err != nil {
        println (err)
        return
    }

    fmt.Printf ("\nRestore %s image, MUST NOT POWER OFF DEVICE ...\n", t.host)

    var cmds = []string {
    "tar xzf "+remotefile+" -C /tmp",
    "rm -rvf "+remotefile,
    "mtd write /tmp/firmware.bin firmware",
    "reboot",
    }
    for _, cmd := range cmds {
        _, err := c.One_cmd (cmd)
        if err != nil {
            fmt.Printf ("\n%s: %s\n",cmd, err.Error())
            return
        }
    }
    fmt.Printf ("\n%s restored to factory image\n", t.host)
}
func choose_restore_firmwire () {
    var targets []Target

    // prepare the list
    for _,n:=range netlist {
        for _,d :=range n.Oak_dev_list {
            switch d.Model {
            case AC_LITE, AC_LR, AC_PRO:
                t := Target { host: d.IPv4, mac: d.Mac, Model: d.Model, SWver: d.Firmware}
                targets = append(targets, t)
            }
        }
    }

    if len(targets) == 0 {
        println("\nNo supported 3rd-party devices found")
        return
    }

    var choice int
    for {
        println("\nChoose which device to restore(ctrl-C to exist):")
        if len(targets) > 1 {
            println("[0]. All devices")
        }
        for i,d := range targets {
            fmt.Printf("[%d]. %s %s %s %s\n", i+1, d.host, d.mac, d.Model, d.SWver)
        }

        if len(targets) > 1 {
            fmt.Printf("Please choose: [0~%d]\n", len(targets))
        } else {
            fmt.Printf("Please choose: [%d]\n", len(targets))
        }
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
            go write_mtd (t, &s)
        }
        s.Wait()
    } else {
        write_mtd (targets[choice-1], nil)
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
        _, ipnet, err := net.ParseCIDR (arg)
        if err != nil {
            log.Error.Fatalln(err)
        }
        nets = append (nets,ipnet.String())
    }

    println("Scanning user input networks ...\n")

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

    choose_restore_firmwire ()

    //list_oakdev_csv ()

    println(Banner_end)
}
