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
				HCL:               g.generateAgentHCL(node, cluster.EnableV2 && node.IsServer()),
				EnterpriseLicense: g.license,
			}))
		}
	}

	svcContainers := []Resource{}
	for _, svc := range node.SortedServices() {
		token := g.sec.ReadServiceToken(node.Cluster, svc.ID)
		switch {
		case svc.IsMeshGateway && !node.IsDataplane():
			svcContainers = append(svcContainers, Eval(tfMeshGatewayT, struct {
				terraformPod
				ImageResource string
				Enterprise    bool
				Service       *topology.Service
				Token         string
			}{
				terraformPod:  pod,
				ImageResource: DockerImageResourceName(node.Images.EnvoyConsulImage()),
				Enterprise:    cluster.Enterprise,
				Service:       svc,
				Token:         token,
			}))
		case svc.IsMeshGateway && node.IsDataplane():
			svcContainers = append(svcContainers, Eval(tfMeshGatewayDataplaneT, &struct {
				terraformPod
				ImageResource string
				Enterprise    bool
				Service       *topology.Service
				Token         string
			}{
				terraformPod:  pod,
				ImageResource: DockerImageResourceName(node.Images.LocalDataplaneImage()),
				Enterprise:    cluster.Enterprise,
				Service:       svc,
				Token:         token,
			}))

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

			if svc.DisableServiceMesh {
				break
			}

			tmpl := tfAppSidecarT
			var img string
			if node.IsDataplane() {
				tmpl = tfAppDataplaneT
				img = DockerImageResourceName(node.Images.LocalDataplaneImage())
			} else {
				img = DockerImageResourceName(node.Images.EnvoyConsulImage())
			}
			svcContainers = append(svcContainers, Eval(tmpl, struct {
				terraformPod
				ImageResource string
				Service       *topology.Service
				Token         string
				Enterprise    bool
			}{
				terraformPod:  pod,
				ImageResource: img,
				Service:       svc,
				Token:         token,
				Enterprise:    cluster.Enterprise,
			}))
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
