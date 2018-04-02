package main

import (
	"bufio"
	"fmt"
	"image_burner/ping"
	"image_burner/spinner"
	"image_burner/util"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
 * global vars
 */
var netlist []Subnet
var convert_targets []Target
var upgrade_targets []Target

const Banner_start = `
Oakridge Firmware Update Utility, Ver 1.01, (c) Oakridge Networks, Inc. 2018
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
NOTE:
1. Make sure AP in the scanned subnet, or can directly input the target subnet
e.g. scan 192.168.1.0/24 subnet: ./convert 192.168.1.0/24
2. Make sure AP is reset back to factory default state
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
`
const Banner_end = "\nThanks for choose Oakridge Networks Inc.\n"

var log oakUtility.OakLogger

// NOTE:  ac-lite/ac-lr/ac-pro share the same img, for handy program, just list them all
const (
	AC_LITE      = oakUtility.AC_LITE
	AC_LR        = oakUtility.AC_LR
	AC_PRO       = oakUtility.AC_PRO
	UBNT_ERX_OLD = oakUtility.UBNT_ERX
	AC_LITE_OLD  = "ubntlite"
	AC_LR_OLD    = "ubntlr"
	AC_PRO_OLD   = "ubntpro"
	UBNT_ERX     = "EdgeRouter_ER-X"
	A923         = "A923"
	A820         = "A820"
	A822         = "A822"
	W282         = "W282"
	A920         = "A920"
	WL8200_I2    = "WL8200-I2"
)

type AP_QTS struct {
	Mac           string
	IPv4          string
	Vendor        string
	OEM           string
	Devname       string
	Board_SN      string // serial number
	Manufact_date string
	LatestFW      string
}

type UBNT_AP struct {
	Mac      string
	IPv4     string
	HWmodel  string
	SWver    string
	LatestFW string
}

func (d *UBNT_AP) OneLineSummary() string {
	return fmt.Sprintf("%-12s%-16s%-18s%-16s%-25s%s", "Ubiquiti", d.HWmodel, d.Mac, d.IPv4, d.SWver, d.LatestFW)
}

func (d *UBNT_AP) Get_latest_version() (version string) {
	version = ""
	d.LatestFW = ""
	url := "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-swversion.txt"
	localfile := "latest-swversion-ubnt.txt"

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Println(err.Error())
		return
	}

	dat, err := ioutil.ReadFile(localfile)
	if err != nil {
		log.Error.Println(err.Error())
		return
	}
	version = strings.TrimSpace(string(dat))

	d.LatestFW = version
	return
}

type Oakridge_Device struct {
	Mac      string
	HWmodel  string
	HWname   string
	IPv4     string
	Firmware string // this is bootloader version
	LatestFW string
}

func (d *Oakridge_Device) Get_latest_version() (version string) {
	version = ""
	d.LatestFW = ""
	url := "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-swversion.txt"
	localfile := "latest-swversion-oakridge.txt"

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Println(err.Error())
		return
	}

	dat, err := ioutil.ReadFile(localfile)
	if err != nil {
		log.Error.Println(err.Error())
		return
	}
	version = strings.TrimSpace(string(dat))

	d.LatestFW = version
	return
}

func (d *Oakridge_Device) OneLineSummary() string {
	return fmt.Sprintf("%-12s%-16s%-18s%-16s%-25s%s", "Oakridge", d.HWname, d.Mac, d.IPv4, d.Firmware, d.LatestFW)
}

func (d *AP_QTS) OneLineSummary() string {
	return fmt.Sprintf("%-12s%-16s%-18s%-16s%-25s%s", d.Vendor, d.OEM, d.Mac, d.IPv4, d.Manufact_date+" "+d.Board_SN, d.LatestFW)
}

