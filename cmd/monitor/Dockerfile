FROM ubuntu:22.04
LABEL maintainer="Canonical Sustaining Engineering <edward.hope-morley@canonical.com>"

RUN apt update
RUN apt --yes install \
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

COPY ./athena-monitor /athena-monitor
