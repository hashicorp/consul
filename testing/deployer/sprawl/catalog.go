// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/hashicorp/consul/testing/deployer/util"
)

// registerAllServicesToAgents registers services in agent-ful mode
func (s *Sprawl) registerAllServicesToAgents() error {
	for _, cluster := range s.topology.Clusters {
		if err := s.registerServicesToAgents(cluster); err != nil {
			return fmt.Errorf("registerServicesToAgents[%s]: %w", cluster.Name, err)
		}
	}
	return nil
}

func (s *Sprawl) syncAllServicesForDataplaneInstances() error {
	for _, cluster := range s.topology.Clusters {
		if err := s.syncWorkloadsForDataplaneInstances(cluster); err != nil {
			return fmt.Errorf("syncWorkloadsForDataplaneInstances[%s]: %w", cluster.Name, err)
		}
	}
	return nil
}

func (s *Sprawl) registerServicesToAgents(cluster *topology.Cluster) error {
	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Workloads) == 0 || node.Disabled {
			continue
		}

		if !node.IsAgent() {
			continue
		}

		agentClient, err := util.ProxyAPIClient(
			node.LocalProxyPort(),
			node.LocalAddress(),
			8500,
			"", /*token will be in request*/
		)
		if err != nil {
			return err
		}

		for _, wrk := range node.Workloads {
			if err := s.registerAgentService(agentClient, cluster, node, wrk); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Sprawl) registerAgentService(
	agentClient *api.Client,
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) error {
	if !node.IsAgent() {
		panic("called wrong method type")
	}

	if wrk.IsMeshGateway {
		return nil // handled at startup time for agent-ful, but won't be for agent-less
	}

	var (
		logger = s.logger.With("cluster", cluster.Name)
	)

	reg := &api.AgentServiceRegistration{
		ID:   wrk.ID.Name,
		Name: wrk.ID.Name,
		Port: wrk.Port,
		Meta: wrk.Meta,
	}
	if cluster.Enterprise {
		reg.Namespace = wrk.ID.Namespace
		reg.Partition = wrk.ID.Partition
	}

	if !wrk.DisableServiceMesh {
		var upstreams []api.Upstream
		for _, us := range wrk.Upstreams {
			uAPI := api.Upstream{
				DestinationPeer:  us.Peer,
				DestinationName:  us.ID.Name,
				LocalBindAddress: us.LocalAddress,
				LocalBindPort:    us.LocalPort,
				// Config               map[string]interface{} `json:",omitempty" bexpr:"-"`
				// MeshGateway          MeshGatewayConfig      `json:",omitempty"`
			}
			if cluster.Enterprise {
				uAPI.DestinationNamespace = us.ID.Namespace
				if us.Peer == "" {
					uAPI.DestinationPartition = us.ID.Partition
				}
			}
			upstreams = append(upstreams, uAPI)
		}
		reg.Connect = &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: upstreams,
				},
			},
		}
	}

	switch {
	case wrk.CheckTCP != "":
		chk := &api.AgentServiceCheck{
			Name:     "up",
			TCP:      wrk.CheckTCP,
			Interval: "5s",
			Timeout:  "1s",
		}
		reg.Checks = append(reg.Checks, chk)
	case wrk.CheckHTTP != "":
		chk := &api.AgentServiceCheck{
			Name:     "up",
			HTTP:     wrk.CheckHTTP,
			Method:   "GET",
			Interval: "5s",
			Timeout:  "1s",
		}
		reg.Checks = append(reg.Checks, chk)
	}

	// Switch token for every request.
	hdr := make(http.Header)
	hdr.Set("X-Consul-Token", s.secrets.ReadWorkloadToken(cluster.Name, wrk.ID))
	agentClient.SetHeaders(hdr)

RETRY:
	if err := agentClient.Agent().ServiceRegister(reg); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("failed to register workload %q to node %q: %w", wrk.ID, node.ID(), err)
	}

	logger.Debug("registered workload to client agent",
		"workload", wrk.ID.Name,
		"node", node.Name,
		"namespace", wrk.ID.Namespace,
		"partition", wrk.ID.Partition,
	)

	return nil
}

