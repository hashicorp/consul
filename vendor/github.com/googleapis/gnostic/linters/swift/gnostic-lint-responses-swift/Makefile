
TGT=gnostic-lint-responses-swift

BINDIR=.build/debug

all:
	swift build

install: all
	cp $(BINDIR)/$(TGT) $(GOPATH)/bin/$(TGT)

clean :
	rm -rf Packages
	rm -rf .build
	rm -f Package.pins Package.resolved

run:	
	gnostic ../../../examples/v2.0/yaml/petstore.yaml --lint-responses-swift
