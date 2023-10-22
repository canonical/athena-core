.PHONY: all
all: lint build test

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
	BRANCH=$(shell git branch --show-current) $${DOCKER_COMPOSE} up --force-recreate --build

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
