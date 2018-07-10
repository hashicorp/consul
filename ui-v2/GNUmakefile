ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: build

deps: node_modules

build: deps
	yarn run build

start: deps
	yarn run start

start-api: deps
	yarn run start:api

test: deps
	yarn run test

test-view: deps
	yarn run test:view

lint: deps
	yarn run lint:js

format: deps
	yarn run format:js

node_modules: yarn.lock package.json
	yarn install

.PHONY: all deps build start test test-view lint format
