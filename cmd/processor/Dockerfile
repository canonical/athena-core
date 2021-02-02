FROM ubuntu:20.04
LABEL maintainer="Jorge Niedbalski <jnr@metaklass.org>"

RUN apt update -yyq && apt -yyq install ca-certificates git xz-utils python3
RUN update-ca-certificates
RUN mkdir /etc/athena/

COPY ./athena-processor /athena-processor
