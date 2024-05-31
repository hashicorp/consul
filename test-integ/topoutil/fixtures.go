// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

const HashicorpDockerProxy = "docker.mirror.hashicorp.services"

func NewFortioWorkloadWithDefaults(
	cluster string,
	sid topology.ID,
	mut func(*topology.Workload),
) *topology.Workload {
	const (
		httpPort  = 8080
		grpcPort  = 8079
		tcpPort   = 8078
		adminPort = 19000
	)
	sid.Normalize()

	wrk := &topology.Workload{
		ID:             sid,
		Image:          HashicorpDockerProxy + "/fortio/fortio",
		EnvoyAdminPort: adminPort,
		Port:           httpPort,
		CheckTCP:       "127.0.0.1:" + strconv.Itoa(httpPort),
		Env: []string{
			"FORTIO_NAME=" + cluster + "::" + sid.String(),
		},
		Command: []string{
			"server",
			"-http-port", strconv.Itoa(httpPort),
			"-grpc-port", strconv.Itoa(grpcPort),
			"-tcp-port", strconv.Itoa(tcpPort),
			"-redirect-port", "-disabled",
		},
	}

	if mut != nil {
		mut(wrk)
	}
	return wrk
}

func NewBlankspaceWorkloadWithDefaults(
	cluster string,
	sid topology.ID,
	mut func(*topology.Workload),
) *topology.Workload {
	const (
		httpPort  = 8080
		grpcPort  = 8079
		tcpPort   = 8078
		adminPort = 19000
	)
	sid.Normalize()

	wrk := &topology.Workload{
		ID:             sid,
		Image:          HashicorpDockerProxy + "/rboyer/blankspace",
		EnvoyAdminPort: adminPort,
		Port:           httpPort,
		CheckTCP:       "127.0.0.1:" + strconv.Itoa(httpPort),
		Command: []string{
			"-name", cluster + "::" + sid.String(),
			"-http-addr", fmt.Sprintf(":%d", httpPort),
			"-grpc-addr", fmt.Sprintf(":%d", grpcPort),
			"-tcp-addr", fmt.Sprintf(":%d", tcpPort),
		},
	}

	if mut != nil {
		mut(wrk)
	}
	return wrk
}

func NewTopologyServerSet(
	namePrefix string,
	num int,
	networks []string,
	mutateFn func(i int, node *topology.Node),
) []*topology.Node {
	var out []*topology.Node
	for i := 1; i <= num; i++ {
		name := namePrefix + strconv.Itoa(i)

		node := &topology.Node{
			Kind: topology.NodeKindServer,
			Name: name,
		}
		for _, net := range networks {
			node.Addresses = append(node.Addresses, &topology.Address{Network: net})
		}

		if mutateFn != nil {
			mutateFn(i, node)
		}

		out = append(out, node)
	}
	return out
}

func NewTopologyMeshGatewaySet(
	nodeKind topology.NodeKind,
	partition string,
	namePrefix string,
	num int,
	networks []string,
	mutateFn func(i int, node *topology.Node),
) []*topology.Node {
	var out []*topology.Node
	sid := topology.ID{
		Name:      "mesh-gateway",
		Partition: topology.DefaultToEmpty(partition),
	}
	for i := 1; i <= num; i++ {
		name := namePrefix + strconv.Itoa(i)

		node := &topology.Node{
			Kind:      nodeKind,
			Partition: sid.Partition,
			Name:      name,
			Workloads: []*topology.Workload{{
				ID:             sid,
				Port:           8443,
				EnvoyAdminPort: 19000,
				IsMeshGateway:  true,
			}},
		}
		for _, net := range networks {
			node.Addresses = append(node.Addresses, &topology.Address{Network: net})
		}

		if mutateFn != nil {
			mutateFn(i, node)
		}

		out = append(out, node)
	}
	return out
}

// Since CE config entries do not contain the partition field,
// this func converts default partition to empty string.
func ConfigEntryPartition(p string) string {
	if p == "default" {
		return "" // make this CE friendly
	}
	return p
}
