all:
	go build

.PHONY: docker
docker:
	GOOS=linux go build -a -tags netgo -installsuffix netgo
	docker build -t $$USER/coredns .
