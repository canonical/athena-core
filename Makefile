.PHONY: all
all: lint build test

.PHONY: docker-compose
docker-compose:
	docker-compose down --remove-orphans
	mkdir --parents tmp
	BRANCH=$(shell git branch --show-current) docker-compose up --force-recreate --build

.PHONY: devel
devel:  athena-monitor athena-processor docker-build docker-compose

.PHONY: common-docker monitor processor
docker-build: athena-monitor docker-build-monitor athena-processor docker-build-processor

.PHONY: docker-build-monitor docker-build-processor
docker-build-monitor docker-build-processor: docker-build-%:
	docker build \
	    --tag athena/athena-$*-linux-amd64:$(subst /,-,$(shell git rev-parse --abbrev-ref HEAD)) \
		--file cmd/$*/Dockerfile \
		$(if $(NOCACHE),--no-cache,) \
		--build-arg ARCH=amd64 \
		--build-arg OS=linux \
		.

.PHONY: build
build: athena-monitor athena-processor

.PHONY: athena-monitor
athena-monitor:
	go build -v -o $@ cmd/monitor/main.go

.PHONY: athena-processor
athena-processor:
	go build -v -o $@ cmd/processor/main.go

.PHONY: lint
lint: check_modules gofmt

.PHONY: check_modules
check_modules:
	go mod tidy

.PHONY: gofmt
gofmt: check_modules
	@fmt_result=$$(gofmt -d $$(find . -name '*.go' -print)); \
		if [ -n "$${fmt_result}" ]; then \
			echo "gofmt checking failed"; \
			echo; \
			echo "$${fmt_result}"; \
			false; \
		fi

.PHONY: test
test:
	go test -v ./...
