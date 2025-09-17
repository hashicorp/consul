module github.com/hashicorp/consul/internal/tools/protoc-gen-consul-rate-limit

go 1.25.0

replace github.com/hashicorp/consul/proto-public => ../../../proto-public

require (
	github.com/hashicorp/consul/proto-public v0.6.5
	google.golang.org/protobuf v1.36.6
)

require github.com/google/go-cmp v0.5.9 // indirect
