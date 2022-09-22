#!/usr/bin/env bash

SET=${1:-all}

echo "Started tests from Set $SET"

mkdir test/integration/connect/envoy/results/ -p

if [ $SET == 0 ]
then

    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-badauthz" -win=true > test/integration/connect/envoy/results/case-badauthz.log
    echo "Completed  33%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-basic" -win=true > test/integration/connect/envoy/results/case-basic.log
    echo "Completed  66%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-centralconf" -win=true > test/integration/connect/envoy/results/case-centralconf.log
    echo "Completed  100%"

elif [ $SET == 1 ]
then
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-consul-exec" -win=true > test/integration/connect/envoy/results/case-consul-exec.log
    echo "Completed  20%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-expose-checks" -win=true > test/integration/connect/envoy/results/case-expose-checks.log
    echo "Completed  40%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-I7-intentions" -win=true > test/integration/connect/envoy/results/case-I7-intentions.log
    echo "Completed  60%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-prometheus" -win=true > test/integration/connect/envoy/results/case-prometheus.log
    echo "Completed  80%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-upstream-config" -win=true > test/integration/connect/envoy/results/case-upstream-config.log
    echo "Completed  100%"

elif [ $SET == 2 ]
then
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-zipkin" -win=true > test/integration/connect/envoy/results/case-zipkin.log
    echo "Completed  10%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-defaultsubset" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-defaultsubset.log
    echo "Completed  20%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-features" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-features.log
    echo "Completed  30%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-onlypassing" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-onlypassing.log
    echo "Completed  40%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-redirect" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-redirect.log
    echo "Completed  50%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-failover" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-failover.log
    echo "Completed  60%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-http" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-http.log
    echo "Completed  70%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-tcp" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-tcp.log
    echo "Completed  80%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-router-features" -win=true > test/integration/connect/envoy/results/case-cfg-router-features.log
    echo "Completed  90%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-splitter-features" -win=true > test/integration/connect/envoy/results/case-cfg-splitter-features.log
    echo "Completed  100%"

elif [ $SET == 3 ]
then
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-grpc" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-grpc.log
    echo "Completed  16%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-http" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-http.log
    echo "Completed  32%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-multiple-services" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-multiple-services.log
    echo "Completed  48%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-sds" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-sds.log
    echo "Completed  66%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-simple" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-simple.log
    echo "Completed  83%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-tls" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-tls.log
    echo "Completed  100%"

elif [ $SET == 4 ]
then
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-gateway-without-services.log
    echo "Completed  20%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-hostnames" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-hostnames.log
    echo "Completed  40%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-simple" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-simple.log
    echo "Completed  60%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-subsets" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-subsets.log
    echo "Completed  80%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-without-services.log
    echo "Completed  100%"

elif [ $SET == 5 ]
then
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-dogstatsd-udp" -win=true > test/integration/connect/envoy/results/case-dogstatsd-udp.log
    echo "Completed  16%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-grpc" -win=true > test/integration/connect/envoy/results/case-grpc.log
    echo "Completed  32%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http" -win=true > test/integration/connect/envoy/results/case-http.log
    echo "Completed  48%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http-badauthz" -win=true > test/integration/connect/envoy/results/case-http-badauthz.log
    echo "Completed  66%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-statsd-udp" -win=true > test/integration/connect/envoy/results/case-statsd-udp.log
    echo "Completed  83%"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-stats-proxy" -win=true > test/integration/connect/envoy/results/case-stats-proxy.log
    echo "Completed  100%"
else
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-badauthz" -win=true > test/integration/connect/envoy/results/case-badauthz.log
    echo "Completed 01"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-basic" -win=true > test/integration/connect/envoy/results/case-basic.log
    echo "Completed 02"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-centralconf" -win=true > test/integration/connect/envoy/results/case-centralconf.log
    echo "Completed 03"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-consul-exec" -win=true > test/integration/connect/envoy/results/case-consul-exec.log
    echo "Completed 04"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-expose-checks" -win=true > test/integration/connect/envoy/results/case-expose-checks.log
    echo "Completed 05"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-l7-intentions" -win=true > test/integration/connect/envoy/results/case-l7-intentions.log
    echo "Completed 06"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-prometheus" -win=true > test/integration/connect/envoy/results/case-prometheus.log
    echo "Completed 07"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-upstream-config" -win=true > test/integration/connect/envoy/results/case-upstream-config.log
    echo "Completed 08"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-zipkin" -win=true > test/integration/connect/envoy/results/case-zipkin.log
    echo "Completed 09"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-defaultsubset" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-defaultsubset.log
    echo "Completed 10"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-features" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-features.log
    echo "Completed 11"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-onlypassing" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-onlypassing.log
    echo "Completed 12"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-subset-redirect" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-subset-redirect.log
    echo "Completed 13"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-failover" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-failover.log
    echo "Completed 14"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-http" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-http.log
    echo "Completed 15"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-resolver-svc-redirect-tcp" -win=true > test/integration/connect/envoy/results/case-cfg-resolver-svc-redirect-tcp.log
    echo "Completed 16"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-router-features" -win=true > test/integration/connect/envoy/results/case-cfg-router-features.log
    echo "Completed 17"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-cfg-splitter-features" -win=true > test/integration/connect/envoy/results/case-cfg-splitter-features.log
    echo "Completed 18"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-grpc" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-grpc.log
    echo "Completed 19"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-http" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-http.log
    echo "Completed 20"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-multiple-services" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-multiple-services.log
    echo "Completed 21"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-sds" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-sds.log
    echo "Completed 22"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-simple" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-simple.log
    echo "Completed 23"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-ingress-gateway-tls" -win=true > test/integration/connect/envoy/results/case-ingress-gateway-tls.log
    echo "Completed 24"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-gateway-without-services.log
    echo "Completed 25"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-hostnames" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-hostnames.log
    echo "Completed 26"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-simple" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-simple.log
    echo "Completed 27"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-subsets" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-subsets.log
    echo "Completed 28"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-terminating-gateway-without-services" -win=true > test/integration/connect/envoy/results/case-terminating-gateway-without-services.log
    echo "Completed 29"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-dogstatsd-udp" -win=true > test/integration/connect/envoy/results/case-dogstatsd-udp.log
    echo "Completed 30"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-grpc" -win=true > test/integration/connect/envoy/results/case-grpc.log
    echo "Completed 31"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http" -win=true > test/integration/connect/envoy/results/case-http.log
    echo "Completed 32"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-http-badauthz" -win=true > test/integration/connect/envoy/results/case-http-badauthz.log
    echo "Completed 33"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-statsd-udp" -win=true > test/integration/connect/envoy/results/case-statsd-udp.log
    echo "Completed 34"
    go test -v -timeout=30m -tags integration ./test/integration/connect/envoy -run="TestEnvoy/case-stats-proxy" -win=true > test/integration/connect/envoy/results/case-stats-proxy.log
    echo "Completed 35"
fi

echo "Completed tests from Set $SET"
