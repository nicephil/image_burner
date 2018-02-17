package main

import (
	"log"
        "io/ioutil"
	"os"
)


// debug level
var (
    log_dbg     *log.Logger
    log_info    *log.Logger
    log_wrn     *log.Logger
    log_err     *log.Logger
)

func main() {

    init_log ()
    scan_local_subnet ()    // result stored in global dev_list
    dump_dev_list ()
}


func init_log() {
    log_dbg = log.New(ioutil.Discard, "DEBUG: ",    log.Ldate|log.Ltime|log.Lshortfile)
    //log_dbg = log.New(os.stderr, "DEBUG: ",    log.Ldate|log.Ltime|log.Lshortfile)
    log_info = log.New(os.Stdout, "INFO: ",         log.Ldate|log.Ltime|log.Lshortfile)
    log_wrn = log.New(os.Stdout, "WARNING: ",       log.Ldate|log.Ltime|log.Lshortfile)
    log_err = log.New(os.Stderr, "ERROR: ",         log.Ldate|log.Ltime|log.Lshortfile)
}

