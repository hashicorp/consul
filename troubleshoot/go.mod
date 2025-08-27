module github.com/hashicorp/consul/troubleshoot

go 1.25.0

replace (
	github.com/hashicorp/consul/api => ../api
	github.com/hashicorp/consul/envoyextensions => ../envoyextensions
	github.com/hashicorp/consul/proto-public => ../proto-public
	github.com/hashicorp/consul/sdk => ../sdk
)

exclude (
	github.com/hashicorp/go-msgpack v1.1.5 // has breaking changes and must be avoided
	github.com/hashicorp/go-msgpack v1.1.6 // contains retractions but same as v1.1.5
)

retract (
	v0.6.4 // tag was mutated
	v0.6.2 // tag has incorrect line of deps
	v0.6.1 // tag has incorrect line of deps
)

require (
	github.com/envoyproxy/go-control-plane v0.13.4
	github.com/envoyproxy/go-control-plane/contrib v1.32.4
	github.com/envoyproxy/go-control-plane/envoy v1.32.4
	github.com/envoyproxy/go-control-plane/ratelimit v0.1.0
	github.com/envoyproxy/go-control-plane/xdsmatcher v0.13.4
	github.com/hashicorp/consul/api v1.29.4
	github.com/hashicorp/consul/envoyextensions v0.7.3
	github.com/hashicorp/consul/sdk v0.16.1
	github.com/stretchr/testify v1.10.0
	google.golang.org/protobuf v1.36.6
)

require (
	cel.dev/expr v0.19.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.2.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	golang.org/x/exp v0.0.0-20250808145144-a408d31f581a // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241202173237-19429a94021a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect
	google.golang.org/grpc v1.70.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
