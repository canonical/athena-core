package test

var DefaultTestConfig = `
monitor:
  api-key: "xxx"
  poll-every: 500ms
  directories:
      - "/uploads"
      - "/uploads/sosreport"
  processor-map:
    - type: filename
      regex: ".*sosreport.*.tar.xz$"
      processor: sosreports

processor:
  subscribers:
    sosreports:
      sf-comment: |
        Athena

        Processor: {{ processor }} has run the following reports on file: {{ filename }}

        {% for report_name, _ in reports %}
         * {{ report_name }}: {{ pastebin_url }}
        {% endfor %}

      reports:
        hotsos:
          exit-codes: 0 2 127 126
          script: |
            #!/bin/bash
            exit 0

pastebin:
  key: "xxx"
  provider: "github"

salesforce:
  endpoint: "https://canonical--obiwan.my.salesforce.com/"
  username: "xxx@canonical.com.obiwan"
  password: "xxxx"
  security-token: "xxxx"
`
