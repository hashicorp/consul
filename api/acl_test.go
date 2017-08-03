package api

import (
	"testing"
)

func TestAPI_ACLBootstrap(t *testing.T) {
	// TODO (slackpad) We currently can't inject the version, and the
	// version in the binary depends on Git tags, so we can't reliably
	// test this until we are just running an agent in-process here and
	// have full control over the config.
}

func TestAPI_ACLCreateDestroy(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	ae := ACLEntry{
		Name:  "API test",
		Type:  ACLClientType,
		Rules: `key "" { policy = "deny" }`,
	}

	id, wm, err := acl.Create(&ae, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	ae2, _, err := acl.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ae2.Name != ae.Name || ae2.Type != ae.Type || ae2.Rules != ae.Rules {
		t.Fatalf("Bad: %#v", ae2)
	}

	wm, err = acl.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}
}

func TestAPI_ACLCloneDestroy(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	id, wm, err := acl.Clone(c.config.Token, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	wm, err = acl.Destroy(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.RequestTime == 0 {
		t.Fatalf("bad: %v", wm)
	}
}

func TestAPI_ACLInfo(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	ae, qm, err := acl.Info(c.config.Token, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}

	if ae == nil || ae.ID != c.config.Token || ae.Type != ACLManagementType {
		t.Fatalf("bad: %#v", ae)
	}
}

func TestAPI_ACLList(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	acls, qm, err := acl.List(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(acls) < 2 {
		t.Fatalf("bad: %v", acls)
	}

	if qm.LastIndex == 0 {
		t.Fatalf("bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("bad: %v", qm)
	}
}

func TestAPI_ACLReplication(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	repl, qm, err := acl.Replication(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if repl == nil {
		t.Fatalf("bad: %v", repl)
	}

	if repl.Running {
		t.Fatal("bad: repl should not be running")
	}

	if repl.Enabled {
		t.Fatal("bad: repl should not be enabled")
	}

	if qm.RequestTime == 0 {
		t.Fatalf("bad: %v", qm)
	}
}
