package oakUtility

import (
    "log"
    "strings"
    "io/ioutil"
    "os"
    "fmt"
)


type OakLogger struct {
    Debug     *log.Logger
    Info    *log.Logger
    Warn    *log.Logger
    Error     *log.Logger
}

func New_OakLogger () OakLogger {
    return OakLogger {
        Debug: log.New(ioutil.Discard, "", 0),
        Info:  log.New(ioutil.Discard, "", 0),
        Warn:  log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile),
        Error: log.New(os.Stderr, "ERROR: ",   log.Ldate|log.Ltime|log.Lshortfile),
    }
}

func (l *OakLogger) Set_level (level string) {
    switch strings.ToLower (level) {
    case "debug":
        l.Debug = log.New(os.Stderr, "DEBUG: ",   log.Ldate|log.Ltime|log.Lshortfile)
        l.Info  = log.New(os.Stdout, "INFO: ",    log.Ldate|log.Ltime|log.Lshortfile)
        l.Warn  = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
        l.Error = log.New(os.Stderr, "ERROR: ",   log.Ldate|log.Ltime|log.Lshortfile)
    case "Info":
        l.Debug = log.New(ioutil.Discard, "DEBUG", log.Ldate|log.Ltime|log.Lshortfile)
        l.Info  = log.New(os.Stdout, "INFO: ",    log.Ldate|log.Ltime|log.Lshortfile)
        l.Warn  = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
        l.Error = log.New(os.Stderr, "ERROR: ",   log.Ldate|log.Ltime|log.Lshortfile)
    case "warning":
        l.Debug = log.New(ioutil.Discard, "", 0)
        l.Info  = log.New(ioutil.Discard, "", 0)
        l.Warn  = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
        l.Error = log.New(os.Stderr, "ERROR: ",   log.Ldate|log.Ltime|log.Lshortfile)
    case "error":
        l.Debug = log.New(ioutil.Discard, "", 0)
        l.Info  = log.New(ioutil.Discard, "", 0)
        l.Warn  = log.New(ioutil.Discard, "", 0)
        l.Error = log.New(os.Stderr, "ERROR: ",   log.Ldate|log.Ltime|log.Lshortfile)
    }
}

func ClearLine() {
    fmt.Printf("\033[2K")
    fmt.Println()
    fmt.Printf("\033[1A")
}
