#!/bin/bash

# To bootstrap cross-compilation:
# $ cd $GOROOT/src
# $ GOOS=darwin GOARCH=amd64 ./make.bash --no-clean
# ...

GOOS=windows GOARCH=amd64 go build -o http.x64.exe ./cli
GOOS=windows GOARCH=386 go build -o http.x86.exe ./cli
GOOS=darwin GOARCH=amd64 go build -o http.darwin.x64.bin ./cli
GOOS=darwin GOARCH=386 go build -o http.darwin.x86.bin ./cli
GOOS=linux GOARCH=amd64 go build -o http.linux.x64.bin ./cli
GOOS=linux GOARCH=386 go build -o http.linux.x86.bin ./cli
