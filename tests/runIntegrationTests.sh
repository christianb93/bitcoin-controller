#
# Run all integration tests, including setup and tear down. This script either uses minikube or
# kind (which is the default), depending on the flag -d.
# We assume that you have kind or minikube and kubectl installed locally. We also need a local
# docker daemon
#

#
# Parse parameters
#
driver=kind
while getopts d:h option
do
  case "${option}"
    in
      d) driver=${OPTARG};;
      h) echo "Usage: ./runIntegrationTest.sh -d <driver> \n Driver can be minikube or kind"; exit;;
  esac
done

rootDir=$GOPATH/src/github.com/christianb93/bitcoin-controller
startTime=$(date)
echo "Starting test run at $startTime"
#
# First spin up a bitcoin daemon locally
#
bitcoind=$(docker run --rm -p 18332:18332 -d christianb93/bitcoind:latest)
echo "Started dockerized bitcoin daemon $bitcoind"
date
#
# Now start a new cluster
#
echo "Using driver $driver"
if [ $driver == "minikube" ]; then
  minikube -p bitcoind-test start
  kubeconfig=$HOME/.kube/config
else
  kind create cluster --name=bitcoind-test --image=kindest/node:v1.14.1
  kubeconfig=$(kind get kubeconfig-path --name=bitcoind-test)
fi
echo "Using kubeconfig file at $kubeconfig"
date
#
# Install CRD, secrets and RBAC profile
#
kubectl --kubeconfig="$kubeconfig" apply -f $rootDir/deployments/crd.yaml
kubectl --kubeconfig="$kubeconfig" apply -f $rootDir/deployments/rbac.yaml
kubectl --kubeconfig="$kubeconfig" apply -f $rootDir/deployments/secret.yaml

#
# Build a new docker image and run it
#
(cd $rootDir/cmd/controller ; CGO_ENABLED=0 go build)
if [ $driver == "minikube" ]; then
  # Build for minikube. We create a docker image directly in the docker daemon running in the minikube VM
  (cd $rootDir/cmd/controller ; eval $(minikube -p bitcoind-test docker-env); docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .)
else
  # Build for kind. We build the image locally and use kind load to load it
  (cd $rootDir/cmd/controller ;  docker build --rm -f ../../build/controller/Dockerfile -t christianb93/bitcoin-controller:latest .)
  kind load docker-image christianb93/bitcoin-controller:latest --name=bitcoind-test
fi
kubectl --kubeconfig="$kubeconfig" run bitcoin-controller --image=christianb93/bitcoin-controller --image-pull-policy=Never --restart=Never --serviceaccount='bitcoin-controller-sva' -n bitcoin-controller
sleep 5
#
# Pre-pull bitcoind image to avoid timeouts during tests later
#
echo "Pre-pulling bitcoind image"
if [ $driver == "minikube" ]; then
  # Build for minikube. We create a docker image directly in the docker daemon running in the minikube VM
  minikube -p bitcoind-test ssh 'docker pull christianb93/bitcoind:latest'
else
  docker pull christianb93/bitcoind:latest
  kind load docker-image christianb93/bitcoind:latest --name=bitcoind-test
fi

date
#
# Run all integration tests - make sure not to use caches by adding -count=1
# We also change the minikube profile in this shell so that the kubectl
# calls in the integration tests pick up the correct context
#
if [ $driver == "minikube" ]; then
  currentContext=$(kubectl --kubeconfig="$kubeconfig"  config current-context)
  echo "Current context is $currentContext - switching to bitcoind-test"
  kubectl --kubeconfig="$kubeconfig" config use-context bitcoind-test
fi
(cd $rootDir ;  KUBECONFIG=$kubeconfig go test ./... -run 'Integration' -count=1 -v )
if [ $driver == "minikube" ]; then
  echo "Switching kubectl context back to $currentContext"
  kubectl config use-context minikube
fi
#
# Delete  cluster again
#
echo "Deleting cluster again"
if [ $driver == "minikube" ]; then
  minikube -p bitcoind-test delete
else
  kind delete cluster --name=bitcoind-test
fi
date


#
# Kill daemon again
#
docker kill $bitcoind
echo "Bitcoind stopped"
stopTime=$(date)
echo Completed test run at $stopTime