func (d *AP_QTS) Get_latest_version() (version string) {
	version = ""
	d.LatestFW = ""
	url := "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-swversion.txt"
	localfile := "latest-swversion-ap152.txt"

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
func Oakdev_PrintHeader() {
	fmt.Printf("\n%-4s %-12s%-16s%-18s%-16s%-25s%s\n", "No.", "SW", "HW", "Mac", "IPv4", "Description", "Latest-OakFirmware")
	fmt.Printf("%s\n", strings.Repeat("=", 116))
}

type Converted_AP struct {
	Mac string
}

var converted_ap []Converted_AP // remember all new converted AP

type Subnet struct {
	Net          string
	holes        []net.IP // skip those ip-addr
	Oak_dev_list []*Oakridge_Device
	UBNT_ap_list []*UBNT_AP
	qts_list     []*AP_QTS
	batch        sync.WaitGroup // this to wait all host finish before exit
}

func New_Subnet(cidr string) Subnet {
	return Subnet{Net: cidr}
}
func (s *Subnet) Holes(h []net.IP) {
	s.holes = h
}

func (s *Subnet) Scan() {
	log.Info.Printf("scanning %s\n", s.Net)
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
		log.Info.Printf("%v is Oakridge device\n", dev)
		s.Oak_dev_list = append(s.Oak_dev_list, dev)
	} else if dev := Is_ubnt_ap(c); dev != nil {
		log.Info.Printf("%s is Ubiqiti AP\n", c.IPv4)
		s.UBNT_ap_list = append(s.UBNT_ap_list, dev)
	} else if dev := Is_ubnt_erx(c); dev != nil {
		log.Info.Printf("%s is Ubiqiti ERX\n", c.IPv4)
		s.UBNT_ap_list = append(s.UBNT_ap_list, dev)
	} else if dev := Is_ap_QTS(c); dev != nil {
		log.Info.Printf("%s is QTS\n", c.IPv4)
		s.qts_list = append(s.qts_list, dev)
	}
}

func (s *Subnet) OneLineSummary() {
	fmt.Printf("✓ %s: %d Oakridge, %d UBNT devices\n", s.Net, len(s.Oak_dev_list), len(s.UBNT_ap_list))
}
func Is_ubnt_erx(c oakUtility.SSHClient) *UBNT_AP {

	if err := c.Open("ubnt", "ubnt"); err != nil {
		return nil
	}
	defer c.Close()

	var dev UBNT_AP

	buf, err := c.One_cmd("/opt/vyatta/bin/vyatta-op-cmd-wrapper show version")
	if err != nil {
		log.Debug.Printf("%s %s: %s\n", c.IPv4, "show version", err.Error())
		return nil
	}
	// pass output string to get mac and hwmodel
	tvs := strings.Split(strings.TrimSpace(string(buf)), "\n")
	for _, t := range tvs {
		kv := strings.Split(t, ":")
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		log.Debug.Printf("%s %s:%s\n", c.IPv4, k, v)
		switch k {
		case "HW S/N":
			dev.Mac = v[:2] + ":" + v[2:4] + ":" + v[4:6] + ":" + v[6:8] + ":" + v[8:10] + ":" + v[10:]
		case "HW model":
			dev.HWmodel = v
			if v == "EdgeRouter X 5-Port" {
				dev.HWmodel = UBNT_ERX
			}
		case "Version":
			dev.SWver = v
		}
	}
	if dev.HWmodel != UBNT_ERX {
		log.Debug.Printf("unsupport erx hw %v\n", tvs)
		return nil
	}
	dev.IPv4 = c.IPv4
	return &dev
}

func Model_to_name(model string) (name string) {
	switch model {
	case AC_LITE, AC_LITE_OLD:
		name = "UBNT_AC-LITE"
		return
	case AC_LR, AC_LR_OLD:
		name = "UBNT_AC-LR"
		return
	case AC_PRO, AC_PRO_OLD:
		name = "UBNT_AC-PRO"
		return
	case UBNT_ERX, UBNT_ERX_OLD:
		name = "UBNT_EdgeRouter-X"
		return
	case WL8200_I2:
		name = "DCN_WL8200-I2"
		return
	case A923:
		name = "DCN_SEAP-380"
		return
	default:
		name = "QTS_" + model
		return
	}
}

