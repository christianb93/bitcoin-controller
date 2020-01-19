#
# Run all integration tests, including setup and tear down. 
# We assume that you have kind  kubectl installed locally. We also need a local
# docker daemon
#


DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
rootDir=$DIR/../
echo "Script directory is $DIR, rootDir is $rootDir"
startTime=$(date)
echo "Starting test run at $startTime"
#
# First spin up a bitcoin daemon locally
#
bitcoind=$(docker run --rm -p 18332:18332 -d christianb93/bitcoind:v1.0)
echo "Started dockerized bitcoin daemon $bitcoind"
date
#
# Now start a new cluster
#
kind create cluster --name=bitcoind-test --image=kindest/node:v1.17.0
date
#
# Install CRD, secrets and RBAC profile
#
kubectl --context kind-bitcoind-test apply -f $rootDir/deployments/crd.yaml
kubectl --context kind-bitcoind-test apply -f $rootDir/deployments/rbac.yaml
kubectl --context kind-bitcoind-test apply -f $rootDir/deployments/secret.yaml

#
# Build a new docker image and run it
#
(cd $rootDir/cmd/controller ; CGO_ENABLED=0 go build)
# Build for kind. We build the image locally and use kind load to load it
(cd $rootDir/cmd/controller ;  docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .)
kind load docker-image christianb93/bitcoin-controller:latest --name=bitcoind-test
kubectl --context kind-bitcoind-test run bitcoin-controller --image=christianb93/bitcoin-controller --image-pull-policy=Never --restart=Never --serviceaccount='bitcoin-controller-sva' -n bitcoin-controller
sleep 5
#
# Pre-pull bitcoind image to avoid timeouts during tests later
#
echo "Pre-pulling bitcoind image"
docker pull christianb93/bitcoind:v1.0
kind load docker-image christianb93/bitcoind:v1.0 --name=bitcoind-test

date
#
# Run all integration tests - make sure not to use caches by adding -count=1
#
currentContext=$(kubectl  config current-context)
echo "Current context is $currentContext - switching to kind-bitcoind-test"
kubectl  config use-context kind-bitcoind-test
(cd $rootDir ;   go test ./... -run 'Integration' -count=1 -v )
echo "Switching kubectl context back to $currentContext"
kubectl config use-context $currentContext
#
# Delete  cluster again
#
echo "Deleting cluster again"
kind delete cluster --name=bitcoind-test
date


#
# Kill daemon again
#
docker kill $bitcoind
echo "Bitcoind stopped"
stopTime=$(date)
echo Completed test run at $stopTime
