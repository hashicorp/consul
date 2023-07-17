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
const dockerfileDataplane = `
ARG DATAPLANE_IMAGE
FROM busybox:latest
FROM ${DATAPLANE_IMAGE}
COPY --from=0 /bin/busybox /bin/busybox
USER 0:0
RUN ["busybox", "--install", "/bin", "-s"]
USER 100:0
ENTRYPOINT []
`

func DockerImages(
	logger hclog.Logger,
	run *runner.Runner,
	t *topology.Topology,
) error {
	logw := logger.Named("docker").StandardWriter(&hclog.StandardLoggerOptions{ForceLevel: hclog.Info})

	built := make(map[string]struct{})
	for _, c := range t.Clusters {
		for _, n := range c.Nodes {
			joint := n.Images.EnvoyConsulImage()
			if _, ok := built[joint]; joint != "" && !ok {
				logger.Info("building image", "image", joint)
				err := run.DockerExec(context.TODO(), []string{
					"build",
					"--build-arg",
					"CONSUL_IMAGE=" + n.Images.Consul,
					"--build-arg",
					"ENVOY_IMAGE=" + n.Images.Envoy,
					"-t", joint,
					"-",
				}, logw, strings.NewReader(dockerfileEnvoy))
				if err != nil {
					return err
				}

				built[joint] = struct{}{}
			}

			cdp := n.Images.LocalDataplaneImage()
			if _, ok := built[cdp]; cdp != "" && !ok {
				logger.Info("building image", "image", cdp)
				err := run.DockerExec(context.TODO(), []string{
					"build",
					"--build-arg",
					"DATAPLANE_IMAGE=" + n.Images.Dataplane,
					"-t", cdp,
					"-",
				}, logw, strings.NewReader(dockerfileDataplane))
				if err != nil {
					return err
				}

				built[cdp] = struct{}{}
			}
		}
	}

	return nil
}
