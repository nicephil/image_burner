package main

import (
    "fmt"
    "strings"
    "image_burner/util"
    "os"
)

/*
 * global vars
*/
var netlist []Subnet
var Banner string
var log oakUtility.OakLogger
var img_url = map[string][]string {
    "UBNT": {"oakridge.firmwire.ubnt.tar.gz",  "http://image.oakridge.vip:8000/images/ap/ubntunifi/sysloader/latest-sysupgrade.bin.tar.gz"},
    "QTS":  {"oakridge.firmwire.ap152.tar.gz", "http://image.oakridge.vip:8000/images/ap/ap152/sysloader/latest-sysupgrade.bin.tar.gz"},
}


func list_all_dev () {
    var oak []*oakUtility.Oakridge_Device
    for _,n:=range netlist {
        for _,d:=range n.Oak_dev_list {
            oak = append (oak, d)
        }
    }
    oakUtility.Oakdev_PrintHeader ()
    for i,d:=range oak {
        fmt.Printf("%-3d %s\n", i+1, d.OneLineSummary())
    }
}

func init () {
    Banner="\nOakridge Firmware Update Utility, Ver 1.01, (c) Oakridge Networks, Inc. 2018\n"
    log = oakUtility.New_OakLogger()
    log.Set_level ("info")
}

func main() {
    var dummy string

    println(Banner)

    scan_local_subnet ()

    list_all_dev ()

    fmt.Printf ("\nDownload latest firmware for UBNT HW?(Y/N):")
    fmt.Scanf("%s\n", &dummy)

    if strings.Compare(strings.ToUpper(dummy), "Y") == 0 {
        local := img_url["UBNT"][0]
        url := img_url["UBNT"][1]
        if err := oakUtility.DownloadFile (local, url, true); err != nil {
            println ("Download fail:", err)
            os.Exit (1)
        }
        println("Debug: Download saved as:", local)
    }
}
