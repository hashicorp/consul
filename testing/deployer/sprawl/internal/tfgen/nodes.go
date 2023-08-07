// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfgen

import (
	"fmt"
	"sort"
	"strconv"
	"text/template"

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

type terraformConsulAgent struct {
	terraformPod
	ImageResource     string
	HCL               string
	EnterpriseLicense string
	Env               []string
}

type terraformMeshGatewayService struct {
	terraformPod
	EnvoyImageResource string
	Service            *topology.Service
	Command            []string
}

type terraformService struct {
	terraformPod
	AppImageResource       string
	EnvoyImageResource     string // agentful
	DataplaneImageResource string // agentless
	Service                *topology.Service
	Env                    []string
	Command                []string
	EnvoyCommand           []string // agentful
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

	var (
		containers []Resource
	)

	if node.IsAgent() {
		agentHCL, err := g.generateAgentHCL(node)
		if err != nil {
			return nil, err
		}

		agent := terraformConsulAgent{
			terraformPod:      pod,
			ImageResource:     DockerImageResourceName(node.Images.Consul),
			HCL:               agentHCL,
			EnterpriseLicense: g.license,
			Env:               node.AgentEnv,
		}

		switch {
		case node.IsServer() && step.StartServers(),
			!node.IsServer() && step.StartAgents():
			containers = append(containers, Eval(tfConsulT, &agent))
		}
	}

	for _, svc := range node.SortedServices() {
		if svc.IsMeshGateway {
			if node.Kind == topology.NodeKindDataplane {
				panic("NOT READY YET")
			}
			gw := terraformMeshGatewayService{
				terraformPod:       pod,
				EnvoyImageResource: DockerImageResourceName(node.Images.EnvoyConsulImage()),
				Service:            svc,
				Command: []string{
					"consul", "connect", "envoy",
					"-register",
					"-mesh-gateway",
				},
			}
			if token := g.sec.ReadServiceToken(node.Cluster, svc.ID); token != "" {
				gw.Command = append(gw.Command, "-token", token)
			}
			if cluster.Enterprise {
				gw.Command = append(gw.Command,
					"-partition",
					svc.ID.Partition,
				)
			}
			gw.Command = append(gw.Command,
				"-address",
				`{{ GetInterfaceIP \"eth0\" }}:`+strconv.Itoa(svc.Port),
				"-wan-address",
				`{{ GetInterfaceIP \"eth1\" }}:`+strconv.Itoa(svc.Port),
			)
			gw.Command = append(gw.Command,
				"-grpc-addr", "http://127.0.0.1:8502",
				"-admin-bind",
				// for demo purposes
				"0.0.0.0:"+strconv.Itoa(svc.EnvoyAdminPort),
				"--",
				"-l",
				"trace",
			)
			if step.StartServices() {
				containers = append(containers, Eval(tfMeshGatewayT, &gw))
			}
		} else {
			tfsvc := terraformService{
				terraformPod:     pod,
				AppImageResource: DockerImageResourceName(svc.Image),
				Service:          svc,
				Command:          svc.Command,
			}
			tfsvc.Env = append(tfsvc.Env, svc.Env...)
			if step.StartServices() {
				containers = append(containers, Eval(tfAppT, &tfsvc))
			}

			setenv := func(k, v string) {
				tfsvc.Env = append(tfsvc.Env, k+"="+v)
			}

			if !svc.DisableServiceMesh {
				if node.IsDataplane() {
					tfsvc.DataplaneImageResource = DockerImageResourceName(node.Images.LocalDataplaneImage())
					tfsvc.EnvoyImageResource = ""
					tfsvc.EnvoyCommand = nil
					// --- REQUIRED ---
					setenv("DP_CONSUL_ADDRESSES", "server."+node.Cluster+"-consulcluster.lan")
					setenv("DP_SERVICE_NODE_NAME", node.PodName())
					setenv("DP_PROXY_SERVICE_ID", svc.ID.Name+"-sidecar-proxy")
				} else {
					tfsvc.DataplaneImageResource = ""
					tfsvc.EnvoyImageResource = DockerImageResourceName(node.Images.EnvoyConsulImage())
					tfsvc.EnvoyCommand = []string{
						"consul", "connect", "envoy",
						"-sidecar-for", svc.ID.Name,
					}
				}
				if cluster.Enterprise {
					if node.IsDataplane() {
						setenv("DP_SERVICE_NAMESPACE", svc.ID.Namespace)
						setenv("DP_SERVICE_PARTITION", svc.ID.Partition)
					} else {
						tfsvc.EnvoyCommand = append(tfsvc.EnvoyCommand,
							"-partition",
							svc.ID.Partition,
							"-namespace",
							svc.ID.Namespace,
						)
					}
				}
				if token := g.sec.ReadServiceToken(node.Cluster, svc.ID); token != "" {
					if node.IsDataplane() {
						setenv("DP_CREDENTIAL_TYPE", "static")
						setenv("DP_CREDENTIAL_STATIC_TOKEN", token)
					} else {
						tfsvc.EnvoyCommand = append(tfsvc.EnvoyCommand, "-token", token)
					}
				}
				if node.IsDataplane() {
					setenv("DP_ENVOY_ADMIN_BIND_ADDRESS", "0.0.0.0") // for demo purposes
					setenv("DP_ENVOY_ADMIN_BIND_PORT", "19000")
					setenv("DP_LOG_LEVEL", "trace")

					setenv("DP_CA_CERTS", "/consul/config/certs/consul-agent-ca.pem")
					setenv("DP_CONSUL_GRPC_PORT", "8503")
					setenv("DP_TLS_SERVER_NAME", "server."+node.Datacenter+".consul")
				} else {
					tfsvc.EnvoyCommand = append(tfsvc.EnvoyCommand,
						"-grpc-addr", "http://127.0.0.1:8502",
						"-admin-bind",
						// for demo purposes
						"0.0.0.0:"+strconv.Itoa(svc.EnvoyAdminPort),
						"--",
						"-l",
						"trace",
					)
				}
				if step.StartServices() {
					sort.Strings(tfsvc.Env)

					if node.IsDataplane() {
						containers = append(containers, Eval(tfAppDataplaneT, &tfsvc))
					} else {
						containers = append(containers, Eval(tfAppSidecarT, &tfsvc))
					}
				}
			}
		}
	}

	// Wait until the very end to render the pod so we know all of the ports.
	pod.Ports = node.SortedPorts()

	// pod placeholder container
	containers = append(containers, Eval(tfPauseT, &pod))

	return containers, nil
}

var tfPauseT = template.Must(template.ParseFS(content, "templates/container-pause.tf.tmpl"))
var tfConsulT = template.Must(template.ParseFS(content, "templates/container-consul.tf.tmpl"))
var tfMeshGatewayT = template.Must(template.ParseFS(content, "templates/container-mgw.tf.tmpl"))
var tfAppT = template.Must(template.ParseFS(content, "templates/container-app.tf.tmpl"))
var tfAppSidecarT = template.Must(template.ParseFS(content, "templates/container-app-sidecar.tf.tmpl"))
var tfAppDataplaneT = template.Must(template.ParseFS(content, "templates/container-app-dataplane.tf.tmpl"))
