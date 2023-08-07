// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfgen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/runner"
	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/secrets"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/hashicorp/consul/testing/deployer/util"
)

type Generator struct {
	logger   hclog.Logger
	runner   *runner.Runner
	topology *topology.Topology
	sec      *secrets.Store
	workdir  string
	license  string

	tfLogger io.Writer

	// set during network phase
	remainingSubnets map[string]struct{}

	launched bool
}

func NewGenerator(
	logger hclog.Logger,
	runner *runner.Runner,
	topo *topology.Topology,
	sec *secrets.Store,
	workdir string,
	license string,
) (*Generator, error) {
	if logger == nil {
		panic("logger is required")
	}
	if runner == nil {
		panic("runner is required")
	}
	if topo == nil {
		panic("topology is required")
	}
	if sec == nil {
		panic("secrets store is required")
	}
	if workdir == "" {
		panic("workdir is required")
	}

	g := &Generator{
		logger:  logger,
		runner:  runner,
		sec:     sec,
		workdir: workdir,
		license: license,

		tfLogger: logger.Named("terraform").StandardWriter(&hclog.StandardLoggerOptions{
			ForceLevel: hclog.Info,
		}),
	}
	g.SetTopology(topo)

	_ = g.terraformDestroy(context.Background(), true) // cleanup prior run

	return g, nil
}

func (g *Generator) MarkLaunched() {
	g.launched = true
}

func (g *Generator) SetTopology(topo *topology.Topology) {
	if topo == nil {
		panic("topology is required")
	}
	g.topology = topo
}

type Step int

const (
	StepAll      Step = 0
	StepNetworks Step = 1
	StepServers  Step = 2
	StepAgents   Step = 3
	StepServices Step = 4
	// StepPeering  Step = XXX5
	StepRelaunch Step = 5
)

func (s Step) String() string {
	switch s {
	case StepAll:
		return "all"
	case StepNetworks:
		return "networks"
	case StepServers:
		return "servers"
	case StepAgents:
		return "agents"
	case StepServices:
		return "services"
	case StepRelaunch:
		return "relaunch"
	// case StepPeering:
	// 	return "peering"
	default:
		return "UNKNOWN--" + strconv.Itoa(int(s))
	}
}

func (s Step) StartServers() bool  { return s >= StepServers }
func (s Step) StartAgents() bool   { return s >= StepAgents }
func (s Step) StartServices() bool { return s >= StepServices }

// func (s Step) InitiatePeering() bool { return s >= StepPeering }

func (g *Generator) Regenerate() error {
	return g.Generate(StepRelaunch)
}

