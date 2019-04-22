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

#
# Generate lister
#
go run $GOPATH/src/k8s.io/code-generator/cmd/lister-gen/main.go \
  --input-dirs  "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"\
  --output-package "github.com/christianb93/bitcoin-controller/internal/generated/listers"

#
# Generate informer
#
go run $GOPATH/src/k8s.io/code-generator/cmd/informer-gen/main.go \
  --input-dirs  "github.com/christianb93/bitcoin-controller/internal/apis/bitcoincontroller/v1"\
  --versioned-clientset-package "github.com/christianb93/bitcoin-controller/internal/generated/clientset/versioned"\
  --listers-package "github.com/christianb93/bitcoin-controller/internal/generated/listers"\
  --output-package "github.com/christianb93/bitcoin-controller/internal/generated/informers"
