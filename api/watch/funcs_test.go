// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package watch_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func makeClient(t *testing.T) (*api.Client, *testutil.TestServer) {
	// Skip test when -short flag provided; any tests that create a test server
	// will take at least 100ms which is undesirable for -short
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Make client config
	conf := api.DefaultConfig()

	// Create server
	server, err := testutil.NewTestServerConfigT(t, nil)
	require.NoError(t, err)
	conf.Address = server.HTTPAddr

	server.WaitForLeader(t)

	// Create client
	client, err := api.NewClient(conf)
	if err != nil {
		server.Stop()
		// guaranteed to fail but will be a nice error message
		require.NoError(t, err)
	}

	return client, server
}

func updateConnectCA(t *testing.T, client *api.Client) {
	t.Helper()

	connect := client.Connect()
	cfg, _, err := connect.CAGetConfig(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg.Config)

	// update the cert
	// Create the private key we'll use for this CA cert.
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	bs, err := x509.MarshalECPrivateKey(pk)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: bs}))
	cfg.Config["PrivateKey"] = buf.String()

	bs, err = x509.MarshalPKIXPublicKey(pk.Public())
	require.NoError(t, err)
	hash := sha256.Sum256(bs)
	kID := hash[:]

	// Create the CA cert
	template := x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "CA Modified"},
		URIs:                  []*url.URL{{Scheme: "spiffe", Host: fmt.Sprintf("11111111-2222-3333-4444-555555555555.%s", "consul")}},
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign |
			x509.KeyUsageDigitalSignature,
		IsCA:           true,
		NotAfter:       time.Now().AddDate(10, 0, 0),
		NotBefore:      time.Now(),
		AuthorityKeyId: kID,
		SubjectKeyId:   kID,
	}

	bs, err = x509.CreateCertificate(rand.Reader, &template, &template, pk.Public(), pk)
	require.NoError(t, err)

	buf.Reset()
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs}))
	cfg.Config["RootCert"] = buf.String()

	_, err = connect.CASetConfig(cfg, nil)
	require.NoError(t, err)
}

func TestKeyWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	var (
		wakeups  []*api.KVPair
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"key", "key":"foo/bar/baz"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		var v *api.KVPair
		if raw == nil { // nil is a valid return value
			v = nil
		} else {
			var ok bool
			if v, ok = raw.(*api.KVPair); !ok {
				return // ignore
			}
		}

		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		kv := c.KV()
		pair := &api.KVPair{
			Key:   "foo/bar/baz",
			Value: []byte("test"),
		}
		if _, err := kv.Put(pair, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Nil(t, v)
	}
	{
		v := wakeups[1]
		require.Equal(t, "test", string(v.Value))
	}
}

func TestKeyWatch_With_PrefixDelete(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	var (
		wakeups  []*api.KVPair
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"key", "key":"foo/bar/baz"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		var v *api.KVPair
		if raw == nil { // nil is a valid return value
			v = nil
		} else {
			var ok bool
			if v, ok = raw.(*api.KVPair); !ok {
				return // ignore
			}
		}

		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh

	{
		kv := c.KV()
		pair := &api.KVPair{
			Key:   "foo/bar/baz",
			Value: []byte("test"),
		}
		if _, err := kv.Put(pair, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Nil(t, v)
	}
	{
		v := wakeups[1]
		require.Equal(t, "test", string(v.Value))
	}
}

func TestKeyPrefixWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	var (
		wakeups  []api.KVPairs
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"keyprefix", "prefix":"foo/"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(api.KVPairs)
		if !ok {
			return
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		kv := c.KV()
		pair := &api.KVPair{
			Key: "foo/bar",
		}
		if _, err := kv.Put(pair, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 0)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 1)
		require.Equal(t, "foo/bar", v[0].Key)
	}
}

func TestServicesWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	var (
		wakeups  []map[string][]string
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"services"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(map[string][]string)
		if !ok {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		agent := c.Agent()

		reg := &api.AgentServiceRegistration{
			ID:   "foo",
			Name: "foo",
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 1)
		_, ok := v["consul"]
		require.True(t, ok)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 2)
		_, ok := v["consul"]
		require.True(t, ok)
		_, ok = v["foo"]
		require.True(t, ok)
	}

}

func TestNodesWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t) // wait for AE to sync

	var (
		wakeups  [][]*api.Node
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"nodes"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*api.Node)
		if !ok {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		catalog := c.Catalog()

		reg := &api.CatalogRegistration{
			Node:       "foobar",
			Address:    "1.1.1.1",
			Datacenter: "dc1",
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	var originalNodeName string
	{
		v := wakeups[0]
		require.Len(t, v, 1)
		originalNodeName = v[0].Node
	}
	{
		v := wakeups[1]
		require.Len(t, v, 2)
		if v[0].Node == originalNodeName {
			require.Equal(t, "foobar", v[1].Node)
		} else {
			require.Equal(t, "foobar", v[0].Node)
		}
	}
}

func TestServiceWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	var (
		wakeups  [][]*api.ServiceEntry
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"service", "service":"foo", "tag":"bar", "passingonly":true}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*api.ServiceEntry)
		if !ok {
			return // ignore
		}

		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		agent := c.Agent()

		reg := &api.AgentServiceRegistration{
			ID:   "foo",
			Name: "foo",
			Tags: []string{"bar"},
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 0)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 1)
		require.Equal(t, "foo", v[0].Service.ID)
	}
}

func TestServiceMultipleTagsWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	var (
		wakeups  [][]*api.ServiceEntry
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"service", "service":"foo", "tag":["bar","buzz"], "passingonly":true}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*api.ServiceEntry)
		if !ok {
			return // ignore
		}

		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		agent := c.Agent()

		// we do not want to find this one.
		reg := &api.AgentServiceRegistration{
			ID:   "foobarbiff",
			Name: "foo",
			Tags: []string{"bar", "biff"},
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}

		// we do not want to find this one.
		reg = &api.AgentServiceRegistration{
			ID:   "foobuzzbiff",
			Name: "foo",
			Tags: []string{"buzz", "biff"},
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}

		// we want to find this one
		reg = &api.AgentServiceRegistration{
			ID:   "foobarbuzzbiff",
			Name: "foo",
			Tags: []string{"bar", "buzz", "biff"},
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 0)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 1)

		require.Equal(t, "foobarbuzzbiff", v[0].Service.ID)
		require.ElementsMatch(t, []string{"bar", "buzz", "biff"}, v[0].Service.Tags)
	}
}

func TestChecksWatch_State(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	var (
		wakeups  [][]*api.HealthCheck
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"checks", "state":"warning"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*api.HealthCheck)
		if !ok {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		catalog := c.Catalog()

		reg := &api.CatalogRegistration{
			Node:       "foobar",
			Address:    "1.1.1.1",
			Datacenter: "dc1",
			Check: &api.AgentCheck{
				Node:    "foobar",
				CheckID: "foobar",
				Name:    "foobar",
				Status:  api.HealthWarning,
			},
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 0)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 1)
		require.Equal(t, "foobar", v[0].CheckID)
		require.Equal(t, "warning", v[0].Status)
	}
}

func TestChecksWatch_Service(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	var (
		wakeups  [][]*api.HealthCheck
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"checks", "service":"foobar"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*api.HealthCheck)
		if !ok {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		catalog := c.Catalog()

		reg := &api.CatalogRegistration{
			Node:       "foobar",
			Address:    "1.1.1.1",
			Datacenter: "dc1",
			Service: &api.AgentService{
				ID:      "foobar",
				Service: "foobar",
			},
			Check: &api.AgentCheck{
				Node:      "foobar",
				CheckID:   "foobar",
				Name:      "foobar",
				Status:    api.HealthPassing,
				ServiceID: "foobar",
			},
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 0)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 1)
		require.Equal(t, "foobar", v[0].CheckID)
	}
}

func TestChecksWatch_Service_Tags(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	var (
		wakeups  [][]*api.HealthCheck
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"checks", "tag":["b", "a"]}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.([]*api.HealthCheck)
		if !ok {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		catalog := c.Catalog()

		// we don't want to find this one
		reg := &api.CatalogRegistration{
			Node:       "foo",
			Address:    "1.1.1.1",
			Datacenter: "dc1",
			Service: &api.AgentService{
				ID:      "foo",
				Service: "foo",
				Tags:    []string{"a"},
			},
			Check: &api.AgentCheck{
				Node:      "foo",
				CheckID:   "foo",
				Name:      "foo",
				Status:    api.HealthPassing,
				ServiceID: "foo",
			},
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}

		// we want to find this one
		reg = &api.CatalogRegistration{
			Node:       "bar",
			Address:    "2.2.2.2",
			Datacenter: "dc1",
			Service: &api.AgentService{
				ID:      "bar",
				Service: "bar",
				Tags:    []string{"a", "b"},
			},
			Check: &api.AgentCheck{
				Node:      "bar",
				CheckID:   "bar",
				Name:      "bar",
				Status:    api.HealthPassing,
				ServiceID: "bar",
			},
		}
		if _, err := catalog.Register(reg, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 0)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 1)
		require.Equal(t, "bar", v[0].CheckID)
	}
}

func TestEventWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	var (
		wakeups  [][]*api.UserEvent
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"event", "name": "foo"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return
		}
		v, ok := raw.([]*api.UserEvent)
		if !ok {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		event := c.Event()

		params := &api.UserEvent{Name: "foo"}
		if _, _, err := event.Fire(params, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Len(t, v, 0)
	}
	{
		v := wakeups[1]
		require.Len(t, v, 1)
		require.Equal(t, "foo", v[0].Name)
	}
}

func TestConnectRootsWatch(t *testing.T) {
	t.Parallel()
	// makeClient will bootstrap a CA
	c, s := makeClient(t)
	defer s.Stop()

	var (
		wakeups  []*api.CARootList
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"connect_roots"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*api.CARootList)
		if !ok || v == nil || len(v.Roots) == 0 {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh

	updateConnectCA(t, c)

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	var originalCAID string
	{ // Only 1 CA is the bootstrapped state (i.e. first response).
		v := wakeups[0]
		require.Len(t, v.Roots, 1)
		originalCAID = v.ActiveRootID
		require.NotEmpty(t, originalCAID)
	}

	{ // This is the new CA showing up.
		v := wakeups[1]
		require.Len(t, v.Roots, 2)
		require.NotEqual(t, originalCAID, v.ActiveRootID)
	}
}

func TestConnectLeafWatch(t *testing.T) {
	t.Parallel()
	// makeClient will bootstrap a CA
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	// Register a web service to get certs for
	{
		agent := c.Agent()
		reg := api.AgentServiceRegistration{
			ID:   "web",
			Name: "web",
			Port: 9090,
		}
		err := agent.ServiceRegister(&reg)
		require.Nil(t, err)
	}

	var (
		wakeups  []*api.LeafCert
		notifyCh = make(chan struct{})
	)

	plan := mustParse(t, `{"type":"connect_leaf", "service":"web"}`)
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*api.LeafCert)
		if !ok || v == nil {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh

	start := time.Now()
	updateConnectCA(t, c)

	// Wait for second wakeup. Note due to the 30s random jitter for this endpoint
	// this may take up to 30s.
	<-notifyCh
	t.Logf("leaf cert regen took %s with jitter", time.Since(start))

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	var lastCert *api.LeafCert
	{ // Initial fetch, just store the cert and return
		v := wakeups[0]
		require.NotNil(t, v)
		lastCert = v
	}

	{ // Rotation completed.
		v := wakeups[1]
		require.NotNil(t, v)

		// TODO(banks): right now the root rotation actually causes Serial numbers
		// to reset so these end up all being the same. That needs fixing but it's
		// a bigger task than I want to bite off for this PR.
		//require.NotEqual(t, lastCert.SerialNumber, v.SerialNumber)
		require.NotEqual(t, lastCert.CertPEM, v.CertPEM)
		require.NotEqual(t, lastCert.PrivateKeyPEM, v.PrivateKeyPEM)
		require.NotEqual(t, lastCert.ModifyIndex, v.ModifyIndex)
	}
}

func TestAgentServiceWatch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	var (
		wakeups  []*api.AgentService
		notifyCh = make(chan struct{})
	)

	// Register a local agent service
	reg := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
	}
	client := c
	agent := client.Agent()
	err := agent.ServiceRegister(reg)
	require.NoError(t, err)

	plan := mustParse(t, `{"type":"agent_service", "service_id":"web"}`)
	plan.HybridHandler = func(blockParamVal watch.BlockingParamVal, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*api.AgentService)
		if !ok || v == nil {
			return // ignore
		}
		wakeups = append(wakeups, v)
		notifyCh <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := plan.Run(s.HTTPAddr); err != nil {
			t.Errorf("err: %v", err)
		}
	}()
	defer plan.Stop()

	// Wait for first wakeup.
	<-notifyCh
	{
		// Change the service definition
		reg.Port = 9090
		err := agent.ServiceRegister(reg)
		require.NoError(t, err)
	}

	// Wait for second wakeup.
	<-notifyCh

	plan.Stop()
	wg.Wait()

	require.Len(t, wakeups, 2)

	{
		v := wakeups[0]
		require.Equal(t, 8080, v.Port)
	}
	{
		v := wakeups[1]
		require.Equal(t, 9090, v.Port)
	}
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
