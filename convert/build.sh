#!/bin/bash

# cross compile platform list
# https://www.digitalocean.com/community/tutorials/how-to-build-go-executables-for-multiple-platforms-on-ubuntu-16-04

target="n/a"
[ $# -eq 1 ] && target="$1"

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BIN=${CURDIR##*/}

case $target in
    "linux" )
        env GOOS=linux GOARCH=amd64 go build -o $BIN.linux
        ;;
    "mac")
        env GOOS=darwin GOARCH=amd64 go build -o $BIN.mac
        ;;
    "windows" )
        env GOOS=windows GOARCH=386 go build -o $BIN.exe
        ;;
    *)
        echo "not support platform $target yet"
        ;;
esac
