#!/bin/bash

# cross compile platform list
# https://www.digitalocean.com/community/tutorials/how-to-build-go-executables-for-multiple-platforms-on-ubuntu-16-04

target="all"
[ $# -eq 1 ] && target="$1"

##
# we use our 1 level up directory name as binary name
#
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
    "all" )
        env GOOS=linux GOARCH=amd64 go build -o $BIN.linux
        env GOOS=darwin GOARCH=amd64 go build -o $BIN.mac
        env GOOS=windows GOARCH=386 go build -o $BIN.exe
        ;;
    *)
        echo "not support platform $target yet"
        ;;
esac
