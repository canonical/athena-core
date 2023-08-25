# Setup
```
sudo apt install golang make
sudo snap install docker
```

## Build Athena

```
$ make common-build
>> building binaries
GO111MODULE=on /home/ubuntu/go/bin/promu build --prefix /home/ubuntu/git/canonical/athena-core 
 >   athena-processor
 >   athena-monitor
$ mv athena-monitor cmd/monitor/
$ mv athena-processor cmd/processor/
```

## Build Images

Get docker network info and ensure that your firewall allows it:

```
# docker network ls
NETWORK ID     NAME      DRIVER    SCOPE
d257092905a7   bridge    bridge    local
a1e33a499c70   host      host      local
2c959b426f97   none      null      local
# docker network inspect d257092905a7
...
```

If a build fails, look in dmesg to see if anything got blocked and update firewall if needed.

See cmd/monitor/Dockerfile and cmd/processor/Dockerfile for image config.

Build monitor:

```
# docker build --file cmd/monitor/Dockerfile .
Sending build context to Docker daemon   17.3MB
Step 1/6 : FROM ubuntu:20.04
20.04: Pulling from library/ubuntu
846c0b181fff: Downloading [=======================>                           ]  13.25MB/28.58MB846c0b181fff: Pull complete 
Digest: sha256:0e0402cd13f68137edb0266e1d2c682f217814420f2d43d300ed8f65479b14fb
Status: Downloaded newer image for ubuntu:20.04
 ---> d5447fc01ae6
Step 2/6 : LABEL maintainer="Canonical Sustaining Engineering <edward.hope-morley@canonical.com>"
 ---> Running in fd6940b8b5d0
Removing intermediate container fd6940b8b5d0
 ---> 55d857651b9d
Step 3/6 : RUN apt update -yyq && apt -yyq install ca-certificates git xz-utils python3 python3-yaml coreutils bsdmainutils jq bc python3-simplejson
 ---> Running in e02cbee5e4f6
...
Successfully built 32ca0a3b8176
```

Build processor:

```
# docker build --file cmd/processor/Dockerfile .
Sending build context to Docker daemon   17.9MB
Step 1/6 : FROM ubuntu:20.04
 ---> d5447fc01ae6
Step 2/6 : LABEL maintainer="Canonical Sustaining Engineering <edward.hope-morley@canonical.com>"
 ---> Using cache
 ---> 55d857651b9d
Step 3/6 : RUN apt update -yyq && apt -yyq install ca-certificates git xz-utils python3 python3-yaml coreutils bsdmainutils jq bc python3-simplejson
 ---> Using cache
 ---> 8014d38bde4e
Step 4/6 : RUN update-ca-certificates
 ---> Using cache
 ---> 2718ebbbbb87
Step 5/6 : RUN mkdir /etc/athena/
 ---> Using cache
 ---> 135bf991e1b1
Step 6/6 : COPY ./athena-processor /athena-processor
 ---> d44deeb0cfb9
Successfully built d44deeb0cfb9
```

Tag/name images:

```
# docker image ls
REPOSITORY   TAG       IMAGE ID       CREATED              SIZE
<none>       <none>    96d6e2650a4e   30 seconds ago       268MB
<none>       <none>    7508809b8547   About a minute ago   267MB
ubuntu       20.04     d5447fc01ae6   4 weeks ago          72.8MB
# docker image tag 32ca0a3b8176 athena-monitor:latest
# docker image tag d44deeb0cfb9 athena-processor:latest
```

The following should return nothing if all build successfully:

```
# docker ps -a
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES
```

# Test Build

```
make devel
```
