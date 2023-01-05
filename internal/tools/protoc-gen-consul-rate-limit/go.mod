module github.com/hashicorp/consul/internal/tools/protoc-gen-consul-rate-limit

go 1.19

replace github.com/hashicorp/consul/proto-public => ../../../proto-public

require (
	github.com/hashicorp/consul/proto-public v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.28.1
)

require github.com/golang/protobuf v1.5.0 // indirect