func Is_oakridge_dev(c oakUtility.SSHClient) *Oakridge_Device {

	if err := c.Open("root", "oakridge"); err != nil {
		log.Debug.Printf("fail login as root to %s: %s\n", c.IPv4, err.Error())
		return nil
	}
	defer c.Close()

	var dev Oakridge_Device

	// mac-addr
	buf, err := c.One_cmd("uci get productinfo.productinfo.mac")
	if err != nil {
		log.Debug.Printf("%s: %s\n", c.IPv4, err.Error())
		return nil
	}
	dev.Mac = strings.TrimSpace(string(buf))

	buf, err = c.One_cmd("uci get productinfo.productinfo.production")
	if err != nil {
		log.Debug.Printf("%s: %s\n", c.IPv4, err.Error())
		return nil
	}
	dev.HWmodel = strings.TrimSpace(string(buf))

	buf, err = c.One_cmd("uci get productinfo.productinfo.model")
	if err != nil {
		log.Debug.Printf("%s: %s\n", c.IPv4, err.Error())
		dev.HWname = Model_to_name(dev.HWmodel)
	} else {
		dev.HWname = strings.TrimSpace(string(buf))
	}

	buf, err = c.One_cmd("uci get productinfo.productinfo.bootversion")
	if err != nil {
		log.Debug.Printf("%s: %s\n", c.IPv4, err.Error())
		buf, err = c.One_cmd("uci get productinfo.productinfo.swversion")
		if err != nil {
			log.Debug.Printf("uci get productinfo.productinfo.swversion: %s\n", err.Error())
			return nil
		}
	}
	dev.Firmware = strings.TrimSpace(string(buf))

	dev.IPv4 = c.IPv4
	dev.LatestFW = dev.Get_latest_version()
	return &dev
}

func Is_ubnt_ap(c oakUtility.SSHClient) *UBNT_AP {

	if err := c.Open("ubnt", "ubnt"); err != nil {
		return nil
	}
	defer c.Close()

	var dev UBNT_AP

	buf, err := c.One_cmd("cat /proc/ubnthal/system.info")
	if err != nil {
		log.Debug.Printf("%s: %s\n", c.IPv4, err.Error())
		return nil
	}
	// pass output string to get mac and hwmodel
	tvs := strings.Split(strings.TrimSpace(string(buf)), "\n")
	for _, t := range tvs {
		switch v := strings.Split(t, "="); v[0] {
		case "eth0.macaddr":
			dev.Mac = v[1]
		case "systemid":
			switch v[1] {
			case "e517":
				dev.HWmodel = AC_LITE
			case "e527":
				dev.HWmodel = AC_LR
			case "e537":
				dev.HWmodel = AC_PRO
			default:
				// only support model above
				log.Debug.Printf("%s: %s not support\n", c.IPv4, v[1])
				return nil
			}
		}
	}

	// sw ver
	buf, err = c.One_cmd("cat /etc/version")
	if err != nil {
		log.Debug.Printf("%s: %s\n", c.IPv4, err.Error())
		return nil
	}
	dev.SWver = strings.TrimSpace(string(buf))
	dev.IPv4 = c.IPv4
	dev.LatestFW = dev.Get_latest_version()
	return &dev
}

func Is_ap_QTS(c oakUtility.SSHClient) *AP_QTS {

	if err := c.Open("admin", "admin"); err != nil {
		log.Debug.Printf("fail login %s: %s\n", c.IPv4, err.Error())
		return nil
	}
	defer c.Close()

	buf, err := c.One_cmd("strings /dev/mtd5 | grep =")
	if err != nil {
		log.Debug.Printf("%s: %s\n", c.IPv4, err.Error())
		return nil
	}

	// now we parse the <key>=<value>
	var dev AP_QTS
	tvs := strings.Split(strings.TrimSpace(string(buf)), "\n")
	for _, t := range tvs {
		kv := strings.Split(t, "=")
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		if len(v) > 0 && v[0] == '"' { // remove <">
			v = v[1:]
		}
		if len(v) > 0 && v[len(v)-1] == '"' {
			v = v[:len(v)-1]
		}
		switch k {
		case "MAC_ADDRESS":
			dev.Mac = v
		case "VENDOR_NAME":
			dev.Vendor = v
		case "DEV_OEMNAME":
			dev.OEM = v
		case "DEV_NAME":
			dev.Devname = v
		case "BOARD_SERIAL_NUMBER":
			dev.Board_SN = v
		case "MANUFACTURING_DATE":
			dev.Manufact_date = v
		}
	}

	switch dev.Devname {
	case A820, A822, A920, W282:
		dev.OEM = "QTS_" + dev.Devname
	case WL8200_I2:
		dev.OEM = "DCN_" + dev.Devname
	case A923:
		dev.OEM = "DCN_SEAP380"
	default:
		return nil
	}

	dev.IPv4 = c.IPv4
	dev.LatestFW = dev.Get_latest_version()
	log.Debug.Printf("%v\n", dev)
	return &dev
}

