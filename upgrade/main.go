package main

import (
	"bufio"
	"fmt"
	"image_burner/spinner"
	"image_burner/util"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

/*
* global vars
 */
var netlist []Subnet
var targets []Target

const Banner_start = "\nFirmware Upgrade Utility, Ver 1.01, (c) Oakridge Networks, Inc. 2018\n"
const Banner_end = "\nThanks for choose Oakridge Networks Inc.\n"

var log oakUtility.OakLogger

// NOTE:  ac-lite/ac-lr/ac-pro share the same img, for handy program, just list them all
const AC_LITE = oakUtility.AC_LITE
const AC_LITE_OLD = "ubntlite"
const AC_LR = oakUtility.AC_LR
const AC_LR_OLD = "ubntlr"
const AC_PRO = oakUtility.AC_PRO
const AC_PRO_OLD = "ubntpro"
const UBNT_ERX = "EdgeRouter_ER-X"
const UBNT_ERX_OLD = oakUtility.UBNT_ERX_OLD
const A923 = "A923"
const A820 = "A820"
const A822 = "A822"
const W282 = "W282"
const A920 = "A920"
const WL8200_I2 = "WL8200-I2"

var local_imgfile = map[string]string{
	AC_LITE: "",
	AC_LR:   "",
	AC_PRO:  "",
}

type Oakridge_Device struct {
	Mac      string
	Model    string
	Name     string
	IPv4     string
	Firmware string
	LatestFW string
}

func (d *Oakridge_Device) Get_latest_version() (version string) {
	version = ""
	d.LatestFW = ""
	localfile := ""
	url := ""

	switch d.Model {
	case AC_LITE, AC_LR, AC_PRO, AC_LITE_OLD, AC_LR_OLD, AC_PRO_OLD, A923, A820, A822, W282, A920, WL8200_I2:
		url = "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-swversion.txt"
		localfile = "latest-swversion-ap152.txt"
	case UBNT_ERX, UBNT_ERX_OLD:
		url = "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-swversion.txt"
		localfile = "latest-swversion-ubnerx.txt"
	default:
		return
	}

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Println(err.Error())
		return
	}

	cmd := exec.Command("/bin/sh", "-c", "cat "+localfile)
	out, err := cmd.Output()
	if err != nil {
		log.Error.Println(err.Error())
		return
	}
	version = strings.TrimSpace(string(out))

	d.LatestFW = version
	return
}

func (d *Oakridge_Device) OneLineSummary() string {
	return fmt.Sprintf("%-12s%-16s%-18s%-16s%-25s%s", "Oakridge", d.Name, d.Mac, d.IPv4, d.Firmware, d.LatestFW)
}
func Oakdev_PrintHeader() {
	fmt.Printf("\n%-4s %-12s%-16s%-18s%-16s%-25s%s\n", "No.", "SW", "HW", "Mac", "IPv4", "Firmware", "Latest-Firmware")
	fmt.Printf("%s\n", strings.Repeat("=", 116))
}

type Subnet struct {
	Net          string
	holes        []net.IP // skip those ip-addr
	Oak_dev_list []*Oakridge_Device
	batch        sync.WaitGroup // this to wait all host finish before exit
}

func New_Subnet(cidr string) Subnet {
	return Subnet{Net: cidr}
}
func (s *Subnet) Holes(h []net.IP) {
	s.holes = h
}

func (s *Subnet) Scan() {
	hosts, err := oakUtility.Net2hosts_exclude(s.Net, s.holes)
	if err != nil {
		fmt.Println(s.Net, err)
		return
	}

	// spinner
	p := spinner.StartNew(s.Net)
	defer func() {
		p.Stop()
	}()

	// do all hosts in one subnet in a parallel
	for _, h := range hosts {
		s.batch.Add(1)
		go s.scan_one(h)
	}
	s.batch.Wait()
}
func (s *Subnet) scan_one(host string) {

	defer s.batch.Done()

	c := oakUtility.New_SSHClient(host)

	if dev := Is_oakridge_dev(c); dev != nil {
		s.Oak_dev_list = append(s.Oak_dev_list, dev)
	}
}

func (s *Subnet) OneLineSummary() {
	fmt.Printf("✓ %s: %d Oakridge devices\n", s.Net, len(s.Oak_dev_list))
}

