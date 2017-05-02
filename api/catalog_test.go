package api

import (
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/pascaldekloe/goe/verify"
)

func TestCatalog_Datacenters(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		datacenters, err := catalog.Datacenters()
		if err != nil {
			t.Log("catalog.Datacenters: ", err)
			continue
		}
		if len(datacenters) == 0 {
			t.Log("got 0 datacenters want at least one")
			continue
		}
		break
	}
}

func TestCatalog_Nodes(t *testing.T) {
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()

	for r := retry.OneSec(); r.NextOr(func() { t.Fatal("no nodes") }); {
		nodes, meta, err := catalog.Nodes(nil)
		if err != nil {
			t.Log("catalog.Nodes: ", err)
			continue
		}
		if meta.LastIndex == 0 {
			t.Log("got last index 0 want > 0")
			continue
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
				Meta:        map[string]string{},
				CreateIndex: meta.LastIndex - 1,
				ModifyIndex: meta.LastIndex,
			},
		}
		if !verify.Values(t, "", nodes, want) {
			continue
		}
		break
	}
}

func TestCatalog_Nodes_MetaFilter(t *testing.T) {
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()
	for r :=

		// Make sure we get the node back when filtering by its metadata
		retry.OneSec(); r.NextOr(t.FailNow); {

		nodes, meta, err := catalog.Nodes(&QueryOptions{NodeMeta: meta})
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(nodes) == 0 {
			t.Logf("Bad: %v", nodes)
			continue
		}

		if _, ok := nodes[0].TaggedAddresses["wan"]; !ok {
			t.Logf("Bad: %v", nodes[0])
			continue
		}

		if v, ok := nodes[0].Meta["somekey"]; !ok || v != "somevalue" {
			t.Logf("Bad: %v", nodes[0].Meta)
			continue
		}

		if nodes[0].Datacenter != "dc1" {
			t.Logf("Bad datacenter: %v", nodes[0])
			continue
		}
		break
	}
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		// Get nothing back when we use an invalid filter

		nodes, meta, err := catalog.Nodes(&QueryOptions{NodeMeta: map[string]string{"nope": "nope"}})
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(nodes) != 0 {
			t.Logf("Bad: %v", nodes)
			continue
		}
		break
	}

}

func TestCatalog_Services(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		services, meta, err := catalog.Services(nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(services) == 0 {
			t.Logf("Bad: %v", services)
			continue
		}
		break
	}

}

func TestCatalog_Services_NodeMetaFilter(t *testing.T) {
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()
	for r :=

		// Make sure we get the service back when filtering by the node's metadata
		retry.OneSec(); r.NextOr(t.FailNow); {

		services, meta, err := catalog.Services(&QueryOptions{NodeMeta: meta})
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(services) == 0 {
			t.Logf("Bad: %v", services)
			continue
		}
		break
	}
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		// Get nothing back when using an invalid filter

		services, meta, err := catalog.Services(&QueryOptions{NodeMeta: map[string]string{"nope": "nope"}})
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(services) != 0 {
			t.Logf("Bad: %v", services)
			continue
		}
		break
	}

}

func TestCatalog_Service(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		services, meta, err := catalog.Service("consul", "", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(services) == 0 {
			t.Logf("Bad: %v", services)
			continue
		}

		if services[0].Datacenter != "dc1" {
			t.Logf("Bad datacenter: %v", services[0])
			continue
		}
		break
	}

}

func TestCatalog_Service_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	catalog := c.Catalog()
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		services, meta, err := catalog.Service("consul", "", &QueryOptions{NodeMeta: meta})
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(services) == 0 {
			t.Logf("Bad: %v", services)
			continue
		}

		if services[0].Datacenter != "dc1" {
			t.Logf("Bad datacenter: %v", services[0])
			continue
		}
		break
	}

}

func TestCatalog_Node(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	catalog := c.Catalog()
	name, _ := c.Agent().NodeName()
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		info, meta, err := catalog.Node(name, nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if meta.LastIndex == 0 {
			t.Logf("Bad: %v", meta)
			continue
		}

		if len(info.Services) == 0 {
			t.Logf("Bad: %v", info)
			continue
		}

		if _, ok := info.Node.TaggedAddresses["wan"]; !ok {
			t.Logf("Bad: %v", info)
			continue
		}

		if info.Node.Datacenter != "dc1" {
			t.Logf("Bad datacenter: %v", info)
			continue
		}
		break
	}

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
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		if _, err := catalog.Register(reg, nil); err != nil {
			t.Log(err)
			continue
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if _, ok := node.Services["redis1"]; !ok {
			t.Log("missing service: redis1")
			continue
		}

		health, _, err := c.Health().Node("foobar", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if health[0].CheckID != "service:redis1" {
			t.Log("missing checkid service:redis1")
			continue
		}

		if v, ok := node.Node.Meta["somekey"]; !ok || v != "somevalue" {
			t.Log("missing node meta pair somekey:somevalue")
			continue
		}
		break
	}

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
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if _, ok := node.Services["redis1"]; ok {
			t.Log("ServiceID:redis1 is not deregistered")
			continue
		}
		break
	}

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
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		health, _, err := c.Health().Node("foobar", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if len(health) != 0 {
			t.Log("CheckID:service:redis1 is not deregistered")
			continue
		}
		break
	}

	// Test node deregistration of the previously registered node
	dereg = &CatalogDeregistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
	}

	if _, err := catalog.Deregister(dereg, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if node != nil {
			t.Logf("node is not deregistered: %v", node)
			continue
		}
		break
	}

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
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		if _, err := catalog.Register(reg, nil); err != nil {
			t.Log(err)
			continue
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if _, ok := node.Services["redis1"]; !ok {
			t.Log("missing service: redis1")
			continue
		}
		if node.Services["redis1"].EnableTagOverride != false {
			t.Log("tag override set")
			continue
		}

		services, _, err := catalog.Service("redis", "", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if len(services) < 1 || services[0].ServiceName != "redis" {
			t.Log("missing service: redis")
			continue
		}
		if services[0].ServiceEnableTagOverride != false {
			t.Log("tag override set")
			continue
		}
		break
	}

	service.EnableTagOverride = true
	for r := retry.OneSec(); r.NextOr(t.FailNow); {

		if _, err := catalog.Register(reg, nil); err != nil {
			t.Log(err)
			continue
		}

		node, _, err := catalog.Node("foobar", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if _, ok := node.Services["redis1"]; !ok {
			t.Log("missing service: redis1")
			continue
		}
		if node.Services["redis1"].EnableTagOverride != true {
			t.Log("tag override not set")
			continue
		}

		services, _, err := catalog.Service("redis", "", nil)
		if err != nil {
			t.Log(err)
			continue
		}

		if len(services) < 1 || services[0].ServiceName != "redis" {
			t.Log("missing service: redis")
			continue
		}
		if services[0].ServiceEnableTagOverride != true {
			t.Log("tag override not set")
			continue
		}
		break
	}

}