func list_scan_result() {
	cnt := 0

	Oakdev_PrintHeader()
	for _, n := range netlist {
		for _, o := range n.Oak_dev_list {
			cnt++
			if o.Firmware != o.LatestFW {
				fmt.Printf("✓%-3d %s\n", cnt, o.OneLineSummary())

				switch o.HWmodel {
				case AC_LITE, AC_LR, AC_PRO, UBNT_ERX, UBNT_ERX_OLD, AC_LITE_OLD, AC_LR_OLD, AC_PRO_OLD, A923, A820, A822, W282, A920, WL8200_I2:
					t := Target{host: o.IPv4, mac: o.Mac, user: "root", pass: "oakridge", HWmodel: o.HWmodel,
						Name: o.HWname, SWver: o.Firmware, LatestSW: o.LatestFW} // we put together the target list, so later it can just be used directly
					upgrade_targets = append(upgrade_targets, t)
				}
			} else {
				fmt.Printf("%-3d %s\n", cnt, o.OneLineSummary())
			}
		}
		for _, u := range n.UBNT_ap_list {
			cnt++
			fmt.Printf("✓%-3d %s\n", cnt, u.OneLineSummary())
			switch u.HWmodel {
			case AC_LITE, AC_LR, AC_PRO, UBNT_ERX:
				t := Target{host: u.IPv4, mac: u.Mac, user: "ubnt", pass: "ubnt", HWmodel: u.HWmodel, Name: ("UBNT_" + u.HWmodel), LatestSW: u.LatestFW}
				convert_targets = append(convert_targets, t)
			}
		}
		for _, s := range n.qts_list {
			cnt++
			fmt.Printf("✓%-3d %s\n", cnt, s.OneLineSummary())
			t := Target{host: s.IPv4, mac: s.Mac, user: "admin", pass: "admin", HWmodel: s.Devname, Name: s.OEM, LatestSW: s.LatestFW}
			convert_targets = append(convert_targets, t)
		}
	}

	return
}

type Target struct {
	host     string
	mac      string
	user     string
	pass     string
	Name     string
	HWmodel  string
	SWver    string
	LatestSW string
	result   string
}

var erx_imgs = map[string][]string{
	"factory":  {"erx_factory.bin.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubnterx/origin/factory.bin.tar.gz"},
	"oakridge": {"oakridge_sysupgrade.bin.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubnterx/sysloader/latest-sysupgrade.bin.tar.gz"},
}

func erx_factory_img(host string) error {

	p := spinner.StartNew("Install factory img ...")
	defer p.Stop()

	c := oakUtility.New_SSHClient(host)
	if err := c.Open("ubnt", "ubnt"); err != nil {
		return err
	}
	defer c.Close()

	file := erx_imgs["factory"][0]
	if _, err := c.Scp(file, "/tmp/"+file, "0644"); err != nil {
		log.Error.Println(err.Error())
		return err
	}
	log.Debug.Printf("done scp %s to %s:%s\n", file, host, "/tmp/"+file)

	if _, err := c.One_cmd("tar xzf /tmp/" + file + " -C /tmp"); err != nil {
		log.Error.Println(err.Error())
		return err
	}
	log.Debug.Printf("done untar %s:%s\n", host, "/tmp/"+file)

	if buf, err := c.One_cmd("/opt/vyatta/bin/vyatta-op-cmd-wrapper add system image /tmp/lede-ramips-mt7621-ubnt-erx-initramfs-factory.tar"); err != nil {
		log.Error.Println(string(buf), err.Error())
		return err
	}

	if _, err := c.One_cmd("/opt/vyatta/bin/vyatta-op-cmd-wrapper reboot now"); err != nil {
		log.Error.Println(err.Error())
		return err
	}
	return nil
}
func install_ubnt_erx_img(host string) {
	for _, img := range erx_imgs {
		if err := oakUtility.On_demand_download(img[0], img[1]); err != nil {
			log.Error.Println(err.Error())
			return
		}
	}
	if err := erx_factory_img(host); err != nil {
		return
	}

	p := spinner.StartNew("Wait device bootup ...")
	pinger, err := ping.NewPinger(host)
	if err != nil {
		panic(err)
	}
	pinger.SetStopAfter(5)
	pinger.OnRecv = func(pkt *ping.Packet) {
		fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v\n", pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
	}
	pinger.Run()
	p.Stop()

	p.SetTitle("Install Oakridge img ...")
	p.Start()
	defer p.Stop()
	c := oakUtility.New_SSHClient(host) // ssh back to device again
	for {
		time.Sleep(1 * time.Second)
		err := c.Open("root", "oakridge")
		if err == nil {
			log.Debug.Printf("ssh connected to %s\n", host)
			break
		}
		log.Debug.Println(err.Error())
	}
	defer c.Close()

	file := erx_imgs["oakridge"][0]
	if _, err := c.Scp(file, "/tmp/"+file, "0644"); err != nil {
		log.Error.Println(err.Error())
		return
	}
	if _, err := c.One_cmd("tar xzf /tmp/" + file + " -C /tmp"); err != nil {
		log.Error.Printf("%s\n", err.Error())
		return
	}
	c.One_cmd("sysupgrade -n lede-ramips-mt7621-ubnt-erx-squashfs-sysupgrade.bin")
}
func record_converted_ap(mac string) {
	converted_ap = append(converted_ap, Converted_AP{Mac: mac})
}
func install_one_device(t Target, s *sync.WaitGroup) {

	if s != nil {
		defer s.Done()
	}

	switch t.HWmodel {
	case AC_LITE, AC_LR, AC_PRO:
		err := install_unifi_ap_img(t)
		if err == nil {
			record_converted_ap(t.mac)
		}
	case A820, A822, W282, A920, A923, WL8200_I2:
		err := install_via_sysupgrade(t)
		if err == nil {
			record_converted_ap(t.mac)
		}
	case UBNT_ERX:
		install_ubnt_erx_img(t.host)
	default:
		fmt.Printf("unsupport model %s\n", t.HWmodel)
		return
	}
}

