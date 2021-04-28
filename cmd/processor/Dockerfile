FROM ubuntu:20.04
LABEL maintainer="Jorge Niedbalski <jnr@metaklass.org>"

RUN apt update -yyq && apt -yyq install ca-certificates git xz-utils python3 python3-yaml coreutils bsdmainutils jq bc python3-simplejson
RUN update-ca-certificates
RUN mkdir /etc/athena/

COPY ./athena-processor /athena-processor
