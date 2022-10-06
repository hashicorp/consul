#!/usr/bin/env bash

SET=${1:-unique}
XDS_TARGET=${2:-server}

echo "Started tests from Set $SET and XDS_TARGET $XDS_TARGET"

mkdir test/integration/connect/envoy/results/ -p

if [ $SET == 0 ]
then

    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-badauthz" -win=true > test/integration/connect/envoy/results/case-badauthz.log
    echo "Completed  33%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-basic" -win=true > test/integration/connect/envoy/results/case-basic.log
    echo "Completed  66%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-centralconf" -win=true > test/integration/connect/envoy/results/case-centralconf.log
    echo "Completed  100%"

elif [ $SET == 1 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-consul-exec" -win=true > test/integration/connect/envoy/results/case-consul-exec.log
    echo "Completed  20%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-expose-checks" -win=true > test/integration/connect/envoy/results/case-expose-checks.log
    echo "Completed  40%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-I7-intentions" -win=true > test/integration/connect/envoy/results/case-I7-intentions.log
    echo "Completed  60%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-prometheus" -win=true > test/integration/connect/envoy/results/case-prometheus.log
    echo "Completed  80%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-upstream-config" -win=true > test/integration/connect/envoy/results/case-upstream-config.log
    echo "Completed  100%"

elif [ $SET == 2 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-zipkin" -win=true > test/integration/connect/envoy/results/case-zipkin.log
    echo "Completed  10%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-defaultsubset" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-defaultsubset.log
    echo "Completed  20%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-features" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-features.log
    echo "Completed  30%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-onlypassing" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-onlypassing.log
    echo "Completed  40%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-redirect" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-redirect.log
    echo "Completed  50%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-failover" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-failover.log
    echo "Completed  60%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-http" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-http.log
    echo "Completed  70%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-tcp" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-tcp.log
    echo "Completed  80%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-router-features" -win=true > test/integration/connect/envoy/results/case-cfg-router-features.log
    echo "Completed  90%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-splitter-features" -win=true > test/integration/connect/envoy/results/case-cfg-splitter-features.log
    echo "Completed  100%"

elif [ $SET == 3 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-grpc" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-grpc.log
    echo "Completed  16%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-http" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-http.log
    echo "Completed  32%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-multiple-services" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-multiple-services.log
    echo "Completed  48%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-sds" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-sds.log
    echo "Completed  66%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-simple" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-simple.log
    echo "Completed  83%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-tls" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-tls.log
    echo "Completed  100%"

elif [ $SET == 4 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-gateway-without-services.log
    echo "Completed  20%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-hostnames" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-hostnames.log
    echo "Completed  40%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-simple" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-simple.log
    echo "Completed  60%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-subsets" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-subsets.log
    echo "Completed  80%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-without-services.log
    echo "Completed  100%"

elif [ $SET == 5 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-dogstatsd-udp" -win=true > test/integration/connect/envoy/results/case-dogstatsd-udp.log
    echo "Completed  16%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-grpc" -win=true > test/integration/connect/envoy/results/case-grpc.log
    echo "Completed  32%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http$" -win=true > test/integration/connect/envoy/results/case-http.log
    echo "Completed  48%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http-badauthz" -win=true > test/integration/connect/envoy/results/case-http-badauthz.log
    echo "Completed  66%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-statsd-udp" -win=true > test/integration/connect/envoy/results/case-statsd-udp.log
    echo "Completed  83%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-stats-proxy" -win=true > test/integration/connect/envoy/results/case-stats-proxy.log
    echo "Completed  100%"

elif [ $SET == 6 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-cluster-peering-failover" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-cluster-peering-failover.log
    echo "Completed  33%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-dc-failover-gateways-none" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-dc-failover-gateways-none.log
    echo "Completed  66%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-dc-failover-gateways-remote" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-dc-failover-gateways-remote.log
    echo "Completed  100%"

elif [ $SET == 7 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers" -win=true > test/integration/connect/envoy/results/case-cross-peers.log
    echo "Completed  25%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers-http" -win=true > test/integration/connect/envoy/results/case-cross-peers-http.log
    echo "Completed  50%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers-http-router" -win=true > test/integration/connect/envoy/results/case-cross-peers-http-router.log
    echo "Completed  75%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers-resolver-redirect-tcp" -win=true > test/integration/connect/envoy/results/case-cross-peers-resolver-redirect-tcp.log
    echo "Completed  100%"

elif [ $SET == 8 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-multidc-rsa-ca" -win=true > test/integration/connect/envoy/results/case-multidc-rsa-ca.log
    echo "Completed  50%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-wanfed-gw" -win=true > test/integration/connect/envoy/results/case-wanfed-gw.log
    echo "Completed  100%"

elif [ $SET == 9 ]
then
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateways-local" -win=true > test/integration/connect/envoy/results/case-gateways-local.log
    echo "Completed  33%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateways-remote" -win=true > test/integration/connect/envoy/results/case-gateways-remote.log
    echo "Completed  66%"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-mesh-gateways-resolver" -win=true > test/integration/connect/envoy/results/case-ingress-mesh-gateways-resolver.log
    echo "Completed  100%"

elif [ $SET == "unique" ]
then
    echo "Total is 35"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-badauthz" -win=true > test/integration/connect/envoy/results/case-badauthz.log
    echo "Completed 01"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-basic" -win=true > test/integration/connect/envoy/results/case-basic.log
    echo "Completed 02"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-centralconf" -win=true > test/integration/connect/envoy/results/case-centralconf.log
    echo "Completed 03"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-defaultsubset" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-defaultsubset.log
    echo "Completed 04"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-features" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-features.log
    echo "Completed 05"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-onlypassing" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-onlypassing.log
    echo "Completed 06"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-redirect" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-redirect.log
    echo "Completed 07"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-failover" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-failover.log
    echo "Completed 08"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-http" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-http.log
    echo "Completed 09"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-tcp" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-tcp.log
    echo "Completed 10"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-router-features" -win=true > test/integration/connect/envoy/results/case-cfg-router-features.log
    echo "Completed 11"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-splitter-features" -win=true > test/integration/connect/envoy/results/case-cfg-splitter-features.log
    echo "Completed 12"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-consul-exec" -win=true > test/integration/connect/envoy/results/case-consul-exec.log
    echo "Completed 13"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-dogstatsd-udp" -win=true > test/integration/connect/envoy/results/case-dogstatsd-udp.log
    echo "Completed 14"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-expose-checks" -win=true > test/integration/connect/envoy/results/case-expose-checks.log
    echo "Completed 15"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-gateway-without-services.log
    echo "Completed 16"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-grpc" -win=true > test/integration/connect/envoy/results/case-grpc.log
    echo "Completed 17"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http$" -win=true > test/integration/connect/envoy/results/case-http.log
    echo "Completed 18"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http-badauthz" -win=true > test/integration/connect/envoy/results/case-http-badauthz.log
    echo "Completed 19"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-grpc" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-grpc.log
    echo "Completed 20"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-http" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-http.log
    echo "Completed 21"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-multiple-services" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-multiple-services.log
    echo "Completed 22"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-sds" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-sds.log
    echo "Completed 23"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-simple" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-simple.log
    echo "Completed 24"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-tls" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-tls.log
    echo "Completed 25"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-l7-intentions" -win=true > test/integration/connect/envoy/results/case-l7-intentions.log
    echo "Completed 26"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-prometheus" -win=true > test/integration/connect/envoy/results/case-prometheus.log
    echo "Completed 27"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-statsd-udp" -win=true > test/integration/connect/envoy/results/case-statsd-udp.log
    echo "Completed 287"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-stats-proxy" -win=true > test/integration/connect/envoy/results/case-stats-proxy.log
    echo "Completed 29"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-hostnames" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-hostnames.log
    echo "Completed 30"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-simple" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-simple.log
    echo "Completed 31"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-subsets" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-subsets.log
    echo "Completed 32"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-without-services.log
    echo "Completed 33"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-upstream-config" -win=true > test/integration/connect/envoy/results/case-upstream-config.log
    echo "Completed 34"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-zipkin" -win=true > test/integration/connect/envoy/results/case-zipkin.log
    echo "Completed 35"

elif [ $SET == "multi" ]
then
    echo "Total is 12"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-cluster-peering-failover" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-cluster-peering-failover.log
    echo "Completed 1"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-dc-failover-gateways-none" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-dc-failover-gateways-none.log
    echo "Completed 2"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-dc-failover-gateways-remote" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-dc-failover-gateways-remote.log
    echo "Completed 3"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers$" -win=true > test/integration/connect/envoy/results/case-cross-peers.log
    echo "Completed 4"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers-http$" -win=true > test/integration/connect/envoy/results/case-cross-peers-http.log
    echo "Completed 5"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers-http-router" -win=true > test/integration/connect/envoy/results/case-cross-peers-http-router.log
    echo "Completed 6"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cross-peers-resolver-redirect-tcp" -win=true > test/integration/connect/envoy/results/case-cross-peers-resolver-redirect-tcp.log
    echo "Completed 7"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateways-local" -win=true > test/integration/connect/envoy/results/case-gateways-local.log
    echo "Completed 8"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateways-remote" -win=true > test/integration/connect/envoy/results/case-gateways-remote.log
    echo "Completed 9"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-mesh-gateways-resolver" -win=true > test/integration/connect/envoy/results/case-ingress-mesh-gateways-resolver.log
    echo "Completed 10"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-multidc-rsa-ca" -win=true > test/integration/connect/envoy/results/case-multidc-rsa-ca.log
    echo "Completed 11"
    XDS_TARGET=$XDS_TARGET go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-wanfed-gw" -win=true > test/integration/connect/envoy/results/case-wanfed-gw.log
    echo "Completed 12"
fi

echo "Completed tests from Set $SET"
