package main

import (
	"bufio"
	"fmt"
	"image_burner/ping"
	"image_burner/spinner"
	"image_burner/util"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
* global vars
 */
var netlist []Subnet
var targets []Target

const Banner_start = `
Firmware Restore Utility, Ver 1.01, (c) Oakridge Networks, Inc. 2018
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
NOTE:
1. Make sure AP in the scanned subnet, or can directly input the target subnet
e.g. scan 192.168.1.0/24 subnet: ./restore 192.168.1.0/24
2. Make sure AP is reset back to factory default state
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
`
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
const WL8200_I2 = "WL8200-I2"
const A920 = "A920"

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
}

func (d *Oakridge_Device) OneLineSummary() string {
	return fmt.Sprintf("%-12s%-16s%-18s%-16s%s", "Oakridge", d.Name, d.Mac, d.IPv4, d.Firmware)
}
func Oakdev_PrintHeader() {
	fmt.Printf("\n%-4s %-12s%-16s%-18s%-16s%s\n", "No.", "SW", "HW", "Mac", "IPv4", "Firmware")
	fmt.Printf("%s\n", strings.Repeat("=", 96))
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
		dev.Name = Model_to_name(dev.Model)
	} else {
		dev.Name = strings.TrimSpace(string(buf))
	}

	buf, err = c.One_cmd("uci get productinfo.productinfo.bootversion")
	if err != nil {
		log.Debug.Printf("uci get productinfo.productinfo.bootversion: %s\n", err.Error())
		buf, err = c.One_cmd("uci get productinfo.productinfo.swversion")
		if err != nil {
			log.Debug.Printf("uci get productinfo.productinfo.swversion: %s\n", err.Error())
			return nil
		}
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
			case AC_LITE, AC_LR, AC_PRO, UBNT_ERX, UBNT_ERX_OLD, AC_LITE_OLD, AC_LR_OLD, AC_PRO_OLD, A923, A820, A822, W282, WL8200_I2, A920:
				fmt.Printf("✓%-3d %s\n", cnt, o.OneLineSummary())
				t := Target{host: o.IPv4, mac: o.Mac, Model: o.Model, Name: o.Name, SWver: o.Firmware} // we put together the target list, so later it can just be used directly
				targets = append(targets, t)
			default:
				fmt.Printf(" %-3d %s\n", cnt, o.OneLineSummary())
			}
		}
	}
}

type Target struct {
	host  string
	mac   string
	Model string
	Name  string
	SWver string
}

var erx_imgs = map[string][]string{
	"recover":    {"erx_recover.tar.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubnterx/origin/recover-ubnt-erx.tar.tar.gz"},
	"squash":     {"erx_squashfs.tmp", "http://image.oakridge.vip:8000/images/ap/ubnterx/origin/squashfs.tmp"},
	"squash_md5": {"erx_squashfs.tmp.md5", "http://image.oakridge.vip:8000/images/ap/ubnterx/origin/squashfs.tmp.md5"},
	"version":    {"erx_version.tmp", "http://image.oakridge.vip:8000/images/ap/ubnterx/origin/version.tmp"},
	"vmlinux":    {"erx_vmlinux.tmp", "http://image.oakridge.vip:8000/images/ap/ubnterx/origin/vmlinux.tmp"},
}

func ubnt_recover_img(host string, file string) error {

	p := spinner.StartNew("Install recover img ...")
	defer p.Stop()

	c := oakUtility.New_SSHClient(host)
	if err := c.Open("root", "oakridge"); err != nil {
		return err
	}
	defer c.Close()

	var cmds = [][]string{
		{"stop", "optional"},
		{"/etc/init.d/supervisor stop", "optional"},
		{"/etc/init.d/capwap stop", "optional"},
		{"/etc/init.d/handle_cloud stop", "optional"},
		{"/etc/init.d/wifidog stop", "optional"},
		{"/etc/init.d/arpwatch stop", "optional"},
	}
	for _, cmd := range cmds {
		buf, err := c.One_cmd(cmd[0])
		if err != nil {
			log.Debug.Printf("\n%v: %s <%s>\n", cmd, err.Error(), string(buf))
			if cmd[1] == "mandatory" {
				return err
			}
		}
	}

	if _, err := c.Scp(file, "/tmp/"+file, "0644"); err != nil {
		return err
	}
	log.Debug.Printf("done scp %s to %s:%s\n", file, host, "/tmp/"+file)
	if _, err := c.One_cmd("tar xzf /tmp/" + file + " -C /tmp"); err != nil {
		return err
	}
	log.Debug.Printf("done untar %s:%s\n", host, "/tmp/"+file)
	//last cmd expect return err
	log.Debug.Printf("sysupgrade -n /tmp/recover-ubnt-erx.tar")
	c.One_cmd("sysupgrade -n /tmp/recover-ubnt-erx.tar")
	return nil
}
func restore_ubnt_erx(host string) {

	log.Debug.Printf("Start restore %s\n", host)

	for _, v := range erx_imgs { // download resource
		if err := oakUtility.On_demand_download(v[0], v[1]); err != nil {
			log.Error.Println(err.Error())
			return
		}
	}

	if err := ubnt_recover_img(host, erx_imgs["recover"][0]); err != nil { // recover img and reboot
		log.Error.Println(err.Error())
		return
	}

	p := spinner.StartNew("Wait device bootup ...")

	pinger, err := ping.NewPinger(host)
	if err != nil {
		panic(err)
	}
	time.Sleep(20 * time.Second)
	pinger.SetStopAfter(30)
	pinger.OnRecv = func(pkt *ping.Packet) {
		fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v\n", pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
	}
	pinger.Run()
	p.Stop()

	p.SetTitle("Restoring factory img ...")
	p.Start()
	defer p.Stop()
	c := oakUtility.New_SSHClient(host) // ssh back to device again
	for {
		time.Sleep(2 * time.Second)
		err := c.Open("root", "oakridge")
		if err == nil {
			log.Debug.Printf("ssh connected to %s\n", host)
			break
		}
		log.Debug.Println(err.Error())
	}
	defer c.Close()

	for k, v := range erx_imgs { // scp other file to device
		if k == "recover" {
			continue
		}
		if _, err := c.Scp(v[0], "/tmp/"+v[0], "0644"); err != nil {
			println(err)
			return
		}
		log.Debug.Printf("done scp %s to %s:%s\n", v[0], host, "/tmp/"+v[0])
	}
	var cmds = []string{
		"ubidetach -m 5",
		"ubiformat /dev/mtd5",
		"ubiattach -p /dev/mtd5",
		"ubimkvol /dev/ubi0 --vol_id=0 --lebs=1925 --name=troot",
		"mount -o sync -t ubifs ubi0:troot /mnt",
		"mtd write /tmp/" + erx_imgs["vmlinux"][0] + " kernel1",
		"mtd write /tmp/" + erx_imgs["vmlinux"][0] + " kernel2",
		"cp /tmp/" + erx_imgs["version"][0] + " /mnt/version",
		"cp /tmp/" + erx_imgs["squash"][0] + " /mnt/squashfs.img",
		"cp /tmp/" + erx_imgs["squash_md5"][0] + " /mnt/squashfs.img.md5",
		"reboot",
	}
	for _, cmd := range cmds {
		log.Debug.Printf("%s ...\n", cmd)
		_, err := c.One_cmd(cmd)
		if err != nil {
			log.Error.Printf("\n%s: %s\n", cmd, err.Error())
			return
		}
	}
	fmt.Printf("\nDevice restored to factory image successfully\n")
}
func restore_one_device(t Target, s *sync.WaitGroup) {
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
		restore_unifi_ap152_ap(t)
	case UBNT_ERX, UBNT_ERX_OLD:
		restore_ubnt_erx(t.host)
	default:
		fmt.Printf("unsupport model %s\n", t.Model)
		return
	}
}

