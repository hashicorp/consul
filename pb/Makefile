# Generate the Go files from the dns.proto protobuf, you need the utilities
# from: https://github.com/golang/protobuf to make this work.
# The generate dns.pb.go is checked into git, so for normal builds we don't need
# to run this generation step.

all: dns.pb.go

dns.pb.go: dns.proto
	protoc --go_out=plugins=grpc:. dns.proto && \
        sed -e s?golang.org/x/net/context?context? < dns.pb.go > dns.pb.go.tmp && \
        mv dns.pb.go.tmp dns.pb.go
