build:	
	go get golang.org/x/tools/cmd/goimports
	go install github.com/googleapis/gnostic
	go install github.com/googleapis/gnostic/plugins/gnostic-go-generator
	rm -f $(GOPATH)/bin/gnostic-go-client $(GOPATH)/bin/gnostic-go-server
	ln -s $(GOPATH)/bin/gnostic-go-generator $(GOPATH)/bin/gnostic-go-client
	ln -s $(GOPATH)/bin/gnostic-go-generator $(GOPATH)/bin/gnostic-go-server

all:	build
	gnostic bookstore.json --go-generator-out=bookstore

clean:  
	rm -rf bookstore bookstore.text service/service

test:	all
	killall service; true # ignore errors due to no matching processes
	cd service; go get .; go build; ./service &
	go test
	killall service

