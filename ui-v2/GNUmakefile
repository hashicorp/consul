ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

server:
	yarn run start

dist:
	yarn run build
	mv dist ../pkg/web_ui/v2

lint:
	yarn run lint:js
format:
	yarn run format:js

.PHONY: server dist lint format

.DEFAULT_GOAL=all
.PHONY: deps test all build start
all: deps
deps: node_modules yarn.lock package.json
node_modules:
	yarn
build:
	yarn run build
start:
	yarn run start
test:
	yarn run test
test-view:
	yarn run test:view
