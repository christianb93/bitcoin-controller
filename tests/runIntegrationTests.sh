#
# Run all integration tests, including setup and tear down
#
rootDir=$GOPATH/src/github.com/christianb93/bitcoin-controller
startTime=$(date)
echo "Starting test run at $startTime"
#
# First spin up a bitcoin daemon locally
#
bitcoind=$(docker run --rm -p 18332:18332 -d christianb93/bitcoind:latest)
echo "Started dockerized bitcoin daemon $bitcoind"
#
# Now start a new minikube environment
#
minikube -p bitcoind-test start
date
#
# Install CRD, secrets and RBAC profile
#
kubectl apply -f $rootDir/deployments/crd.yaml
kubectl apply -f $rootDir/deployments/rbac.yaml
kubectl apply -f $rootDir/deployments/secret.yaml
#
# Build a new docker image and run it
#
(cd $rootDir/cmd/controller ; CGO_ENABLED=0 go build)
(cd $rootDir/cmd/controller ; eval $(minikube -p bitcoind-test docker-env); docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .)
kubectl run bitcoin-controller --image=christianb93/bitcoin-controller --image-pull-policy=Never --restart=Never --serviceaccount='bitcoin-controller-sva' -n bitcoin-controller
sleep 5
#
# Pre-pull bitcoind image to avoid timeouts during tests later
#
echo "Pre-pulling bitcoind image"
minikube -p bitcoind-test ssh 'docker pull christianb93/bitcoind:latest'
date
#
# Run all integration tests - make sure not to use caches by adding -count=1
# We also change the minikube profile in this shell so that the kubectl
# calls in the integration tests pick up the correct context
#
currentContext=$(kubectl config current-context)
echo "Current context is $currentContext - switching to bitcoind-test"
kubectl config use-context bitcoind-test
(cd $rootDir ;  go test ./... -run "Integration" -count=1)
echo "Switching kubectl context back to $currentContext"
kubectl config use-context minikube
#
# Delete minikube cluster again
#
minikube -p bitcoind-test delete

#
# Kill daemon again
#
docker kill $bitcoind
echo "Bitcoind stopped"
stopTime=$(date)
echo "Completed test run at $stopTime"