var ap152_imgs = map[string][]string{
	A820:      {"oakridge.a820.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	A822:      {"oakridge.a822.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	W282:      {"oakridge.w282.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	A920:      {"oakridge.a920.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	A923:      {"oakridge.a923.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
	WL8200_I2: {"oakridge.wl8200_i2.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
}

func install_via_sysupgrade(t Target) error {
	localfile := ap152_imgs[t.HWmodel][0]
	url := ap152_imgs[t.HWmodel][1]

	log.Info.Printf("install %s %s from %s\n", t.host, localfile, url)

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Printf("on-demand-download %s fail: %s\n", url, err.Error())
		return err
	}

	c := oakUtility.New_SSHClient(t.host)
	if err := c.Open(t.user, t.pass); err != nil {
		log.Error.Printf("ssh connected to %s: %s\n", t.host, err.Error())
		return err
	}
	defer c.Close()

	p := spinner.StartNew("copy img ...")

	if _, err := c.Scp(localfile, "/tmp/"+localfile, "0644"); err != nil {
		log.Error.Println(err.Error())
		p.Stop()
		return err
	}
	p.Stop()

	fmt.Printf("\nWriting flash, MUST NOT POWER OFF, it might take several minutes!\n")
	p.SetTitle("writing flash ...")
	p.Start()

	if _, err := c.One_cmd("tar xzf /tmp/" + localfile + " -C /tmp"); err != nil {
		log.Error.Printf("%s\n", err.Error())
		p.Stop()
		return err
	}
	buf, err := c.One_cmd("sysupgrade -n /tmp/openwrt-ar71xx-generic-ap152-16M-squashfs-sysupgrade.bin")
	log.Debug.Printf("<%s> %s\n", string(buf), err.Error())
	p.Stop()
	return nil
}

var unifi_ap_imgs = map[string][]string{
	AC_LITE: {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
	AC_LR:   {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
	AC_PRO:  {"oakridge.sysloader.ubnt.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
}

func install_unifi_ap_img(t Target) error {

	localfile := unifi_ap_imgs[t.HWmodel][0]
	url := unifi_ap_imgs[t.HWmodel][1]

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Printf("on-demand-download %s fail: %s\n", url, err.Error())
		return err
	}

	p := spinner.StartNew("Install " + t.host + " ...")
	defer p.Stop()

	c := oakUtility.New_SSHClient(t.host)
	if err := c.Open(t.user, t.pass); err != nil {
		log.Error.Printf("ssh %s: %s\n", t.host, err.Error())
		return err
	}
	defer c.Close()

	remotefile := "/tmp/oakridge.tar.gz"
	if _, err := c.Scp(localfile, remotefile, "0644"); err != nil {
		log.Error.Println(err.Error())
		return err
	}

	fmt.Printf("\nWriting flash, MUST NOT POWER OFF, it might take several minutes!\n")

	var cmds = []string{
		"tar xzf " + remotefile + " -C /tmp",
		"rm -rvf " + remotefile,
		"dd if=/tmp/openwrt-ar71xx-generic-ubnt-unifi-squashfs-sysupgrade.bin of=/tmp/kernel0.bin bs=7929856 count=1",
		"dd if=/tmp/openwrt-ar71xx-generic-ubnt-unifi-squashfs-sysupgrade.bin of=/tmp/kernel1.bin bs=7929856 count=1 skip=1",
		"mtd write /tmp/kernel0.bin kernel0",
		"mtd write /tmp/kernel1.bin kernel1",
		"reboot",
	}
	for _, cmd := range cmds {
		_, err := c.One_cmd(cmd)
		if err != nil {
			log.Error.Printf("%s: %s\n", cmd, err.Error())
			return err
		}
	}
	fmt.Printf("\n%s upgraded to Oakridge OS, please power cycle device\n", t.host)
	return nil
}
func install_oak_firmware() {

	var choice int
	for {
		println("\nChoose which device to convert(ctrl-C to exist):")
		println("[0]. All devices")
		for i, d := range convert_targets {
			fmt.Printf("[%d]. %-16s %-18s %s %s\n", i+1, d.host, d.mac, d.Name, d.LatestSW)
		}

		fmt.Printf("Please choose: [0~%d]\n", len(convert_targets))
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

		if choice >= 0 && choice <= len(convert_targets) {
			oakUtility.ClearLine()
			fmt.Printf("You choose: %d\n", choice)
			break
		}

		fmt.Printf("Invalid choicse: %d\n", choice)
	}

	if choice == 0 {
		var s sync.WaitGroup
		for _, t := range convert_targets {
			s.Add(1)
			go install_one_device(t, &s)
		}
		s.Wait()
	} else {
		install_one_device(convert_targets[choice-1], nil)
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

// write scanned Oakridge AP and newly converted AP to a csv file for easy import to oakmgr
func write_OakAP_csv() {
	var maclist []string
	for _, n := range netlist {
		for _, ap := range n.Oak_dev_list {
			if ap.HWmodel == UBNT_ERX {
				continue
			}
			maclist = append(maclist, ap.Mac)
		}
	}
	for _, ap := range converted_ap {
		maclist = append(maclist, ap.Mac)
	}

	if len(maclist) == 0 {
		return
	}

	const file string = "oakridge_ap.csv"
	const tablehead string = "MAC"

	f, err := os.Create(file)
	if err != nil {
		log.Error.Printf("%s\n", err.Error())
		return
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	w.WriteString("# Automatically generated at " + time.Now().Format(time.RFC3339) + "\n")
	w.WriteString(tablehead + "\n")

	for _, m := range maclist {
		w.WriteString(m + "\n")
	}

	w.Flush()

	fmt.Printf("\nAll Oakridge AP saved in %s to be import into management system\n", file)
}

func init() {
	log = oakUtility.New_OakLogger()
	log.Set_level("error")
	cleanup()
	prepare_sshconf()
}

const (
	OPERATION_INVALID = -1
	OPERATION_CONVERT = 1
	OPERATION_UPGRADE = 2
)

func select_operation() (choice int) {
	choice = OPERATION_INVALID
	for {
		println("\nChoose what do you want to do(ctrl-C to exist):")
		fmt.Printf("[%d]. Convert Vendor Devices to OakFirmware\n", OPERATION_CONVERT)
		fmt.Printf("[%d]. Upgrade Oak Devices to Latest OakFirmware\n", OPERATION_UPGRADE)

		fmt.Printf("Please choose: [0~1]\n")
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

		if choice > 0 && choice <= 2 {
			oakUtility.ClearLine()
			fmt.Printf("You choose: %d\n", choice)
			break
		}

		fmt.Printf("Invalid choicse: %d\n", choice)
	}
	return
}

func cleanup() {
	os.Remove("latest-swversion-oakridge.txt")
	os.Remove("latest-swversion-ap152.txt")
	os.Remove("latest-swversion-ubnt.txt")
}

func prepare_sshconf() {
	if runtime.GOOS != "windows" {
		exec.Command("mkdir -p ~/.ssh;[ -z \"$(sed -n '/TCPKeepAlive.*yes/p' ~/.ssh/config 2>/dev/null)\" ] && sed -i '1 iTCPKeepAlive yes' ~/.ssh/config 2>/dev/null")
	}
}

func main() {

	println(Banner_start)

	if len(os.Args) > 1 {
		scan_input_subnet(os.Args[1:])
	} else {
		scan_local_subnet()
	}

	list_scan_result()

	upgrade_cnt := len(upgrade_targets)
	convert_cnt := len(convert_targets)
	operation_choice := OPERATION_INVALID
	if upgrade_cnt == 0 && convert_cnt == 0 {
		println("\nNo supported 3rd-party device found")
		operation_choice = OPERATION_INVALID
	} else if upgrade_cnt == 0 {
		println("\nOnly can be converted devices found")
		operation_choice = OPERATION_CONVERT
	} else if convert_cnt == 0 {
		println("\nOnly can be upgraded devices found")
		operation_choice = OPERATION_UPGRADE
	} else {
		operation_choice = select_operation()
	}

	switch operation_choice {
	case OPERATION_CONVERT:
		println("\n**To do convert Vendor devices now**\n")
		install_oak_firmware()
		write_OakAP_csv()
	case OPERATION_UPGRADE:
		println("\n**To do upgrade Oak devices now**\n")
		upgrade_oak_firmware()
	}

	println(Banner_end)
}

var local_imgfile = map[string]string{
	AC_LITE: "",
	AC_LR:   "",
	AC_PRO:  "",
}

func upgrade_one_device(t Target, s *sync.WaitGroup) {
	if s != nil {
		defer s.Done()
	}

	switch t.HWmodel {
	case AC_LITE, AC_LR, AC_PRO, AC_LITE_OLD, AC_LR_OLD, AC_PRO_OLD, A923, A820, A822, W282, A920, WL8200_I2:
		switch t.HWmodel {
		case AC_LITE_OLD:
			t.HWmodel = AC_LITE
		case AC_LR_OLD:
			t.HWmodel = AC_LR
		case AC_PRO_OLD:
			t.HWmodel = AC_PRO
		}
		upgrade_unifi_ap152_ap(t)
	case UBNT_ERX, UBNT_ERX_OLD:
		switch t.HWmodel {
		case UBNT_ERX_OLD:
			t.HWmodel = UBNT_ERX
		}
		upgrade_unifi_ap152_ap(t)
	default:
		fmt.Printf("unsupport model %s\n", t.HWmodel)
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

	localfile := ap_origin_imgs[t.HWmodel][0]
	url := ap_origin_imgs[t.HWmodel][1]

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Println(err.Error())
		return
	}

	p := spinner.StartNew("Upgrade " + t.host + " " + t.HWmodel + " ...")
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
		{"/etc/init.d/capwap stop", "optional"},
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
			if err.Error() != "EOF" && cmd[1] == "mandatory" {
				return
			}
		}
	}
	fmt.Printf("\n%s upgrade image, please waiting boot up\n", t.host)
}

func upgrade_oak_firmware() {
	var choice int
	for {
		println("\nChoose which device to upgrade(ctrl-C to exist):")
		if len(upgrade_targets) > 1 {
			println("[0]. All devices")
		}
		for i, d := range upgrade_targets {
			fmt.Printf("[%d]. %s %s %s %s %s\n", i+1, d.host, d.mac, d.Name, d.SWver, d.LatestSW)
		}

		if len(upgrade_targets) > 1 {
			fmt.Printf("Please choose: [0~%d]\n", len(upgrade_targets))
		} else {
			fmt.Printf("Please choose: [%d]\n", len(upgrade_targets))
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

		if choice >= 0 && choice <= len(upgrade_targets) {
			oakUtility.ClearLine()
			fmt.Printf("You choose: %d\n", choice)
			break
		}

		fmt.Printf("Invalid choicse: %d\n", choice)
	}

	if choice == 0 {
		var s sync.WaitGroup
		for _, t := range upgrade_targets {
			s.Add(1)
			go upgrade_one_device(t, &s)
		}
		s.Wait()
	} else {
		upgrade_one_device(upgrade_targets[choice-1], nil)
	}
}
