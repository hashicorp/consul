# Bookstore Example

This directory contains an OpenAPI description of a simple bookstore API.

Use this example to try the `gnostic-go-generator` plugin, which implements
`gnostic-go-client` and `gnostic-go-server` for generating API client and
server code, respectively.

Run "make all" to build and install `gnostic` and the Go plugins.
It will generate both client and server code. The API client and
server code will be in the `bookstore` package. 

The `service` directory contains additional code that completes the server.
To build and run the service, `cd service` and do the following:

    go get .
    go build
    ./service &

To test the service with the generated client, go back up to the top-level
directory and run `go test`. The test in `bookstore_test.go` uses client
code generated in `bookstore` to verify the service.

