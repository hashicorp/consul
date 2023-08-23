package consul

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/testrpc"
)

func TestSession_Apply(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Just add a node
	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})

	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			Name: "my-session",
		},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	id := out

	// Verify
	state := s1.fsm.State()
	_, s, err := state.SessionGet(nil, out, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Fatalf("should not be nil")
	}
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
	if s.Name != "my-session" {
		t.Fatalf("bad: %v", s)
	}

	// Do a delete
	arg.Op = structs.SessionDestroy
	arg.Session.ID = out
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify
	_, s, err = state.SessionGet(nil, id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s != nil {
		t.Fatalf("bad: %v", s)
	}
}

func TestSession_DeleteApply(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Just add a node
	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})

	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node:     "foo",
			Name:     "my-session",
			Behavior: structs.SessionKeysDelete,
		},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	id := out

	// Verify
	state := s1.fsm.State()
	_, s, err := state.SessionGet(nil, out, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Fatalf("should not be nil")
	}
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
	if s.Name != "my-session" {
		t.Fatalf("bad: %v", s)
	}
	if s.Behavior != structs.SessionKeysDelete {
		t.Fatalf("bad: %v", s)
	}

	// Do a delete
	arg.Op = structs.SessionDestroy
	arg.Session.ID = out
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify
	_, s, err = state.SessionGet(nil, id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s != nil {
		t.Fatalf("bad: %v", s)
	}
}

func TestSession_Apply_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	rules := `
session "foo" {
	policy = "write"
}
`
	token := createToken(t, codec, rules)

	// Just add a node.
	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})

	// Try to create without a token, which will be denied.
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			Name: "my-session",
		},
	}
	var id string
	err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &id)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Now set a token and try again. This should go through.
	arg.Token = token
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the delete of the session fails without a token.
	var out string
	arg.Op = structs.SessionDestroy
	arg.Session.ID = id
	arg.Token = ""
	err = msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Now set a token and try again. This should go through.
	arg.Token = token
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSession_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
		},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	getR := structs.SessionSpecificRequest{
		Datacenter: "dc1",
		SessionID:  out,
	}
	var sessions structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.Get", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 1 {
		t.Fatalf("Bad: %v", sessions)
	}
	s := sessions.Sessions[0]
	if s.ID != out {
		t.Fatalf("bad: %v", s)
	}
}

func TestSession_Get_Compat(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
		},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	getR := structs.SessionSpecificRequest{
		Datacenter: "dc1",
		// this should get converted to the SessionID field internally
		Session: out,
	}
	var sessions structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.Get", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 1 {
		t.Fatalf("Bad: %v", sessions)
	}
	s := sessions.Sessions[0]
	if s.ID != out {
		t.Fatalf("bad: %v", s)
	}
}

func TestSession_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	ids := []string{}
	for i := 0; i < 5; i++ {
		arg := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: "foo",
			},
		}
		var out string
		if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		ids = append(ids, out)
	}

	getR := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var sessions structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.List", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 5 {
		t.Fatalf("Bad: %v", sessions.Sessions)
	}
	for i := 0; i < len(sessions.Sessions); i++ {
		s := sessions.Sessions[i]
		if !stringslice.Contains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}
	}
}