var ap_origin_imgs = map[string][]string{ //NOTE these 3 are use same img
	AC_LITE:   {"aclite.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/origin/AC-LITE/firmware.bin.tar.gz"},
	AC_LR:     {"aclr.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/origin/AC-LR/firmware.bin.tar.gz"},
	AC_PRO:    {"acpro.tar.gz", "http://image.oakridge.vip:8000/images/ap/ubntunifi/origin/AC-PRO/firmware.bin.tar.gz"},
	A923:      {"a923.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/origin/A923/firmware.bin.tar.gz"},
	A820:      {"a820.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/origin/A820/firmware.bin.tar.gz"},
	A822:      {"a822.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/origin/A822/firmware.bin.tar.gz"},
	W282:      {"w282.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/origin/W282/firmware.bin.tar.gz"},
	A920:      {"a920.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/origin/A920/firmware.bin.tar.gz"},
	WL8200_I2: {"wl8200_i2.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/origin/WL8200-I2/firmware.bin.tar.gz"},
}

func restore_unifi_ap152_ap(t Target) {

	localfile := ap_origin_imgs[t.Model][0]
	url := ap_origin_imgs[t.Model][1]

	if err := oakUtility.On_demand_download(localfile, url); err != nil {
		log.Error.Println(err.Error())
		return
	}

	p := spinner.StartNew("Restore " + t.host + " " + t.Model + " ...")
	defer p.Stop()

	c := oakUtility.New_SSHClient(t.host)
	if err := c.Open("root", "oakridge"); err != nil {
		println(err)
		return
	}
	defer c.Close()

	var cmds = [][]string{
		{"stop", "optional"},
		{"/etc/init.d/supervisor stop", "optional"},
		{"/etc/init.d/capwap stop", "optional"},
		{"/etc/init.d/handle_cloud stop", "optional"},
		{"/etc/init.d/wifidog stop", "optional"},
		{"/etc/init.d/arpwatch stop", "optional"},
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

	remotefile := "/tmp/oak.tar.gz"
	_, err := c.Scp(localfile, remotefile, "0644")
	if err != nil {
		println(err)
		return
	}

	fmt.Printf("\nWrite flash, MUST NOT POWER OFF, it might take several minutes!\n")

	cmds = [][]string{
		{"stop", "optional"},
		{"/etc/init.d/capwap stop", "optional"},
		{"/etc/init.d/handle_cloud stop", "optional"},
		{"/etc/init.d/wifidog stop", "optional"},
		{"/etc/init.d/arpwatch stop", "optional"},
		{"tar xzf " + remotefile + " -C /tmp", "mandatory"},
		{"rm -rvf " + remotefile, "mandatory"},
		{"mtd write /tmp/firmware.bin firmware", "mandatory"},
		{"reboot", "mandatory"},
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
	fmt.Printf("\n%s restored to factory image, please power cycle device\n", t.host)
}
func choose_restore_firmwire() {
	// targets is put together in list_scan_result
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
		for i, d := range targets {
			fmt.Printf("[%d]. %s %s %s %s\n", i+1, d.host, d.mac, d.Name, d.SWver)
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
			go restore_one_device(t, &s)
		}
		s.Wait()
	} else {
		restore_one_device(targets[choice-1], nil)
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
	log.Set_level("debug")
}

func main() {

	println(Banner_start)

	if len(os.Args) > 1 {
		scan_input_subnet(os.Args[1:])
	} else {
		scan_local_subnet()
	}

	list_scan_result()

	choose_restore_firmwire()

	//list_oakdev_csv ()

	println(Banner_end)
}
