#!/bin/bash
eval $(minikube docker-env)
cd $GOPATH/src/github.com/christianb93/bitcoin-controller/cmd/controller
CGO_ENABLED=0 go build
docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .
