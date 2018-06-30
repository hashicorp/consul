
build:	
	go get golang.org/x/tools/cmd/goimports
	go install github.com/googleapis/gnostic
	go install github.com/googleapis/gnostic/plugins/gnostic-go-generator
	rm -f $(GOPATH)/bin/gnostic-go-client $(GOPATH)/bin/gnostic-go-server
	ln -s $(GOPATH)/bin/gnostic-go-generator $(GOPATH)/bin/gnostic-go-client
	ln -s $(GOPATH)/bin/gnostic-go-generator $(GOPATH)/bin/gnostic-go-server