func Is_oakridge_dev(c oakUtility.SSHClient) *Oakridge_Device {

	if err := c.Open("root", "oakridge"); err != nil {
		log.Debug.Printf("fail login as root to %s\n", c.IPv4)
		return nil
	}
	defer c.Close()

	var dev Oakridge_Device

	// mac-addr
	buf, err := c.One_cmd("uci get productinfo.productinfo.mac")
	if err != nil {
		log.Debug.Printf("uci get productinfo.productinfo.mac: %s %s\n", c.IPv4, err.Error())
		return nil
	}
	dev.Mac = strings.TrimSpace(string(buf))

	buf, err = c.One_cmd("uci get productinfo.productinfo.production")
	if err != nil {
		log.Debug.Printf("uci get productinfo.productinfo.production: %s\n", err.Error())
		return nil
	}
	dev.Model = strings.TrimSpace(string(buf))

	buf, err = c.One_cmd("uci get productinfo.productinfo.model")
	if err != nil {
		log.Debug.Printf("uci get productinfo.productinfo.model: %s\n", err.Error())
		return nil
	}
	dev.Name = strings.TrimSpace(string(buf))

	buf, err = c.One_cmd("uci get productinfo.productinfo.bootversion")
	if err != nil {
		log.Debug.Printf("uci get productinfo.productinfo.swversion: %s\n", err.Error())
		return nil
	}
	dev.Firmware = strings.TrimSpace(string(buf))

	dev.IPv4 = c.IPv4
	return &dev
}

func list_scan_result() {
	cnt := 0

	Oakdev_PrintHeader()
	for _, n := range netlist {
		for _, o := range n.Oak_dev_list {
			cnt++
			switch o.Model {
			case AC_LITE, AC_LR, AC_PRO, UBNT_ERX, UBNT_ERX_OLD, AC_LITE_OLD, AC_LR_OLD, AC_PRO_OLD, A923, A820, A822, W282, A920, WL8200_I2:
				o.Get_latest_version()
				fmt.Printf("✓%-3d %s\n", cnt, o.OneLineSummary())
				if o.Firmware != o.LatestFW {
					t := Target{host: o.IPv4, mac: o.Mac, Model: o.Model,
						Name: o.Name, SWver: o.Firmware, LatestSW: o.LatestFW} // we put together the target list, so later it can just be used directly
					targets = append(targets, t)
				}
			default:
				fmt.Printf(" %-3d %s\n", cnt, o.OneLineSummary())
			}
		}
	}
}

type Target struct {
	host     string
	mac      string
	Model    string
	Name     string
	SWver    string
	LatestSW string
}

func upgrade_one_device(t Target, s *sync.WaitGroup) {
	if s != nil {
		defer s.Done()
	}

	switch t.Model {
	case AC_LITE, AC_LR, AC_PRO, AC_LITE_OLD, AC_LR_OLD, AC_PRO_OLD, A923, A820, A822, W282, A920, WL8200_I2:
		switch t.Model {
		case AC_LITE_OLD:
			t.Model = AC_LITE
		case AC_LR_OLD:
			t.Model = AC_LR
		case AC_PRO_OLD:
			t.Model = AC_PRO
		}
		upgrade_unifi_ap152_ap(t)
	case UBNT_ERX, UBNT_ERX_OLD:
		switch t.Model {
		case UBNT_ERX_OLD:
			t.Model = UBNT_ERX
		}
		upgrade_unifi_ap152_ap(t)
	default:
		fmt.Printf("unsupport model %s\n", t.Model)
		return
	}
}

