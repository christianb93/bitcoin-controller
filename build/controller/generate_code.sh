#!/bin/bash
#
# First generate the deep copy routine
#
go run $GOPATH/src/k8s.io/code-generator/cmd/deepcopy-gen/main.go \
  --bounding-dirs github.com/christianb93/bitcoin-controller/internal/apis \
  --input-dirs github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1

#
# Generate clientset
#
go run $GOPATH/src/k8s.io/code-generator/cmd/client-gen/main.go \
  --input-base "github.com/christianb93/bitcoin-controller/internal/apis" \
  --input "bitcoincontroller/v1" \
  --output-package "github.com/christianb93/bitcoin-controller/internal/generated/clientset" \
  --clientset-name "versioned"
