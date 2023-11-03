// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"strconv"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

const HashicorpDockerProxy = "docker.mirror.hashicorp.services"

func NewFortioServiceWithDefaults(
	cluster string,
	sid topology.ServiceID,
	nodeVersion topology.NodeVersion,
	mut func(s *topology.Service),
) *topology.Service {
	const (
		httpPort  = 8080
		grpcPort  = 8079
		adminPort = 19000
	)
	sid.Normalize()

	svc := &topology.Service{
		ID:             sid,
		Image:          HashicorpDockerProxy + "/fortio/fortio",
		EnvoyAdminPort: adminPort,
		CheckTCP:       "127.0.0.1:" + strconv.Itoa(httpPort),
		Env: []string{
			"FORTIO_NAME=" + cluster + "::" + sid.String(),
		},
		Command: []string{
			"server",
			"-http-port", strconv.Itoa(httpPort),
			"-grpc-port", strconv.Itoa(grpcPort),
			"-redirect-port", "-disabled",
		},
	}

	if nodeVersion == topology.NodeVersionV2 {
		svc.Ports = map[string]*topology.Port{
			// TODO(rb/v2): once L7 works in v2 switch these back
			"http":     {Number: httpPort, Protocol: "tcp"},
			"http-alt": {Number: httpPort, Protocol: "tcp"},
			"grpc":     {Number: grpcPort, Protocol: "tcp"},
			// "http":     {Number: httpPort, Protocol: "http"},
			// "http-alt": {Number: httpPort, Protocol: "http"},
			// "grpc":     {Number: grpcPort, Protocol: "grpc"},
		}
	} else {
		svc.Port = httpPort
	}

	if mut != nil {
		mut(svc)
	}
	return svc
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
	for i := 1; i <= num; i++ {
		name := namePrefix + strconv.Itoa(i)

		node := &topology.Node{
			Kind:      nodeKind,
			Partition: partition,
			Name:      name,
			Services: []*topology.Service{{
				ID:             topology.ServiceID{Name: "mesh-gateway"},
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
