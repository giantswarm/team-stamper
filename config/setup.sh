#!/bin/bash

# Make sure we run on KinD
kind get kubeconfig > /tmp/kind.kubeconfig

export KUBECONFIG=/tmp/kind.kubeconfig

# Local development follows different rules
# hence we need to know that
local=0
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -l|--local) local=1 ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
    shift
done

# Taken from Makefile.gen.go.mk
REGISTRY="gsoci.azurecr.io/giantswarm"
REPOSITORY=$(go list -m | cut -d '/' -f 3)
TAG=$(architect project version)

# If local development build the image first and
# then load it into KinD
if [[ $local -eq 1 ]];
then
    make build-docker
    docker image tag ${REPOSITORY}:${TAG} ${REGISTRY}/${REPOSITORY}:${TAG}
    kind load docker-image ${REGISTRY}/${REPOSITORY}:${TAG}
fi



# Install latest Flux app
helm repo add giantswarm https://giantswarm.github.io/giantswarm-catalog
helm repo update
helm install --wait flux giantswarm/flux-app --set podMonitors.enabled=false

# Install Team Stamper
helm install --wait stamper ./helm/team-stamper --set image.tag=$TAG
