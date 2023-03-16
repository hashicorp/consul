SHELL := /bin/bash

.PHONY: all
all: install

.PHONY: install
install:
	@go install

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run -v

.PHONY: format
format:
	@for f in $$(find . -name '*.go' -print); do \
		gofmt -s -w $$f ; \
	done

.PHONY: update-defaults
update-defaults: update-consul update-envoy update-dataplane

.PHONY: update-consul
update-consul:
	@docker pull hashicorp/consul:latest || true
	@mkdir -p tmp
	@docker run --rm hashicorp/consul:latest version -format=json | jq -r .Version > tmp/default_consul.val
	@printf "package topology\n\nconst DefaultConsulImage = \"hashicorp/consul:$$(cat tmp/default_consul.val)\"\n" > topology/default_consul.go
	@printf "const DefaultConsulEnterpriseImage = \"hashicorp/consul-enterprise:$$(cat tmp/default_consul.val)-ent\"\n" >> topology/default_consul.go
	@rm -rf tmp

.PHONY: update-envoy
update-envoy:
	@docker rm -f consul-envoy-check &>/dev/null || true
	@docker pull hashicorp/consul:latest || true
	@docker run -d --name consul-envoy-check hashicorp/consul:latest
	@mkdir -p tmp
	@docker exec consul-envoy-check sh -c 'wget -q localhost:8500/v1/agent/self -O -' | jq -r '.xDS.SupportedProxies.envoy[0]' > tmp/default_envoy.val
	@docker rm -f consul-envoy-check &>/dev/null || true
	@printf "package topology\n\nconst DefaultEnvoyImage = \"envoyproxy/envoy:v$$(cat tmp/default_envoy.val)\"\n" > topology/default_envoy.go
	@rm -rf tmp

.PHONY: update-dataplane
update-dataplane:
	@docker pull hashicorp/consul-dataplane:latest || true
	@mkdir -p tmp
	@docker run --rm hashicorp/consul-dataplane:latest --version | head -n 1 | awk '{print $$3}' | cut -d'v' -f2- > tmp/default_dataplane.val
	@printf "package topology\n\nconst DefaultDataplaneImage = \"hashicorp/consul-dataplane:$$(cat tmp/default_dataplane.val)\"\n" > topology/default_cdp.go
	@rm -rf tmp


.PHONY: help
help:
	$(info available make targets)
	$(info ----------------------)
	@grep "^[a-z0-9-][a-z0-9.-]*:" Makefile  | cut -d':' -f1 | sort
