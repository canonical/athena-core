package test

var DefaultTestConfig = `
monitor:
  api-key: "xxxxx"
  poll-every: 500ms 
  filetypes:
    - "sosreport-*"
  directories:
    - "/uploads"
    - "/uploads/sosreport"

  processor-map:
    - type: filename
      regex: ".*sosreport.*$"
      processor: sosreports

    - type: case
      regex: 00295561
      processor: process-00295561

processor:
  subscribe-to:
    - topic: sosreports
      reports:
        tcp_mem:
          command: cat /proc/sys/net/ipv4/tcp*mem
          exit-codes: 0
        sar:
          exit-codes: 0 127 126
          script: |
            #!/bin/bash
            echo "testing"

    - topic: kernel
      reports:
        tcp_mem:
          command: cat /proc/sys/net/ipv4/tcp*mem
          exit-codes: 0

          # scripts can be defined inline
        sar:
          exit-codes: 0 127 126
          script: |
            #!/bin/bash
            echo "testing"
`