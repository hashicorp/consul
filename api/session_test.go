// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_SessionCreateDestroy(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	id, meta, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	meta, err = session.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}
}

func TestAPI_SessionCreateRenewDestroy(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	se := &SessionEntry{
		TTL: "10s",
	}

	id, meta, err := session.Create(se, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	renew, meta, err := session.Renew(id, nil)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if renew == nil {
		t.Fatalf("should get session")
	}

	if renew.ID != id {
		t.Fatalf("should have matching id")
	}

	if renew.TTL != "10s" {
		t.Fatalf("should get session with TTL")
	}
}

func TestAPI_SessionCreateRenewDestroyRenew(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	entry := &SessionEntry{
		Behavior: SessionBehaviorDelete,
		TTL:      "500s", // disable ttl
	}

	id, meta, err := session.Create(entry, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	// Extend right after create. Everything should be fine.
	entry, _, err = session.Renew(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if entry == nil {
		t.Fatal("session unexpectedly vanished")
	}

	// Simulate TTL loss by manually destroying the session.
	meta, err = session.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	// Extend right after delete. The 404 should proxy as a nil.
	entry, _, err = session.Renew(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if entry != nil {
		t.Fatal("session still exists")
	}
}

func TestAPI_SessionCreateDestroyRenewPeriodic(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	entry := &SessionEntry{
		Behavior: SessionBehaviorDelete,
		TTL:      "500s", // disable ttl
	}

	id, meta, err := session.Create(entry, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	// This only tests Create/Destroy/RenewPeriodic to avoid the more
	// difficult case of testing all of the timing code.

	// Simulate TTL loss by manually destroying the session.
	meta, err = session.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	// Extend right after delete. The 404 should terminate the loop quickly and return ErrSessionExpired.
	errCh := make(chan error, 1)
	doneCh := make(chan struct{})
	go func() { errCh <- session.RenewPeriodic("1s", id, nil, doneCh) }()
	defer close(doneCh)

	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timedout: missing session did not terminate renewal loop")
	case err = <-errCh:
		if err != ErrSessionExpired {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestAPI_SessionRenewPeriodic_Cancel(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()
	entry := &SessionEntry{
		Behavior: SessionBehaviorDelete,
		TTL:      "500s", // disable ttl
	}

	t.Run("done channel", func(t *testing.T) {
		id, _, err := session.Create(entry, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		errCh := make(chan error, 1)
		doneCh := make(chan struct{})
		go func() { errCh <- session.RenewPeriodic("1s", id, nil, doneCh) }()

		close(doneCh)

		select {
		case <-time.After(1 * time.Second):
			t.Fatal("renewal loop didn't terminate")
		case err = <-errCh:
			if err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		sess, _, err := session.Info(id, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if sess != nil {
			t.Fatalf("session was not expired")
		}
	})

	t.Run("context", func(t *testing.T) {
		id, _, err := session.Create(entry, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		wo := new(WriteOptions).WithContext(ctx)

		errCh := make(chan error, 1)
		go func() { errCh <- session.RenewPeriodic("1s", id, wo, nil) }()

		cancel()

		select {
		case <-time.After(1 * time.Second):
			t.Fatal("renewal loop didn't terminate")
		case err = <-errCh:
			if err == nil || !strings.Contains(err.Error(), "context canceled") {
				t.Fatalf("err: %v", err)
			}
		}

		// See comment in session.go for why the session isn't removed
		// in this case.
		sess, _, err := session.Info(id, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if sess == nil {
			t.Fatalf("session should not be expired")
		}
	})
}

func TestAPI_SessionInfo(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	id, _, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, qm, err := session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}

	if info.CreateIndex == 0 {
		t.Fatalf("bad: %v", info)
	}
	info.CreateIndex = 0

	want := &SessionEntry{
		ID:         id,
		Node:       s.Config.NodeName,
		NodeChecks: []string{"serfHealth"},
		LockDelay:  15 * time.Second,
		Behavior:   SessionBehaviorRelease,
	}
	if info.ID != want.ID {
		t.Fatalf("bad ID: %s", info.ID)
	}
	if info.Node != want.Node {
		t.Fatalf("bad Node: %s", info.Node)
	}
	if info.LockDelay != want.LockDelay {
		t.Fatalf("bad LockDelay: %d", info.LockDelay)
	}
	if info.Behavior != want.Behavior {
		t.Fatalf("bad Behavior: %s", info.Behavior)
	}
	if len(info.NodeChecks) != len(want.NodeChecks) {
		t.Fatalf("expected %d nodechecks, got %d", len(want.NodeChecks), len(info.NodeChecks))
	}
	if info.NodeChecks[0] != want.NodeChecks[0] {
		t.Fatalf("expected nodecheck %s, got %s", want.NodeChecks, info.NodeChecks)
	}
}

func TestAPI_SessionInfo_NoChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	id, _, err := session.CreateNoChecks(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, qm, err := session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}

	if info.CreateIndex == 0 {
		t.Fatalf("bad: %v", info)
	}
	info.CreateIndex = 0

	want := &SessionEntry{
		ID:         id,
		Node:       s.Config.NodeName,
		NodeChecks: []string{},
		LockDelay:  15 * time.Second,
		Behavior:   SessionBehaviorRelease,
	}
	if info.ID != want.ID {
		t.Fatalf("bad ID: %s", info.ID)
	}
	if info.Node != want.Node {
		t.Fatalf("bad Node: %s", info.Node)
	}
	if info.LockDelay != want.LockDelay {
		t.Fatalf("bad LockDelay: %d", info.LockDelay)
	}
	if info.Behavior != want.Behavior {
		t.Fatalf("bad Behavior: %s", info.Behavior)
	}
	assert.Equal(t, want.Checks, info.Checks)
	assert.Equal(t, want.NodeChecks, info.NodeChecks)
}

func TestAPI_SessionNode(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	id, _, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, _, err := session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sessions, qm, err := session.Node(info.Node, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("bad: %v", sessions)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
}

func TestAPI_SessionList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	session := c.Session()

	id, _, err := session.Create(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	sessions, qm, err := session.List(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("bad: %v", sessions)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
}

func TestAPI_SessionNodeChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	// Node check that doesn't exist should yield error on creation
	se := SessionEntry{
		NodeChecks: []string{"dne"},
	}
	session := c.Session()

	_, _, err := session.Create(&se, nil)
	if err == nil {
		t.Fatalf("should have failed")
	}

	// Empty node check should lead to serf check
	se.NodeChecks = []string{}
	id, _, err := session.Create(&se, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, qm, err := session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
	if info.CreateIndex == 0 {
		t.Fatalf("bad: %v", info)
	}
	info.CreateIndex = 0

	want := &SessionEntry{
		ID:         id,
		Node:       s.Config.NodeName,
		NodeChecks: []string{"serfHealth"},
		LockDelay:  15 * time.Second,
		Behavior:   SessionBehaviorRelease,
	}
	want.Namespace = info.Namespace
	assert.Equal(t, want, info)

	// Register a new node with a non-serf check
	cr := CatalogRegistration{
		Datacenter: "dc1",
		Node:       "foo",
		ID:         "e0155642-135d-4739-9853-a1ee6c9f945b",
		Address:    "127.0.0.2",
		Checks: HealthChecks{
			&HealthCheck{
				Node:    "foo",
				CheckID: "foo:alive",
				Name:    "foo-liveness",
				Status:  HealthPassing,
				Notes:   "foo is alive and well",
			},
		},
	}
	catalog := c.Catalog()
	if _, err := catalog.Register(&cr, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// If a custom node check is provided, it should overwrite serfHealth default
	se.Node = "foo"
	se.NodeChecks = []string{"foo:alive"}

	id, _, err = session.Create(&se, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, qm, err = session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
	if info.CreateIndex == 0 {
		t.Fatalf("bad: %v", info)
	}
	info.CreateIndex = 0

	want = &SessionEntry{
		ID:         id,
		Node:       "foo",
		NodeChecks: []string{"foo:alive"},
		LockDelay:  15 * time.Second,
		Behavior:   SessionBehaviorRelease,
	}
	want.Namespace = info.Namespace
	assert.Equal(t, want, info)
}

func TestAPI_SessionServiceChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)

	// Node check that doesn't exist should yield error on creation
	se := SessionEntry{
		ServiceChecks: []ServiceCheck{
			{"dne", ""},
		},
	}
	session := c.Session()

	_, _, err := session.Create(&se, nil)
	if err == nil {
		t.Fatalf("should have failed")
	}

	// Register a new service with a check
	cr := CatalogRegistration{
		Datacenter:     "dc1",
		Node:           s.Config.NodeName,
		SkipNodeUpdate: true,
		Service: &AgentService{
			Kind:    ServiceKindTypical,
			ID:      "redisV2",
			Service: "redis",
			Port:    1235,
			Address: "198.18.1.2",
		},
		Checks: HealthChecks{
			&HealthCheck{
				Node:      s.Config.NodeName,
				CheckID:   "redis:alive",
				Status:    HealthPassing,
				ServiceID: "redisV2",
			},
		},
	}
	catalog := c.Catalog()
	if _, err := catalog.Register(&cr, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// If a custom check is provided, it should be present in session info
	se.ServiceChecks = []ServiceCheck{
		{"redis:alive", ""},
	}

	id, _, err := session.Create(&se, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer session.Destroy(id, nil)

	info, qm, err := session.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
	if info.CreateIndex == 0 {
		t.Fatalf("bad: %v", info)
	}
	info.CreateIndex = 0

	want := &SessionEntry{
		ID:            id,
		Node:          s.Config.NodeName,
		ServiceChecks: []ServiceCheck{{"redis:alive", ""}},
		NodeChecks:    []string{"serfHealth"},
		LockDelay:     15 * time.Second,
		Behavior:      SessionBehaviorRelease,
	}
	want.Namespace = info.Namespace
	assert.Equal(t, want, info)
}
