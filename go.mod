module github.com/hashicorp/consul

go 1.13

replace github.com/hashicorp/consul/api => ./api

replace github.com/hashicorp/consul/sdk => ./sdk

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20140225173054-eb6ee6f84d0a

require (
	github.com/Microsoft/go-winio v0.4.3 // indirect
	github.com/NYTimes/gziphandler v1.0.1
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.3.10
	github.com/armon/go-radix v1.0.0
	github.com/aws/aws-sdk-go v1.42.34
	github.com/coredns/coredns v1.1.2
	github.com/coreos/go-oidc v2.1.0+incompatible
	github.com/digitalocean/godo v1.10.0 // indirect
	github.com/docker/go-connections v0.3.0
	github.com/elazarl/go-bindata-assetfs v0.0.0-20160803192304-e1a2a7ec64b0
	github.com/envoyproxy/go-control-plane v0.9.5
	github.com/frankban/quicktest v1.11.0 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.5.6
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/gofuzz v1.2.0
	github.com/google/pprof v0.0.0-20210601050228-01bbb1931b22
	github.com/google/tcpproxy v0.0.0-20180808230851-dfa16c61dad2
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/hashicorp/consul/api v1.11.0
	github.com/hashicorp/consul/sdk v0.8.0
	github.com/hashicorp/go-bexpr v0.1.2
	github.com/hashicorp/go-checkpoint v0.5.0
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-connlimit v0.3.0
	github.com/hashicorp/go-discover v0.0.0-20210818145131-c573d69da192
	github.com/hashicorp/go-hclog v0.14.1
	github.com/hashicorp/go-memdb v1.3.2
	github.com/hashicorp/go-msgpack v0.5.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-raftchunking v0.6.2
	github.com/hashicorp/go-retryablehttp v0.6.7 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.2
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hil v0.0.0-20200423225030-a18a1cd20038
	github.com/hashicorp/memberlist v0.3.1
	github.com/hashicorp/net-rpc-msgpackrpc v0.0.0-20151116020338-a14192a58a69
	github.com/hashicorp/raft v1.3.3
	github.com/hashicorp/raft-autopilot v0.1.5
	github.com/hashicorp/raft-boltdb v0.0.0-20211202195631-7d34b9fb3f42 // indirect
	github.com/hashicorp/raft-boltdb/v2 v2.2.0
	github.com/hashicorp/serf v0.9.6
	github.com/hashicorp/vault/api v1.0.5-0.20200717191844-f687267c8086
	github.com/hashicorp/vault/sdk v0.1.14-0.20200519221838-e0cfd64bc267
	github.com/hashicorp/yamux v0.0.0-20210826001029-26ff87cf9493
	github.com/imdario/mergo v0.3.6
	github.com/joyent/triton-go v1.7.1-0.20200416154420-6801d15b779f // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/text v0.2.0
	github.com/miekg/dns v1.1.41
	github.com/mitchellh/cli v1.1.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-testing-interface v1.14.0
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452
	github.com/mitchellh/mapstructure v1.4.1
	github.com/mitchellh/pointerstructure v1.2.1
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/pquerna/cachecontrol v0.0.0-20180517163645-1555304b9b35 // indirect
	github.com/prometheus/client_golang v1.4.0
	github.com/rboyer/safeio v0.2.1
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/shirou/gopsutil/v3 v3.21.10
	github.com/stretchr/testify v1.7.0
	go.etcd.io/bbolt v1.3.5
	go.opencensus.io v0.22.0 // indirect
	go.uber.org/goleak v1.1.10
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211013075003-97ac67df715c
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/api v0.9.0 // indirect
	google.golang.org/appengine v1.6.0 // indirect
	google.golang.org/genproto v0.0.0-20190819201941-24fa4b261c55
	google.golang.org/grpc v1.25.1
	gopkg.in/square/go-jose.v2 v2.5.1
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
)

replace istio.io/gogo-genproto v0.0.0-20190124151557-6d926a6e6feb => github.com/istio/gogo-genproto v0.0.0-20190124151557-6d926a6e6feb
