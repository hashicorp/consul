.PHONY: install build test watch

.DEFAULT_GOAL=install

hcl:
	git clone --depth 1 --branch v1.0.0 https://github.com/hashicorp/hcl.git ./hcl

install:
	yarn

build: # install
	yarn run build

test: hcl build
	yarn run test

watch: hcl
	yarn run watch
