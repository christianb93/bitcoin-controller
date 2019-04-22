#!/bin/bash
cd $GOPATH/src/github.com/christianb93/bitcoin-controller/cmd/controller
CGO_ENABLED=0 go build
# This assumes that you have done a docker login
docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .
docker push christianb93/bitcoin-controller:latest
