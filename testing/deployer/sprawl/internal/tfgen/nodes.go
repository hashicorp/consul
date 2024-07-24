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

	wrkContainers := []Resource{}
	for _, wrk := range node.SortedWorkloads() {
		token := g.sec.ReadWorkloadToken(node.Cluster, wrk.ID)
		switch {
		case wrk.IsMeshGateway && !node.IsDataplane():
			wrkContainers = append(wrkContainers, Eval(tfMeshGatewayT, struct {
				terraformPod
				ImageResource string
				Enterprise    bool
				Workload      *topology.Workload
				Token         string
			}{
				terraformPod:  pod,
				ImageResource: DockerImageResourceName(node.Images.EnvoyConsulImage()),
				Enterprise:    cluster.Enterprise,
				Workload:      wrk,
				Token:         token,
			}))
		case wrk.IsMeshGateway && node.IsDataplane():
			wrkContainers = append(wrkContainers, Eval(tfMeshGatewayDataplaneT, &struct {
				terraformPod
				ImageResource string
				Enterprise    bool
				Workload      *topology.Workload
				Token         string
			}{
				terraformPod:  pod,
				ImageResource: DockerImageResourceName(node.Images.LocalDataplaneImage()),
				Enterprise:    cluster.Enterprise,
				Workload:      wrk,
				Token:         token,
			}))

		case !wrk.IsMeshGateway:
			wrkContainers = append(wrkContainers, Eval(tfAppT, struct {
				terraformPod
				ImageResource string
				Workload      *topology.Workload
			}{
				terraformPod:  pod,
				ImageResource: DockerImageResourceName(wrk.Image),
				Workload:      wrk,
			}))

			if wrk.DisableServiceMesh {
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
			wrkContainers = append(wrkContainers, Eval(tmpl, struct {
				terraformPod
				ImageResource string
				Workload      *topology.Workload
				Token         string
				Enterprise    bool
			}{
				terraformPod:  pod,
				ImageResource: img,
				Workload:      wrk,
				Token:         token,
				Enterprise:    cluster.Enterprise,
			}))
		}

		if step.StartServices() {
			containers = append(containers, wrkContainers...)
		}
	}

	// Wait until the very end to render the pod so we know all of the ports.
	pod.Ports = node.SortedPorts()

	// pod placeholder container
	containers = append(containers, Eval(tfPauseT, &pod))

	return containers, nil
}
