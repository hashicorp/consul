ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

server:
	yarn run start

dist:
	yarn run build

lint:
	yarn run lint:js
format:
	yarn run format:js

.PHONY: server dist lint format
