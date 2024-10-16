// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package build

import (
	"context"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/runner"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

const dockerfileEnvoy = `
ARG CONSUL_IMAGE
ARG ENVOY_IMAGE
FROM ${CONSUL_IMAGE}
FROM ${ENVOY_IMAGE}
COPY --from=0 /bin/consul /bin/consul
`

// FROM hashicorp/consul-dataplane:latest
// COPY --from=busybox:uclibc /bin/sh /bin/sh
// TODO: busybox:latest doesn't work, see https://hashicorp.slack.com/archives/C03EUN3QF1C/p1691784078972959
const dockerfileDataplane = `
ARG DATAPLANE_IMAGE
FROM busybox:1.34
FROM ${DATAPLANE_IMAGE}
COPY --from=0 /bin/busybox /bin/busybox
USER 0:0
RUN ["busybox", "--install", "/bin", "-s"]
USER 100:0
ENTRYPOINT []
`

const dockerfileDataplaneForTProxy = `
ARG DATAPLANE_IMAGE
ARG CONSUL_IMAGE
FROM ${CONSUL_IMAGE} AS consul
FROM ${DATAPLANE_IMAGE} AS distroless
FROM debian:bullseye-slim

# undo the distroless aspect
COPY --from=distroless /usr/local/bin/discover /usr/local/bin/
COPY --from=distroless /usr/local/bin/envoy /usr/local/bin/
COPY --from=distroless /usr/local/bin/consul-dataplane /usr/local/bin/
COPY --from=distroless /licenses/copyright.txt /licenses/

COPY --from=consul /bin/consul /bin/

# Install iptables and sudo, needed for tproxy.
RUN apt update -y \
	&& apt install -y iptables sudo curl dnsutils

RUN sed '/_apt/d' /etc/passwd > /etc/passwd.new \
    && mv -f /etc/passwd.new /etc/passwd \
    && adduser --uid=100 consul --no-create-home --disabled-password --system \
	&& adduser consul sudo \
	&& echo 'consul ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

COPY <<'EOF' /bin/tproxy-startup.sh
#!/bin/sh

set -ex

# HACK: UID of consul in the consul-client container
# This is conveniently also the UID of apt in the envoy container
CONSUL_UID=100
ENVOY_UID=$(id -u)

# - We allow 19000 so that the test can directly visit the envoy admin page.
# - We allow 20000 so that envoy can receive mTLS traffic from other nodes.
# - We (reluctantly) allow 8080 so that we can bypass envoy and talk to fortio
#   to do test assertions.
sudo consul connect redirect-traffic \
    -proxy-uid $ENVOY_UID \
    -exclude-uid $CONSUL_UID \
	-proxy-inbound-port=15001 \
	-exclude-inbound-port=19000 \
	-exclude-inbound-port=20000 \
	-exclude-inbound-port=8080
exec "$@"
EOF

RUN chmod +x /bin/tproxy-startup.sh \
	&& chown 100:0 /bin/tproxy-startup.sh

RUN echo 'consul ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

USER 100:0
ENTRYPOINT []
`

func DockerImages(
	logger hclog.Logger,
	run *runner.Runner,
	t *topology.Topology,
) error {

	built := make(map[string]struct{})
	for _, c := range t.Clusters {
		for _, n := range c.Nodes {
			const needsTproxy = false // TODO: see if we can bring this back for v1 CDP

			joint := n.Images.EnvoyConsulImage()
			if _, ok := built[joint]; joint != "" && !ok {
				logger.Info("building envoy+consul image", "image", joint)
				logw := logger.Named("docker_envoy_consul").StandardWriter(&hclog.StandardLoggerOptions{ForceLevel: hclog.Debug})

				err := run.DockerExecWithStderr(context.TODO(), []string{
					"build",
					// provenance causes non-idempotent builds, which leads to spurious terraform replacements
					"--provenance=false",
					"--build-arg",
					"CONSUL_IMAGE=" + n.Images.Consul,
					"--build-arg",
					"ENVOY_IMAGE=" + n.Images.Envoy,
					"-t", joint,
					"-",
				}, logw, logw, strings.NewReader(dockerfileEnvoy))
				if err != nil {
					return err
				}

				built[joint] = struct{}{}
			}

			cdp := n.Images.LocalDataplaneImage()
			if _, ok := built[cdp]; cdp != "" && !ok {
				logger.Info("building dataplane image", "image", cdp)
				logw := logger.Named("docker_dataplane").StandardWriter(&hclog.StandardLoggerOptions{ForceLevel: hclog.Debug})
				err := run.DockerExecWithStderr(context.TODO(), []string{
					"build",
					"--provenance=false",
					"--build-arg",
					"DATAPLANE_IMAGE=" + n.Images.Dataplane,
					"-t", cdp,
					"-",
				}, logw, logw, strings.NewReader(dockerfileDataplane))
				if err != nil {
					return err
				}

				built[cdp] = struct{}{}
			}

			cdpTproxy := n.Images.LocalDataplaneTProxyImage()
			if _, ok := built[cdpTproxy]; cdpTproxy != "" && !ok && needsTproxy {
				logger.Info("building image", "image", cdpTproxy)
				logw := logger.Named("docker_dataplane_tproxy").StandardWriter(&hclog.StandardLoggerOptions{ForceLevel: hclog.Debug})
				err := run.DockerExecWithStderr(context.TODO(), []string{
					"build",
					"--build-arg",
					"DATAPLANE_IMAGE=" + n.Images.Dataplane,
					"--build-arg",
					"CONSUL_IMAGE=" + n.Images.Consul,
					"-t", cdpTproxy,
					"-",
				}, logw, logw, strings.NewReader(dockerfileDataplaneForTProxy))
				if err != nil {
					return err
				}

				built[cdpTproxy] = struct{}{}
			}
		}
	}

	return nil
}
