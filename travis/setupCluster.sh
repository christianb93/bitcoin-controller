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
