FROM quay.io/prometheus/busybox:latest

LABEL maintainer="Jorge Niedbalski <jnr@metaklass.org>"

COPY ./athena-monitor /bin/athena-monitor

ENTRYPOINT ["/bin/athena-monitor"]
