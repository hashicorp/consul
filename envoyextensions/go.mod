module github.com/hashicorp/consul/envoyextensions

go 1.22.11

replace (
	github.com/hashicorp/consul/api => ../api
	github.com/hashicorp/consul/proto-public => ../proto-public
	github.com/hashicorp/consul/sdk => ../sdk
)

retract v0.7.2 // tag was mutated

require (
	github.com/envoyproxy/go-control-plane v0.12.0
	github.com/google/go-cmp v0.6.0
	github.com/hashicorp/consul/api v1.29.4
	github.com/hashicorp/consul/sdk v0.16.1
	github.com/hashicorp/go-hclog v1.5.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-version v1.2.1
	github.com/stretchr/testify v1.8.4
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cncf/xds/go v0.0.0-20230607035331-e9ce68804cb4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/serf v0.10.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/sys v0.29.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230711160842-782d3b101e98 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
