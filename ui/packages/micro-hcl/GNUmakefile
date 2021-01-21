.PHONY: install build test

.DEFAULT_GOAL=install

hcl:
	git clone https://github.com/hashicorp/hcl.git ./hcl

install:
	yarn

build: # install
	yarn run build

test: hcl build
	yarn run test
