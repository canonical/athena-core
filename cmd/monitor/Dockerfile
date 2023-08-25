FROM ubuntu:20.04
LABEL maintainer="Canonical Sustaining Engineering <edward.hope-morley@canonical.com>"

# Reference: https://canonical-ubuntu-pro-client.readthedocs-hosted.com/en/latest/howtoguides/enable_in_dockerfile.html
RUN --mount=type=secret,id=pro_attach_config \
    # First we update apt so we install the correct versions of packages in
    # the next step
    apt-get update \
    #
    # Here we install `pro` (ubuntu-advantage-tools) as well as ca-certificates,
    # which is required to talk to the Ubuntu Pro authentication server securely.
    && apt-get install --no-install-recommends -y ubuntu-advantage-tools ca-certificates \
    #
    # With pro installed, we attach using our attach config file from the
    # previous step
    && (if grep -q ^token /run/secrets/pro_attach_config; then pro attach --attach-config /run/secrets/pro_attach_config; fi) \
    #
    ###########################################################################
    # At this point, the container has access to all Ubuntu Pro services
    # specified in the attach config file.
    ###########################################################################
    #
    # Always upgrade all packages to the latest available version with the Ubuntu Pro
    # services enabled.
    && apt-get upgrade -y \
    #
    # Then, you can install any specific packages you need for your docker
    # container.
    # Install them here, while Ubuntu Pro is enabled, so that you get the appropriate
    # versions.
    # Any `apt-get install ...` commands you have in an existing Dockerfile
    # that you may be migrating to use Ubuntu Pro should probably be moved here.
    && apt-get install -y ca-certificates git xz-utils python3 python3-yaml coreutils bsdmainutils jq bc python3-simplejson \
    #
    ###########################################################################
    # Now that we have upgraded and installed any packages from the Ubuntu Pro
    # services, we can clean up.
    ###########################################################################
    #
    # This purges ubuntu-advantage-tools, including all Ubuntu Pro related
    # secrets from the system.
    ###########################################################################
    # IMPORTANT: As written here, this command assumes your container does not
    # need ca-certificates so it is purged as well.
    # If your container needs ca-certificates, then do not purge it from the
    # system here.
    ###########################################################################
    && apt-get purge --auto-remove -y ubuntu-advantage-tools \
    #
    # Finally, we clean up the apt lists which should not be needed anymore
    # because any `apt-get install`s should have happened above. Cleaning these
    # lists keeps your image smaller.
    && rm -rf /var/lib/apt/lists/*

RUN update-ca-certificates
RUN mkdir /etc/athena/

COPY ./athena-monitor /athena-monitor
