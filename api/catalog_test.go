package api

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
)

func TestCatalog_Datacenters(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	retry.Fatal(t, func() error {
		datacenters, err := catalog.Datacenters()
		if err != nil {
			return err
		}

		if len(datacenters) == 0 {
			return fmt.Errorf("Bad: %v", datacenters)
		}

		return nil
	})
}

func TestCatalog_Nodes(t *testing.T) {
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	retry.Fatal(t, func() error {
		nodes, meta, err := catalog.Nodes(nil)
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(nodes) == 0 {
			return fmt.Errorf("Bad: %v", nodes)
		}

		if _, ok := nodes[0].TaggedAddresses["wan"]; !ok {
			return fmt.Errorf("Bad: %v", nodes[0])
		}

		if nodes[0].Datacenter != "dc1" {
			return fmt.Errorf("Bad datacenter: %v", nodes[0])
		}

		return nil
	})
}

func TestCatalog_Nodes_MetaFilter(t *testing.T) {
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()

	// Make sure we get the node back when filtering by its metadata
	retry.Fatal(t, func() error {
		nodes, meta, err := catalog.Nodes(&QueryOptions{NodeMeta: meta})
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(nodes) == 0 {
			return fmt.Errorf("Bad: %v", nodes)
		}

		if _, ok := nodes[0].TaggedAddresses["wan"]; !ok {
			return fmt.Errorf("Bad: %v", nodes[0])
		}

		if v, ok := nodes[0].Meta["somekey"]; !ok || v != "somevalue" {
			return fmt.Errorf("Bad: %v", nodes[0].Meta)
		}

		if nodes[0].Datacenter != "dc1" {
			return fmt.Errorf("Bad datacenter: %v", nodes[0])
		}

		return nil
	})

	// Get nothing back when we use an invalid filter
	retry.Fatal(t, func() error {
		nodes, meta, err := catalog.Nodes(&QueryOptions{NodeMeta: map[string]string{"nope": "nope"}})
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(nodes) != 0 {
			return fmt.Errorf("Bad: %v", nodes)
		}

		return nil
	})
}

func TestCatalog_Services(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	retry.Fatal(t, func() error {
		services, meta, err := catalog.Services(nil)
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(services) == 0 {
			return fmt.Errorf("Bad: %v", services)
		}

		return nil
	})
}

func TestCatalog_Services_NodeMetaFilter(t *testing.T) {
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()

	// Make sure we get the service back when filtering by the node's metadata
	retry.Fatal(t, func() error {
		services, meta, err := catalog.Services(&QueryOptions{NodeMeta: meta})
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(services) == 0 {
			return fmt.Errorf("Bad: %v", services)
		}

		return nil
	})

	// Get nothing back when using an invalid filter
	retry.Fatal(t, func() error {
		services, meta, err := catalog.Services(&QueryOptions{NodeMeta: map[string]string{"nope": "nope"}})
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(services) != 0 {
			return fmt.Errorf("Bad: %v", services)
		}

		return nil
	})
}

func TestCatalog_Service(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	retry.Fatal(t, func() error {
		services, meta, err := catalog.Service("consul", "", nil)
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(services) == 0 {
			return fmt.Errorf("Bad: %v", services)
		}

		if services[0].Datacenter != "dc1" {
			return fmt.Errorf("Bad datacenter: %v", services[0])
		}

		return nil
	})
}

func TestCatalog_Service_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()

	retry.Fatal(t, func() error {
		services, meta, err := catalog.Service("consul", "", &QueryOptions{NodeMeta: meta})
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(services) == 0 {
			return fmt.Errorf("Bad: %v", services)
		}

		if services[0].Datacenter != "dc1" {
			return fmt.Errorf("Bad datacenter: %v", services[0])
		}

		return nil
	})
}

func TestCatalog_Node(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	name, _ := c.Agent().NodeName()

	retry.Fatal(t, func() error {
		info, meta, err := catalog.Node(name, nil)
		if err != nil {
			return err
		}

		if meta.LastIndex == 0 {
			return fmt.Errorf("Bad: %v", meta)
		}

		if len(info.Services) == 0 {
			return fmt.Errorf("Bad: %v", info)
		}

		if _, ok := info.Node.TaggedAddresses["wan"]; !ok {
			return fmt.Errorf("Bad: %v", info)
		}

		if info.Node.Datacenter != "dc1" {
			return fmt.Errorf("Bad datacenter: %v", info)
		}

		return nil
	})
}

