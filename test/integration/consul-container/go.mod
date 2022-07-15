module github.com/hashicorp/consul/integration/consul-container

go 1.16

require (
	github.com/docker/docker v20.10.11+incompatible
	github.com/hashicorp/consul v1.12.1
	github.com/hashicorp/consul/api v1.11.0
<<<<<<< HEAD
	github.com/hashicorp/consul/sdk v0.8.0
||||||| parent of d523d005d9 (Latest submodule versions (#13750))
	github.com/hashicorp/consul/sdk v0.8.0
	github.com/hashicorp/go-uuid v1.0.2
	github.com/hashicorp/hcl v1.0.0
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.13.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/Microsoft/hcsshim v0.8.24 // indirect
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/cenkalti/backoff/v4 v4.1.2 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/containerd v1.5.13 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
=======
	github.com/hashicorp/consul/sdk v0.10.0
	github.com/hashicorp/go-uuid v1.0.2
	github.com/hashicorp/hcl v1.0.0
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.13.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/Microsoft/hcsshim v0.8.24 // indirect
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/cenkalti/backoff/v4 v4.1.2 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/containerd v1.5.13 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
>>>>>>> d523d005d9 (Latest submodule versions (#13750))
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
