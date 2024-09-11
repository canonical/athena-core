FROM ubuntu:24.04
LABEL maintainer="Canonical Sustaining Engineering <edward.hope-morley@canonical.com>"
LABEL org.opencontainers.image.description "Athena Monitor"

RUN apt-get update
RUN DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends apt-utils
RUN DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends \
    bc \
    bsdmainutils \
    ca-certificates \
    coreutils \
    git \
    jq \
    python3 \
    python3-simplejson \
    python3-yaml \
    xz-utils
RUN update-ca-certificates
RUN mkdir /etc/athena/

COPY ./build/athena-monitor /athena-monitor
RUN chmod 755 /athena-monitor
