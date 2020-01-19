#!/bin/bash
eval $(minikube docker-env)
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
echo "Script directory is $DIR"
cd $DIR/../../cmd/controller
CGO_ENABLED=0 go build
docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .
