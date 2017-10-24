package watch_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/watch"
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

func mustParse(t *testing.T, q string) *watch.Plan {
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
