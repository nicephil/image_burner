package main

import (
    "fmt"
    "image_burner/util"
)

/*
 * global vars
*/
var netlist []Subnet
var Banner string
var log oakUtility.OakLogger


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
    println ("\nScan finished successfully\n")
    list_all_dev ()
    fmt.Println ("\npress ENTER to quit")
    fmt.Scanf("%s\n", &dummy)
}
