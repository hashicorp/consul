module github.com/hashicorp/consul/test/integration/consul-container

go 1.20

require (
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/docker/docker v24.0.5+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/hashicorp/consul/api v1.21.0
	github.com/hashicorp/consul/envoyextensions v0.2.1
	github.com/hashicorp/consul/sdk v0.13.1
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-uuid v1.0.2
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/serf v0.10.1
	github.com/itchyny/gojq v0.12.9
	github.com/mitchellh/copystructure v1.2.0
	github.com/otiai10/copy v1.10.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.4
	github.com/teris-io/shortid v0.0.0-20220617161101-71ec9f2aa569
	github.com/testcontainers/testcontainers-go v0.22.0
	golang.org/x/mod v0.12.0
)

require (
	dario.cat/mergo v1.0.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cncf/xds/go v0.0.0-20230607035331-e9ce68804cb4 // indirect
	github.com/containerd/containerd v1.7.3 // indirect
	github.com/cpuguy83/dockercfg v0.3.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/envoyproxy/go-control-plane v0.11.1-0.20230524094728-9239064ad72f // indirect
	github.com/envoyproxy/protoc-gen-validate v0.10.1 // indirect
	github.com/fatih/color v1.14.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-hclog v1.2.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-msgpack v1.1.5 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/memberlist v0.5.0 // indirect
	github.com/itchyny/timefmt-go v0.1.4 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/miekg/dns v1.1.41 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/patternmatcher v0.5.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc4 // indirect
	github.com/opencontainers/runc v1.1.8 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/exp v0.0.0-20230801115018-d63ba01acd4b // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230726155614-23370e0ffb3e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230807174057-1744710a1577 // indirect
	google.golang.org/grpc v1.57.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/hashicorp/consul/api => ../../../api

replace github.com/hashicorp/consul/sdk => ../../../sdk

replace github.com/hashicorp/consul => ../../..

replace github.com/hashicorp/consul/envoyextensions => ../../../envoyextensions
