#!/bin/bash
kubectl delete pod bitcoin-controller
kubectl run bitcoin-controller --image=christianb93/bitcoin-controller --image-pull-policy=Never --restart=Never
