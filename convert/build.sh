#!/bin/bash

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
