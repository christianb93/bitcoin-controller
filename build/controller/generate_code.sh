#!/bin/bash
#
# First generate the deep copy routine
# TODO: currently this assumes that you have checked out the generator to the 
# GOPATH directory
# Note that the code generation was done with go 1.10 without Go modules enabled and using
# the commit ff26e7842f9d5224562ef78defed0785ae8395cf from k8s.io/code-generator, as of
# Fri Apr 19 11:09:55 2019 -0700 (merge of pull request #76796)
# so if you have problems running this again from scratch you might want to checkout 
# this version again and use GO111MODULES=off
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
