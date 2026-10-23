FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm
WORKDIR /

ARG TARGETARCH
COPY team-stamper-linux-${TARGETARCH} /manager

USER 65532:65532

ENTRYPOINT ["/manager"]
