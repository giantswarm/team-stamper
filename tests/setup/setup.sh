#!/bin/bash

kind get clusters

helm --version
kubectl --help

#kind get kubeconfig > /tmp/kind.kubeconfig

#export KUBECONFIG=/tmp/kind.kubeconfig

# Install Flux app
#helm repo add giantswarm https://giantswarm.github.io/giantswarm-catalog
#helm repo update
#helm install --wait giantswarm flux --version 1.8.1
