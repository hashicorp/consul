module github.com/hashicorp/consul

go 1.13

replace github.com/hashicorp/consul/api => ./api

replace github.com/hashicorp/consul/sdk => ./sdk

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20140225173054-eb6ee6f84d0a

require (
	github.com/Microsoft/go-winio v0.4.3 // indirect
	github.com/NYTimes/gziphandler v1.0.1
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6 // indirect
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878
	github.com/armon/go-radix v1.0.0
	github.com/aws/aws-sdk-go v1.25.41
	github.com/coredns/coredns v1.1.2
	github.com/coreos/go-oidc v2.1.0+incompatible
	github.com/digitalocean/godo v1.10.0 // indirect
	github.com/docker/go-connections v0.3.0
	github.com/elazarl/go-bindata-assetfs v0.0.0-20160803192304-e1a2a7ec64b0
	github.com/envoyproxy/go-control-plane v0.8.0
	github.com/go-ole/go-ole v1.2.1 // indirect
	github.com/gogo/googleapis v1.1.0
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d
	github.com/golang/protobuf v1.3.1
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/gofuzz v1.0.0
	github.com/google/tcpproxy v0.0.0-20180808230851-dfa16c61dad2
	github.com/hashicorp/consul/api v1.4.0
	github.com/hashicorp/consul/sdk v0.4.0
	github.com/hashicorp/errwrap v1.0.0
	github.com/hashicorp/go-bexpr v0.1.2
	github.com/hashicorp/go-checkpoint v0.0.0-20171009173528-1545e56e46de
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-connlimit v0.2.0
	github.com/hashicorp/go-discover v0.0.0-20200501174627-ad1e96bde088
	github.com/hashicorp/go-hclog v0.12.0
	github.com/hashicorp/go-memdb v1.0.3
	github.com/hashicorp/go-msgpack v0.5.5
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-raftchunking v0.6.1
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.2
	github.com/hashicorp/go-version v1.2.0
	github.com/hashicorp/golang-lru v0.5.1
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hil v0.0.0-20160711231837-1e86c6b523c5
	github.com/hashicorp/memberlist v0.2.2
	github.com/hashicorp/net-rpc-msgpackrpc v0.0.0-20151116020338-a14192a58a69
	github.com/hashicorp/raft v1.1.2
	github.com/hashicorp/raft-boltdb v0.0.0-20171010151810-6e5ba93211ea
	github.com/hashicorp/serf v0.9.2
	github.com/hashicorp/vault/api v1.0.4
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d
	github.com/imdario/mergo v0.3.6
	github.com/joyent/triton-go v1.7.1-0.20200416154420-6801d15b779f // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/text v0.1.0
	github.com/miekg/dns v1.1.26
	github.com/mitchellh/cli v1.1.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-testing-interface v1.14.0
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452
	github.com/mitchellh/mapstructure v1.2.3
	github.com/mitchellh/pointerstructure v1.0.0
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/pascaldekloe/goe v0.1.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.8.1
	github.com/pquerna/cachecontrol v0.0.0-20180517163645-1555304b9b35 // indirect
	github.com/prometheus/client_golang v1.0.0
	github.com/rboyer/safeio v0.2.1
	github.com/ryanuber/columnize v2.1.0+incompatible
	github.com/shirou/gopsutil v0.0.0-20181107111621-48177ef5f880
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.0 // indirect
	golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	google.golang.org/api v0.7.0 // indirect
	google.golang.org/appengine v1.6.0 // indirect
	google.golang.org/genproto v0.0.0-20190530194941-fb225487d101 // indirect
	google.golang.org/grpc v1.23.0
	gopkg.in/square/go-jose.v2 v2.4.1
	k8s.io/api v0.16.9
	k8s.io/apimachinery v0.16.9
	k8s.io/client-go v0.16.9
)

replace istio.io/gogo-genproto v0.0.0-20190124151557-6d926a6e6feb => github.com/istio/gogo-genproto v0.0.0-20190124151557-6d926a6e6feb
