module github.com/hashicorp/consul/internal/tools/proto-gen-rpc-glue/e2e

go 1.25.1

replace github.com/hashicorp/consul => ./consul

require github.com/hashicorp/consul v1.11.4

require google.golang.org/protobuf v1.28.1 // indirect
