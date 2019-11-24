module github.com/coredns/coredns

go 1.12

require (
	cloud.google.com/go v0.41.0 // indirect
	github.com/Azure/azure-sdk-for-go v32.4.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.2
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.0
	github.com/DataDog/datadog-go v2.2.0+incompatible // indirect
	github.com/Shopify/sarama v1.21.0 // indirect
	github.com/aws/aws-sdk-go v1.25.36
	github.com/caddyserver/caddy v1.0.4
	github.com/coredns/federation v0.0.0-20190818181423-e032b096babe
	github.com/coreos/go-systemd v0.0.0-20190212144455-93d5ec2c7f76 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dnstap/golang-dnstap v0.0.0-20170829151710-2cf77a2b5e11
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/farsightsec/golang-framestream v0.0.0-20181102145529-8a0cb8ba8710
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/golang/protobuf v1.3.2
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/infobloxopen/go-trees v0.0.0-20190313150506-2af4e13f9062
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/miekg/dns v1.1.22
	github.com/opentracing-contrib/go-observer v0.0.0-20170622124052-a52f23424492 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/openzipkin-contrib/zipkin-go-opentracing v0.3.5
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.7.0
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	go.etcd.io/etcd v0.5.0-alpha.5.0.20190917205325-a14579fbfb1a
	golang.org/x/crypto v0.0.0-20191002192127-34f69633bfdc
	golang.org/x/net v0.0.0-20191003171128-d98b1b443823 // indirect
	golang.org/x/sys v0.0.0-20191010194322-b09406accb47
	google.golang.org/api v0.14.0
	google.golang.org/grpc v1.25.1
	gopkg.in/DataDog/dd-trace-go.v1 v1.19.0
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.0.0-20190620084959-7cf5895f2711
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/klog v0.4.0
	k8s.io/kube-openapi v0.0.0-20190306001800-15615b16d372 // indirect
	k8s.io/utils v0.0.0-20190529001817-6999998975a7 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.0.0+incompatible
	github.com/miekg/dns v1.1.3 => github.com/miekg/dns v1.1.22
)
