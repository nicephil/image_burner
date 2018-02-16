package main

import (
	"fmt"
        //"time"
        //"net"
	"log"
	"os"
	"bytes"
	"golang.org/x/crypto/ssh"
)

func main() {
    if len(os.Args) != 2 {
        log.Fatalf("Usage: %s <a.b.c.d>", os.Args[0])
    }
/*
        net := os.Args[1]
        for i:=1; i<=254; i++ {
            worker (net + strconv.Itoa(i))
        }
*/
    worker (os.Args[1])
}

func worker (host string) {
    worker1 (host)
}

func worker1 (host string) {
    var dst bytes.Buffer
    sshConfig := &ssh.ClientConfig{
        User: "root",
        Auth: []ssh.AuthMethod{ssh.Password("oakridge")},
            HostKeyCallback: ssh.InsecureIgnoreHostKey(),
    }

    dst.WriteString(host)
    dst.WriteString(":22")
    fmt.Println("connecting ", dst.String())
    client, err := ssh.Dial("tcp", dst.String(), sshConfig)

    if err != nil {
        fmt.Println ("Dial:", err)
        return
    }
    defer client.Close()

    session, err := client.NewSession()
    if err != nil {
        fmt.Println ("NewSession:", err)
        return
    }

    out, err := session.CombinedOutput("cat /etc/issue")
    if err != nil {
        fmt.Println ("CombinedOutput:", err)
        return
    }
    fmt.Println(string(out))
    fmt.Println("Finished")
}
