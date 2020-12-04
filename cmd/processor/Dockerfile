FROM quay.io/prometheus/busybox:latest

LABEL maintainer="Jorge Niedbalski <jnr@metaklass.org>"

COPY ./athena-processor /bin/athena-processor

ENTRYPOINT ["/bin/athena-processor"]
