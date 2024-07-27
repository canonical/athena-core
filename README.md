# Athena

[![Athena Processor](https://img.shields.io/badge/Container_Image-Athena_Processor-blue)](https://github.com/canonical/athena-core/pkgs/container/athena-core%2Fathena-processor)
[![Athena Monitor](https://img.shields.io/badge/Container_Image-Athena_Monitor-blue)](https://github.com/canonical/athena-core/pkgs/container/athena-core%2Fathena-monitor)
[![Docker publish (ghcr.io)](https://github.com/canonical/athena-core/actions/workflows/ghcr-publish.yaml/badge.svg)](https://github.com/canonical/athena-core/actions/workflows/ghcr-publish.yaml)

Athena is a file processor service, that consumes files stored in the files.com
API and runs a series of reports over the downloaded artifacts and subsequently
it talks with the Salesforce API for performing actions (currently, only
comments are supported)

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

## Configuration

The monitor and the processor can be configured via `yaml` configuration files
by calling

```console
athena-monitor --config config.yaml [--config config2.yaml [...]]
athena-processor --config config.yaml [--config config2.yaml [...]]
```

where configuration files are loaded in the order they are given on the command
line and values in later configuration files superseed values read earlier.

The basic structure looks like

```yaml
monitor:
  files-delta: 1m
  poll-every: 10s
  base-tmpdir: "/tmp/athena"
  directories:
    - "/uploads"
  processor-map:
    - type: filename
      regex: ".*sosreport.*.tar.[xz|gz]+$"
      processor: sosreports
```

### Monitor Configuration

### Processor Configuration

## Hacking

In order to stand up a development environment, you will need

- `make`
- `docker`
- `docker-compose`
- `golang >= 1.19`

For running a docker based installation locally you will need a sandbox account
on Salesforce and a sandbox directory on files.com. Supply

1. A list of the corresponding credentials in `creds.yaml`,

      ```yaml
      db:
        dialect: mysql
        dsn: "athena:athena@tcp(db:3306)/athena?charset=utf8&parseTime=true"

      filescom:
        key : "***"
        endpoint: "https://..."

      salesforce:
        endpoint: "https://..."
        username: "***"
        password: "***"
        security-token: "***"
      ```

1. A list of directories to monitor in `athena-monitor-directories.yaml`,

      ```yaml
      monitor:
        directories:
          - "/sandbox/..."
          - "/sandbox/..."
      ```

3. A path for where the report uploads will go in
   `athena-processor-upload.yaml`,

      ```yaml
      processor:
        reports-upload-dir: "/sandbox/..."
      ```

And finally run

```shell
make devel
```

In case the `docker-build` step fails you can try to re-run the `make` command
without using the cache,

```shell
NOCACHE=1 make devel
```

The `devel` deployment includes a `debug` container which can be used to
inspect the database.

```shell
$ docker exec --interactive --tty debug bash
# mysql -h db -u athena -pathena athena
mysql> describe files;
+------------+---------------------+------+-----+---------+----------------+
| Field      | Type                | Null | Key | Default | Extra          |
+------------+---------------------+------+-----+---------+----------------+
| id         | bigint(20) unsigned | NO   | PRI | NULL    | auto_increment |
| created_at | datetime(3)         | YES  |     | NULL    |                |
| updated_at | datetime(3)         | YES  |     | NULL    |                |
| deleted_at | datetime(3)         | YES  | MUL | NULL    |                |
| created    | datetime(3)         | YES  |     | NULL    |                |
| dispatched | tinyint(1)          | YES  |     | 0       |                |
| path       | longtext            | YES  |     | NULL    |                |
+------------+---------------------+------+-----+---------+----------------+
7 rows in set (0.01 sec)
```