func TestSession_Get_List_NodeSessions_ACLFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	deniedToken := createTokenWithPolicyName(t, codec, "denied", `
		session "foo" {
			policy = "deny"
		}
	`, "root")

	allowedToken := createTokenWithPolicyName(t, codec, "allowed", `
		session "foo" {
			policy = "read"
		}
	`, "root")

	// Create a node and a session.
	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out)
	require.NoError(t, err)

	t.Run("Get", func(t *testing.T) {

		req := &structs.SessionSpecificRequest{
			Datacenter: "dc1",
			SessionID:  out,
		}
		req.Token = deniedToken

		// ACL-restricted results filtered out.
		var sessions structs.IndexedSessions

		err := msgpackrpc.CallWithCodec(codec, "Session.Get", req, &sessions)
		require.NoError(t, err)
		require.Empty(t, sessions.Sessions)
		require.True(t, sessions.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

		// ACL-restricted results included.
		req.Token = allowedToken

		err = msgpackrpc.CallWithCodec(codec, "Session.Get", req, &sessions)
		require.NoError(t, err)
		require.Len(t, sessions.Sessions, 1)
		require.False(t, sessions.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")

		// Try to get a session that doesn't exist to make sure that's handled
		// correctly by the filter (it will get passed a nil slice).
		req.SessionID = "adf4238a-882b-9ddc-4a9d-5b6758e4159e"

		err = msgpackrpc.CallWithCodec(codec, "Session.Get", req, &sessions)
		require.NoError(t, err)
		require.Empty(t, sessions.Sessions)
		require.False(t, sessions.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("List", func(t *testing.T) {

		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		req.Token = deniedToken

		// ACL-restricted results filtered out.
		var sessions structs.IndexedSessions

		err := msgpackrpc.CallWithCodec(codec, "Session.List", req, &sessions)
		require.NoError(t, err)
		require.Empty(t, sessions.Sessions)
		require.True(t, sessions.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

		// ACL-restricted results included.
		req.Token = allowedToken

		err = msgpackrpc.CallWithCodec(codec, "Session.List", req, &sessions)
		require.NoError(t, err)
		require.Len(t, sessions.Sessions, 1)
		require.False(t, sessions.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("NodeSessions", func(t *testing.T) {

		req := &structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       "foo",
		}
		req.Token = deniedToken

		// ACL-restricted results filtered out.
		var sessions structs.IndexedSessions

		err := msgpackrpc.CallWithCodec(codec, "Session.NodeSessions", req, &sessions)
		require.NoError(t, err)
		require.Empty(t, sessions.Sessions)
		require.True(t, sessions.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

		// ACL-restricted results included.
		req.Token = allowedToken

		err = msgpackrpc.CallWithCodec(codec, "Session.NodeSessions", req, &sessions)
		require.NoError(t, err)
		require.Len(t, sessions.Sessions, 1)
		require.False(t, sessions.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})
}

func TestSession_ApplyTimers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			TTL:  "10s",
		},
	}
	var out string
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the session map
	if s1.sessionTimers.Get(out) == nil {
		t.Fatalf("missing session timer")
	}

	// Destroy the session
	arg.Op = structs.SessionDestroy
	arg.Session.ID = out
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the session map
	if s1.sessionTimers.Get(out) != nil {
		t.Fatalf("session timer exists")
	}
}

func TestSession_Renew(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This method is timing sensitive, disable Parallel
	//t.Parallel()
	ttl := 1 * time.Second
	TTL := ttl.String()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.SessionTTLMin = ttl
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	ids := []string{}
	for i := 0; i < 5; i++ {
		arg := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: "foo",
				TTL:  TTL,
			},
		}
		var out string
		if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		ids = append(ids, out)
	}

	// Verify the timer map is setup
	if s1.sessionTimers.Len() != 5 {
		t.Fatalf("missing session timers")
	}

	getR := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	var sessions structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.List", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 5 {
		t.Fatalf("Bad: %v", sessions.Sessions)
	}
	for i := 0; i < len(sessions.Sessions); i++ {
		s := sessions.Sessions[i]
		if !stringslice.Contains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}
		if s.TTL != TTL {
			t.Fatalf("bad session TTL: %s %v", s.TTL, s)
		}
		t.Logf("Created session '%s'", s.ID)
	}

	// Sleep for time shorter than internal destroy ttl
	time.Sleep(ttl * structs.SessionTTLMultiplier / 2)

	// renew 3 out of 5 sessions
	for i := 0; i < 3; i++ {
		renewR := structs.SessionSpecificRequest{
			Datacenter: "dc1",
			SessionID:  ids[i],
		}
		var session structs.IndexedSessions
		if err := msgpackrpc.CallWithCodec(codec, "Session.Renew", &renewR, &session); err != nil {
			t.Fatalf("err: %v", err)
		}

		if session.Index == 0 {
			t.Fatalf("Bad: %v", session)
		}
		if len(session.Sessions) != 1 {
			t.Fatalf("Bad: %v", session.Sessions)
		}

		s := session.Sessions[0]
		if !stringslice.Contains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}

		t.Logf("Renewed session '%s'", s.ID)
	}

	// now sleep for 2/3 the internal destroy TTL time for renewed sessions
	// which is more than the internal destroy TTL time for the non-renewed sessions
	time.Sleep((ttl * structs.SessionTTLMultiplier) * 2.0 / 3.0)

	var sessionsL1 structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.List", &getR, &sessionsL1); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessionsL1.Index == 0 {
		t.Fatalf("Bad: %v", sessionsL1)
	}

	t.Logf("Expect 2 sessions to be destroyed")

	for i := 0; i < len(sessionsL1.Sessions); i++ {
		s := sessionsL1.Sessions[i]
		if !stringslice.Contains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}
		if s.TTL != TTL {
			t.Fatalf("bad: %v", s)
		}
		if i > 2 {
			t.Errorf("session '%s' should be destroyed", s.ID)
		}
	}

	if len(sessionsL1.Sessions) != 3 {
		t.Fatalf("Bad: %v", sessionsL1.Sessions)
	}

	// now sleep again for ttl*2 - no sessions should still be alive
	time.Sleep(ttl * structs.SessionTTLMultiplier)

	var sessionsL2 structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.List", &getR, &sessionsL2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessionsL2.Index == 0 {
		t.Fatalf("Bad: %v", sessionsL2)
	}
	if len(sessionsL2.Sessions) != 0 {
		for i := 0; i < len(sessionsL2.Sessions); i++ {
			s := sessionsL2.Sessions[i]
			if !stringslice.Contains(ids, s.ID) {
				t.Fatalf("bad: %v", s)
			}
			if s.Node != "foo" {
				t.Fatalf("bad: %v", s)
			}
			if s.TTL != TTL {
				t.Fatalf("bad: %v", s)
			}
			t.Errorf("session '%s' should be destroyed", s.ID)
		}

		t.Fatalf("Bad: %v", sessionsL2.Sessions)
	}
}