// syncWorkloadsForDataplaneInstances register/deregister services in the given cluster
func (s *Sprawl) syncWorkloadsForDataplaneInstances(cluster *topology.Cluster) error {
	// registerWorkloadToNode is called when node is not disabled
	registerWorkloadToNode := func(node *topology.Node, wrk *topology.Workload) error {
		if err := s.registerCatalogServiceV1(cluster, node, wrk); err != nil {
			return fmt.Errorf("error registering service: %w", err)
		}
		if !wrk.DisableServiceMesh {
			if err := s.registerCatalogSidecarServiceV1(cluster, node, wrk); err != nil {
				return fmt.Errorf("error registering sidecar service: %w", err)
			}
		}
		return nil
	}

	// deregisterWorkloadFromNode is called when node is disabled
	deregisterWorkloadFromNode := func(node *topology.Node, wrk *topology.Workload) error {
		if err := s.deregisterCatalogServiceV1(cluster, node, wrk); err != nil {
			return fmt.Errorf("error deregistering service: %w", err)
		}
		if !wrk.DisableServiceMesh {
			if err := s.deregisterCatalogSidecarServiceV1(cluster, node, wrk); err != nil {
				return fmt.Errorf("error deregistering sidecar service: %w", err)
			}
		}
		return nil
	}

	var syncWorkload func(node *topology.Node, wrk *topology.Workload) error

	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Workloads) == 0 {
			continue
		}

		if !node.IsDataplane() {
			continue
		}

		// Register virtual node service first if node is not disabled
		if !node.Disabled {
			if err := s.registerCatalogNode(cluster, node); err != nil {
				return fmt.Errorf("error registering virtual node: %w", err)
			}
		}

		// Register/deregister services on the node
		for _, wrk := range node.Workloads {
			if !node.Disabled {
				syncWorkload = registerWorkloadToNode
			} else {
				syncWorkload = deregisterWorkloadFromNode
			}
			if err := syncWorkload(node, wrk); err != nil {
				return err
			}
		}

		// Deregister the virtual node if node is disabled
		if node.Disabled {
			if err := s.deregisterCatalogNode(cluster, node); err != nil {
				return fmt.Errorf("error deregistering virtual node: %w", err)
			}
		}
	}

	return nil
}

func (s *Sprawl) registerCatalogNode(
	cluster *topology.Cluster,
	node *topology.Node,
) error {
	return s.registerCatalogNodeV1(cluster, node)
}

func (s *Sprawl) deregisterCatalogNode(
	cluster *topology.Cluster,
	node *topology.Node,
) error {
	return s.deregisterCatalogNodeV1(cluster, node)
}

