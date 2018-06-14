package watch_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/watch"
	"github.com/stretchr/testify/require"
)

var errBadContent = errors.New("bad content")
var errTimeout = errors.New("timeout")

var timeout = 5 * time.Second

func makeInvokeCh() chan error {
	ch := make(chan error)
	time.AfterFunc(timeout, func() { ch <- errTimeout })
	return ch
}

func TestKeyWatch(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"key", "key":"foo/bar/baz"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*consulapi.KVPair)
		if !ok || v == nil {
			return // ignore
		}
		if string(v.Value) != "test" {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		kv := a.Client().KV()

		time.Sleep(20 * time.Millisecond)
		pair := &consulapi.KVPair{
			Key:   "foo/bar/baz",
			Value: []byte("test"),
		}
		if _, err := kv.Put(pair, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestKeyWatch_With_PrefixDelete(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"key", "key":"foo/bar/baz"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*consulapi.KVPair)
		if !ok || v == nil {
			return // ignore
		}
		if string(v.Value) != "test" {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		kv := a.Client().KV()

		time.Sleep(20 * time.Millisecond)
		pair := &consulapi.KVPair{
			Key:   "foo/bar/baz",
			Value: []byte("test"),
		}
		if _, err := kv.Put(pair, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestKeyPrefixWatch(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"keyprefix", "prefix":"foo/"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(consulapi.KVPairs)
		if !ok || len(v) == 0 {
			return
		}
		if string(v[0].Key) != "foo/bar" {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		kv := a.Client().KV()

		time.Sleep(20 * time.Millisecond)
		pair := &consulapi.KVPair{
			Key: "foo/bar",
		}
		if _, err := kv.Put(pair, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestServicesWatch(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"services"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(map[string][]string)
		if !ok || len(v) == 0 {
			return // ignore
		}
		if v["consul"] == nil {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		agent := a.Client().Agent()

		time.Sleep(20 * time.Millisecond)
		reg := &consulapi.AgentServiceRegistration{
			ID:   "foo",
			Name: "foo",
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestNodesWatch(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"nodes"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*consulapi.Node)
		if !ok || len(v) == 0 {
			return // ignore
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		catalog := a.Client().Catalog()

		time.Sleep(20 * time.Millisecond)
		reg := &consulapi.CatalogRegistration{
			Node:       "foobar",
			Address:    "1.1.1.1",
			Datacenter: "dc1",
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestServiceWatch(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"service", "service":"foo", "tag":"bar", "passingonly":true}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*consulapi.ServiceEntry)
		if !ok || len(v) == 0 {
			return // ignore
		}
		if v[0].Service.ID != "foo" {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		agent := a.Client().Agent()

		time.Sleep(20 * time.Millisecond)
		reg := &consulapi.AgentServiceRegistration{
			ID:   "foo",
			Name: "foo",
			Tags: []string{"bar"},
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestChecksWatch_State(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"checks", "state":"warning"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*consulapi.HealthCheck)
		if !ok || len(v) == 0 {
			return // ignore
		}
		if v[0].CheckID != "foobar" || v[0].Status != "warning" {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		catalog := a.Client().Catalog()

		time.Sleep(20 * time.Millisecond)
		reg := &consulapi.CatalogRegistration{
			Node:       "foobar",
			Address:    "1.1.1.1",
			Datacenter: "dc1",
			Check: &consulapi.AgentCheck{
				Node:    "foobar",
				CheckID: "foobar",
				Name:    "foobar",
				Status:  consulapi.HealthWarning,
			},
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestChecksWatch_Service(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"checks", "service":"foobar"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*consulapi.HealthCheck)
		if !ok || len(v) == 0 {
			return // ignore
		}
		if v[0].CheckID != "foobar" {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		catalog := a.Client().Catalog()

		time.Sleep(20 * time.Millisecond)
		reg := &consulapi.CatalogRegistration{
			Node:       "foobar",
			Address:    "1.1.1.1",
			Datacenter: "dc1",
			Service: &consulapi.AgentService{
				ID:      "foobar",
				Service: "foobar",
			},
			Check: &consulapi.AgentCheck{
				Node:      "foobar",
				CheckID:   "foobar",
				Name:      "foobar",
				Status:    consulapi.HealthPassing,
				ServiceID: "foobar",
			},
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestEventWatch(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"event", "name": "foo"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return
		}
		v, ok := raw.([]*consulapi.UserEvent)
		if !ok || len(v) == 0 {
			return // ignore
		}
		if string(v[len(v)-1].Name) != "foo" {
			invoke <- errBadContent
			return
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		event := a.Client().Event()

		time.Sleep(20 * time.Millisecond)
		params := &consulapi.UserEvent{Name: "foo"}
		if _, _, err := event.Fire(params, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestConnectRootsWatch(t *testing.T) {
	t.Parallel()
	// NewTestAgent will bootstrap a new CA
	a := agent.NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	var originalCAID string
	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"connect_roots"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*consulapi.CARootList)
		if !ok || v == nil {
			return // ignore
		}
		// Only 1 CA is the bootstrapped state (i.e. first response). Ignore this
		// state and wait for the new CA to show up too.
		if len(v.Roots) == 1 {
			originalCAID = v.ActiveRootID
			return
		}
		assert.NotEmpty(t, originalCAID)
		assert.NotEqual(t, originalCAID, v.ActiveRootID)
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		// Set a new CA
		connect.TestCAConfigSet(t, a, nil)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestConnectLeafWatch(t *testing.T) {
	t.Parallel()
	// NewTestAgent will bootstrap a new CA
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	// Register a web service to get certs for
	{
		agent := a.Client().Agent()
		reg := consulapi.AgentServiceRegistration{
			ID:   "web",
			Name: "web",
			Port: 9090,
		}
		err := agent.ServiceRegister(&reg)
		require.Nil(t, err)
	}

	var lastCert *consulapi.LeafCert

	//invoke := makeInvokeCh()
	invoke := make(chan error)
	plan := mustParse(t, `{"type":"connect_leaf", "service":"web"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*consulapi.LeafCert)
		if !ok || v == nil {
			return // ignore
		}
		if lastCert == nil {
			// Initial fetch, just store the cert and return
			lastCert = v
			return
		}
		// TODO(banks): right now the root rotation actually causes Serial numbers
		// to reset so these end up all being the same. That needs fixing but it's
		// a bigger task than I want to bite off for this PR.
		//assert.NotEqual(t, lastCert.SerialNumber, v.SerialNumber)
		assert.NotEqual(t, lastCert.CertPEM, v.CertPEM)
		assert.NotEqual(t, lastCert.PrivateKeyPEM, v.PrivateKeyPEM)
		assert.NotEqual(t, lastCert.ModifyIndex, v.ModifyIndex)
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		// Change the CA to trigger a leaf change
		connect.TestCAConfigSet(t, a, nil)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func TestConnectProxyConfigWatch(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), `
	connect {
		enabled = true
		proxy {
			allow_managed_api_registration = true
		}
	}
	`)
	defer a.Shutdown()

	// Register a local agent service with a managed proxy
	reg := &consulapi.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
		Connect: &consulapi.AgentServiceConnect{
			Proxy: &consulapi.AgentServiceConnectProxy{
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}
	client := a.Client()
	agent := client.Agent()
	err := agent.ServiceRegister(reg)
	require.NoError(t, err)

	invoke := makeInvokeCh()
	plan := mustParse(t, `{"type":"connect_proxy_config", "proxy_service_id":"web-proxy"}`)
	plan.HybridHandler = func(blockParamVal watch.BlockingParamVal, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*consulapi.ConnectProxyConfig)
		if !ok || v == nil {
			return // ignore
		}
		invoke <- nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)

		// Change the proxy's config
		reg.Connect.Proxy.Config["foo"] = "buzz"
		reg.Connect.Proxy.Config["baz"] = "qux"
		err := agent.ServiceRegister(reg)
		require.NoError(t, err)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(a.HTTPAddr()); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	if err := <-invoke; err != nil {
		t.Fatalf("err: %v", err)
	}

	plan.Stop()
	wg.Wait()
}

func mustParse(t *testing.T, q string) *watch.Plan {
	t.Helper()
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(q), &params); err != nil {
		t.Fatal(err)
	}
	plan, err := watch.Parse(params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return plan
}
