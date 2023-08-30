// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tfgen

import (
	"fmt"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

type terraformPod struct {
	PodName           string
	Node              *topology.Node
	Ports             []int
	Labels            map[string]string
	TLSVolumeName     string
	DNSAddress        string
	DockerNetworkName string
}

func (g *Generator) generateNodeContainers(
	step Step,
	cluster *topology.Cluster,
	node *topology.Node,
) ([]Resource, error) {
	if node.Disabled {
		return nil, fmt.Errorf("cannot generate containers for a disabled node")
	}

	pod := terraformPod{
		PodName: node.PodName(),
		Node:    node,
		Labels: map[string]string{
			"consulcluster-topology-id":  g.topology.ID,
			"consulcluster-cluster-name": node.Cluster,
		},
		TLSVolumeName: cluster.TLSVolumeName,
		DNSAddress:    "8.8.8.8",
	}

	cluster, ok := g.topology.Clusters[node.Cluster]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", node.Cluster)
	}

	net, ok := g.topology.Networks[cluster.NetworkName]
	if !ok {
		return nil, fmt.Errorf("no local network: %s", cluster.NetworkName)
	}
	if net.DNSAddress != "" {
		pod.DNSAddress = net.DNSAddress
	}
	pod.DockerNetworkName = net.DockerName

	containers := []Resource{}

	if node.IsAgent() {
		switch {
		case node.IsServer() && step.StartServers(),
			!node.IsServer() && step.StartAgents():
			containers = append(containers, Eval(tfConsulT, struct {
				terraformPod
				ImageResource     string
				HCL               string
				EnterpriseLicense string
			}{
				terraformPod:      pod,
				ImageResource:     DockerImageResourceName(node.Images.Consul),
				HCL:               g.generateAgentHCL(node),
				EnterpriseLicense: g.license,
			}))
		}
	}

	svcContainers := []Resource{}
	for _, svc := range node.SortedServices() {
		token := g.sec.ReadServiceToken(node.Cluster, svc.ID)
		switch {
		case svc.IsMeshGateway && node.IsDataplane():
			panic("NOT READY YET")

		case svc.IsMeshGateway && !node.IsDataplane():
			tfin := struct {
				terraformPod
				ImageResource string
				Enterprise    bool
				Service       *topology.Service
				Token         string
			}{
				terraformPod:  pod,
				Enterprise:    cluster.Enterprise,
				ImageResource: DockerImageResourceName(node.Images.EnvoyConsulImage()),
				Service:       svc,
				Token:         token,
			}
			svcContainers = append(svcContainers, Eval(tfMeshGatewayT, &tfin))

		case !svc.IsMeshGateway:
			svcContainers = append(svcContainers, Eval(tfAppT, struct {
				terraformPod
				ImageResource string
				Service       *topology.Service
			}{
				terraformPod:  pod,
				ImageResource: DockerImageResourceName(svc.Image),
				Service:       svc,
			}))

			if !svc.DisableServiceMesh {
				break
			}

			switch node.IsDataplane() {
			case false:
				svcContainers = append(svcContainers, Eval(tfAppSidecarT, struct {
					terraformPod
					ImageResource string
					Service       *topology.Service
					Token         string
					Enterprise    bool
					// TODO: we used to use the env from the app container, doubt we need it, seems leaky
					// Env                []string
				}{
					terraformPod:  pod,
					ImageResource: DockerImageResourceName(node.Images.EnvoyConsulImage()),
					Service:       svc,
					Token:         token,
					Enterprise:    cluster.Enterprise,
				}))

			case true:
				svcContainers = append(svcContainers, Eval(tfAppDataplaneT, &struct {
					terraformPod
					ImageResource string
					Token         string
				}{
					terraformPod:  pod,
					ImageResource: DockerImageResourceName(node.Images.LocalDataplaneImage()),
					Token:         token,
				}))
			}

		default:
			panic(fmt.Sprintf("unhandled node kind/dataplane type: %#v", svc))
		}

		if step.StartServices() {
			containers = append(containers, svcContainers...)
		}
	}

	// Wait until the very end to render the pod so we know all of the ports.
	pod.Ports = node.SortedPorts()

	// pod placeholder container
	containers = append(containers, Eval(tfPauseT, &pod))

	return containers, nil
}
