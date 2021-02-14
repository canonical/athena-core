package test

var DefaultTestConfig = `
db:
  dialect: mysql
  dsn: "athena:athena@tcp(db:3306)/athena?charset=utf8&parseTime=true"

monitor:
  poll-every: 1s
  files-delta: 1s
  directories:
      - "/uploads"
  processor-map:
    - type: filename
      regex: ".*sosreport.*.tar.xz$"
      processor: sosreports

processor:
  batch-comments-every: 1m
  base-tmpdir: "/tmp/athena"
  subscribers:
    sosreports:
      sf-comment-enabled: true
      sf-comment-batch: 5m
      sf-comment-public: false
      sf-comment: |
        Athena

        Processor: {{ processor }} Subscriber: {{ subscriber }} has run the following reports:

        {% for report in reports %}
          * {{ report.Name }}: https://files.support.canonical.com/files/{{report.UploadLocation}}
        {% endfor %}

      reports:
        hotsos:
          exit-codes: 0 2 127 126
          script: |
            #!/bin/bash
            echo ${{filepath}} ${{basedir}} && exit 0

filescom:
  key: "xxx"
  endpoint: "https://app.files.com"

salesforce:
  endpoint: "https://xxx--xxxx.my.salesforce.com/"
  username: "xxx@xxx.com.xxx"
  password: "xxx"
  security-token: "xxx"`
