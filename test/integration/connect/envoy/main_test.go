// +build integration

package envoy

import (
	"os"
	"os/exec"
	"testing"
)

func TestEnvoy(t *testing.T) {
	var testcases = []string{
		"case-badauthz",
		"case-basic",
		"case-centralconf",
		"case-cfg-resolver-dc-failover-gateways-none",
		"case-cfg-resolver-dc-failover-gateways-remote",
		"case-cfg-resolver-defaultsubset",
		"case-cfg-resolver-features",
		"case-cfg-resolver-subset-onlypassing",
		"case-cfg-resolver-subset-redirect",
		"case-cfg-resolver-svc-failover",
		"case-cfg-resolver-svc-redirect-http",
		"case-cfg-resolver-svc-redirect-tcp",
		"case-cfg-router-features",
		"case-cfg-splitter-features",
		"case-consul-exec",
		"case-dogstatsd-udp",
		"case-expose-checks",
		"case-gateways-local",
		"case-gateways-remote",
		"case-gateway-without-services",
		"case-grpc",
		"case-http",
		"case-http-badauthz",
		"case-ingress-gateway-http",
		"case-ingress-gateway-grpc",
		"case-ingress-gateway-multiple-services",
		"case-ingress-gateway-simple",
		"case-ingress-gateway-tls",
		"case-ingress-mesh-gateways-resolver",
		"case-multidc-rsa-ca",
		"case-prometheus",
		"case-statsd-udp",
		"case-stats-proxy",
		"case-terminating-gateway-hostnames",
		"case-terminating-gateway-simple",
		"case-terminating-gateway-subsets",
		"case-terminating-gateway-without-services",
		"case-upstream-config",
		"case-wanfed-gw",
		"case-zipkin",
	}

	runCmd(t, "suite_setup")
	defer runCmd(t, "suite_teardown")

	for _, tc := range testcases {
		t.Run(tc, func(t *testing.T) {
			runCmd(t, "run_tests", "CASE_DIR="+tc)
		})
	}
}

func runCmd(t *testing.T, c string, env ...string) {
	t.Helper()

	cmd := exec.Command("./run-tests.sh", c)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
}
