# builds the Cli and Svc1 & Svc2 projects

all: clean
	@go get -v ./...
	@go build -v -o build/svc1 ./svc1/cmd
	@go build -v -o build/svc2 ./svc2/cmd
	@go build -v -o build/cli  ./cli

clean:
	@rm -rf build