func (g *Generator) Generate(step Step) error {
	if g.launched && step != StepRelaunch {
		return fmt.Errorf("cannot use step %q after successful launch; see Regenerate()", step)
	}

	g.logger.Info("generating and creating resources", "step", step.String())
	var (
		networks   []Resource
		volumes    []Resource
		images     []Resource
		containers []Resource

		imageNames = make(map[string]string)
	)

	addVolume := func(name string) {
		volumes = append(volumes, DockerVolume(name))
	}
	addImage := func(name, image string) {
		if image == "" {
			return
		}
		if _, ok := imageNames[image]; ok {
			return
		}

		if name == "" {
			name = DockerImageResourceName(image)
		}

		imageNames[image] = name

		g.logger.Info("registering image", "resource", name, "image", image)

		images = append(images, DockerImage(name, image))
	}

	if g.remainingSubnets == nil {
		g.remainingSubnets = util.GetPossibleDockerNetworkSubnets()
	}
	if len(g.remainingSubnets) == 0 {
		return fmt.Errorf("exhausted all docker networks")
	}

	addImage("nginx", "nginx:latest")
	addImage("coredns", "coredns/coredns:latest")
	for _, net := range g.topology.SortedNetworks() {
		if net.Subnet == "" {
			// Because this harness runs on a linux or macos host, we can't
			// directly invoke the moby libnetwork calls to check for free
			// subnets as it would have to cross into the docker desktop vm on
			// mac.
			//
			// Instead rely on map iteration order being random to avoid
			// collisions, but detect the terraform failure and retry until
			// success.

			var ipnet string
			for ipnet = range g.remainingSubnets {
			}
			if ipnet == "" {
				return fmt.Errorf("could not get a free docker network")
			}
			delete(g.remainingSubnets, ipnet)

			if _, err := net.SetSubnet(ipnet); err != nil {
				return fmt.Errorf("assigned subnet is invalid %q: %w", ipnet, err)
			}
		}
		networks = append(networks, DockerNetwork(net.DockerName, net.Subnet))

		var (
			// We always ask for a /24, so just blindly pick x.x.x.252 as our
			// proxy address. There's an offset of 2 in the list of available
			// addresses here because we removed x.x.x.0 and x.x.x.1 from the
			// pool.
			proxyIPAddress = net.IPByIndex(250)
			// Grab x.x.x.253 for the dns server
			dnsIPAddress = net.IPByIndex(251)
		)

		{
			// wrote, hashes, err := g.write
		}

		{ // nginx forward proxy
			_, hash, err := g.writeNginxConfig(net)
			if err != nil {
				return fmt.Errorf("writeNginxConfig[%s]: %w", net.Name, err)
			}

			containers = append(containers, g.getForwardProxyContainer(net, proxyIPAddress, hash))

		}

		net.ProxyAddress = proxyIPAddress
		net.DNSAddress = ""

		if net.IsLocal() {
			wrote, hashes, err := g.writeCoreDNSFiles(net, dnsIPAddress)
			if err != nil {
				return fmt.Errorf("writeCoreDNSFiles[%s]: %w", net.Name, err)
			}
			if wrote {
				net.DNSAddress = dnsIPAddress
				containers = append(containers, g.getCoreDNSContainer(net, dnsIPAddress, hashes))
			}
		}
	}

	for _, c := range g.topology.SortedClusters() {
		if c.TLSVolumeName == "" {
			c.TLSVolumeName = c.Name + "-tls-material-" + g.topology.ID
		}
		addVolume(c.TLSVolumeName)
	}

	addImage("pause", "registry.k8s.io/pause:3.3")

	if step.StartServers() {
		for _, c := range g.topology.SortedClusters() {
			for _, node := range c.SortedNodes() {
				if node.Disabled {
					continue
				}
				addImage("", node.Images.Consul)
				addImage("", node.Images.EnvoyConsulImage())
				addImage("", node.Images.LocalDataplaneImage())

				if node.IsAgent() {
					addVolume(node.DockerName())
				}

				for _, svc := range node.Services {
					addImage("", svc.Image)
				}

				myContainers, err := g.generateNodeContainers(step, c, node)
				if err != nil {
					return err
				}

				containers = append(containers, myContainers...)
			}
		}
	}

	tfpath := func(p string) string {
		return filepath.Join(g.workdir, "terraform", p)
	}

	if _, err := WriteHCLResourceFile(g.logger, []Resource{Text(terraformPrelude)}, tfpath("init.tf"), 0644); err != nil {
		return err
	}
	if netResult, err := WriteHCLResourceFile(g.logger, networks, tfpath("networks.tf"), 0644); err != nil {
		return err
	} else if netResult == UpdateResultModified {
		if step != StepNetworks {
			return fmt.Errorf("cannot change networking details after they are established")
		}
	}
	if _, err := WriteHCLResourceFile(g.logger, volumes, tfpath("volumes.tf"), 0644); err != nil {
		return err
	}
	if _, err := WriteHCLResourceFile(g.logger, images, tfpath("images.tf"), 0644); err != nil {
		return err
	}
	if _, err := WriteHCLResourceFile(g.logger, containers, tfpath("containers.tf"), 0644); err != nil {
		return err
	}

	if err := g.terraformApply(context.TODO()); err != nil {
		return err
	}

	out, err := g.terraformOutputs(context.TODO())
	if err != nil {
		return err
	}

	return g.digestOutputs(out)
}

func (g *Generator) DestroyAll() error {
	return g.terraformDestroy(context.TODO(), false)
}

func (g *Generator) DestroyAllQuietly() error {
	return g.terraformDestroy(context.TODO(), true)
}