func TestSession_Renew_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	rules := `
session "foo" {
	policy = "write"
}
`
	token := createToken(t, codec, rules)

	// Just add a node.
	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})

	// Create a session.
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			Name: "my-session",
		},
		WriteRequest: structs.WriteRequest{Token: token},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Renew without a token should be rejected.
	renewR := structs.SessionSpecificRequest{
		Datacenter: "dc1",
		SessionID:  id,
	}
	var session structs.IndexedSessions
	err := msgpackrpc.CallWithCodec(codec, "Session.Renew", &renewR, &session)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Set the token and it should go through.
	renewR.Token = token
	if err := msgpackrpc.CallWithCodec(codec, "Session.Renew", &renewR, &session); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSession_Renew_Compat(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This method is timing sensitive, disable Parallel
	//t.Parallel()
	ttl := 5 * time.Second
	TTL := ttl.String()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.SessionTTLMin = ttl
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	var id string
	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			TTL:  TTL,
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// renew the session
	renewR := structs.SessionSpecificRequest{
		Datacenter: "dc1",
		// this will get ranslated internally to the SessionID field
		Session: id,
	}
	var session structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.Renew", &renewR, &session); err != nil {
		t.Fatalf("err: %v", err)
	}

	if session.Index == 0 {
		t.Fatalf("Bad: %v", session)
	}
	if len(session.Sessions) != 1 {
		t.Fatalf("Bad: %v", session.Sessions)
	}

	s := session.Sessions[0]
	if id != s.ID {
		t.Fatalf("bad: %v", s)
	}
	if s.Node != "foo" {
		t.Fatalf("bad: %v", s)
	}
}

func TestSession_NodeSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "bar", Address: "127.0.0.1"})
	ids := []string{}
	for i := 0; i < 10; i++ {
		arg := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: "bar",
			},
		}
		if i < 5 {
			arg.Session.Node = "foo"
		}
		var out string
		if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		if i < 5 {
			ids = append(ids, out)
		}
	}

	getR := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var sessions structs.IndexedSessions
	if err := msgpackrpc.CallWithCodec(codec, "Session.NodeSessions", &getR, &sessions); err != nil {
		t.Fatalf("err: %v", err)
	}

	if sessions.Index == 0 {
		t.Fatalf("Bad: %v", sessions)
	}
	if len(sessions.Sessions) != 5 {
		t.Fatalf("Bad: %v", sessions.Sessions)
	}
	for i := 0; i < len(sessions.Sessions); i++ {
		s := sessions.Sessions[i]
		if !stringslice.Contains(ids, s.ID) {
			t.Fatalf("bad: %v", s)
		}
		if s.Node != "foo" {
			t.Fatalf("bad: %v", s)
		}
	}
}

func TestSession_Apply_BadTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.SessionRequest{
		Datacenter: "dc1",
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node: "foo",
			Name: "my-session",
		},
	}

	// Session with illegal TTL
	arg.Session.TTL = "10z"

	var out string
	err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != `Session TTL '10z' invalid: time: unknown unit "z" in duration "10z"` {
		t.Fatalf("incorrect error message: %s", err.Error())
	}

	// less than SessionTTLMin
	arg.Session.TTL = "5s"

	err = msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "Invalid Session TTL '5000000000', must be between [10s=24h0m0s]" {
		t.Fatalf("incorrect error message: %s", err.Error())
	}

	// more than SessionTTLMax
	arg.Session.TTL = "100000s"

	err = msgpackrpc.CallWithCodec(codec, "Session.Apply", &arg, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "Invalid Session TTL '100000000000000', must be between [10s=24h0m0s]" {
		t.Fatalf("incorrect error message: %s", err.Error())
	}
}
