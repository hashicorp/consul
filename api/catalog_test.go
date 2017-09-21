package api

import (
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/pascaldekloe/goe/verify"
)

func TestAPI_CatalogDatacenters(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	retry.Run(t, func(r *retry.R) {
		datacenters, err := catalog.Datacenters()
		if err != nil {
			r.Fatal(err)
		}
		if len(datacenters) < 1 {
			r.Fatal("got 0 datacenters want at least one")
		}
	})
}

func TestAPI_CatalogNodes(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		nodes, meta, err := catalog.Nodes(nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatal("got last index 0 want > 0")
		}
		want := []*Node{
			{
				ID:         s.Config.NodeID,
				Node:       s.Config.NodeName,
				Address:    "127.0.0.1",
				Datacenter: "dc1",
				TaggedAddresses: map[string]string{
					"lan": "127.0.0.1",
					"wan": "127.0.0.1",
				},
				Meta: map[string]string{
					"consul-network-segment": "",
				},
				CreateIndex: meta.LastIndex - 1,
				ModifyIndex: meta.LastIndex,
			},
		}
		if !verify.Values(r, "", nodes, want) {
			r.FailNow()
		}
	})
}

func TestAPI_CatalogNodes_MetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()
	// Make sure we get the node back when filtering by its metadata
	retry.Run(t, func(r *retry.R) {
		nodes, meta, err := catalog.Nodes(&QueryOptions{NodeMeta: meta})
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(nodes) == 0 {
			r.Fatalf("Bad: %v", nodes)
		}

		if _, ok := nodes[0].TaggedAddresses["wan"]; !ok {
			r.Fatalf("Bad: %v", nodes[0])
		}

		if v, ok := nodes[0].Meta["somekey"]; !ok || v != "somevalue" {
			r.Fatalf("Bad: %v", nodes[0].Meta)
		}

		if nodes[0].Datacenter != "dc1" {
			r.Fatalf("Bad datacenter: %v", nodes[0])
		}
	})

	retry.Run(t, func(r *retry.R) {
		// Get nothing back when we use an invalid filter
		nodes, meta, err := catalog.Nodes(&QueryOptions{NodeMeta: map[string]string{"nope": "nope"}})
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(nodes) != 0 {
			r.Fatalf("Bad: %v", nodes)
		}
	})
}

func TestAPI_CatalogServices(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	retry.Run(t, func(r *retry.R) {
		services, meta, err := catalog.Services(nil)
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(services) == 0 {
			r.Fatalf("Bad: %v", services)
		}
	})
}

func TestAPI_CatalogServices_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()
	// Make sure we get the service back when filtering by the node's metadata
	retry.Run(t, func(r *retry.R) {
		services, meta, err := catalog.Services(&QueryOptions{NodeMeta: meta})
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(services) == 0 {
			r.Fatalf("Bad: %v", services)
		}
	})

	retry.Run(t, func(r *retry.R) {
		// Get nothing back when using an invalid filter
		services, meta, err := catalog.Services(&QueryOptions{NodeMeta: map[string]string{"nope": "nope"}})
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(services) != 0 {
			r.Fatalf("Bad: %v", services)
		}
	})
}

func TestAPI_CatalogService(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	retry.Run(t, func(r *retry.R) {
		services, meta, err := catalog.Service("consul", "", nil)
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(services) == 0 {
			r.Fatalf("Bad: %v", services)
		}

		if services[0].Datacenter != "dc1" {
			r.Fatalf("Bad datacenter: %v", services[0])
		}
	})
}

func TestAPI_CatalogService_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()
	retry.Run(t, func(r *retry.R) {
		services, meta, err := catalog.Service("consul", "", &QueryOptions{NodeMeta: meta})
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(services) == 0 {
			r.Fatalf("Bad: %v", services)
		}

		if services[0].Datacenter != "dc1" {
			r.Fatalf("Bad datacenter: %v", services[0])
		}
	})
}

func TestAPI_CatalogNode(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	name, _ := c.Agent().NodeName()
	retry.Run(t, func(r *retry.R) {
		info, meta, err := catalog.Node(name, nil)
		if err != nil {
			r.Fatal(err)
		}

		if meta.LastIndex == 0 {
			r.Fatalf("Bad: %v", meta)
		}

		if len(info.Services) == 0 {
			r.Fatalf("Bad: %v", info)
		}

		if _, ok := info.Node.TaggedAddresses["wan"]; !ok {
			r.Fatalf("Bad: %v", info)
		}

		if info.Node.Datacenter != "dc1" {
			r.Fatalf("Bad datacenter: %v", info)
		}
	})
}

func TestAPI_CatalogRegistration(t *testing.T) {
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
	retry.Run(t, func(r *retry.R) {
		if _, err := catalog.Register(reg, nil); err != nil {
			r.Fatal(err)
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			r.Fatal(err)
		}

		if _, ok := node.Services["redis1"]; !ok {
			r.Fatal("missing service: redis1")
		}

		health, _, err := c.Health().Node("foobar", nil)
		if err != nil {
			r.Fatal(err)
		}

		if health[0].CheckID != "service:redis1" {
			r.Fatal("missing checkid service:redis1")
		}

		if v, ok := node.Node.Meta["somekey"]; !ok || v != "somevalue" {
			r.Fatal("missing node meta pair somekey:somevalue")
		}
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

	retry.Run(t, func(r *retry.R) {
		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			r.Fatal(err)
		}

		if _, ok := node.Services["redis1"]; ok {
			r.Fatal("ServiceID:redis1 is not deregistered")
		}
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

	retry.Run(t, func(r *retry.R) {
		health, _, err := c.Health().Node("foobar", nil)
		if err != nil {
			r.Fatal(err)
		}

		if len(health) != 0 {
			r.Fatal("CheckID:service:redis1 is not deregistered")
		}
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

	retry.Run(t, func(r *retry.R) {
		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			r.Fatal(err)
		}

		if node != nil {
			r.Fatalf("node is not deregistered: %v", node)
		}
	})
}

func TestAPI_CatalogEnableTagOverride(t *testing.T) {
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

	retry.Run(t, func(r *retry.R) {
		if _, err := catalog.Register(reg, nil); err != nil {
			r.Fatal(err)
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			r.Fatal(err)
		}

		if _, ok := node.Services["redis1"]; !ok {
			r.Fatal("missing service: redis1")
		}
		if node.Services["redis1"].EnableTagOverride != false {
			r.Fatal("tag override set")
		}

		services, _, err := catalog.Service("redis", "", nil)
		if err != nil {
			r.Fatal(err)
		}

		if len(services) < 1 || services[0].ServiceName != "redis" {
			r.Fatal("missing service: redis")
		}
		if services[0].ServiceEnableTagOverride != false {
			r.Fatal("tag override set")
		}
	})

	service.EnableTagOverride = true

	retry.Run(t, func(r *retry.R) {
		if _, err := catalog.Register(reg, nil); err != nil {
			r.Fatal(err)
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			r.Fatal(err)
		}

		if _, ok := node.Services["redis1"]; !ok {
			r.Fatal("missing service: redis1")
		}
		if node.Services["redis1"].EnableTagOverride != true {
			r.Fatal("tag override not set")
		}

		services, _, err := catalog.Service("redis", "", nil)
		if err != nil {
			r.Fatal(err)
		}

		if len(services) < 1 || services[0].ServiceName != "redis" {
			r.Fatal("missing service: redis")
		}
		if services[0].ServiceEnableTagOverride != true {
			r.Fatal("tag override not set")
		}
	})
}
