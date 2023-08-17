// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

const (
	consulUID     = "100"
	consulGID     = "1000"
	consulUserArg = consulUID + ":" + consulGID
)

func tlsPrefixFromNode(node *topology.Node) string {
	switch node.Kind {
	case topology.NodeKindServer:
		return node.Partition + "." + node.Name + ".server"
	case topology.NodeKindClient:
		return node.Partition + "." + node.Name + ".client"
	default:
		return ""
	}
}

func tlsCertCreateCommand(node *topology.Node) string {
	if node.IsServer() {
		return fmt.Sprintf(`consul tls cert create -server -dc=%s -node=%s`, node.Datacenter, node.PodName())
	} else {
		return fmt.Sprintf(`consul tls cert create -client -dc=%s`, node.Datacenter)
	}
}

func (s *Sprawl) initTLS(ctx context.Context) error {
	for _, cluster := range s.topology.Clusters {

		var buf bytes.Buffer

		// Create the CA if not already done, and proceed to do all of the
		// consul CLI calls inside of a throwaway temp directory.
		buf.WriteString(`
if [[ ! -f consul-agent-ca-key.pem || ! -f consul-agent-ca.pem ]]; then
	consul tls ca create
fi
rm -rf tmp
mkdir -p tmp
cp -a consul-agent-ca-key.pem consul-agent-ca.pem tmp
cd tmp
`)

		for _, node := range cluster.Nodes {
			if !node.IsAgent() || node.Disabled {
				continue
			}

			node.TLSCertPrefix = tlsPrefixFromNode(node)
			if node.TLSCertPrefix == "" {
				continue
			}

			expectPrefix := cluster.Datacenter + "-" + string(node.Kind) + "-consul-0"

			// Conditionally generate these in isolation and rename them to
			// not rely upon the numerical indexing.
			buf.WriteString(fmt.Sprintf(`
if [[ ! -f %[1]s || ! -f %[2]s ]]; then
	rm -f %[3]s %[4]s
	%[5]s
	mv -f %[3]s %[1]s
	mv -f %[4]s %[2]s
fi
`,
				"../"+node.TLSCertPrefix+"-key.pem", "../"+node.TLSCertPrefix+".pem",
				expectPrefix+"-key.pem", expectPrefix+".pem",
				tlsCertCreateCommand(node),
			))
		}

		err := s.runner.DockerExec(ctx, []string{
			"run",
			"--rm",
			"-i",
			"--net=none",
			"-v", cluster.TLSVolumeName + ":/data",
			"busybox:latest",
			"sh", "-c",
			// Need this so the permissions stick; docker seems to treat unused volumes differently.
			`touch /data/VOLUME_PLACEHOLDER && chown -R ` + consulUserArg + ` /data`,
		}, io.Discard, nil)
		if err != nil {
			return fmt.Errorf("could not initialize docker volume for cert data %q: %w", cluster.TLSVolumeName, err)
		}

		err = s.runner.DockerExec(ctx, []string{"run",
			"--rm",
			"-i",
			"--net=none",
			"-u", consulUserArg,
			"-v", cluster.TLSVolumeName + ":/data",
			"-w", "/data",
			"--entrypoint", "",
			cluster.Images.Consul,
			"/bin/sh", "-ec", buf.String(),
		}, io.Discard, nil)
		if err != nil {
			return fmt.Errorf("could not create all necessary TLS certificates in docker volume: %v", err)
		}
	}

	return nil
}
