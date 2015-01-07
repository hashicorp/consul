package api

import (
	"testing"
)

func TestCatalog_Datacenters(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	catalog := c.Catalog()

	datacenters, err := catalog.Datacenters()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(datacenters) == 0 {
		t.Fatalf("Bad: %v", datacenters)
	}
}

func TestCatalog_Nodes(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	catalog := c.Catalog()

	nodes, meta, err := catalog.Nodes(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("Bad: %v", meta)
	}

	if len(nodes) == 0 {
		t.Fatalf("Bad: %v", nodes)
	}
}

func TestCatalog_Services(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	catalog := c.Catalog()

	services, meta, err := catalog.Services(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("Bad: %v", meta)
	}

	if len(services) == 0 {
		t.Fatalf("Bad: %v", services)
	}
}

func TestCatalog_Service(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	catalog := c.Catalog()

	services, meta, err := catalog.Service("consul", "", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("Bad: %v", meta)
	}

	if len(services) == 0 {
		t.Fatalf("Bad: %v", services)
	}
}

func TestCatalog_Node(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	catalog := c.Catalog()

	name, _ := c.Agent().NodeName()
	info, meta, err := catalog.Node(name, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("Bad: %v", meta)
	}
	if len(info.Services) == 0 {
		t.Fatalf("Bad: %v", info)
	}
}

func TestCatalog_Registration(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

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
		Status:    "passing",
		ServiceID: "redis1",
	}

	reg := &CatalogRegistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
		Service:    service,
		Check:      check,
	}

	_, err := catalog.Register(reg, nil)

	if err != nil {
		t.Fatalf("err: %v", err)
	}

	node, _, err := catalog.Node("foobar", nil)

	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := node.Services["redis1"]; !ok {
		t.Fatalf("missing service: redis1")
	}

	health, _, err := c.Health().Node("foobar", nil)

	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if health[0].CheckID != "service:redis1" {
		t.Fatalf("missing checkid service:redis1")
	}

	dereg := &CatalogDeregistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
		ServiceID:  "redis1",
	}

	if _, err := catalog.Deregister(dereg, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	node, _, err = catalog.Node("foobar", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := node.Services["redis1"]; ok {
		t.Fatalf("ServiceID:redis1 is not deregistered")
	}

	dereg = &CatalogDeregistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
		CheckID:    "service:redis1",
	}

	if _, err := catalog.Deregister(dereg, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	health, _, err = c.Health().Node("foobar", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(health) != 0 {
		t.Fatalf("CheckID:service:redis1 is not deregistered")
	}

	dereg = &CatalogDeregistration{
		Datacenter: "dc1",
		Node:       "foobar",
		Address:    "192.168.10.10",
	}

	if _, err = catalog.Deregister(dereg, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	node, _, err = catalog.Node("foobar", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if node != nil {
		t.Fatalf("node is not deregistered: %v", node)
	}
}