func (s *Sprawl) writeResource(cluster *topology.Cluster, res *pbresource.Resource) (*pbresource.Resource, error) {
	var (
		client = s.getResourceClient(cluster.Name)
		logger = s.logger.With("cluster", cluster.Name)
	)

	ctx := s.getManagementTokenContext(context.Background(), cluster.Name)
RETRY:
	wrote, err := client.Write(ctx, &pbresource.WriteRequest{
		Resource: res,
	})
	if err != nil {
		if isACLNotFound(err) { // TODO: is this right for v2?
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return nil, fmt.Errorf("error creating resource %s: %w", util.IDToString(res.Id), err)
	}

	logger.Info("resource upserted", "id", util.IDToString(res.Id))
	return wrote.Resource, nil
}

func (s *Sprawl) registerCatalogNodeV1(
	cluster *topology.Cluster,
	node *topology.Node,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	reg := &api.CatalogRegistration{
		Node:    node.PodName(),
		Address: node.LocalAddress(),
		NodeMeta: map[string]string{
			"dataplane-faux": "1",
		},
	}
	if cluster.Enterprise {
		reg.Partition = node.Partition
	}

	// register synthetic node
RETRY:
	if _, err := client.Catalog().Register(reg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error registering virtual node %s: %w", node.ID(), err)
	}

	logger.Debug("virtual node created",
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) deregisterCatalogNodeV1(
	cluster *topology.Cluster,
	node *topology.Node,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	dereg := &api.CatalogDeregistration{
		Node:    node.PodName(),
		Address: node.LocalAddress(),
	}
	if cluster.Enterprise {
		dereg.Partition = node.Partition
	}

	// deregister synthetic node
RETRY:
	if _, err := client.Catalog().Deregister(dereg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error deregistering virtual node %s: %w", node.ID(), err)
	}

	logger.Info("virtual node removed",
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) deregisterCatalogServiceV1(
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	dereg := &api.CatalogDeregistration{
		Node:      node.PodName(),
		ServiceID: wrk.ID.Name,
	}
RETRY:
	if _, err := client.Catalog().Deregister(dereg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error deregistering service %s at node %s: %w", wrk.ID, node.ID(), err)
	}

	logger.Info("dataplane service removed",
		"service", wrk.ID,
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) registerCatalogServiceV1(
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	reg := workloadToCatalogRegistration(cluster, node, wrk)

RETRY:
	if _, err := client.Catalog().Register(reg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error registering service %s to node %s: %w", wrk.ID, node.ID(), err)
	}

	logger.Debug("dataplane service created",
		"service", wrk.ID,
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) deregisterCatalogSidecarServiceV1(
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}
	if wrk.DisableServiceMesh {
		panic("not valid")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	pid := wrk.ID
	pid.Name += "-sidecar-proxy"
	dereg := &api.CatalogDeregistration{
		Node:      node.PodName(),
		ServiceID: pid.Name,
	}

RETRY:
	if _, err := client.Catalog().Deregister(dereg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error deregistering service %s to node %s: %w", wrk.ID, node.ID(), err)
	}

	logger.Info("dataplane sidecar service removed",
		"service", pid,
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) registerCatalogSidecarServiceV1(
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}
	if wrk.DisableServiceMesh {
		panic("not valid")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	pid, reg := workloadToSidecarCatalogRegistration(cluster, node, wrk)
RETRY:
	if _, err := client.Catalog().Register(reg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error registering service %s to node %s: %w", wrk.ID, node.ID(), err)
	}

	logger.Debug("dataplane sidecar service created",
		"service", pid,
		"node", node.ID(),
	)

	return nil
}

type Resource[V proto.Message] struct {
	Resource *pbresource.Resource
	Data     V
}

func (r *Resource[V]) Build() (*pbresource.Resource, error) {
	anyData, err := anypb.New(r.Data)
	if err != nil {
		return nil, err
	}
	r.Resource.Data = anyData
	return r.Resource, nil
}

func workloadToCatalogRegistration(
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) *api.CatalogRegistration {
	reg := &api.CatalogRegistration{
		Node:           node.PodName(),
		SkipNodeUpdate: true,
		Service: &api.AgentService{
			Kind:    api.ServiceKindTypical,
			ID:      wrk.ID.Name,
			Service: wrk.ID.Name,
			Meta:    wrk.Meta,
			Port:    wrk.Port,
			Address: node.LocalAddress(),
		},
	}
	if wrk.IsMeshGateway {
		reg.Service.Kind = api.ServiceKindMeshGateway
		reg.Service.Proxy = &api.AgentServiceConnectProxyConfig{
			Config: map[string]interface{}{
				"envoy_gateway_no_default_bind":       true,
				"envoy_gateway_bind_tagged_addresses": true,
			},
			MeshGateway: api.MeshGatewayConfig{
				Mode: api.MeshGatewayModeLocal,
			},
		}
	}
	if node.HasPublicAddress() {
		reg.TaggedAddresses = map[string]string{
			"lan":      node.LocalAddress(),
			"lan_ipv4": node.LocalAddress(),
			"wan":      node.PublicAddress(),
			"wan_ipv4": node.PublicAddress(),
		}
		// TODO: not sure what the difference is between these, but with just the
		// top-level set, it appeared to not get set in either :/
		reg.Service.TaggedAddresses = map[string]api.ServiceAddress{
			"lan": {
				Address: node.LocalAddress(),
				Port:    wrk.Port,
			},
			"lan_ipv4": {
				Address: node.LocalAddress(),
				Port:    wrk.Port,
			},
			"wan": {
				Address: node.PublicAddress(),
				Port:    wrk.Port,
			},
			"wan_ipv4": {
				Address: node.PublicAddress(),
				Port:    wrk.Port,
			},
		}
	}
	if cluster.Enterprise {
		reg.Partition = wrk.ID.Partition
		reg.Service.Namespace = wrk.ID.Namespace
		reg.Service.Partition = wrk.ID.Partition
	}

	if wrk.HasCheck() {
		chk := &api.HealthCheck{
			Name: "external sync",
			// Type:      "external-sync",
			Status:      "passing", //  TODO
			ServiceID:   wrk.ID.Name,
			ServiceName: wrk.ID.Name,
			Output:      "",
		}
		if cluster.Enterprise {
			chk.Namespace = wrk.ID.Namespace
			chk.Partition = wrk.ID.Partition
		}
		switch {
		case wrk.CheckTCP != "":
			chk.Definition.TCP = wrk.CheckTCP
		case wrk.CheckHTTP != "":
			chk.Definition.HTTP = wrk.CheckHTTP
			chk.Definition.Method = "GET"
		}
		reg.Checks = append(reg.Checks, chk)
	}
	return reg
}

func workloadToSidecarCatalogRegistration(
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) (topology.ID, *api.CatalogRegistration) {
	pid := wrk.ID
	pid.Name += "-sidecar-proxy"
	reg := &api.CatalogRegistration{
		Node:           node.PodName(),
		SkipNodeUpdate: true,
		Service: &api.AgentService{
			Kind:    api.ServiceKindConnectProxy,
			ID:      pid.Name,
			Service: pid.Name,
			Meta:    wrk.Meta,
			Port:    wrk.EnvoyPublicListenerPort,
			Address: node.LocalAddress(),
			Proxy: &api.AgentServiceConnectProxyConfig{
				DestinationServiceName: wrk.ID.Name,
				DestinationServiceID:   wrk.ID.Name,
				LocalServicePort:       wrk.Port,
			},
		},
		Checks: []*api.HealthCheck{{
			Name: "external sync",
			// Type:      "external-sync",
			Status:      "passing", //  TODO
			ServiceID:   pid.Name,
			ServiceName: pid.Name,
			Definition: api.HealthCheckDefinition{
				TCP: fmt.Sprintf("%s:%d", node.LocalAddress(), wrk.EnvoyPublicListenerPort),
			},
			Output: "",
		}},
	}
	if node.HasPublicAddress() {
		reg.TaggedAddresses = map[string]string{
			"lan":      node.LocalAddress(),
			"lan_ipv4": node.LocalAddress(),
			"wan":      node.PublicAddress(),
			"wan_ipv4": node.PublicAddress(),
		}
	}
	if cluster.Enterprise {
		reg.Partition = pid.Partition
		reg.Service.Namespace = pid.Namespace
		reg.Service.Partition = pid.Partition
		reg.Checks[0].Namespace = pid.Namespace
		reg.Checks[0].Partition = pid.Partition
	}

	for _, us := range wrk.Upstreams {
		pu := api.Upstream{
			DestinationName:  us.ID.Name,
			DestinationPeer:  us.Peer,
			LocalBindAddress: us.LocalAddress,
			LocalBindPort:    us.LocalPort,
		}
		if cluster.Enterprise {
			pu.DestinationNamespace = us.ID.Namespace
			if us.Peer == "" {
				pu.DestinationPartition = us.ID.Partition
			}
		}
		reg.Service.Proxy.Upstreams = append(reg.Service.Proxy.Upstreams, pu)
	}

	return pid, reg
}
