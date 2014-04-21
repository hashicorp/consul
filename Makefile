server:
	python -m SimpleHTTPServer

watch:
	sass styles --watch

build:
	sass styles

.PHONY: server watch build
