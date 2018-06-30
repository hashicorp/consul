
TGT=gnostic-swift-sample

BINDIR=.build/debug

all:
	swift build

install: all
	cp $(BINDIR)/$(TGT) $(GOPATH)/bin/$(TGT)

clean :
	rm -rf Packages
	rm -rf .build
	rm -f Package.pins Package.resolved
