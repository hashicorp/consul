#VERBOSE :=
VERBOSE := -v

all:
	go build $(VERBOSE)

.PHONY: docker
docker:
	GOOS=linux go build -a -tags netgo -installsuffix netgo
	docker build -t $$USER/coredns .

.PHONY: deps
deps:
	go get

.PHONY: test
test:
	go test

.PHONY: clean
clean:
	go clean
