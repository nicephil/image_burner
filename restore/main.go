package main

import (
    "image_burner/util"
)

var log oakUtility.OakLogger

func init () {
    log = oakUtility.New_OakLogger()
    log.Set_level ("info")
}

func main() {
    scan_local_subnet ()    // result stored in global dev_list
    dump_dev_list ()
}
