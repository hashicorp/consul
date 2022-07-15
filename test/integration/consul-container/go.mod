module github.com/hashicorp/consul/integration/consul-container

go 1.16

require (
	github.com/docker/docker v20.10.11+incompatible
	github.com/hashicorp/consul v1.12.1
	github.com/hashicorp/consul/api v1.11.0
	github.com/hashicorp/consul/sdk v0.8.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v0.16.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-uuid v1.0.2
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.13.0
	google.golang.org/grpc v1.41.0 // indirect
)

replace github.com/hashicorp/consul/api => ../../../api

replace github.com/hashicorp/consul/sdk => ../../../sdk
