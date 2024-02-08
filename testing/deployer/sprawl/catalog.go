// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/consul/api"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

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
		if node.IsV2() {
			panic("don't call this")
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
	if node.IsV2() {
		panic("don't call this")
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
		for _, dest := range wrk.Destinations {
			uAPI := api.Upstream{
				DestinationPeer:  dest.Peer,
				DestinationName:  dest.ID.Name,
				LocalBindAddress: dest.LocalAddress,
				LocalBindPort:    dest.LocalPort,
				// Config               map[string]interface{} `json:",omitempty" bexpr:"-"`
				// MeshGateway          MeshGatewayConfig      `json:",omitempty"`
			}
			if cluster.Enterprise {
				uAPI.DestinationNamespace = dest.ID.Namespace
				if dest.Peer == "" {
					uAPI.DestinationPartition = dest.ID.Partition
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
	identityInfo := make(map[topology.ID]*Resource[*pbauth.WorkloadIdentity])

	// registerWorkloadToNode is called when node is not disabled
	registerWorkloadToNode := func(node *topology.Node, wrk *topology.Workload) error {
		if node.IsV2() {
			pending := workloadInstanceToResources(node, wrk)

			workloadID := topology.NewID(wrk.WorkloadIdentity, wrk.ID.Namespace, wrk.ID.Partition)
			if _, ok := identityInfo[workloadID]; !ok {
				identityInfo[workloadID] = pending.WorkloadIdentity
			}

			// Write workload
			res, err := pending.Workload.Build()
			if err != nil {
				return fmt.Errorf("error serializing resource %s: %w", util.IDToString(pending.Workload.Resource.Id), err)
			}
			workload, err := s.writeResource(cluster, res)
			if err != nil {
				return err
			}
			// Write check linked to workload
			for _, check := range pending.HealthStatuses {
				check.Resource.Owner = workload.Id
				res, err := check.Build()
				if err != nil {
					return fmt.Errorf("error serializing resource %s: %w", util.IDToString(check.Resource.Id), err)
				}
				if _, err := s.writeResource(cluster, res); err != nil {
					return err
				}
			}
			// maybe write destinations
			if pending.Destinations != nil {
				res, err := pending.Destinations.Build()
				if err != nil {
					return fmt.Errorf("error serializing resource %s: %w", util.IDToString(pending.Destinations.Resource.Id), err)
				}
				if _, err := s.writeResource(cluster, res); err != nil {
					return err
				}
			}
			if pending.ProxyConfiguration != nil {
				res, err := pending.ProxyConfiguration.Build()
				if err != nil {
					return fmt.Errorf("error serializing resource %s: %w", util.IDToString(pending.ProxyConfiguration.Resource.Id), err)
				}
				if _, err := s.writeResource(cluster, res); err != nil {
					return err
				}
			}
		} else {
			if err := s.registerCatalogServiceV1(cluster, node, wrk); err != nil {
				return fmt.Errorf("error registering service: %w", err)
			}
			if !wrk.DisableServiceMesh {
				if err := s.registerCatalogSidecarServiceV1(cluster, node, wrk); err != nil {
					return fmt.Errorf("error registering sidecar service: %w", err)
				}
			}
		}
		return nil
	}

	// deregisterWorkloadFromNode is called when node is disabled
	deregisterWorkloadFromNode := func(node *topology.Node, wrk *topology.Workload) error {
		if node.IsV2() {
			// TODO: implement deregister workload for v2
			panic("deregister workload is not implemented for V2")
		} else {
			if err := s.deregisterCatalogServiceV1(cluster, node, wrk); err != nil {
				return fmt.Errorf("error deregistering service: %w", err)
			}
			if !wrk.DisableServiceMesh {
				if err := s.deregisterCatalogSidecarServiceV1(cluster, node, wrk); err != nil {
					return fmt.Errorf("error deregistering sidecar service: %w", err)
				}
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

	if cluster.EnableV2 {
		for _, identity := range identityInfo {
			res, err := identity.Build()
			if err != nil {
				return fmt.Errorf("error serializing resource %s: %w", util.IDToString(identity.Resource.Id), err)
			}
			if _, err := s.writeResource(cluster, res); err != nil {
				return err
			}
		}

		for id, svcData := range cluster.Services {
			svcInfo := &Resource[*pbcatalog.Service]{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Type: pbcatalog.ServiceType,
						Name: id.Name,
						Tenancy: &pbresource.Tenancy{
							Partition: id.Partition,
							Namespace: id.Namespace,
						},
					},
				},
				Data: svcData,
			}

			res, err := svcInfo.Build()
			if err != nil {
				return fmt.Errorf("error serializing resource %s: %w", util.IDToString(svcInfo.Resource.Id), err)
			}
			if _, err := s.writeResource(cluster, res); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Sprawl) registerCatalogNode(
	cluster *topology.Cluster,
	node *topology.Node,
) error {
	if node.IsV2() {
		return s.registerCatalogNodeV2(cluster, node)
	}
	return s.registerCatalogNodeV1(cluster, node)
}

func (s *Sprawl) deregisterCatalogNode(
	cluster *topology.Cluster,
	node *topology.Node,
) error {
	if node.IsV2() {
		panic("deregister V2 node is not implemented")
	}
	return s.deregisterCatalogNodeV1(cluster, node)
}

func (s *Sprawl) registerCatalogNodeV2(
	cluster *topology.Cluster,
	node *topology.Node,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}

	nodeRes := &Resource[*pbcatalog.Node]{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type: pbcatalog.NodeType,
				Name: node.PodName(),
				Tenancy: &pbresource.Tenancy{
					Partition: node.Partition,
				},
			},
			Metadata: map[string]string{
				"dataplane-faux": "1",
			},
		},
		Data: &pbcatalog.Node{
			Addresses: []*pbcatalog.NodeAddress{
				{Host: node.LocalAddress()},
			},
		},
	}

	res, err := nodeRes.Build()
	if err != nil {
		return err
	}

	_, err = s.writeResource(cluster, res)
	return err
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
	if node.IsV2() {
		panic("don't call this")
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
	if node.IsV2() {
		panic("don't call this")
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
	if node.IsV2() {
		panic("don't call this")
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
	if node.IsV2() {
		panic("don't call this")
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

type ServiceResources struct {
	Workload           *Resource[*pbcatalog.Workload]
	HealthStatuses     []*Resource[*pbcatalog.HealthStatus]
	Destinations       *Resource[*pbmesh.Destinations]
	WorkloadIdentity   *Resource[*pbauth.WorkloadIdentity]
	ProxyConfiguration *Resource[*pbmesh.ProxyConfiguration]
}

func workloadInstanceToResources(
	node *topology.Node,
	wrk *topology.Workload,
) *ServiceResources {
	if wrk.IsMeshGateway {
		panic("v2 does not yet support mesh gateways")
	}

	tenancy := &pbresource.Tenancy{
		Partition: wrk.ID.Partition,
		Namespace: wrk.ID.Namespace,
	}

	var (
		wlPorts = map[string]*pbcatalog.WorkloadPort{}
	)
	for name, port := range wrk.Ports {
		wlPorts[name] = &pbcatalog.WorkloadPort{
			Port:     uint32(port.Number),
			Protocol: port.ActualProtocol,
		}
	}

	var (
		selector = &pbcatalog.WorkloadSelector{
			Names: []string{wrk.Workload},
		}

		workloadRes = &Resource[*pbcatalog.Workload]{
			Resource: &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    pbcatalog.WorkloadType,
					Name:    wrk.Workload,
					Tenancy: tenancy,
				},
				Metadata: wrk.Meta,
			},
			Data: &pbcatalog.Workload{
				NodeName: node.PodName(),
				Identity: wrk.WorkloadIdentity,
				Ports:    wlPorts,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: node.LocalAddress()},
				},
			},
		}
		workloadIdentityRes = &Resource[*pbauth.WorkloadIdentity]{
			Resource: &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    pbauth.WorkloadIdentityType,
					Name:    wrk.WorkloadIdentity,
					Tenancy: tenancy,
				},
			},
			Data: &pbauth.WorkloadIdentity{},
		}

		healthResList   []*Resource[*pbcatalog.HealthStatus]
		destinationsRes *Resource[*pbmesh.Destinations]
		proxyConfigRes  *Resource[*pbmesh.ProxyConfiguration]
	)

	if wrk.HasCheck() {
		// TODO: needs ownerId
		checkRes := &Resource[*pbcatalog.HealthStatus]{
			Resource: &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    pbcatalog.HealthStatusType,
					Name:    wrk.Workload + "-check-0",
					Tenancy: tenancy,
				},
			},
			Data: &pbcatalog.HealthStatus{
				Type:   "external-sync",
				Status: pbcatalog.Health_HEALTH_PASSING,
			},
		}

		healthResList = []*Resource[*pbcatalog.HealthStatus]{checkRes}
	}

	if node.HasPublicAddress() {
		workloadRes.Data.Addresses = append(workloadRes.Data.Addresses,
			&pbcatalog.WorkloadAddress{Host: node.PublicAddress(), External: true},
		)
	}

	if !wrk.DisableServiceMesh {
		destinationsRes = &Resource[*pbmesh.Destinations]{
			Resource: &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    pbmesh.DestinationsType,
					Name:    wrk.Workload,
					Tenancy: tenancy,
				},
			},
			Data: &pbmesh.Destinations{
				Workloads: selector,
			},
		}

		for _, dest := range wrk.Destinations {
			meshDest := &pbmesh.Destination{
				DestinationRef: &pbresource.Reference{
					Type: pbcatalog.ServiceType,
					Name: dest.ID.Name,
					Tenancy: &pbresource.Tenancy{
						Partition: dest.ID.Partition,
						Namespace: dest.ID.Namespace,
					},
				},
				DestinationPort: dest.PortName,
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   dest.LocalAddress,
						Port: uint32(dest.LocalPort),
					},
				},
			}
			destinationsRes.Data.Destinations = append(destinationsRes.Data.Destinations, meshDest)
		}

		if wrk.EnableTransparentProxy {
			proxyConfigRes = &Resource[*pbmesh.ProxyConfiguration]{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Type:    pbmesh.ProxyConfigurationType,
						Name:    wrk.Workload,
						Tenancy: tenancy,
					},
				},
				Data: &pbmesh.ProxyConfiguration{
					Workloads: selector,
					DynamicConfig: &pbmesh.DynamicConfig{
						Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
					},
				},
			}
		}
	}

	return &ServiceResources{
		Workload:           workloadRes,
		HealthStatuses:     healthResList,
		Destinations:       destinationsRes,
		WorkloadIdentity:   workloadIdentityRes,
		ProxyConfiguration: proxyConfigRes,
	}
}

func workloadToCatalogRegistration(
	cluster *topology.Cluster,
	node *topology.Node,
	wrk *topology.Workload,
) *api.CatalogRegistration {
	if node.IsV2() {
		panic("don't call this")
	}
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
	if node.IsV2() {
		panic("don't call this")
	}
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

	for _, dest := range wrk.Destinations {
		pu := api.Upstream{
			DestinationName:  dest.ID.Name,
			DestinationPeer:  dest.Peer,
			LocalBindAddress: dest.LocalAddress,
			LocalBindPort:    dest.LocalPort,
		}
		if cluster.Enterprise {
			pu.DestinationNamespace = dest.ID.Namespace
			if dest.Peer == "" {
				pu.DestinationPartition = dest.ID.Partition
			}
		}
		reg.Service.Proxy.Upstreams = append(reg.Service.Proxy.Upstreams, pu)
	}

	return pid, reg
}
