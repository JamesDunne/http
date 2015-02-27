#!/bin/bash

# To bootstrap cross-compilation:
# $ cd $GOROOT/src
# $ GOOS=darwin GOARCH=amd64 ./make.bash --no-clean
# ...

GOOS=darwin GOARCH=amd64 go build -o http-cli.darwin.x64.bin ./cli
GOOS=darwin GOARCH=386 go build -o http-cli.darwin.x86.bin ./cli
GOOS=windows GOARCH=amd64 go build -o http-cli.x64.exe ./cli
GOOS=windows GOARCH=386 go build -o http-cli.x86.exe ./cli
GOOS=linux GOARCH=amd64 go build -o http-cli.linux.x64.bin ./cli
GOOS=linux GOARCH=386 go build -o http-cli.linux.x86.bin ./cli
