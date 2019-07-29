module github.com/coredns/coredns

go 1.12

require (
	cloud.google.com/go v0.41.0 // indirect
	github.com/Shopify/sarama v1.21.0 // indirect
	github.com/apache/thrift v0.12.0 // indirect
	github.com/aws/aws-sdk-go v1.21.6
	github.com/caddyserver/caddy v1.0.1
	github.com/coreos/bbolt v1.3.2 // indirect
	github.com/coreos/etcd v3.3.13+incompatible
	github.com/coreos/go-semver v0.2.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190212144455-93d5ec2c7f76 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/dnstap/golang-dnstap v0.0.0-20170829151710-2cf77a2b5e11
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/farsightsec/golang-framestream v0.0.0-20181102145529-8a0cb8ba8710
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/golang/protobuf v1.3.2
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gophercloud/gophercloud v0.0.0-20190307220656-fe1ba5ce12dd // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.8.3 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/miekg/dns v1.1.15
	github.com/opentracing-contrib/go-observer v0.0.0-20170622124052-a52f23424492 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/openzipkin-contrib/zipkin-go-opentracing v0.3.5 // indirect
	github.com/openzipkin/zipkin-go-opentracing v0.3.5
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.6.0
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.2 // indirect
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1 // indirect
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7 // indirect
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7 // indirect
	google.golang.org/genproto v0.0.0-20190701230453-710ae3a149df // indirect
	google.golang.org/grpc v1.22.1
	gopkg.in/DataDog/dd-trace-go.v1 v1.16.0
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.0.0-20190620084959-7cf5895f2711
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/klog v0.3.3
	k8s.io/kube-openapi v0.0.0-20190306001800-15615b16d372 // indirect
	k8s.io/utils v0.0.0-20190529001817-6999998975a7 // indirect
)

replace github.com/miekg/dns v1.1.3 => github.com/miekg/dns v1.1.15
