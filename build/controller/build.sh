#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
echo "Script directory is $DIR"
cd $DIR/../../cmd/controller
CGO_ENABLED=0 go build
# This assumes that you have done a docker login
docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .
docker push christianb93/bitcoin-controller:latest
