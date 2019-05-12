#
# Prepare a virtual machine for the integration tests. We will install the needed software and then
# bring up a cluster using kind
#
# We expect that the following environment variables are set:
# TRAVIS_HOME      - home directory of the travis user

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
# Install kind
#
wget https://github.com/kubernetes-sigs/kind/releases/download/0.2.1/kind-linux-amd64
chmod +x kind-linux-amd64 && sudo mv kind-linux-amd64 /usr/local/bin/kind


#
# Create cluster and install helm
#
kind create cluster
export KUBECONFIG=$(kind get kubeconfig-path --name="kind")
kubectl create serviceaccount tiller -n kube-system
kubectl create clusterrolebinding tiller --clusterrole=cluster-admin  --serviceaccount=kube-system:tiller
helm init --service-account tiller

#
# Wait for tiller container to come up
#
status="ContainerCreating"
while [ "$status" != "Running" ]; do
  sleep 5
  status=$(kubectl get pods -n kube-system |  grep "tiller" | awk '{ print $3 '})
  echo "Current status of Tiller pod : $status"
done

#
# Install go client library. We need this as we need to build our integration tests
#
go get k8s.io/client-go/...

#
# Fetch the source code of the controller. For that to work, we need to make sure
# that we have the version of the integration tests from the commit that triggered
# this build. Therefore we need to get this from the Chart.yaml file first
#
tag=$(cat Chart.yaml | grep "appVersion:" | awk {' print $2 '})
echo "Using tag $tag"
cd $TRAVIS_HOME
git clone https://github.com/christianb93/bitcoin-controller
git checkout $tag
