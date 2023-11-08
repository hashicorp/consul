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
	"github.com/hashicorp/consul/proto-public/pbresource"
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
		if err := s.syncServicesForDataplaneInstances(cluster); err != nil {
			return fmt.Errorf("syncServicesForDataplaneInstances[%s]: %w", cluster.Name, err)
		}
	}
	return nil
}

func (s *Sprawl) registerServicesToAgents(cluster *topology.Cluster) error {
	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Services) == 0 || node.Disabled {
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

		for _, svc := range node.Services {
			if err := s.registerAgentService(agentClient, cluster, node, svc); err != nil {
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
	svc *topology.Service,
) error {
	if !node.IsAgent() {
		panic("called wrong method type")
	}
	if node.IsV2() {
		panic("don't call this")
	}

	if svc.IsMeshGateway {
		return nil // handled at startup time for agent-ful, but won't be for agent-less
	}

	var (
		logger = s.logger.With("cluster", cluster.Name)
	)

	reg := &api.AgentServiceRegistration{
		ID:   svc.ID.Name,
		Name: svc.ID.Name,
		Port: svc.Port,
		Meta: svc.Meta,
	}
	if cluster.Enterprise {
		reg.Namespace = svc.ID.Namespace
		reg.Partition = svc.ID.Partition
	}

	if !svc.DisableServiceMesh {
		var upstreams []api.Upstream
		for _, u := range svc.Upstreams {
			uAPI := api.Upstream{
				DestinationPeer:  u.Peer,
				DestinationName:  u.ID.Name,
				LocalBindAddress: u.LocalAddress,
				LocalBindPort:    u.LocalPort,
				// Config               map[string]interface{} `json:",omitempty" bexpr:"-"`
				// MeshGateway          MeshGatewayConfig      `json:",omitempty"`
			}
			if cluster.Enterprise {
				uAPI.DestinationNamespace = u.ID.Namespace
				if u.Peer == "" {
					uAPI.DestinationPartition = u.ID.Partition
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
	case svc.CheckTCP != "":
		chk := &api.AgentServiceCheck{
			Name:     "up",
			TCP:      svc.CheckTCP,
			Interval: "5s",
			Timeout:  "1s",
		}
		reg.Checks = append(reg.Checks, chk)
	case svc.CheckHTTP != "":
		chk := &api.AgentServiceCheck{
			Name:     "up",
			HTTP:     svc.CheckHTTP,
			Method:   "GET",
			Interval: "5s",
			Timeout:  "1s",
		}
		reg.Checks = append(reg.Checks, chk)
	}

	// Switch token for every request.
	hdr := make(http.Header)
	hdr.Set("X-Consul-Token", s.secrets.ReadServiceToken(cluster.Name, svc.ID))
	agentClient.SetHeaders(hdr)

RETRY:
	if err := agentClient.Agent().ServiceRegister(reg); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("failed to register service %q to node %q: %w", svc.ID, node.ID(), err)
	}

	logger.Debug("registered service to client agent",
		"service", svc.ID.Name,
		"node", node.Name,
		"namespace", svc.ID.Namespace,
		"partition", svc.ID.Partition,
	)

	return nil
}

// syncServicesForDataplaneInstances register/deregister services in the given cluster
func (s *Sprawl) syncServicesForDataplaneInstances(cluster *topology.Cluster) error {
	identityInfo := make(map[topology.ServiceID]*Resource[*pbauth.WorkloadIdentity])

	// registerServiceAtNode is called when node is not disabled
	registerServiceAtNode := func(node *topology.Node, svc *topology.Service) error {
		if node.IsV2() {
			pending := serviceInstanceToResources(node, svc)

			workloadID := topology.NewServiceID(svc.WorkloadIdentity, svc.ID.Namespace, svc.ID.Partition)
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
			if err := s.registerCatalogServiceV1(cluster, node, svc); err != nil {
				return fmt.Errorf("error registering service: %w", err)
			}
			if !svc.DisableServiceMesh {
				if err := s.registerCatalogSidecarServiceV1(cluster, node, svc); err != nil {
					return fmt.Errorf("error registering sidecar service: %w", err)
				}
			}
		}
		return nil
	}

	// deregisterServiceAtNode is called when node is disabled
	deregisterServiceAtNode := func(node *topology.Node, svc *topology.Service) error {
		if node.IsV2() {
			// TODO: implement deregister services for v2
			panic("deregister services is not implemented for V2")
		} else {
			if err := s.deregisterCatalogServiceV1(cluster, node, svc); err != nil {
				return fmt.Errorf("error deregistering service: %w", err)
			}
			if !svc.DisableServiceMesh {
				if err := s.deregisterCatalogSidecarServiceV1(cluster, node, svc); err != nil {
					return fmt.Errorf("error deregistering sidecar service: %w", err)
				}
			}
		}
		return nil
	}

	var syncService func(node *topology.Node, svc *topology.Service) error

	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Services) == 0 {
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
		for _, svc := range node.Services {
			if !node.Disabled {
				syncService = registerServiceAtNode
			} else {
				syncService = deregisterServiceAtNode
			}
			syncService(node, svc)
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

		// TODO(rb): nodes are optional in v2 and won't be used in k8s by
		// default. There are some scoping issues with the Node Type in 1.17 so
		// disable it for now.
		//
		// To re-enable you also need to link it to the Workload by setting the
		// NodeName field.
		//
		// return s.registerCatalogNodeV2(cluster, node)
		return nil
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
					Namespace: "default", // temporary requirement
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
	svc *topology.Service,
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
		ServiceID: svc.ID.Name,
	}
RETRY:
	if _, err := client.Catalog().Deregister(dereg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error deregistering service %s at node %s: %w", svc.ID, node.ID(), err)
	}

	logger.Info("dataplane service removed",
		"service", svc.ID,
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) registerCatalogServiceV1(
	cluster *topology.Cluster,
	node *topology.Node,
	svc *topology.Service,
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

	reg := serviceToCatalogRegistration(cluster, node, svc)

RETRY:
	if _, err := client.Catalog().Register(reg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error registering service %s to node %s: %w", svc.ID, node.ID(), err)
	}

	logger.Debug("dataplane service created",
		"service", svc.ID,
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) deregisterCatalogSidecarServiceV1(
	cluster *topology.Cluster,
	node *topology.Node,
	svc *topology.Service,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}
	if svc.DisableServiceMesh {
		panic("not valid")
	}
	if node.IsV2() {
		panic("don't call this")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	pid := svc.ID
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
		return fmt.Errorf("error deregistering service %s to node %s: %w", svc.ID, node.ID(), err)
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
	svc *topology.Service,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}
	if svc.DisableServiceMesh {
		panic("not valid")
	}
	if node.IsV2() {
		panic("don't call this")
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	pid, reg := serviceToSidecarCatalogRegistration(cluster, node, svc)
RETRY:
	if _, err := client.Catalog().Register(reg, nil); err != nil {
		if isACLNotFound(err) {
			time.Sleep(50 * time.Millisecond)
			goto RETRY
		}
		return fmt.Errorf("error registering service %s to node %s: %w", svc.ID, node.ID(), err)
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

func serviceInstanceToResources(
	node *topology.Node,
	svc *topology.Service,
) *ServiceResources {
	if svc.IsMeshGateway {
		panic("v2 does not yet support mesh gateways")
	}

	tenancy := &pbresource.Tenancy{
		Partition: svc.ID.Partition,
		Namespace: svc.ID.Namespace,
	}

	var (
		wlPorts = map[string]*pbcatalog.WorkloadPort{}
	)
	for name, port := range svc.Ports {
		wlPorts[name] = &pbcatalog.WorkloadPort{
			Port:     uint32(port.Number),
			Protocol: port.ActualProtocol,
		}
	}

	var (
		selector = &pbcatalog.WorkloadSelector{
			Names: []string{svc.Workload},
		}

		workloadRes = &Resource[*pbcatalog.Workload]{
			Resource: &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    pbcatalog.WorkloadType,
					Name:    svc.Workload,
					Tenancy: tenancy,
				},
				Metadata: svc.Meta,
			},
			Data: &pbcatalog.Workload{
				// TODO(rb): disabling this until node scoping makes sense again
				// NodeName: node.PodName(),
				Identity: svc.ID.Name,
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
					Name:    svc.WorkloadIdentity,
					Tenancy: tenancy,
				},
			},
			Data: &pbauth.WorkloadIdentity{},
		}

		healthResList   []*Resource[*pbcatalog.HealthStatus]
		destinationsRes *Resource[*pbmesh.Destinations]
		proxyConfigRes  *Resource[*pbmesh.ProxyConfiguration]
	)

	if svc.HasCheck() {
		// TODO: needs ownerId
		checkRes := &Resource[*pbcatalog.HealthStatus]{
			Resource: &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    pbcatalog.HealthStatusType,
					Name:    svc.Workload + "-check-0",
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

	if !svc.DisableServiceMesh {
		destinationsRes = &Resource[*pbmesh.Destinations]{
			Resource: &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    pbmesh.DestinationsType,
					Name:    svc.Workload,
					Tenancy: tenancy,
				},
			},
			Data: &pbmesh.Destinations{
				Workloads: selector,
			},
		}

		for _, u := range svc.Upstreams {
			dest := &pbmesh.Destination{
				DestinationRef: &pbresource.Reference{
					Type: pbcatalog.ServiceType,
					Name: u.ID.Name,
					Tenancy: &pbresource.Tenancy{
						Partition: u.ID.Partition,
						Namespace: u.ID.Namespace,
					},
				},
				DestinationPort: u.PortName,
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   u.LocalAddress,
						Port: uint32(u.LocalPort),
					},
				},
			}
			destinationsRes.Data.Destinations = append(destinationsRes.Data.Destinations, dest)
		}

		if svc.EnableTransparentProxy {
			proxyConfigRes = &Resource[*pbmesh.ProxyConfiguration]{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Type:    pbmesh.ProxyConfigurationType,
						Name:    svc.Workload,
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

func serviceToCatalogRegistration(
	cluster *topology.Cluster,
	node *topology.Node,
	svc *topology.Service,
) *api.CatalogRegistration {
	if node.IsV2() {
		panic("don't call this")
	}
	reg := &api.CatalogRegistration{
		Node:           node.PodName(),
		SkipNodeUpdate: true,
		Service: &api.AgentService{
			Kind:    api.ServiceKindTypical,
			ID:      svc.ID.Name,
			Service: svc.ID.Name,
			Meta:    svc.Meta,
			Port:    svc.Port,
			Address: node.LocalAddress(),
		},
	}
	if svc.IsMeshGateway {
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
				Port:    svc.Port,
			},
			"lan_ipv4": {
				Address: node.LocalAddress(),
				Port:    svc.Port,
			},
			"wan": {
				Address: node.PublicAddress(),
				Port:    svc.Port,
			},
			"wan_ipv4": {
				Address: node.PublicAddress(),
				Port:    svc.Port,
			},
		}
	}
	if cluster.Enterprise {
		reg.Partition = svc.ID.Partition
		reg.Service.Namespace = svc.ID.Namespace
		reg.Service.Partition = svc.ID.Partition
	}

	if svc.HasCheck() {
		chk := &api.HealthCheck{
			Name: "external sync",
			// Type:      "external-sync",
			Status:      "passing", //  TODO
			ServiceID:   svc.ID.Name,
			ServiceName: svc.ID.Name,
			Output:      "",
		}
		if cluster.Enterprise {
			chk.Namespace = svc.ID.Namespace
			chk.Partition = svc.ID.Partition
		}
		switch {
		case svc.CheckTCP != "":
			chk.Definition.TCP = svc.CheckTCP
		case svc.CheckHTTP != "":
			chk.Definition.HTTP = svc.CheckHTTP
			chk.Definition.Method = "GET"
		}
		reg.Checks = append(reg.Checks, chk)
	}
	return reg
}

func serviceToSidecarCatalogRegistration(
	cluster *topology.Cluster,
	node *topology.Node,
	svc *topology.Service,
) (topology.ServiceID, *api.CatalogRegistration) {
	if node.IsV2() {
		panic("don't call this")
	}
	pid := svc.ID
	pid.Name += "-sidecar-proxy"
	reg := &api.CatalogRegistration{
		Node:           node.PodName(),
		SkipNodeUpdate: true,
		Service: &api.AgentService{
			Kind:    api.ServiceKindConnectProxy,
			ID:      pid.Name,
			Service: pid.Name,
			Meta:    svc.Meta,
			Port:    svc.EnvoyPublicListenerPort,
			Address: node.LocalAddress(),
			Proxy: &api.AgentServiceConnectProxyConfig{
				DestinationServiceName: svc.ID.Name,
				DestinationServiceID:   svc.ID.Name,
				LocalServicePort:       svc.Port,
			},
		},
		Checks: []*api.HealthCheck{{
			Name: "external sync",
			// Type:      "external-sync",
			Status:      "passing", //  TODO
			ServiceID:   pid.Name,
			ServiceName: pid.Name,
			Definition: api.HealthCheckDefinition{
				TCP: fmt.Sprintf("%s:%d", node.LocalAddress(), svc.EnvoyPublicListenerPort),
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

	for _, u := range svc.Upstreams {
		pu := api.Upstream{
			DestinationName:  u.ID.Name,
			DestinationPeer:  u.Peer,
			LocalBindAddress: u.LocalAddress,
			LocalBindPort:    u.LocalPort,
		}
		if cluster.Enterprise {
			pu.DestinationNamespace = u.ID.Namespace
			if u.Peer == "" {
				pu.DestinationPartition = u.ID.Partition
			}
		}
		reg.Service.Proxy.Upstreams = append(reg.Service.Proxy.Upstreams, pu)
	}

	return pid, reg
}
