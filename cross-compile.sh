#!/bin/bash

# To bootstrap cross-compilation:
# $ cd $GOROOT/src
# $ GOOS=darwin GOARCH=amd64 ./make.bash --no-clean
# ...

GOOS=windows GOARCH=amd64 go build -o http.x64.exe
GOOS=windows GOARCH=386 go build -o http.x86.exe
GOOS=darwin GOARCH=amd64 go build -o http.darwin.x64.bin
GOOS=darwin GOARCH=386 go build -o http.darwin.x86.bin
GOOS=linux GOARCH=amd64 go build -o http.linux.x64.bin
GOOS=linux GOARCH=386 go build -o http.linux.x86.bin
