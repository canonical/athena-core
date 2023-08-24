[![athena-monitor docker image](https://github.com/canonical/athena-core/pkgs/container/athena-core%2Fathena-monitor/status "Docker Repository on Github")](https://github.com/canonical/athena-core/pkgs/container/athena-core%2Fathena-monitor)
[![athena-processor docker image](https://github.com/canonical/athena-core/pkgs/container/athena-core%2Fathena-processor/status "Docker Repository on Github")](https://github.com/canonical/athena-core/pkgs/container/athena-core%2Fathena-processor)
![Build workflows](https://github.com/project-athena/athena-core/workflows/Test/badge.svg)

# Athena

Athena is a file processor service, that consumes files stored in
the files.com API and runs a series of reports over the downloaded artifacts
and subsequently it talks with the Salesforce API for performing actions (currently,
only comments are supported)

## Basics

There are 3 software components in athena:

1. *Athena-monitor*: Monitor changes in several directories across a file.com
   account and if new files are found those are sent to the processor for
   background processing.

2. *Nats*: Nats is a light messaging daemon that allows a pubsub system to be
   implemented on top, its used to dispatch messages from *athena-monitor* to a
   *athena-processor*

3. *Athena-processor*: Subscribes to messages from monitor and routes the
   reports that have to be run over a given detected file, subsequently it will
   perform an action on salesforce (such as posting a comment, etc)

The basic flowchart of interaction is as follows

[![](https://mermaid.ink/img/eyJjb2RlIjoiZ3JhcGggVERcbiAgICBBW0F0aGVuYSBNb25pdG9yXSAtLT58RmV0Y2ggRmlsZXN8IEIoRmlsZXMuY29tIEFQSSlcbiAgICBCIC0tPiBDe05ldyBmaWxlcyB0byBwcm9jZXNzP31cbiAgICBDIC0tPnxZZXN8IEQoTmF0cyBNZXNzYWdlKVxuICAgIEMgLS0-fE5vfCBBXG4gICAgRCAtLT58RmlsZXBhdGh8RVtBdGhlbmEgUHJvY2Vzc29yXVxuICAgIEVbQXRoZW5hIFByb2Nlc3Nvcl0tLT4gRntQb3N0IGNvbW1lbnQgb24gY2FzZT99XG4gICAgRiAtLT58WWVzfCBHKFNhbGVzZm9yY2UgQVBJKVxuICAgIEYgLS0-fE5vfEFcbiIsIm1lcm1haWQiOnsidGhlbWUiOiJkZWZhdWx0In0sInVwZGF0ZUVkaXRvciI6ZmFsc2V9)](https://mermaid-js.github.io/mermaid-live-editor/#/edit/eyJjb2RlIjoiZ3JhcGggVERcbiAgICBBW0F0aGVuYSBNb25pdG9yXSAtLT58RmV0Y2ggRmlsZXN8IEIoRmlsZXMuY29tIEFQSSlcbiAgICBCIC0tPiBDe05ldyBmaWxlcyB0byBwcm9jZXNzP31cbiAgICBDIC0tPnxZZXN8IEQoTmF0cyBNZXNzYWdlKVxuICAgIEMgLS0-fE5vfCBBXG4gICAgRCAtLT58RmlsZXBhdGh8RVtBdGhlbmEgUHJvY2Vzc29yXVxuICAgIEVbQXRoZW5hIFByb2Nlc3Nvcl0tLT4gRntQb3N0IGNvbW1lbnQgb24gY2FzZT99XG4gICAgRiAtLT58WWVzfCBHKFNhbGVzZm9yY2UgQVBJKVxuICAgIEYgLS0-fE5vfEFcbiIsIm1lcm1haWQiOnsidGhlbWUiOiJkZWZhdWx0In0sInVwZGF0ZUVkaXRvciI6ZmFsc2V9)

## Hacking

Everything needed to stand up a development environment is contained under a
makefile, docker, docker-compose and golang >= 1.14 are required.

Clone this repository and run the following command to build:

```sh
make common-build
```

For running a docker based installation locally

```shell
make devel
```