func TestCatalog_Registration(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	service := &AgentService{
		ID:      "redis1",
		Service: "redis",
		Tags:    []string{"master", "v1"},
		Port:    8000,
	}

	check := &AgentCheck{
		Node:      "foobar",
		CheckID:   "service:redis1",
		Name:      "Redis health check",
		Notes:     "Script based health check",
		Status:    HealthPassing,
		ServiceID: "redis1",
	}

	reg := &CatalogRegistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Service:    service,
		Check:      check,
	}

	retry.Fatal(t, func() error {
		if _, err := catalog.Register(reg, nil); err != nil {
			return err
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			return err
		}

		if _, ok := node.Services["redis1"]; !ok {
			return fmt.Errorf("missing service: redis1")
		}

		health, _, err := c.Health().Node("foobar", nil)
		if err != nil {
			return err
		}

		if health[0].CheckID != "service:redis1" {
			return fmt.Errorf("missing checkid service:redis1")
		}

		if v, ok := node.Node.Meta["somekey"]; !ok || v != "somevalue" {
			return fmt.Errorf("missing node meta pair somekey:somevalue")
		}

		return nil
	})

	// Test catalog deregistration of the previously registered service
	dereg := &CatalogDeregistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
		ServiceID:  "redis1",
	}

	if _, err := catalog.Deregister(dereg, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Fatal(t, func() error {
		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			return err
		}

		if _, ok := node.Services["redis1"]; ok {
			return fmt.Errorf("ServiceID:redis1 is not deregistered")
		}

		return nil
	})

	// Test deregistration of the previously registered check
	dereg = &CatalogDeregistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
		CheckID:    "service:redis1",
	}

	if _, err := catalog.Deregister(dereg, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Fatal(t, func() error {
		health, _, err := c.Health().Node("foobar", nil)
		if err != nil {
			return err
		}

		if len(health) != 0 {
			return fmt.Errorf("CheckID:service:redis1 is not deregistered")
		}

		return nil
	})

	// Test node deregistration of the previously registered node
	dereg = &CatalogDeregistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
	}

	if _, err := catalog.Deregister(dereg, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Fatal(t, func() error {
		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			return err
		}

		if node != nil {
			return fmt.Errorf("node is not deregistered: %v", node)
		}

		return nil
	})
}

func TestCatalog_EnableTagOverride(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	service := &AgentService{
		ID:      "redis1",
		Service: "redis",
		Tags:    []string{"master", "v1"},
		Port:    8000,
	}

	reg := &CatalogRegistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
		Service:    service,
	}

	retry.Fatal(t, func() error {
		if _, err := catalog.Register(reg, nil); err != nil {
			return err
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			return err
		}

		if _, ok := node.Services["redis1"]; !ok {
			return fmt.Errorf("missing service: redis1")
		}
		if node.Services["redis1"].EnableTagOverride != false {
			return fmt.Errorf("tag override set")
		}

		services, _, err := catalog.Service("redis", "", nil)
		if err != nil {
			return err
		}

		if len(services) < 1 || services[0].ServiceName != "redis" {
			return fmt.Errorf("missing service: redis")
		}
		if services[0].ServiceEnableTagOverride != false {
			return fmt.Errorf("tag override set")
		}

		return nil
	})

	service.EnableTagOverride = true
	retry.Fatal(t, func() error {
		if _, err := catalog.Register(reg, nil); err != nil {
			return err
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			return err
		}

		if _, ok := node.Services["redis1"]; !ok {
			return fmt.Errorf("missing service: redis1")
		}
		if node.Services["redis1"].EnableTagOverride != true {
			return fmt.Errorf("tag override not set")
		}

		services, _, err := catalog.Service("redis", "", nil)
		if err != nil {
			return err
		}

		if len(services) < 1 || services[0].ServiceName != "redis" {
			return fmt.Errorf("missing service: redis")
		}
		if services[0].ServiceEnableTagOverride != true {
			return fmt.Errorf("tag override not set")
		}

		return nil
	})
}
