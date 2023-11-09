// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"fmt"
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
		tcpPort   = 8078
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
			"-tcp-port", strconv.Itoa(tcpPort),
			"-redirect-port", "-disabled",
		},
	}

	if nodeVersion == topology.NodeVersionV2 {
		svc.Ports = map[string]*topology.Port{
			"http":  {Number: httpPort, Protocol: "http"},
			"http2": {Number: httpPort, Protocol: "http2"},
			"grpc":  {Number: grpcPort, Protocol: "grpc"},
			"tcp":   {Number: tcpPort, Protocol: "tcp"},
		}
	} else {
		svc.Port = httpPort
	}

	if mut != nil {
		mut(svc)
	}
	return svc
}

func NewBlankspaceServiceWithDefaults(
	cluster string,
	sid topology.ServiceID,
	nodeVersion topology.NodeVersion,
	mut func(s *topology.Service),
) *topology.Service {
	const (
		httpPort  = 8080
		grpcPort  = 8079
		tcpPort   = 8078
		adminPort = 19000
	)
	sid.Normalize()

	svc := &topology.Service{
		ID:             sid,
		Image:          HashicorpDockerProxy + "/rboyer/blankspace",
		EnvoyAdminPort: adminPort,
		CheckTCP:       "127.0.0.1:" + strconv.Itoa(httpPort),
		Command: []string{
			"-name", cluster + "::" + sid.String(),
			"-http-addr", fmt.Sprintf(":%d", httpPort),
			"-grpc-addr", fmt.Sprintf(":%d", grpcPort),
			"-tcp-addr", fmt.Sprintf(":%d", tcpPort),
		},
	}

	if nodeVersion == topology.NodeVersionV2 {
		svc.Ports = map[string]*topology.Port{
			"http":  {Number: httpPort, Protocol: "http"},
			"http2": {Number: httpPort, Protocol: "http2"},
			"grpc":  {Number: grpcPort, Protocol: "grpc"},
			"tcp":   {Number: tcpPort, Protocol: "tcp"},
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
	sid := topology.ServiceID{
		Name:      "mesh-gateway",
		Partition: ConfigEntryPartition(partition),
	}
	for i := 1; i <= num; i++ {
		name := namePrefix + strconv.Itoa(i)

		node := &topology.Node{
			Kind:      nodeKind,
			Partition: sid.Partition,
			Name:      name,
			Services: []*topology.Service{{
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
