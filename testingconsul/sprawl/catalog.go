package sprawl

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/testingconsul"
	"github.com/hashicorp/consul/testingconsul/util"
)

func (s *Sprawl) registerAllServicesToAgents() error {
	for _, cluster := range s.topology.Clusters {
		if err := s.registerServicesToAgents(cluster); err != nil {
			return fmt.Errorf("registerServicesToAgents[%s]: %w", cluster.Name, err)
		}
	}
	return nil
}

func (s *Sprawl) registerAllServicesForDataplaneInstances() error {
	for _, cluster := range s.topology.Clusters {
		if err := s.registerServicesForDataplaneInstances(cluster); err != nil {
			return fmt.Errorf("registerServicesForDataplaneInstances[%s]: %w", cluster.Name, err)
		}
	}
	return nil
}

func (s *Sprawl) registerServicesToAgents(cluster *testingconsul.Cluster) error {
	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Services) == 0 || node.Disabled {
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
	cluster *testingconsul.Cluster,
	node *testingconsul.Node,
	svc *testingconsul.Service,
) error {
	if !node.IsAgent() {
		panic("called wrong method type")
	}

	if svc.IsMeshGateway {
		return nil // handled at startup time for agent-full, but won't be for agent-less
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

	logger.Info("registered service to client agent",
		"service", svc.ID.Name,
		"node", node.Name,
		"namespace", svc.ID.Namespace,
		"partition", svc.ID.Partition,
	)

	return nil
}

func (s *Sprawl) registerServicesForDataplaneInstances(cluster *testingconsul.Cluster) error {
	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Services) == 0 || node.Disabled {
			continue
		}

		if !node.IsDataplane() {
			continue
		}

		if err := s.registerCatalogNode(cluster, node); err != nil {
			return fmt.Errorf("error registering virtual node: %w", err)
		}

		for _, svc := range node.Services {
			if err := s.registerCatalogService(cluster, node, svc); err != nil {
				return fmt.Errorf("error registering service: %w", err)
			}
			if !svc.DisableServiceMesh {
				if err := s.registerCatalogSidecarService(cluster, node, svc); err != nil {
					return fmt.Errorf("error registering sidecar service: %w", err)
				}
			}
		}
	}

	return nil
}

func (s *Sprawl) registerCatalogNode(
	cluster *testingconsul.Cluster,
	node *testingconsul.Node,
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

	logger.Info("virtual node created",
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) registerCatalogService(
	cluster *testingconsul.Cluster,
	node *testingconsul.Node,
	svc *testingconsul.Service,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
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

	logger.Info("dataplane service created",
		"service", svc.ID,
		"node", node.ID(),
	)

	return nil
}

func (s *Sprawl) registerCatalogSidecarService(
	cluster *testingconsul.Cluster,
	node *testingconsul.Node,
	svc *testingconsul.Service,
) error {
	if !node.IsDataplane() {
		panic("called wrong method type")
	}
	if svc.DisableServiceMesh {
		panic("not valid")
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

	logger.Info("dataplane sidecar service created",
		"service", pid,
		"node", node.ID(),
	)

	return nil
}

func serviceToCatalogRegistration(
	cluster *testingconsul.Cluster,
	node *testingconsul.Node,
	svc *testingconsul.Service,
) *api.CatalogRegistration {
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
	if node.HasPublicAddress() {
		reg.TaggedAddresses = map[string]string{
			"lan":      node.LocalAddress(),
			"lan_ipv4": node.LocalAddress(),
			"wan":      node.PublicAddress(),
			"wan_ipv4": node.PublicAddress(),
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
	cluster *testingconsul.Cluster,
	node *testingconsul.Node,
	svc *testingconsul.Service,
) (testingconsul.ServiceID, *api.CatalogRegistration) {
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
