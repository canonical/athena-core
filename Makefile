.PHONY: all
all: lint build test install

.PHONY: devel
devel:  athena-monitor athena-processor docker-build docker-deploy

.PHONY: docker-deploy
docker-deploy:
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

.PHONY: k8s-deploy
k8s-deploy:
	mkdir --parents tmp

.PHONY: k8s
k8s: db nats configmaps athena

.PHONY: db
db:
	kubectl apply \
		--filename db-deployment.yaml \
		--filename db-service.yaml

.PHONY: nats
nats:
	kubectl apply \
		--filename nats-streaming-deployment.yaml \
		--filename nats-streaming-service.yaml

.PHONY: configmaps
configmaps: athena-configmap-monitor.yaml athena-configmap-processor.yaml athena-secret-credentials.yaml

athena-configmap-%.yaml: athena-%.yaml athena-configmap-%-template.yaml
	yq eval ".data.[\"$*.yaml\"] = \"$$(cat $<)\"" athena-configmap-$*-template.yaml | tee $@

athena-secret-credentials.yaml:  credentials.yaml athena-secret-credentials-template.yaml
	yq eval ".stringData.[\"credentials.yaml\"] = \"$$(cat $<)\"" athena-secret-credentials-template.yaml | tee $@

.PHONY: athena
athena: configmaps
	kubectl apply \
		--filename athena-claim.yaml \
		--filename athena-networkpolicy.yaml \
		--filename athena-configmap-monitor.yaml \
		--filename athena-configmap-processor.yaml \
		--filename athena-secret-credentials.yaml \
		--filename athena-deployment.yaml
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
