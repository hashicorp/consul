package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestAPI_AgentSelf(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	name := info["Config"]["NodeName"].(string)
	if name == "" {
		t.Fatalf("bad: %v", info)
	}
}

func TestAPI_AgentMetrics(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)

	timer := &retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}
	retry.RunWith(timer, t, func(r *retry.R) {
		metrics, err := agent.Metrics()
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		for _, g := range metrics.Gauges {
			if g.Name == "consul.runtime.alloc_bytes" {
				return
			}
		}
		r.Fatalf("missing runtime metrics")
	})
}

func TestAPI_AgentHost(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	timer := &retry.Timer{}
	retry.RunWith(timer, t, func(r *retry.R) {
		host, err := agent.Host()
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		// CollectionTime should exist on all responses
		if host["CollectionTime"] == nil {
			r.Fatalf("missing host response")
		}
	})
}

func TestAPI_AgentReload(t *testing.T) {
	t.Parallel()

	// Create our initial empty config file, to be overwritten later
	cfgDir := testutil.TempDir(t, "consul-config")

	cfgFilePath := filepath.Join(cfgDir, "reload.json")
	configFile, err := os.Create(cfgFilePath)
	if err != nil {
		t.Fatalf("Unable to create file %v, got error:%v", cfgFilePath, err)
	}

	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Args = []string{"-config-file", configFile.Name()}
	})
	defer s.Stop()

	agent := c.Agent()

	// Update the config file with a service definition
	config := `{"service":{"name":"redis", "port":1234, "Meta": {"some": "meta"}}}`
	err = ioutil.WriteFile(configFile.Name(), []byte(config), 0644)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err = agent.Reload(); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	service, ok := services["redis"]
	if !ok {
		t.Fatalf("bad: %v", ok)
	}
	if service.Port != 1234 {
		t.Fatalf("bad: %v", service.Port)
	}
	if service.Meta["some"] != "meta" {
		t.Fatalf("Missing metadata some:=meta in %v", service)
	}
}

func TestAPI_AgentMembersOpts(t *testing.T) {
	t.Parallel()
	c, s1 := makeClient(t)
	_, s2 := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.Datacenter = "dc2"
	})
	defer s1.Stop()
	defer s2.Stop()

	agent := c.Agent()

	s2.JoinWAN(t, s1.WANAddr)

	members, err := agent.MembersOpts(MembersOpts{WAN: true})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("bad: %v", members)
	}
}

func TestAPI_AgentMembers(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	members, err := agent.Members(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("bad: %v", members)
	}
}

func TestAPI_AgentServiceAndReplaceChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)

	reg := &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo",
		Tags: []string{"bar", "baz"},
		TaggedAddresses: map[string]ServiceAddress{
			"lan": {
				Address: "198.18.0.1",
				Port:    80,
			},
		},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}

	regupdate := &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo",
		Tags: []string{"bar", "baz"},
		TaggedAddresses: map[string]ServiceAddress{
			"lan": {
				Address: "198.18.0.1",
				Port:    80,
			},
		},
		Port: 9000,
	}

	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	ctx := context.Background()
	opts := ServiceRegisterOpts{ReplaceExistingChecks: true}.WithContext(ctx)
	if err := agent.ServiceRegisterOpts(regupdate, opts); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %#v", services)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(checks) != 0 {
		t.Fatalf("checks are not removed: %v", checks)
	}

	state, out, err := agent.AgentHealthServiceByID("foo")
	require.Nil(t, err)
	require.NotNil(t, out)
	require.Equal(t, HealthPassing, state)
	require.Equal(t, 9000, out.Service.Port)

	state, outs, err := agent.AgentHealthServiceByName("foo")
	require.Nil(t, err)
	require.NotNil(t, outs)
	require.Equal(t, HealthPassing, state)
	require.Equal(t, 9000, outs[0].Service.Port)

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_ServiceRegisterOpts_WithContextTimeout(t *testing.T) {
	c, err := NewClient(DefaultConfig())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	t.Cleanup(cancel)

	opts := ServiceRegisterOpts{}.WithContext(ctx)
	err = c.Agent().ServiceRegisterOpts(&AgentServiceRegistration{}, opts)
	require.True(t, errors.Is(err, context.DeadlineExceeded), "expected timeout")
}

