# A bitcoin controller for Kubernetes

## What is it

This repository contains a simple bitcoin controller designed to run on top of Kubernetes which I created while working on a series of posts on Kubernetes controllers on [my blog](https://leftasexercise.com). You specify a bitcoin cluster in a YAML manifest file, and the controller will

* Start the specified number of bitcoin nodes as Kubernetes pods
* Monitor your pods and make sure that they are replaced if needed
* Allow you to dynamically scale up and down
* Talk to the individual bitcoin nodes using JSON RPC to make sure that they connect to each other
* Create events and publish status information

**WARNING** -  this controller is not meant to be used for production environments! It uses ephemeral storage so that your entire data, including your keys and wallets, will be lost if a node goes down, and the credentials to access the bitcoin node are stored in a Kubernetes secret, so that everyone with a certain set of privileges in the cluster can retrieve them! This controller is meant to be a proof-of-concept and will bring up a bitcoin regtest network only which is only visible within the cluster.

## Running the controller

To run the controller on an existing Kubernetes cluster, you can use the image from my Docker Hub repository. Assuming that you have a working kubectl pointing to your cluster, here are the steps needed.

First, you need to create the CRD that defines a bitcoin network. To do this, run

``
kubectl apply -f https://raw.githubusercontent.com/christianb93/bitcoin-controller/master/deployments/crd.yaml
``

Next, you will have to create a cluster role and a service account for the bitcoin controller. This will also create a new namespace `bitcoin-controller` in which the bitcoin controller itself is running (though you can of course use one controller in your cluster to control an arbitrary number of networks in every namespace).

``
kubectl apply -f https://raw.githubusercontent.com/christianb93/bitcoin-controller/master/deployments/rbac.yaml
``


To be able to access the bitcoin daemon running on the individual nodes, credentials are required that are stored in a secret. The following command creates a secret called `bitcoin-default-secret` in the default namespace that contains default credentials which of course you are strongly advised to change. Note that you will have to create credentials for every namespace in which you want to run a bitcoin network.

``
kubectl apply -f https://raw.githubusercontent.com/christianb93/bitcoin-controller/master/deployments/secret.yaml
``

Finally, let us start the controller.

``
kubectl apply -f https://raw.githubusercontent.com/christianb93/bitcoin-controller/master/deployments/controller.yaml
``

After some time, `kubectl get pods -n bitcoin-controller` should show you that the controller is up and running. Now you can bring up a bitcoin network. The following command will create a CRD instance which will instruct the controller to bring up a test network with three nodes.

```
kubectl apply -f - << EOF
apiVersion: bitcoincontroller.christianb93.github.com/v1
kind: BitcoinNetwork
metadata:
  name: my-network
spec:
  nodes: 3
  secret: bitcoin-default-secret
EOF
```

Note that this definition refers to the secret created earlier. If you leave the secret field empty, default credentials will be used. This should bring up three pods, each running a bitcoin daemon as part of a stateful set. To verify that the bitcoin daemons are connected to each other, we can use the bitcoin CLI which is installed in each node.

``
kubectl exec my-network-sts-0 -- /usr/local/bin/bitcoin-cli -conf=/bitcoin.conf -regtest getpeerinfo
``

When you inspect the bitcoin network that we have created using, you will also see that the status subresource of the bitcoin network has been populated with a list of the nodes running in the network and that events have been logged by the controller.

``
kubectl describe bitcoinnetwork my-network
``
