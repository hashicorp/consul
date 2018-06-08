ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

server:
	yarn run start

init:
	yarn install

build:
	yarn run build

dist: build
	mv dist ../pkg/web_ui/v2

lint:
	yarn run lint:js
format:
	yarn run format:js

.PHONY: server build dist lint format
