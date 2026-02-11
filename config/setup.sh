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

if [[ $CIRCLECI == true ]];
then
    ARCHITECT_LATEST="$(curl -fsSL -o /dev/null -w '%{url_effective}' https://github.com/giantswarm/architect/releases/latest)"
    ARCHITECT_LATEST="${ARCHITECT_LATEST##*/}"
    wget "https://github.com/giantswarm/architect/releases/download/${ARCHITECT_LATEST}/architect-${ARCHITECT_LATEST}-linux-amd64.tar.gz"
    tar -xzvf "architect-${ARCHITECT_LATEST}-linux-amd64.tar.gz" "architect-${ARCHITECT_LATEST}-linux-amd64/architect"
    mv "architect-${ARCHITECT_LATEST}-linux-amd64/architect" ./architect
    TAG=$(./architect project version)
    NEWTAG=$TAG
else
    # This is so that user can test not yet commited changes locally
    TAG=$(architect project version)
    NEWTAG=${TAG}-$(date +%s)
fi

# Taken from Makefile.gen.go.mk
REGISTRY="gsoci.azurecr.io/giantswarm"
REPOSITORY=$(go list -m | cut -d '/' -f 3)

# If local development build the image first and
# then load it into KinD
if [[ $local -eq 1 ]];
then
    make build-docker
    docker image tag "${REPOSITORY}:${TAG}" "${REGISTRY}/${REPOSITORY}:${NEWTAG}"
    kind load docker-image "${REGISTRY}/${REPOSITORY}:${NEWTAG}"
fi

# Install latest Flux app
helm repo add giantswarm https://giantswarm.github.io/giantswarm-catalog
helm repo update
helm upgrade --install --wait flux giantswarm/flux-app --set podMonitors.enabled=false

k create namespace giantswarm --dry-run=client -o yaml | kubectl apply -f -

# Install Team Stamper
helm upgrade --install --wait stamper ./helm/team-stamper --set image.tag="$NEWTAG"