func TestAPI_AgentServices(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)

	reg := &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo",
		Tags: []string{"bar", "baz"},
		TaggedAddresses: map[string]ServiceAddress{
			"lan": {
				Address: "198.18.0.1",
				Port:    80,
			},
		},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %#v", services)
	}
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["service:foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}

	// Checks should default to critical
	if chk.Status != HealthCritical {
		t.Fatalf("Bad: %#v", chk)
	}

	state, out, err := agent.AgentHealthServiceByID("foo2")
	require.Nil(t, err)
	require.Nil(t, out)
	require.Equal(t, HealthCritical, state)

	state, out, err = agent.AgentHealthServiceByID("foo")
	require.Nil(t, err)
	require.NotNil(t, out)
	require.Equal(t, HealthCritical, state)
	require.Equal(t, 8000, out.Service.Port)

	state, outs, err := agent.AgentHealthServiceByName("foo")
	require.Nil(t, err)
	require.NotNil(t, outs)
	require.Equal(t, HealthCritical, state)
	require.Equal(t, 8000, outs[0].Service.Port)

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentServicesWithFilterOpts(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	require.NoError(t, agent.ServiceRegister(reg))

	reg = &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo2",
		Tags: []string{"foo", "baz"},
		Port: 8001,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	require.NoError(t, agent.ServiceRegister(reg))

	opts := &QueryOptions{Namespace: defaultNamespace}
	services, err := agent.ServicesWithFilterOpts("foo in Tags", opts)
	require.NoError(t, err)
	require.Len(t, services, 1)
	_, ok := services["foo2"]
	require.True(t, ok)
}

func TestAPI_AgentServices_SidecarService(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// Register service
	reg := &AgentServiceRegistration{
		Name: "foo",
		Port: 8000,
		Connect: &AgentServiceConnect{
			SidecarService: &AgentServiceRegistration{},
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if _, ok := services["foo-sidecar-proxy"]; !ok {
		t.Fatalf("missing sidecar service: %v", services)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister should have removed both service and it's sidecar
	services, err = agent.Services()
	require.NoError(t, err)

	if _, ok := services["foo"]; ok {
		t.Fatalf("didn't remove service: %v", services)
	}
	if _, ok := services["foo-sidecar-proxy"]; ok {
		t.Fatalf("didn't remove sidecar service: %v", services)
	}
}

func TestAPI_AgentServices_ExternalConnectProxy(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// Register service
	reg := &AgentServiceRegistration{
		Name: "foo",
		Port: 8000,
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Register proxy
	reg = &AgentServiceRegistration{
		Kind: ServiceKindConnectProxy,
		Name: "foo-proxy",
		Port: 8001,
		Proxy: &AgentServiceConnectProxyConfig{
			DestinationServiceName: "foo",
			Mode:                   ProxyModeTransparent,
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if _, ok := services["foo-proxy"]; !ok {
		t.Fatalf("missing proxy service: %v", services)
	}
	if services["foo-proxy"].Proxy.Mode != ProxyModeTransparent {
		t.Fatalf("expected transparent proxy mode to be enabled")
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.ServiceDeregister("foo-proxy"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentServices_CheckPassing(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL:    "15s",
			Status: HealthPassing,
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["service:foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}

	if chk.Status != HealthPassing {
		t.Fatalf("Bad: %#v", chk)
	}
	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentServices_CheckBadStatus(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL:    "15s",
			Status: "fluffy",
		},
	}
	if err := agent.ServiceRegister(reg); err == nil {
		t.Fatalf("bad status accepted")
	}
}

func TestAPI_AgentServices_CheckID(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			CheckID: "foo-ttl",
			TTL:     "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["foo-ttl"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}
}

func TestAPI_AgentServiceAddress(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg1 := &AgentServiceRegistration{
		Name:    "foo1",
		Port:    8000,
		Address: "192.168.0.42",
	}
	reg2 := &AgentServiceRegistration{
		Name: "foo2",
		Port: 8000,
		TaggedAddresses: map[string]ServiceAddress{
			"lan": {
				Address: "192.168.0.43",
				Port:    8000,
			},
			"wan": {
				Address: "198.18.0.1",
				Port:    80,
			},
		},
	}
	if err := agent.ServiceRegister(reg1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.ServiceRegister(reg2); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := services["foo1"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if _, ok := services["foo2"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	if services["foo1"].Address != "192.168.0.42" {
		t.Fatalf("missing Address field in service foo1: %v", services)
	}
	if services["foo2"].Address != "" {
		t.Fatalf("missing Address field in service foo2: %v", services)
	}
	require.NotNil(t, services["foo2"].TaggedAddresses)
	require.Contains(t, services["foo2"].TaggedAddresses, "lan")
	require.Contains(t, services["foo2"].TaggedAddresses, "wan")
	require.Equal(t, services["foo2"].TaggedAddresses["lan"].Address, "192.168.0.43")
	require.Equal(t, services["foo2"].TaggedAddresses["lan"].Port, 8000)
	require.Equal(t, services["foo2"].TaggedAddresses["wan"].Address, "198.18.0.1")
	require.Equal(t, services["foo2"].TaggedAddresses["wan"].Port, 80)

	if err := agent.ServiceDeregister("foo1"); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := agent.ServiceDeregister("foo2"); err != nil {
		t.Fatalf("err: %v", err)
	}
}
func TestAPI_AgentServiceSocket(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg1 := &AgentServiceRegistration{
		Name:    "foo1",
		Port:    8000,
		Address: "192.168.0.42",
	}
	reg2 := &AgentServiceRegistration{
		Name:       "foo2",
		SocketPath: "/tmp/foo2.sock",
	}

	if err := agent.ServiceRegister(reg1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.ServiceRegister(reg2); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.Contains(t, services, "foo1", "missing service foo1")
	require.Contains(t, services, "foo2", "missing service foo2")

	require.Equal(t, "192.168.0.42", services["foo1"].Address,
		"missing Address field in service foo1: %v", services["foo1"])

	require.Equal(t, "", services["foo2"].Address,
		"unexpected Address field in service foo1: %v", services["foo2"])
	require.Equal(t, "/tmp/foo2.sock", services["foo2"].SocketPath,
		"missing SocketPath field in service foo1: %v", services["foo2"])
}

func TestAPI_AgentEnableTagOverride(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg1 := &AgentServiceRegistration{
		Name:              "foo1",
		Port:              8000,
		Address:           "192.168.0.42",
		EnableTagOverride: true,
	}
	reg2 := &AgentServiceRegistration{
		Name: "foo2",
		Port: 8000,
	}
	if err := agent.ServiceRegister(reg1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.ServiceRegister(reg2); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := services["foo1"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if services["foo1"].EnableTagOverride != true {
		t.Fatalf("tag override not set on service foo1: %v", services)
	}
	if _, ok := services["foo2"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if services["foo2"].EnableTagOverride != false {
		t.Fatalf("tag override set on service foo2: %v", services)
	}
}

func TestAPI_AgentServices_MultipleChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Checks: AgentServiceChecks{
			&AgentServiceCheck{
				TTL: "15s",
			},
			&AgentServiceCheck{
				TTL: "30s",
			},
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["service:foo:1"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if _, ok := checks["service:foo:2"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}
}

func TestAPI_AgentService(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Checks: AgentServiceChecks{
			&AgentServiceCheck{
				TTL: "15s",
			},
			&AgentServiceCheck{
				TTL: "30s",
			},
		},
	}
	require.NoError(t, agent.ServiceRegister(reg))

	got, qm, err := agent.Service("foo", nil)
	require.NoError(t, err)

	expect := &AgentService{
		ID:          "foo",
		Service:     "foo",
		Tags:        []string{"bar", "baz"},
		ContentHash: "3e352f348d44f7eb",
		Port:        8000,
		Weights: AgentWeights{
			Passing: 1,
			Warning: 1,
		},
		Meta:       map[string]string{},
		Namespace:  defaultNamespace,
		Partition:  defaultPartition,
		Datacenter: "dc1",
	}
	require.Equal(t, expect, got)
	require.Equal(t, expect.ContentHash, qm.LastContentHash)

	// Sanity check blocking behavior - this is more thoroughly tested in the
	// agent endpoint tests but this ensures that the API package is at least
	// passing the hash param properly.
	opts := QueryOptions{
		WaitHash: qm.LastContentHash,
		WaitTime: 100 * time.Millisecond, // Just long enough to be reliably measurable
	}
	start := time.Now()
	_, _, err = agent.Service("foo", &opts)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed >= opts.WaitTime)
}

func TestAPI_AgentSetTTLStatus(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)

	reg := &AgentServiceRegistration{
		Name: "foo",
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	verify := func(status, output string) {
		checks, err := agent.Checks()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		chk, ok := checks["service:foo"]
		if !ok {
			t.Fatalf("missing check: %v", checks)
		}
		if chk.Status != status {
			t.Fatalf("Bad: %#v", chk)
		}
		if chk.Output != output {
			t.Fatalf("Bad: %#v", chk)
		}
	}

	if err := agent.WarnTTL("service:foo", "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthWarning, "foo")

	if err := agent.PassTTL("service:foo", "bar"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthPassing, "bar")

	if err := agent.FailTTL("service:foo", "baz"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthCritical, "baz")

	if err := agent.UpdateTTL("service:foo", "foo", "warn"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthWarning, "foo")

	if err := agent.UpdateTTL("service:foo", "bar", "pass"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthPassing, "bar")

	if err := agent.UpdateTTL("service:foo", "baz", "fail"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthCritical, "baz")

	if err := agent.UpdateTTL("service:foo", "foo", HealthWarning); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthWarning, "foo")

	if err := agent.UpdateTTL("service:foo", "bar", HealthPassing); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthPassing, "bar")

	if err := agent.UpdateTTL("service:foo", "baz", HealthCritical); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthCritical, "baz")

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentUpdateTTLOpts(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)

	reg := &AgentServiceRegistration{
		Name: "foo",
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	verify := func(status, output string) {
		checks, err := agent.Checks()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		chk, ok := checks["service:foo"]
		if !ok {
			t.Fatalf("missing check: %v", checks)
		}
		if chk.Status != status {
			t.Fatalf("Bad: %#v", chk)
		}
		if chk.Output != output {
			t.Fatalf("Bad: %#v", chk)
		}
	}

	opts := &QueryOptions{Namespace: defaultNamespace}

	if err := agent.UpdateTTLOpts("service:foo", "foo", HealthWarning, opts); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthWarning, "foo")

	if err := agent.UpdateTTLOpts("service:foo", "bar", HealthPassing, opts); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthPassing, "bar")

	if err := agent.UpdateTTL("service:foo", "baz", HealthCritical); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthCritical, "baz")

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentCheckRegistration{
		Name: "foo",
	}
	reg.TTL = "15s"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if chk.Status != HealthCritical {
		t.Fatalf("check not critical: %v", chk)
	}
	if chk.Type != "ttl" {
		t.Fatalf("expected type ttl, got %s", chk.Type)
	}

	if err := agent.CheckDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentChecksWithFilterOpts(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentCheckRegistration{
		Name: "foo",
	}
	reg.TTL = "15s"
	require.NoError(t, agent.CheckRegister(reg))
	reg = &AgentCheckRegistration{
		Name: "bar",
	}
	reg.TTL = "15s"
	require.NoError(t, agent.CheckRegister(reg))

	opts := &QueryOptions{Namespace: defaultNamespace}
	checks, err := agent.ChecksWithFilterOpts("Name == foo", opts)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	_, ok := checks["foo"]
	require.True(t, ok)
}

func TestAPI_AgentScriptCheck(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.EnableScriptChecks = true
	})
	defer s.Stop()

	agent := c.Agent()

	t.Run("node script check", func(t *testing.T) {
		reg := &AgentCheckRegistration{
			Name: "foo",
			AgentServiceCheck: AgentServiceCheck{
				Interval: "10s",
				Args:     []string{"sh", "-c", "false"},
			},
		}
		if err := agent.CheckRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}

		checks, err := agent.Checks()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if _, ok := checks["foo"]; !ok {
			t.Fatalf("missing check: %v", checks)
		}
	})

	t.Run("service script check", func(t *testing.T) {
		reg := &AgentServiceRegistration{
			Name: "bar",
			Port: 1234,
			Checks: AgentServiceChecks{
				&AgentServiceCheck{
					Interval: "10s",
					Args:     []string{"sh", "-c", "false"},
				},
			},
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}

		services, err := agent.Services()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if _, ok := services["bar"]; !ok {
			t.Fatalf("missing service: %v", services)
		}

		checks, err := agent.Checks()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if _, ok := checks["service:bar"]; !ok {
			t.Fatalf("missing check: %v", checks)
		}
	})
}

func TestAPI_AgentCheckStartPassing(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentCheckRegistration{
		Name: "foo",
		AgentServiceCheck: AgentServiceCheck{
			Status: HealthPassing,
		},
	}
	reg.TTL = "15s"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if chk.Status != HealthPassing {
		t.Fatalf("check not passing: %v", chk)
	}

	if err := agent.CheckDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentChecks_serviceBound(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check bound to the service
	reg := &AgentCheckRegistration{
		Name:      "redischeck",
		ServiceID: "redis",
	}
	reg.TTL = "15s"
	reg.DeregisterCriticalServiceAfter = "nope"
	err := agent.CheckRegister(reg)
	if err == nil || !strings.Contains(err.Error(), "invalid duration") {
		t.Fatalf("err: %v", err)
	}

	reg.DeregisterCriticalServiceAfter = "90m"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check, ok := checks["redischeck"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if check.ServiceID != "redis" {
		t.Fatalf("missing service association for check: %v", check)
	}
	if check.Type != "ttl" {
		t.Fatalf("expected type ttl, got %s", check.Type)
	}
}

func TestAPI_AgentChecks_Docker(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.EnableScriptChecks = true
	})
	defer s.Stop()

	agent := c.Agent()

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check bound to the service
	reg := &AgentCheckRegistration{
		Name:      "redischeck",
		ServiceID: "redis",
		AgentServiceCheck: AgentServiceCheck{
			DockerContainerID: "f972c95ebf0e",
			Args:              []string{"/bin/true"},
			Shell:             "/bin/bash",
			Interval:          "10s",
		},
	}
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check, ok := checks["redischeck"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if check.ServiceID != "redis" {
		t.Fatalf("missing service association for check: %v", check)
	}
	if check.Type != "docker" {
		t.Fatalf("expected type docker, got %s", check.Type)
	}
}

func TestAPI_AgentJoin(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Join ourself
	addr := info["DebugConfig"]["SerfAdvertiseAddrLAN"].(string)
	// strip off 'tcp://'
	addr = addr[len("tcp://"):]
	err = agent.Join(addr, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentLeave(t *testing.T) {
	t.Parallel()
	c1, s1 := makeClient(t)
	defer s1.Stop()

	c2, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Server = false
		conf.Bootstrap = false
	})
	defer s2.Stop()

	if err := c2.Agent().Join(s1.LANAddr, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We sometimes see an EOF response to this one, depending on timing.
	err := c2.Agent().Leave()
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("err: %v", err)
	}

	// Make sure the second agent's status is 'Left'
	members, err := c1.Agent().Members(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	member := members[0]
	if member.Name == s1.Config.NodeName {
		member = members[1]
	}
	if member.Status != int(serf.StatusLeft) {
		t.Fatalf("bad: %v", *member)
	}
}

func TestAPI_AgentForceLeave(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// Eject somebody
	err := agent.ForceLeave(s.Config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentForceLeavePrune(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// Eject somebody
	err := agent.ForceLeavePrune(s.Config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentMonitor(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	logCh, err := agent.Monitor("debug", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		{
			// Register a service to be sure something happens in secs
			serviceReg := &AgentServiceRegistration{
				Name: "redis",
			}
			if err := agent.ServiceRegister(serviceReg); err != nil {
				r.Fatalf("err: %v", err)
			}
		}
		// Wait for the first log message and validate it
		select {
		case log := <-logCh:
			if !(strings.Contains(log, "[INFO]") || strings.Contains(log, "[DEBUG]")) {
				r.Fatalf("bad: %q", log)
			}
		case <-time.After(10 * time.Second):
			r.Fatalf("failed to get a log message")
		}
	})
}

func TestAPI_AgentMonitorJSON(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	logCh, err := agent.MonitorJSON("debug", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		{
			// Register a service to be sure something happens in secs
			serviceReg := &AgentServiceRegistration{
				Name: "redis",
			}
			if err := agent.ServiceRegister(serviceReg); err != nil {
				r.Fatalf("err: %v", err)
			}
		}
		// Wait for the first log message and validate it is valid JSON
		select {
		case log := <-logCh:
			var output map[string]interface{}
			if err := json.Unmarshal([]byte(log), &output); err != nil {
				r.Fatalf("log output was not JSON: %q", log)
			}
		case <-time.After(10 * time.Second):
			r.Fatalf("failed to get a log message")
		}
	})
}

func TestAPI_ServiceMaintenanceOpts(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Specify namespace in query option
	opts := &QueryOptions{Namespace: defaultNamespace}

	// Enable maintenance mode
	if err := agent.EnableServiceMaintenanceOpts("redis", "broken", opts); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure a critical check was added
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			found = true
			if check.Status != HealthCritical || check.Notes != "broken" {
				t.Fatalf("bad: %#v", checks)
			}
		}
	}
	if !found {
		t.Fatalf("bad: %#v", checks)
	}

	// Disable maintenance mode
	if err := agent.DisableServiceMaintenanceOpts("redis", opts); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the critical health check was removed
	checks, err = agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			t.Fatalf("should have removed health check")
		}
		if check.Type != "maintenance" {
			t.Fatalf("expected type 'maintenance', got %s", check.Type)
		}
	}
}

func TestAPI_NodeMaintenance(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)

	// Enable maintenance mode
	if err := agent.EnableNodeMaintenance("broken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that a critical check was added
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			found = true
			if check.Status != HealthCritical || check.Notes != "broken" {
				t.Fatalf("bad: %#v", checks)
			}
		}
	}
	if !found {
		t.Fatalf("bad: %#v", checks)
	}

	// Disable maintenance mode
	if err := agent.DisableNodeMaintenance(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was removed
	checks, err = agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			t.Fatalf("should have removed health check")
		}
		if check.Type != "maintenance" {
			t.Fatalf("expected type 'maintenance', got %s", check.Type)
		}
	}
}

func TestAPI_AgentUpdateToken(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	t.Run("deprecated", func(t *testing.T) {
		agent := c.Agent()
		if _, err := agent.UpdateACLToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := agent.UpdateACLAgentToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := agent.UpdateACLAgentMasterToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := agent.UpdateACLReplicationToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("new with no fallback", func(t *testing.T) {
		agent := c.Agent()
		if _, err := agent.UpdateDefaultACLToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := agent.UpdateAgentACLToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := agent.UpdateAgentMasterACLToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := agent.UpdateAgentRecoveryACLToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := agent.UpdateReplicationACLToken("root", nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("new with fallback", func(t *testing.T) {
		// Respond with 404 for the new paths to trigger fallback.
		failer := func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(404)
		}
		notfound := httptest.NewServer(http.HandlerFunc(failer))
		defer notfound.Close()

		raw := c // real consul client

		// Set up a reverse proxy that will send some requests to the
		// 404 server and pass everything else through to the real Consul
		// server.
		director := func(req *http.Request) {
			req.URL.Scheme = "http"

			switch req.URL.Path {
			case "/v1/agent/token/default",
				"/v1/agent/token/agent",
				"/v1/agent/token/agent_master",
				"/v1/agent/token/replication":
				req.URL.Host = notfound.URL[7:] // Strip off "http://".
			default:
				req.URL.Host = raw.config.Address
			}
		}
		proxy := httptest.NewServer(&httputil.ReverseProxy{Director: director})
		defer proxy.Close()

		// Make another client that points at the proxy instead of the real
		// Consul server.
		config := raw.config
		config.Address = proxy.URL[7:] // Strip off "http://".
		c, err := NewClient(&config)
		require.NoError(t, err)

		agent := c.Agent()

		_, err = agent.UpdateDefaultACLToken("root", nil)
		require.NoError(t, err)

		_, err = agent.UpdateAgentACLToken("root", nil)
		require.NoError(t, err)

		_, err = agent.UpdateAgentMasterACLToken("root", nil)
		require.NoError(t, err)

		_, err = agent.UpdateAgentRecoveryACLToken("root", nil)
		require.NoError(t, err)

		_, err = agent.UpdateReplicationACLToken("root", nil)
		require.NoError(t, err)
	})

	t.Run("new with 403s", func(t *testing.T) {
		failer := func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(403)
		}
		authdeny := httptest.NewServer(http.HandlerFunc(failer))
		defer authdeny.Close()

		raw := c // real consul client

		// Make another client that points at the proxy instead of the real
		// Consul server.
		config := raw.config
		config.Address = authdeny.URL[7:] // Strip off "http://".
		c, err := NewClient(&config)
		require.NoError(t, err)

		agent := c.Agent()

		_, err = agent.UpdateDefaultACLToken("root", nil)
		require.Error(t, err)

		_, err = agent.UpdateAgentACLToken("root", nil)
		require.Error(t, err)

		_, err = agent.UpdateAgentMasterACLToken("root", nil)
		require.Error(t, err)

		_, err = agent.UpdateReplicationACLToken("root", nil)
		require.Error(t, err)
	})
}

func TestAPI_AgentConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		// Explicitly disable Connect to prevent CA being bootstrapped
		c.Connect = map[string]interface{}{
			"enabled": false,
		}
	})
	defer s.Stop()

	agent := c.Agent()
	_, _, err := agent.ConnectCARoots(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Connect must be enabled")
}

func TestAPI_AgentConnectCARoots_list(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)
	list, meta, err := agent.ConnectCARoots(nil)
	require.NoError(t, err)
	require.True(t, meta.LastIndex > 0)
	require.Len(t, list.Roots, 1)
}

func TestAPI_AgentConnectCALeaf(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	// ensure we don't try to sign a leaf cert before connect has been initialized
	s.WaitForActiveCARoot(t)

	agent := c.Agent()
	// Setup service
	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
	}
	require.NoError(t, agent.ServiceRegister(reg))

	leaf, meta, err := agent.ConnectCALeaf("foo", nil)
	require.NoError(t, err)
	require.True(t, meta.LastIndex > 0)
	// Sanity checks here as we have actual certificate validation checks at many
	// other levels.
	require.NotEmpty(t, leaf.SerialNumber)
	require.NotEmpty(t, leaf.CertPEM)
	require.NotEmpty(t, leaf.PrivateKeyPEM)
	require.Equal(t, "foo", leaf.Service)
	require.True(t, strings.HasSuffix(leaf.ServiceURI, "/svc/foo"))
	require.True(t, leaf.ModifyIndex > 0)
	require.True(t, leaf.ValidAfter.Before(time.Now()))
	require.True(t, leaf.ValidBefore.After(time.Now()))
}

func TestAPI_AgentConnectAuthorize(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	s.WaitForSerfCheck(t)
	params := &AgentAuthorizeParams{
		Target:           "foo",
		ClientCertSerial: "fake",
		// Importing connect.TestSpiffeIDService creates an import cycle
		ClientCertURI: "spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/ny1/svc/web",
	}
	auth, err := agent.ConnectAuthorize(params)
	require.Nil(t, err)
	require.True(t, auth.Authorized)
	require.Equal(t, auth.Reason, "Default behavior configured by ACLs")
}

func TestAPI_AgentHealthServiceOpts(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	requireServiceHealthID := func(t *testing.T, serviceID, expected string, shouldExist bool) {
		msg := fmt.Sprintf("service id:%s, shouldExist:%v, expectedStatus:%s : bad %%s", serviceID, shouldExist, expected)

		opts := &QueryOptions{Namespace: defaultNamespace}
		state, out, err := agent.AgentHealthServiceByIDOpts(serviceID, opts)
		require.Nil(t, err, msg, "err")
		require.Equal(t, expected, state, msg, "state")
		if !shouldExist {
			require.Nil(t, out, msg, "shouldExist")
		} else {
			require.NotNil(t, out, msg, "output")
			require.Equal(t, serviceID, out.Service.ID, msg, "output")
		}
	}
	requireServiceHealthName := func(t *testing.T, serviceName, expected string, shouldExist bool) {
		msg := fmt.Sprintf("service name:%s, shouldExist:%v, expectedStatus:%s : bad %%s", serviceName, shouldExist, expected)

		opts := &QueryOptions{Namespace: defaultNamespace}
		state, outs, err := agent.AgentHealthServiceByNameOpts(serviceName, opts)
		require.Nil(t, err, msg, "err")
		require.Equal(t, expected, state, msg, "state")
		if !shouldExist {
			require.Equal(t, 0, len(outs), msg, "output")
		} else {
			require.True(t, len(outs) > 0, msg, "output")
			for _, o := range outs {
				require.Equal(t, serviceName, o.Service.Service, msg, "output")
			}
		}
	}

	requireServiceHealthID(t, "_i_do_not_exist_", HealthCritical, false)
	requireServiceHealthName(t, "_i_do_not_exist_", HealthCritical, false)

	testServiceID1 := "foo"
	testServiceID2 := "foofoo"
	testServiceName := "bar"

	// register service
	reg := &AgentServiceRegistration{
		Name: testServiceName,
		ID:   testServiceID1,
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	err := agent.ServiceRegister(reg)
	require.Nil(t, err)
	requireServiceHealthID(t, testServiceID1, HealthCritical, true)
	requireServiceHealthName(t, testServiceName, HealthCritical, true)

	err = agent.WarnTTL(fmt.Sprintf("service:%s", testServiceID1), "I am warn")
	require.Nil(t, err)
	requireServiceHealthName(t, testServiceName, HealthWarning, true)
	requireServiceHealthID(t, testServiceID1, HealthWarning, true)

	err = agent.PassTTL(fmt.Sprintf("service:%s", testServiceID1), "I am good :)")
	require.Nil(t, err)
	requireServiceHealthName(t, testServiceName, HealthPassing, true)
	requireServiceHealthID(t, testServiceID1, HealthPassing, true)

	err = agent.FailTTL(fmt.Sprintf("service:%s", testServiceID1), "I am dead.")
	require.Nil(t, err)
	requireServiceHealthName(t, testServiceName, HealthCritical, true)
	requireServiceHealthID(t, testServiceID1, HealthCritical, true)

	// register another service
	reg = &AgentServiceRegistration{
		Name: testServiceName,
		ID:   testServiceID2,
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	err = agent.ServiceRegister(reg)
	require.Nil(t, err)
	requireServiceHealthName(t, testServiceName, HealthCritical, true)

	err = agent.PassTTL(fmt.Sprintf("service:%s", testServiceID1), "I am good :)")
	require.Nil(t, err)
	requireServiceHealthName(t, testServiceName, HealthCritical, true)

	err = agent.WarnTTL(fmt.Sprintf("service:%s", testServiceID2), "I am warn")
	require.Nil(t, err)
	requireServiceHealthName(t, testServiceName, HealthWarning, true)

	err = agent.PassTTL(fmt.Sprintf("service:%s", testServiceID2), "I am good :)")
	require.Nil(t, err)
	requireServiceHealthName(t, testServiceName, HealthPassing, true)
}

func TestAgentService_JSON_OmitTaggedAdddresses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		as   AgentService
	}{
		{
			"nil",
			AgentService{
				TaggedAddresses: nil,
			},
		},
		{
			"empty",
			AgentService{
				TaggedAddresses: make(map[string]ServiceAddress),
			},
		},
	}

	for _, tc := range cases {
		name := tc.name
		as := tc.as
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(as)
			require.NoError(t, err)
			var raw map[string]interface{}
			err = json.Unmarshal(data, &raw)
			require.NoError(t, err)
			require.NotContains(t, raw, "TaggedAddresses")
			require.NotContains(t, raw, "tagged_addresses")
		})
	}
}

func TestAgentService_Register_MeshGateway(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := AgentServiceRegistration{
		Kind:    ServiceKindMeshGateway,
		Name:    "mesh-gateway",
		Address: "10.1.2.3",
		Port:    8443,
		Proxy: &AgentServiceConnectProxyConfig{
			Config: map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	err := agent.ServiceRegister(&reg)
	require.NoError(t, err)

	svc, _, err := agent.Service("mesh-gateway", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.Equal(t, ServiceKindMeshGateway, svc.Kind)
	require.NotNil(t, svc.Proxy)
	require.Contains(t, svc.Proxy.Config, "foo")
	require.Equal(t, "bar", svc.Proxy.Config["foo"])
}

func TestAgentService_Register_TerminatingGateway(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := AgentServiceRegistration{
		Kind:    ServiceKindTerminatingGateway,
		Name:    "terminating-gateway",
		Address: "10.1.2.3",
		Port:    8443,
		Proxy: &AgentServiceConnectProxyConfig{
			Config: map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	err := agent.ServiceRegister(&reg)
	require.NoError(t, err)

	svc, _, err := agent.Service("terminating-gateway", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.Equal(t, ServiceKindTerminatingGateway, svc.Kind)
	require.NotNil(t, svc.Proxy)
	require.Contains(t, svc.Proxy.Config, "foo")
	require.Equal(t, "bar", svc.Proxy.Config["foo"])
}

func TestAgentService_ExposeChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	path := ExposePath{
		LocalPathPort: 8080,
		ListenerPort:  21500,
		Path:          "/metrics",
		Protocol:      "http2",
	}
	reg := AgentServiceRegistration{
		Kind:    ServiceKindConnectProxy,
		Name:    "expose-proxy",
		Address: "10.1.2.3",
		Port:    8443,
		Proxy: &AgentServiceConnectProxyConfig{
			DestinationServiceName: "expose",
			Expose: ExposeConfig{
				Checks: true,
				Paths: []ExposePath{
					path,
				},
			},
		},
	}

	err := agent.ServiceRegister(&reg)
	require.NoError(t, err)

	svc, _, err := agent.Service("expose-proxy", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.Equal(t, ServiceKindConnectProxy, svc.Kind)
	require.NotNil(t, svc.Proxy)
	require.Len(t, svc.Proxy.Expose.Paths, 1)
	require.True(t, svc.Proxy.Expose.Checks)
	require.Equal(t, path, svc.Proxy.Expose.Paths[0])
}

func TestMemberACLMode(t *testing.T) {
	type testCase struct {
		tagValue     string
		expectedMode MemberACLMode
	}

	cases := map[string]testCase{
		"disabled": {
			tagValue:     "0",
			expectedMode: ACLModeDisabled,
		},
		"enabled": {
			tagValue:     "1",
			expectedMode: ACLModeEnabled,
		},
		"legacy": {
			tagValue:     "2",
			expectedMode: ACLModeLegacy,
		},
		"unknown-3": {
			tagValue:     "3",
			expectedMode: ACLModeUnknown,
		},
		"unknown-other": {
			tagValue:     "77",
			expectedMode: ACLModeUnknown,
		},
		"unknown-not-present": {
			tagValue:     "",
			expectedMode: ACLModeUnknown,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			tags := map[string]string{}

			if tcase.tagValue != "" {
				tags[MemberTagKeyACLMode] = tcase.tagValue
			}

			m := AgentMember{
				Tags: tags,
			}

			require.Equal(t, tcase.expectedMode, m.ACLMode())
		})
	}
}

func TestMemberIsConsulServer(t *testing.T) {
	type testCase struct {
		tagValue string
		isServer bool
	}

	cases := map[string]testCase{
		"not-present": {
			tagValue: "",
			isServer: false,
		},
		"server": {
			tagValue: MemberTagValueRoleServer,
			isServer: true,
		},
		"client": {
			tagValue: "client",
			isServer: false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			tags := map[string]string{}

			if tcase.tagValue != "" {
				tags[MemberTagKeyRole] = tcase.tagValue
			}

			m := AgentMember{
				Tags: tags,
			}

			require.Equal(t, tcase.isServer, m.IsConsulServer())
		})
	}
}