func (g *Generator) terraformApply(ctx context.Context) error {
	tfdir := filepath.Join(g.workdir, "terraform")

	if _, err := os.Stat(filepath.Join(tfdir, ".terraform")); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// On the fly init
		g.logger.Info("Running 'terraform init'...")
		if err := g.runner.TerraformExec(ctx, []string{"init", "-input=false"}, g.tfLogger, tfdir); err != nil {
			return err
		}
	}

	g.logger.Info("Running 'terraform apply'...")
	return g.runner.TerraformExec(ctx, []string{"apply", "-input=false", "-auto-approve"}, g.tfLogger, tfdir)
}

func (g *Generator) terraformDestroy(ctx context.Context, quiet bool) error {
	g.logger.Info("Running 'terraform destroy'...")

	var out io.Writer
	if quiet {
		out = io.Discard
	} else {
		out = g.tfLogger
	}

	tfdir := filepath.Join(g.workdir, "terraform")
	return g.runner.TerraformExec(ctx, []string{
		"destroy", "-input=false", "-auto-approve", "-refresh=false",
	}, out, tfdir)
}

func (g *Generator) terraformOutputs(ctx context.Context) (*Outputs, error) {
	tfdir := filepath.Join(g.workdir, "terraform")

	var buf bytes.Buffer
	err := g.runner.TerraformExec(ctx, []string{
		"output", "-json",
	}, &buf, tfdir)
	if err != nil {
		return nil, err
	}

	type outputVar struct {
		// may be map[string]any
		Value any `json:"value"`
	}

	raw := make(map[string]*outputVar)
	dec := json.NewDecoder(&buf)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}

	out := &Outputs{}

	for key, rv := range raw {
		switch {
		case strings.HasPrefix(key, "ports_"):
			cluster, nid, ok := extractNodeOutputKey("ports_", key)
			if !ok {
				return nil, fmt.Errorf("unexpected output var: %s", key)
			}

			ports := make(map[int]int)
			for k, v := range rv.Value.(map[string]any) {
				ki, err := strconv.Atoi(k)
				if err != nil {
					return nil, fmt.Errorf("unexpected port value %q: %w", k, err)
				}
				ports[ki] = int(v.(float64))
			}
			out.SetNodePorts(cluster, nid, ports)
		case strings.HasPrefix(key, "forwardproxyport_"):
			netname := strings.TrimPrefix(key, "forwardproxyport_")

			found := rv.Value.(map[string]any)
			if len(found) != 1 {
				return nil, fmt.Errorf("found unexpected ports: %v", found)
			}
			got, ok := found[strconv.Itoa(proxyInternalPort)]
			if !ok {
				return nil, fmt.Errorf("found unexpected ports: %v", found)
			}

			out.SetProxyPort(netname, int(got.(float64)))
		}
	}

	return out, nil
}

func extractNodeOutputKey(prefix, key string) (string, topology.NodeID, bool) {
	clusterNode := strings.TrimPrefix(key, prefix)

	cluster, nodeid, ok := strings.Cut(clusterNode, "_")
	if !ok {
		return "", topology.NodeID{}, false
	}

	partition, node, ok := strings.Cut(nodeid, "_")
	if !ok {
		return "", topology.NodeID{}, false
	}

	nid := topology.NewNodeID(node, partition)
	return cluster, nid, true
}

type Outputs struct {
	ProxyPorts map[string]int                             // net -> exposed port
	Nodes      map[string]map[topology.NodeID]*NodeOutput // clusterID -> node -> stuff
}

func (o *Outputs) SetNodePorts(cluster string, nid topology.NodeID, ports map[int]int) {
	nodeOut := o.getNode(cluster, nid)
	nodeOut.Ports = ports
}

func (o *Outputs) SetProxyPort(net string, port int) {
	if o.ProxyPorts == nil {
		o.ProxyPorts = make(map[string]int)
	}
	o.ProxyPorts[net] = port
}

func (o *Outputs) getNode(cluster string, nid topology.NodeID) *NodeOutput {
	if o.Nodes == nil {
		o.Nodes = make(map[string]map[topology.NodeID]*NodeOutput)
	}
	cnodes, ok := o.Nodes[cluster]
	if !ok {
		cnodes = make(map[topology.NodeID]*NodeOutput)
		o.Nodes[cluster] = cnodes
	}

	nodeOut, ok := cnodes[nid]
	if !ok {
		nodeOut = &NodeOutput{}
		cnodes[nid] = nodeOut
	}

	return nodeOut
}

type NodeOutput struct {
	Ports map[int]int `json:",omitempty"`
}
