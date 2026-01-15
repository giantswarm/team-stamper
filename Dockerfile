FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm
WORKDIR /

ADD team-stamper manager

USER 65532:65532

ENTRYPOINT ["/manager"]
