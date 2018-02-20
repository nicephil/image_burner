#!/bin/bash

# cross compile platform list
# https://www.digitalocean.com/community/tutorials/how-to-build-go-executables-for-multiple-platforms-on-ubuntu-16-04

target="mac"
[ $# -eq 1 ] && target="$1"

case $target in
    "linux" )
        env GOOS=linux GOARCH=amd64 go build
        ;;
    "mac")
        go build
        ;;
    *)
        echo "not support platform $target yet"
        ;;
esac
