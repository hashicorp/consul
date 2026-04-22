module github.com/hashicorp/consul/internal/tools/protoc-gen-consul-rate-limit

go 1.25.0

replace github.com/hashicorp/consul/proto-public => ../../../proto-public

require (
	github.com/hashicorp/consul/proto-public v0.7.4
	google.golang.org/protobuf v1.36.11
)
