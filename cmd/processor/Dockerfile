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
    pipx \
    python3 \
    python3-simplejson \
    python3-yaml \
    xz-utils
RUN update-ca-certificates
RUN mkdir /etc/athena/

COPY ./build/athena-processor /athena-processor
RUN chmod 755 /athena-processor
