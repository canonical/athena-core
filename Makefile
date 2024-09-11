.PHONY: all
all: lint build test install

.PHONY: docker-compose
docker-compose:
	if docker-compose version > /dev/null 2>&1; then \
		DOCKER_COMPOSE="docker-compose"; \
	else \
		if docker compose version > /dev/null 2>&1; then  \
			DOCKER_COMPOSE="docker compose"; \
		else \
			echo "Please install the docker-compose command"; \
			exit 1; \
		fi; \
	fi; \
	$${DOCKER_COMPOSE} down --remove-orphans; \
	mkdir --parents tmp; \
	BRANCH=$(shell git branch --show-current | sed -e 's:/:-:g') $${DOCKER_COMPOSE} up --force-recreate --build

.PHONY: devel
devel:  athena-monitor athena-processor docker-build docker-compose

.PHONY: common-docker monitor processor
docker-build: athena-monitor docker-build-monitor athena-processor docker-build-processor docker-build-debug-container

.PHONY: docker-build-monitor docker-build-processor
docker-build-monitor docker-build-processor: docker-build-%:
	docker build \
		--tag athena/athena-$*:$(subst /,-,$(shell git rev-parse --abbrev-ref HEAD)) \
		--file cmd/$*/Dockerfile \
		$(if $(NOCACHE),--no-cache,) \
		--build-arg ARCH=amd64 \
		--build-arg OS=linux \
		.

.PHONY: docker-build-debug-container
docker-build-debug-container:
	docker build \
		--tag debug-container \
		--file Dockerfile-debug \
		.

.PHONY: build
build: athena-monitor athena-processor salesforce-test

.PHONY: athena-monitor
athena-monitor:
	go build -v -o $@ -ldflags="-X main.commit=$$(git describe --tags)" cmd/monitor/main.go

.PHONY: athena-processor
athena-processor:
	go build -v -o $@ -ldflags="-X main.commit=$$(git describe --tags)" cmd/processor/main.go

.PHONY: salesforce-test
salesforce-test:
	go build -v -o $@ -ldflags="-X main.commit=$$(git describe --tags)" cmd/salesforce-test/main.go

.PHONY: files.com-test
files.com-test:
	go build -v -o $@ -ldflags="-X main.commit=$$(git describe --tags)" cmd/files.com-test/main.go

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

.PHONY: install
install: build
	rm -rf build
	mkdir build
	cp athena-monitor athena-processor build/
