FROM ubuntu:24.04
LABEL maintainer="Canonical Sustaining Engineering <edward.hope-morley@canonical.com>"
LABEL org.opencontainers.image.description "Athena Processor"

RUN apt-get update
RUN DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends apt-utils
RUN DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends \
    bc \
    bsdmainutils \
    ca-certificates \
    coreutils \
    git \
    jq \
    patool \
    python3 \
    python3-pip \
    python3-simplejson \
    python3-yaml \
    xz-utils
RUN update-ca-certificates
RUN mkdir /etc/athena/

COPY ./athena-processor /athena-processor
