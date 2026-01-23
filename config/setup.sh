#!/bin/bash

kind get clusters

helm version
kubectl --help

kind get kubeconfig > /tmp/kind.kubeconfig

export KUBECONFIG=/tmp/kind.kubeconfig

echo $KUBECONFIG

ls -l ../helm/team-stamper

# Install Flux app
#helm repo add giantswarm https://giantswarm.github.io/giantswarm-catalog
#helm repo update
#helm install --wait flux giantswarm/flux --version 1.8.1

# Install Team Stamper
#helm install --wait stamper ../helm/team-stamper