var ap_origin_imgs = map[string][]string{ //NOTE these 3 are use same img
	AC_LITE:   {"aclite.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
	AC_LR:     {"aclr.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
	AC_PRO:    {"acpro.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
	A923:      {"a923.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	A820:      {"a820.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	A822:      {"a822.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	W282:      {"w282.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	A920:      {"a920.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	WL8200_I2: {"wl8200_i2.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	UBNT_ERX:  {"ubnterx.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubnterx/sysloader/latest-sysupgrade.bin.tar.gz"},
}

func upgrade_unifi_ap152_ap(t Target) {

	localfile := ap_origin_imgs[t.Model][0]
	url := ap_origin_imgs[t.Model][1]

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Println(err.Error())
		return
	}

	p := spinner.StartNew("Upgrade " + t.host + " " + t.Model + " ...")
	defer p.Stop()

	c := oakUtility.New_SSHClient(t.host)
	if err := c.Open("root", "oakridge"); err != nil {
		println(err)
		return
	}
	defer c.Close()

	remotefile := "/tmp/oak.tar.gz"
	_, err := c.Scp(localfile, remotefile, "0644")
	if err != nil {
		println(err)
		return
	}

	fmt.Printf("\nWrite flash, MUST NOT POWER OFF, it might take several minutes!\n")

	var cmds = [][]string{
		{"echo 'Auto Upgrade Now...'|logger -p2", "optional"},
		{"stop", "optional"},
		{"echo /etc/init.d/capwap stop", "optional"},
		{"/etc/init.d/handle_cloud stop", "optional"},
		{"/etc/init.d/wifidog stop", "optional"},
		{"/etc/init.d/arpwatch stop", "optional"},
		{"tar xzf " + remotefile + " -C /tmp", "mandatory"},
		{"rm -rvf " + remotefile, "mandatory"},
		{"sysupgrade -n /tmp/*-squashfs-sysupgrade.bin", "mandatory"},
	}
	for _, cmd := range cmds {
		buf, err := c.One_cmd(cmd[0])
		if err != nil {
			log.Debug.Printf("\n%v: %s <%s>\n", cmd, err.Error(), string(buf))
			if cmd[1] == "mandatory" {
				return
			}
		}
	}
	fmt.Printf("\n%s upgrade image, please waiting boot up\n", t.host)
}
func choose_upgrade_firmware() {
	// targets is put together in list_scan_result
	if len(targets) == 0 {
		println("\nNo supported 3rd-party devices found")
		return
	}

	var choice int
	for {
		println("\nChoose which device to upgrade(ctrl-C to exist):")
		if len(targets) > 1 {
			println("[0]. All devices")
		}
		for i, d := range targets {
			fmt.Printf("[%d]. %s %s %s %s %s\n", i+1, d.host, d.mac, d.Name, d.SWver, d.LatestSW)
		}

		if len(targets) > 1 {
			fmt.Printf("Please choose: [0~%d]\n", len(targets))
		} else {
			fmt.Printf("Please choose: [%d]\n", len(targets))
		}
		r := bufio.NewReader(os.Stdin)
		input, err := r.ReadString('\n')
		if err != nil {
			println(err.Error())
			continue
		}

		if choice, err = strconv.Atoi(strings.TrimSpace(input)); err != nil {
			println(err.Error())
			continue
		}

		if choice >= 0 && choice <= len(targets) {
			oakUtility.ClearLine()
			fmt.Printf("You choose: %d\n", choice)
			break
		}

		fmt.Printf("Invalid choicse: %d\n", choice)
	}

	if choice == 0 {
		var s sync.WaitGroup
		for _, t := range targets {
			s.Add(1)
			go upgrade_one_device(t, &s)
		}
		s.Wait()
	} else {
		upgrade_one_device(targets[choice-1], nil)
	}
}
func scan_local_subnet() {
	nets, selfs, err := oakUtility.Get_local_subnets()
	if err != nil {
		log.Error.Fatalln(err)
	}

	println("Scanning local networks ...\n")

	// scan each subnet
	for _, net := range nets {
		n := New_Subnet(net)
		n.Holes(selfs)
		n.Scan()
		n.OneLineSummary()
		netlist = append(netlist, n)
	}
}

func scan_input_subnet(args []string) {
	var nets []string
	for _, arg := range args {
		net, err := oakUtility.String2netstring(arg)
		if err != nil {
			log.Error.Fatalln(err)
		}
		nets = append(nets, net)
	}

	println("Scan user input networks ...\n")

	// scan each subnet
	for _, net := range nets {
		n := New_Subnet(net)
		n.Scan()
		n.OneLineSummary()
		netlist = append(netlist, n)
	}
}

func init() {
	log = oakUtility.New_OakLogger()
	log.Set_level("error")
}

func cleanup() {
	cmd := exec.Command("/bin/sh", "-c", "rm -rf *.tar.gz *.txt")
	_, err := cmd.Output()
	if err != nil {
		log.Error.Println(err.Error())
		return
	}
}

func main() {

	cleanup()

	println(Banner_start)

	if len(os.Args) > 1 {
		scan_input_subnet(os.Args[1:])
	} else {
		scan_local_subnet()
	}

	list_scan_result()

	choose_upgrade_firmware()

	//list_oakdev_csv ()

	println(Banner_end)
}
