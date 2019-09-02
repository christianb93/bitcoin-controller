#!/bin/bash
#
# Prepare a virtual machine for the integration tests. We will install the needed software and then
# bring up a cluster using kind
#
# We expect that the following environment variables are set:
# TRAVIS_HOME         - home directory of the travis user
# TRAVIS_BUILD_DIR    - directory into which Travis clones the bitcoin-controller-helm-qa repo

set -e

#
#  Install helm
#
sudo snap install helm --classic

#
# Install kubectl
#
curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl
chmod +x ./kubectl && sudo mv ./kubectl /usr/local/bin/kubectl

#
# Create cache directory if it does not exist yet
#
mkdir -p $TRAVIS_HOME/cache

#
# Install kind. We first check whether it exists in the cache, if
# not we download it and add it to the cache
#
echo "Installing kind"
date
if [ -f "$TRAVIS_HOME/cache/kind-linux-amd64" ]; then
  echo "Retrieving kind binary from cache"
  cp $TRAVIS_HOME/cache/kind-linux-amd64 .
else
  echo "Need to get kind binary from Github"
  wget https://github.com/kubernetes-sigs/kind/releases/download/0.2.1/kind-linux-amd64
  cp kind-linux-amd64 $TRAVIS_HOME/cache/kind-linux-amd64
fi
chmod +x kind-linux-amd64 && sudo mv kind-linux-amd64 /usr/local/bin/kind

#
# If there is an image for kind in the cache directory, load it
#
if [ -f "$TRAVIS_HOME/cache/kind_node_image.tar" ]; then
  echo "Retrieving kind node image from cache"
  docker load --input $TRAVIS_HOME/cache/kind_node_image.tar
else
  echo "No cached version of kind node image found"
fi

#
# Create cluster and install helm
#
echo "Bringing up test cluster"
date
kind create cluster
export KUBECONFIG=$(kind get kubeconfig-path --name="kind")
echo "Installing helm in test cluster"
date
kubectl create serviceaccount tiller -n kube-system
kubectl create clusterrolebinding tiller --clusterrole=cluster-admin  --serviceaccount=kube-system:tiller
helm init --service-account tiller

#
# Wait for tiller container to come up
#
echo "Waiting for Tiller pod to come up"
date
sleep 5
status="ContainerCreating"
while [ "$status" != "Running" ]; do
  status=$(kubectl get pods -n kube-system |  grep "tiller" | awk '{ print $3 '})
  echo "Current status of Tiller pod : $status"
  sleep 5
done


#
# Fetch the source code of the controller. For that to work, we need to make sure
# that we have the version of the integration tests from the commit that triggered
# this build. Therefore we need to get this from the Chart.yaml file first
#
tag=$(cat Chart.yaml | grep "appVersion:" | awk {' print $2 '})
echo "Fetching source code, using tag $tag"
date
cd $GOPATH/src/github.com/christianb93
git clone https://github.com/christianb93/bitcoin-controller
cd bitcoin-controller
git checkout $tag
date

#
# Install go client library. We need this as we need to build our integration tests
#
echo "Running go get"
go get -d -t ./...
# Make sure that we have version 0.4.0 of klog - this is a workaround for a broken client-go build with later versions
(cd $GOPATH/src/k8s.io/klog && git checkout v0.4.0)
date
