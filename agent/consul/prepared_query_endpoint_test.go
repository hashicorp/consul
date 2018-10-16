package consul

import (
	"bytes"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreparedQuery_Apply(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "test",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
	}
	var reply string

	// Set an ID which should fail the create.
	query.Query.ID = "nope"
	err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "ID must be empty") {
		t.Fatalf("bad: %v", err)
	}

	// Change it to a bogus modify which should also fail.
	query.Op = structs.PreparedQueryUpdate
	query.Query.ID = generateUUID()
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "Cannot modify non-existent prepared query") {
		t.Fatalf("bad: %v", err)
	}

	// Fix up the ID but invalidate the query itself. This proves we call
	// parseQuery for a create, but that function is checked in detail as
	// part of another test so we don't have to exercise all the checks
	// here.
	query.Op = structs.PreparedQueryCreate
	query.Query.ID = ""
	query.Query.Service.Failover.NearestN = -1
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "Bad NearestN") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and make sure it propagates an error from the Raft apply.
	query.Query.Service.Failover.NearestN = 0
	query.Query.Session = "nope"
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "failed session lookup") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and make sure the apply goes through.
	query.Query.Session = ""
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and read the query back to verify.
	query.Query.ID = reply
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter: "dc1",
			QueryID:    query.Query.ID,
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Make the op an update. This should go through now that we have an ID.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Service.Failover.NearestN = 2
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Read back again to verify the update worked.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter: "dc1",
			QueryID:    query.Query.ID,
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Give a bogus op and make sure it fails.
	query.Op = "nope"
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "Unknown prepared query operation:") {
		t.Fatalf("bad: %v", err)
	}

	// Prove that an update also goes through the parseQuery validation.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Service.Failover.NearestN = -1
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "Bad NearestN") {
		t.Fatalf("bad: %v", err)
	}

	// Now change the op to delete; the bad query field should be ignored
	// because all we care about for a delete op is the ID.
	query.Op = structs.PreparedQueryDelete
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that this query is deleted.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter: "dc1",
			QueryID:    query.Query.ID,
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			if err.Error() != ErrQueryNotFound.Error() {
				t.Fatalf("err: %v", err)
			}
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestPreparedQuery_Apply_ACLDeny(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with write permissions for redis queries.
	var token string
	{
		var rules = `
                    query "redis" {
                        policy = "write"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "redis-master",
			Service: structs.ServiceQuery{
				Service: "the-redis",
			},
		},
	}
	var reply string

	// Creating without a token should fail since the default policy is to
	// deny.
	err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Now add the token and try again.
	query.WriteRequest.Token = token
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and set the token, then read back the query to verify.
	// Note that unlike previous versions of Consul, we DO NOT capture the
	// token. We will set that here just to be explicit about it.
	query.Query.ID = reply
	query.Query.Token = ""
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Try to do an update without a token; this should get rejected.
	query.Op = structs.PreparedQueryUpdate
	query.WriteRequest.Token = ""
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Try again with the original token; this should go through.
	query.WriteRequest.Token = token
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try to do a delete with no token; this should get rejected.
	query.Op = structs.PreparedQueryDelete
	query.WriteRequest.Token = ""
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Try again with the original token. This should go through.
	query.WriteRequest.Token = token
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the query got deleted.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			if err.Error() != ErrQueryNotFound.Error() {
				t.Fatalf("err: %v", err)
			}
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Make the query again.
	query.Op = structs.PreparedQueryCreate
	query.Query.ID = ""
	query.WriteRequest.Token = token
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that it's there, and again make sure that the token did not get
	// captured.
	query.Query.ID = reply
	query.Query.Token = ""
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// A management token should be able to update the query no matter what.
	query.Op = structs.PreparedQueryUpdate
	query.WriteRequest.Token = "root"
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// That last update should not have captured a token.
	query.Query.Token = ""
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// A management token should be able to delete the query no matter what.
	query.Op = structs.PreparedQueryDelete
	query.WriteRequest.Token = "root"
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the query got deleted.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			if err.Error() != ErrQueryNotFound.Error() {
				t.Fatalf("err: %v", err)
			}
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Use the root token to make a query under a different name.
	query.Op = structs.PreparedQueryCreate
	query.Query.ID = ""
	query.Query.Name = "cassandra"
	query.WriteRequest.Token = "root"
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that it's there and that the token did not get captured.
	query.Query.ID = reply
	query.Query.Token = ""
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Now try to change that to redis with the valid redis token. This will
	// fail because that token can't change cassandra queries.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Name = "redis"
	query.WriteRequest.Token = token
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}
}

func TestPreparedQuery_Apply_ForwardLeader(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec1 := rpcClient(t, s1)
	defer codec1.Close()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	// Try to join.
	joinLAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Use the follower as the client.
	var codec rpc.ClientCodec
	if !s1.IsLeader() {
		codec = codec1
	} else {
		codec = codec2
	}

	// Set up a node and service in the catalog.
	{
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "redis",
				Tags:    []string{"master"},
				Port:    8000,
			},
		}
		var reply struct{}
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "test",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
	}

	// Make sure the apply works even when forwarded through the non-leader.
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestPreparedQuery_parseQuery(t *testing.T) {
	t.Parallel()
	query := &structs.PreparedQuery{}

	err := parseQuery(query, true)
	if err == nil || !strings.Contains(err.Error(), "Must be bound to a session") {
		t.Fatalf("bad: %v", err)
	}

	query.Session = "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
	err = parseQuery(query, true)
	if err == nil || !strings.Contains(err.Error(), "Must provide a Service") {
		t.Fatalf("bad: %v", err)
	}

	query.Session = ""
	query.Template.Type = "some-kind-of-template"
	err = parseQuery(query, true)
	if err == nil || !strings.Contains(err.Error(), "Must provide a Service") {
		t.Fatalf("bad: %v", err)
	}

	query.Template.Type = ""
	err = parseQuery(query, false)
	if err == nil || !strings.Contains(err.Error(), "Must provide a Service") {
		t.Fatalf("bad: %v", err)
	}

	// None of the rest of these care about version 8 ACL enforcement.
	for _, version8 := range []bool{true, false} {
		query = &structs.PreparedQuery{}
		query.Session = "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
		query.Service.Service = "foo"
		if err := parseQuery(query, version8); err != nil {
			t.Fatalf("err: %v", err)
		}

		query.Token = redactedToken
		err = parseQuery(query, version8)
		if err == nil || !strings.Contains(err.Error(), "Bad Token") {
			t.Fatalf("bad: %v", err)
		}

		query.Token = "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
		if err := parseQuery(query, version8); err != nil {
			t.Fatalf("err: %v", err)
		}

		query.Service.Failover.NearestN = -1
		err = parseQuery(query, version8)
		if err == nil || !strings.Contains(err.Error(), "Bad NearestN") {
			t.Fatalf("bad: %v", err)
		}

		query.Service.Failover.NearestN = 3
		if err := parseQuery(query, version8); err != nil {
			t.Fatalf("err: %v", err)
		}

		query.DNS.TTL = "two fortnights"
		err = parseQuery(query, version8)
		if err == nil || !strings.Contains(err.Error(), "Bad DNS TTL") {
			t.Fatalf("bad: %v", err)
		}

		query.DNS.TTL = "-3s"
		err = parseQuery(query, version8)
		if err == nil || !strings.Contains(err.Error(), "must be >=0") {
			t.Fatalf("bad: %v", err)
		}

		query.DNS.TTL = "3s"
		if err := parseQuery(query, version8); err != nil {
			t.Fatalf("err: %v", err)
		}

		query.Service.NodeMeta = map[string]string{"": "somevalue"}
		err = parseQuery(query, version8)
		if err == nil || !strings.Contains(err.Error(), "cannot be blank") {
			t.Fatalf("bad: %v", err)
		}

		query.Service.NodeMeta = map[string]string{"somekey": "somevalue"}
		if err := parseQuery(query, version8); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestPreparedQuery_ACLDeny_Catchall_Template(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with write permissions for any prefix.
	var token string
	{
		var rules = `
                    query "" {
                        policy = "write"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a catch-all template.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name:  "",
			Token: "5e1e24e5-1329-f86f-18c6-3d3734edb2cd",
			Template: structs.QueryTemplateOptions{
				Type: structs.QueryTemplateTypeNamePrefixMatch,
			},
			Service: structs.ServiceQuery{
				Service: "${name.full}",
			},
		},
	}
	var reply string

	// Creating without a token should fail since the default policy is to
	// deny.
	err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Now add the token and try again.
	query.WriteRequest.Token = token
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and read back the query to verify. Note that the token
	// will be redacted since this isn't a management token.
	query.Query.ID = reply
	query.Query.Token = redactedToken
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Try to query by ID without a token and make sure it gets denied, even
	// though this has an empty name and would normally be shown.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter: "dc1",
			QueryID:    query.Query.ID,
		}
		var resp structs.IndexedPreparedQueries
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp)
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("bad: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// We should get the same result listing all the queries without a
	// token.
	{
		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// But a management token should be able to see it, and the real token.
	query.Query.Token = "5e1e24e5-1329-f86f-18c6-3d3734edb2cd"
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Explaining should also be denied without a token.
	{
		req := &structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "anything",
		}
		var resp structs.PreparedQueryExplainResponse
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Explain", req, &resp)
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("bad: %v", err)
		}
	}

	// The user can explain and see the redacted token.
	query.Query.Token = redactedToken
	query.Query.Service.Service = "anything"
	{
		req := &structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "anything",
			QueryOptions:  structs.QueryOptions{Token: token},
		}
		var resp structs.PreparedQueryExplainResponse
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Explain", req, &resp)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		actual := &resp.Query
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Make sure the management token can also explain and see the token.
	query.Query.Token = "5e1e24e5-1329-f86f-18c6-3d3734edb2cd"
	query.Query.Service.Service = "anything"
	{
		req := &structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "anything",
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.PreparedQueryExplainResponse
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Explain", req, &resp)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		actual := &resp.Query
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}
}

func TestPreparedQuery_Get(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with write permissions for redis queries.
	var token string
	{
		var rules = `
                    query "redis" {
                        policy = "write"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "redis-master",
			Service: structs.ServiceQuery{
				Service: "the-redis",
			},
		},
		WriteRequest: structs.WriteRequest{Token: token},
	}
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID, then read back the query to verify.
	query.Query.ID = reply
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Try again with no token, which should return an error.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: ""},
		}
		var resp structs.IndexedPreparedQueries
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp)
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("bad: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// A management token should be able to read no matter what.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Create a session.
	var session string
	{
		req := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: s1.config.NodeName,
			},
		}
		if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &req, &session); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Now update the query to take away its name.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Name = ""
	query.Query.Session = session
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try again with no token, this should work since this query is only
	// managed by an ID (no name) so no ACLs apply to it.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: ""},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Capture a token.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Token = "le-token"
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// This should get redacted when we read it back without a token.
	query.Query.Token = redactedToken
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: ""},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// But a management token should be able to see it.
	query.Query.Token = "le-token"
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      query.Query.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Try to get an unknown ID.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:   "dc1",
			QueryID:      generateUUID(),
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Get", req, &resp); err != nil {
			if err.Error() != ErrQueryNotFound.Error() {
				t.Fatalf("err: %v", err)
			}
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestPreparedQuery_List(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with write permissions for redis queries.
	var token string
	{
		var rules = `
                    query "redis" {
                        policy = "write"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Query with a legit token but no queries.
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name:  "redis-master",
			Token: "le-token",
			Service: structs.ServiceQuery{
				Service: "the-redis",
			},
		},
		WriteRequest: structs.WriteRequest{Token: token},
	}
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and read back the query to verify. We also make sure
	// the captured token gets redacted.
	query.Query.ID = reply
	query.Query.Token = redactedToken
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// An empty token should result in an empty list because of ACL
	// filtering.
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: ""},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// But a management token should work, and be able to see the captured
	// token.
	query.Query.Token = "le-token"
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Create a session.
	var session string
	{
		req := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: s1.config.NodeName,
			},
		}
		if err := msgpackrpc.CallWithCodec(codec, "Session.Apply", &req, &session); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Now take away the query name.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Name = ""
	query.Query.Session = session
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// A query with the redis token shouldn't show anything since it doesn't
	// match any un-named queries.
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// But a management token should work.
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Queries[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}
}

func TestPreparedQuery_Explain(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with write permissions for prod- queries.
	var token string
	{
		var rules = `
                    query "prod-" {
                        policy = "write"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a template.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name:  "prod-",
			Token: "5e1e24e5-1329-f86f-18c6-3d3734edb2cd",
			Template: structs.QueryTemplateOptions{
				Type: structs.QueryTemplateTypeNamePrefixMatch,
			},
			Service: structs.ServiceQuery{
				Service: "${name.full}",
			},
		},
		WriteRequest: structs.WriteRequest{Token: token},
	}
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Explain via the management token.
	query.Query.ID = reply
	query.Query.Service.Service = "prod-redis"
	{
		req := &structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "prod-redis",
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.PreparedQueryExplainResponse
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Explain", req, &resp)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		actual := &resp.Query
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Explain via the user token, which will redact the captured token.
	query.Query.Token = redactedToken
	query.Query.Service.Service = "prod-redis"
	{
		req := &structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "prod-redis",
			QueryOptions:  structs.QueryOptions{Token: token},
		}
		var resp structs.PreparedQueryExplainResponse
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Explain", req, &resp)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		actual := &resp.Query
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, query.Query) {
			t.Fatalf("bad: %v", actual)
		}
	}

	// Explaining should be denied without a token, since the user isn't
	// allowed to see the query.
	{
		req := &structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "prod-redis",
		}
		var resp structs.PreparedQueryExplainResponse
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Explain", req, &resp)
		if !acl.IsErrPermissionDenied(err) {
			t.Fatalf("bad: %v", err)
		}
	}

	// Try to explain a bogus ID.
	{
		req := &structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: generateUUID(),
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Explain", req, &resp); err != nil {
			if err.Error() != ErrQueryNotFound.Error() {
				t.Fatalf("err: %v", err)
			}
		}
	}
}

// This is a beast of a test, but the setup is so extensive it makes sense to
// walk through the different cases once we have it up. This is broken into
// sections so it's still pretty easy to read.
func TestPreparedQuery_Execute(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec1 := rpcClient(t, s1)
	defer codec1.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Try to WAN join.
	joinWAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members want %d", got, want)
		}
	})

	// Create an ACL with read permission to the service.
	var execToken string
	{
		var rules = `
                    service "foo" {
                        policy = "read"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec1, "ACL.Apply", &req, &execToken); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up some nodes in each DC that host the service.
	{
		for i := 0; i < 10; i++ {
			for _, dc := range []string{"dc1", "dc2"} {
				req := structs.RegisterRequest{
					Datacenter: dc,
					Node:       fmt.Sprintf("node%d", i+1),
					Address:    fmt.Sprintf("127.0.0.%d", i+1),
					NodeMeta: map[string]string{
						"group":         fmt.Sprintf("%d", i/5),
						"instance_type": "t2.micro",
					},
					Service: &structs.NodeService{
						Service: "foo",
						Port:    8000,
						Tags:    []string{dc, fmt.Sprintf("tag%d", i+1)},
						Meta: map[string]string{
							"svc-group": fmt.Sprintf("%d", i%2),
							"foo":       "true",
						},
					},
					WriteRequest: structs.WriteRequest{Token: "root"},
				}
				if i == 0 {
					req.NodeMeta["unique"] = "true"
					req.Service.Meta["unique"] = "true"
				}

				var codec rpc.ClientCodec
				if dc == "dc1" {
					codec = codec1
				} else {
					codec = codec2
				}

				var reply struct{}
				if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply); err != nil {
					t.Fatalf("err: %v", err)
				}
			}
		}
	}

	// Set up a service query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "test",
			Service: structs.ServiceQuery{
				Service: "foo",
			},
			DNS: structs.QueryDNSOptions{
				TTL: "10s",
			},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Run a query that doesn't exist.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "nope",
		}

		var reply structs.PreparedQueryExecuteResponse
		err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply)
		if err == nil || err.Error() != ErrQueryNotFound.Error() {
			t.Fatalf("bad: %v", err)
		}

		if len(reply.Nodes) != 0 {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Run the registered query.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 10 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Try with a limit.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			Limit:         3,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Run various service queries with node metadata filters.
	{
		cases := []struct {
			filters  map[string]string
			numNodes int
		}{
			{
				filters:  map[string]string{},
				numNodes: 10,
			},
			{
				filters:  map[string]string{"instance_type": "t2.micro"},
				numNodes: 10,
			},
			{
				filters:  map[string]string{"group": "1"},
				numNodes: 5,
			},
			{
				filters:  map[string]string{"group": "0", "unique": "true"},
				numNodes: 1,
			},
		}

		for _, tc := range cases {
			nodeMetaQuery := structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Service: structs.ServiceQuery{
						Service:  "foo",
						NodeMeta: tc.filters,
					},
					DNS: structs.QueryDNSOptions{
						TTL: "10s",
					},
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &nodeMetaQuery, &nodeMetaQuery.Query.ID); err != nil {
				t.Fatalf("err: %v", err)
			}

			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: nodeMetaQuery.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}

			if len(reply.Nodes) != tc.numNodes {
				t.Fatalf("bad: %v, %v", len(reply.Nodes), tc.numNodes)
			}

			for _, node := range reply.Nodes {
				if !structs.SatisfiesMetaFilters(node.Node.Meta, tc.filters) {
					t.Fatalf("bad: %v", node.Node.Meta)
				}
			}
		}
	}

	// Run various service queries with service metadata filters
	{
		cases := []struct {
			filters  map[string]string
			numNodes int
		}{
			{
				filters:  map[string]string{},
				numNodes: 10,
			},
			{
				filters:  map[string]string{"foo": "true"},
				numNodes: 10,
			},
			{
				filters:  map[string]string{"svc-group": "0"},
				numNodes: 5,
			},
			{
				filters:  map[string]string{"svc-group": "1"},
				numNodes: 5,
			},
			{
				filters:  map[string]string{"svc-group": "0", "unique": "true"},
				numNodes: 1,
			},
		}

		for _, tc := range cases {
			svcMetaQuery := structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Service: structs.ServiceQuery{
						Service:     "foo",
						ServiceMeta: tc.filters,
					},
					DNS: structs.QueryDNSOptions{
						TTL: "10s",
					},
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}

			require.NoError(t, msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &svcMetaQuery, &svcMetaQuery.Query.ID))

			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: svcMetaQuery.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply))
			require.Len(t, reply.Nodes, tc.numNodes)
			for _, node := range reply.Nodes {
				require.True(t, structs.SatisfiesMetaFilters(node.Service.Meta, tc.filters))
			}
		}
	}

	// Push a coordinate for one of the nodes so we can try an RTT sort. We
	// have to sleep a little while for the coordinate batch to get flushed.
	{
		req := structs.CoordinateUpdateRequest{
			Datacenter: "dc1",
			Node:       "node3",
			Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
		}
		var out struct{}
		if err := msgpackrpc.CallWithCodec(codec1, "Coordinate.Update", &req, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		time.Sleep(3 * s1.config.CoordinateUpdatePeriod)
	}

	// Try an RTT sort. We don't have any other coordinates in there but
	// showing that the node with a coordinate is always first proves we
	// call the RTT sorting function, which is tested elsewhere.
	for i := 0; i < 100; i++ {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			Source: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node3",
			},
			QueryOptions: structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 10 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		if reply.Nodes[0].Node.Node != "node3" {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Make sure the shuffle looks like it's working.
	uniques := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 10 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		var names []string
		for _, node := range reply.Nodes {
			names = append(names, node.Node.Node)
		}
		key := strings.Join(names, "|")
		uniques[key] = struct{}{}
	}

	// We have to allow for the fact that there won't always be a unique
	// shuffle each pass, so we just look for smell here without the test
	// being flaky.
	if len(uniques) < 50 {
		t.Fatalf("unique shuffle ratio too low: %d/100", len(uniques))
	}

	// Set the query to return results nearest to node3. This is the only
	// node with coordinates, and it carries the service we are asking for,
	// so node3 should always show up first.
	query.Op = structs.PreparedQueryUpdate
	query.Query.Service.Near = "node3"
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now run the query and make sure the sort looks right.
	{
		req := structs.PreparedQueryExecuteRequest{
			Agent: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node3",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			if n := len(reply.Nodes); n != 10 {
				t.Fatalf("expect 10 nodes, got: %d", n)
			}
			if node := reply.Nodes[0].Node.Node; node != "node3" {
				t.Fatalf("expect node3 first, got: %q", node)
			}
		}
	}

	// Query again, but this time set a client-supplied query source. This
	// proves that we allow overriding the baked-in value with ?near.
	{
		// Set up the query with a non-existent node. This will cause the
		// nodes to be shuffled if the passed node is respected, proving
		// that we allow the override to happen.
		req := structs.PreparedQueryExecuteRequest{
			Source: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "foo",
			},
			Agent: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node3",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		shuffled := false
		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			if n := len(reply.Nodes); n != 10 {
				t.Fatalf("expect 10 nodes, got: %d", n)
			}
			if node := reply.Nodes[0].Node.Node; node != "node3" {
				shuffled = true
				break
			}
		}

		if !shuffled {
			t.Fatalf("expect nodes to be shuffled")
		}
	}

	// If the exact node we are sorting near appears in the list, make sure it
	// gets popped to the front of the result.
	{
		req := structs.PreparedQueryExecuteRequest{
			Source: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node1",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			if n := len(reply.Nodes); n != 10 {
				t.Fatalf("expect 10 nodes, got: %d", n)
			}
			if node := reply.Nodes[0].Node.Node; node != "node1" {
				t.Fatalf("expect node1 first, got: %q", node)
			}
		}
	}

	// Bake the magic "_agent" flag into the query.
	query.Query.Service.Near = "_agent"
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that we sort the local agent first when the magic flag is set.
	{
		req := structs.PreparedQueryExecuteRequest{
			Agent: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node3",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			if n := len(reply.Nodes); n != 10 {
				t.Fatalf("expect 10 nodes, got: %d", n)
			}
			if node := reply.Nodes[0].Node.Node; node != "node3" {
				t.Fatalf("expect node3 first, got: %q", node)
			}
		}
	}

	// Check that the query isn't just sorting "node3" first because we
	// provided it in the Agent query source. Proves that we use the
	// Agent source when the magic "_agent" flag is passed.
	{
		req := structs.PreparedQueryExecuteRequest{
			Agent: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "foo",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		// Expect the set to be shuffled since we have no coordinates
		// on the "foo" node.
		shuffled := false
		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			if n := len(reply.Nodes); n != 10 {
				t.Fatalf("expect 10 nodes, got: %d", n)
			}
			if node := reply.Nodes[0].Node.Node; node != "node3" {
				shuffled = true
				break
			}
		}

		if !shuffled {
			t.Fatal("expect nodes to be shuffled")
		}
	}

	// Shuffles if the response comes from a non-local DC. Proves that the
	// agent query source does not interfere with the order.
	{
		req := structs.PreparedQueryExecuteRequest{
			Source: structs.QuerySource{
				Datacenter: "dc2",
				Node:       "node3",
			},
			Agent: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node3",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		shuffled := false
		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			if n := len(reply.Nodes); n != 10 {
				t.Fatalf("expect 10 nodes, got: %d", n)
			}
			if reply.Nodes[0].Node.Node != "node3" {
				shuffled = true
				break
			}
		}

		if !shuffled {
			t.Fatal("expect node shuffle for remote results")
		}
	}

	// Un-bake the near parameter.
	query.Query.Service.Near = ""
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the health of a node to mark it critical.
	setHealth := func(node string, health string) {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       node,
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "foo",
				Port:    8000,
				Tags:    []string{"dc1", "tag1"},
			},
			Check: &structs.HealthCheck{
				Name:      "failing",
				Status:    health,
				ServiceID: "foo",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var reply struct{}
		if err := msgpackrpc.CallWithCodec(codec1, "Catalog.Register", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	setHealth("node1", api.HealthCritical)

	// The failing node should be filtered.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 9 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node1" {
				t.Fatalf("bad: %v", node)
			}
		}
	}

	// Upgrade it to a warning and re-query, should be 10 nodes again.
	setHealth("node1", api.HealthWarning)
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 10 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Make the query more picky so it excludes warning nodes.
	query.Query.Service.OnlyPassing = true
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// The node in the warning state should be filtered.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 9 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node1" {
				t.Fatalf("bad: %v", node)
			}
		}
	}

	// Make the query ignore all our health checks (which have "failing" ID
	// implicitly from their name).
	query.Query.Service.IgnoreCheckIDs = []types.CheckID{"failing"}
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should end up with 10 nodes again
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 10 ||
			reply.Datacenter != "dc1" ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Undo that so all the following tests aren't broken!
	query.Query.Service.IgnoreCheckIDs = nil
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make the query more picky by adding a tag filter. This just proves we
	// call into the tag filter, it is tested more thoroughly in a separate
	// test.
	query.Query.Service.Tags = []string{"!tag3"}
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// The node in the warning state should be filtered as well as the node
	// with the filtered tag.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 8 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node1" || node.Node.Node == "node3" {
				t.Fatalf("bad: %v", node)
			}
		}
	}

	// Make a new exec token that can't read the service.
	var denyToken string
	{
		var rules = `
                    service "foo" {
                        policy = "deny"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec1, "ACL.Apply", &req, &denyToken); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure the query gets denied with this token.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 0 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Bake the exec token into the query.
	query.Query.Token = execToken
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now even querying with the deny token should work.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 8 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node1" || node.Node.Node == "node3" {
				t.Fatalf("bad: %v", node)
			}
		}
	}

	// Un-bake the token.
	query.Query.Token = ""
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the query gets denied again with the deny token.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 0 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Turn on version 8 ACLs, which will start to filter even with the exec
	// token.
	s1.config.ACLEnforceVersion8 = true
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 0 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Revert version 8 ACLs and make sure the query works again.
	s1.config.ACLEnforceVersion8 = false
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 8 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node1" || node.Node.Node == "node3" {
				t.Fatalf("bad: %v", node)
			}
		}
	}

	// Now fail everything in dc1 and we should get an empty list back.
	for i := 0; i < 10; i++ {
		setHealth(fmt.Sprintf("node%d", i+1), api.HealthCritical)
	}
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 0 ||
			reply.Datacenter != "dc1" || reply.Failovers != 0 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Modify the query to have it fail over to a bogus DC and then dc2.
	query.Query.Service.Failover.Datacenters = []string{"bogus", "dc2"}
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now we should see 9 nodes from dc2 (we have the tag filter still).
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 9 ||
			reply.Datacenter != "dc2" || reply.Failovers != 1 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node3" {
				t.Fatalf("bad: %v", node)
			}
		}
	}

	// Make sure the limit and query options are forwarded.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			Limit:         3,
			QueryOptions: structs.QueryOptions{
				Token:             execToken,
				RequireConsistent: true,
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc2" || reply.Failovers != 1 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node3" {
				t.Fatalf("bad: %v", node)
			}
		}
	}

	// Make sure the remote shuffle looks like it's working.
	uniques = make(map[string]struct{})
	for i := 0; i < 100; i++ {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 9 ||
			reply.Datacenter != "dc2" || reply.Failovers != 1 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		var names []string
		for _, node := range reply.Nodes {
			names = append(names, node.Node.Node)
		}
		key := strings.Join(names, "|")
		uniques[key] = struct{}{}
	}

	// We have to allow for the fact that there won't always be a unique
	// shuffle each pass, so we just look for smell here without the test
	// being flaky.
	if len(uniques) < 50 {
		t.Fatalf("unique shuffle ratio too low: %d/100", len(uniques))
	}

	// Make sure the query response from dc2 gets denied with the deny token.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 0 ||
			reply.Datacenter != "dc2" || reply.Failovers != 1 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Bake the exec token into the query.
	query.Query.Token = execToken
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now even querying with the deny token should work.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 9 ||
			reply.Datacenter != "dc2" || reply.Failovers != 1 ||
			reply.Service != query.Query.Service.Service ||
			!reflect.DeepEqual(reply.DNS, query.Query.DNS) ||
			!reply.QueryMeta.KnownLeader {
			t.Fatalf("bad: %v", reply)
		}
		for _, node := range reply.Nodes {
			if node.Node.Node == "node3" {
				t.Fatalf("bad: %v", node)
			}
		}
	}
}

func TestPreparedQuery_Execute_ForwardLeader(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec1 := rpcClient(t, s1)
	defer codec1.Close()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	// Try to join.
	joinLAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Use the follower as the client.
	var codec rpc.ClientCodec
	if !s1.IsLeader() {
		codec = codec1
	} else {
		codec = codec2
	}

	// Set up a node and service in the catalog.
	{
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "redis",
				Tags:    []string{"master"},
				Port:    8000,
			},
		}
		var reply struct{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "test",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
	}
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Execute it through the follower.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: reply,
		}
		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 1 {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Execute it through the follower with consistency turned on.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: reply,
			QueryOptions:  structs.QueryOptions{RequireConsistent: true},
		}
		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Execute", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 1 {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Remote execute it through the follower.
	{
		req := structs.PreparedQueryExecuteRemoteRequest{
			Datacenter: "dc1",
			Query:      *query.Query,
		}
		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.ExecuteRemote", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 1 {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Remote execute it through the follower with consistency turned on.
	{
		req := structs.PreparedQueryExecuteRemoteRequest{
			Datacenter:   "dc1",
			Query:        *query.Query,
			QueryOptions: structs.QueryOptions{RequireConsistent: true},
		}
		var reply structs.PreparedQueryExecuteResponse
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.ExecuteRemote", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(reply.Nodes) != 1 {
			t.Fatalf("bad: %v", reply)
		}
	}
}

func TestPreparedQuery_Execute_Blocking(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerDC(t, "dc1")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec1 := rpcClient(t, s1)
	queryCodec := rpcClient(t, s1)
	defer codec1.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Try to WAN join.
	joinWAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members want %d", got, want)
		}
	})

	// Set up a service query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "test",
			Service: structs.ServiceQuery{
				Service: "foo",
				Failover: structs.QueryDatacenterOptions{
					NearestN: 1,
				},
			},
			DNS: structs.QueryDNSOptions{
				TTL: "10s",
			},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	if err := msgpackrpc.CallWithCodec(codec1, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	registerNode := func(dc string, serviceName string, serviceID string) {
		var codec rpc.ClientCodec
		switch dc {
		case "dc1":
			codec = codec1
		case "dc2":
			codec = codec2
		}

		req := structs.RegisterRequest{
			Datacenter: dc,
			Node:       fmt.Sprintf("node-%s", serviceID),
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: serviceName,
				ID:      serviceID,
				Port:    8000,
				Tags:    []string{},
			},
		}

		var reply struct{}
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply)
		require.Nil(t, err, "error registering node dc:%s, service_name:%s, service_id:%s", dc, serviceName, serviceID)
	}

	deregisterNode := func(dc string, serviceID string) {
		var codec rpc.ClientCodec
		switch dc {
		case "dc1":
			codec = codec1
		case "dc2":
			codec = codec2
		}

		arg := structs.DeregisterRequest{
			Datacenter: dc,
			ServiceID:  serviceID,
			Node:       fmt.Sprintf("node-%s", serviceID),
		}
		var out struct{}

		err := msgpackrpc.CallWithCodec(codec, "Catalog.Deregister", &arg, &out)
		require.Nil(t, err)
	}

	exec := func(cb func(structs.PreparedQueryExecuteResponse)) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
		}

		var reply structs.PreparedQueryExecuteResponse
		err := msgpackrpc.CallWithCodec(queryCodec, "PreparedQuery.Execute", &req, &reply)
		require.Nil(t, err)

		cb(reply)
	}

	execShouldUnblock := func(indexes map[string]uint64, do func(), cb func(structs.PreparedQueryExecuteResponse)) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions: structs.QueryOptions{
				MaxQueryTime:         1 * time.Second,
				MinQueryCrossDCIndex: indexes,
			},
		}

		c := make(chan structs.PreparedQueryExecuteResponse)

		go func() {
			var reply structs.PreparedQueryExecuteResponse
			err := msgpackrpc.CallWithCodec(queryCodec, "PreparedQuery.Execute", &req, &reply)
			require.Nil(t, err)
			c <- reply
		}()

		time.Sleep(100 * time.Millisecond)
		select {
		case <-c:
			t.Fatalf("unexpected blocking query release")
		default:
		}

		do()

		timer := time.NewTimer(2 * time.Second)
		select {
		case reply := <-c:
			cb(reply)
		case <-timer.C:
			t.Fatalf("query timeout: %s", string(debug.Stack()))
		}
	}

	execShouldNotUnblock := func(indexes map[string]uint64, do func()) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions: structs.QueryOptions{
				MaxQueryTime:         time.Second,
				MinQueryCrossDCIndex: indexes,
			},
		}

		c := make(chan structs.PreparedQueryExecuteResponse)

		do()

		start := time.Now()
		go func() {
			var reply structs.PreparedQueryExecuteResponse
			err := msgpackrpc.CallWithCodec(queryCodec, "PreparedQuery.Execute", &req, &reply)
			require.Nil(t, err)
			c <- reply
		}()

		timer := time.NewTimer(2 * time.Second)
		select {
		case <-c:
			require.True(t, time.Since(start) > time.Second)
		case <-timer.C:
			t.Fatalf("query timeout: %s", string(debug.Stack()))
		}
	}

	var indexes map[string]uint64

	// run the query a first time to get the DCs indexes
	exec(func(reply structs.PreparedQueryExecuteResponse) {
		require.Equal(t, 0, len(reply.Nodes))
		indexes = reply.CrossDCIndex
	})

	// test MaxQueryTime
	req := structs.PreparedQueryExecuteRequest{
		Datacenter:    "dc1",
		QueryIDOrName: query.Query.ID,
		QueryOptions: structs.QueryOptions{
			MaxQueryTime:         1 * time.Second,
			MinQueryCrossDCIndex: indexes,
		},
	}
	//start := time.Now()
	var reply structs.PreparedQueryExecuteResponse
	err := msgpackrpc.CallWithCodec(queryCodec, "PreparedQuery.Execute", &req, &reply)
	require.Nil(t, err)
	//require.InDelta(t, time.Second, time.Since(start), float64(100*time.Millisecond))

	// we should block is registered on either DC
	// from local DC
	execShouldUnblock(indexes,
		func() {
			registerNode("dc1", "foo", "foo-02")
		},
		func(reply structs.PreparedQueryExecuteResponse) {
			require.Equal(t, 1, len(reply.Nodes))
			require.Equal(t, "dc1", reply.Datacenter)
			require.Equal(t, 0, reply.Failovers)
			require.True(t, reply.CrossDCIndex["dc1"] > indexes["dc1"])
			require.True(t, reply.CrossDCIndex["dc2"] == 0)
			indexes = reply.CrossDCIndex
		},
	)
	// reset
	deregisterNode("dc1", "foo-02")
	exec(func(reply structs.PreparedQueryExecuteResponse) {
		require.Equal(t, 0, len(reply.Nodes))
		indexes = reply.CrossDCIndex
	})
	// from remote DC
	execShouldUnblock(indexes,
		func() {
			registerNode("dc2", "foo", "foo-01")
		},
		func(reply structs.PreparedQueryExecuteResponse) {
			require.Equal(t, 1, len(reply.Nodes))
			require.Equal(t, "dc2", reply.Datacenter)
			require.Equal(t, 1, reply.Failovers)
			require.True(t, reply.CrossDCIndex["dc1"] == indexes["dc1"])
			require.True(t, reply.CrossDCIndex["dc2"] > indexes["dc2"])
			indexes = reply.CrossDCIndex
		},
	)

	// register a node on dc1, this should unblock the query and return dc1's node since it has higher priority than dc2
	execShouldUnblock(indexes,
		func() {
			registerNode("dc1", "foo", "foo-02")
		},
		func(reply structs.PreparedQueryExecuteResponse) {
			require.Equal(t, 1, len(reply.Nodes))
			require.Equal(t, "dc1", reply.Datacenter)
			require.Equal(t, 0, reply.Failovers)
			require.True(t, reply.CrossDCIndex["dc1"] > indexes["dc1"])
			require.True(t, reply.CrossDCIndex["dc2"] == 0)
			indexes = reply.CrossDCIndex
		},
	)

	// modifications happening in other DCs should not impact the query if we have results from a closer DC
	execShouldNotUnblock(indexes, func() {
		deregisterNode("dc2", "foo-01")
	})
	execShouldNotUnblock(indexes, func() {
		registerNode("dc2", "foo", "foo-01")
	})

	// deregister dc1's node, we should fallback to dc2
	execShouldUnblock(indexes,
		func() {
			deregisterNode("dc1", "foo-02")
		},
		func(reply structs.PreparedQueryExecuteResponse) {
			require.Equal(t, 1, len(reply.Nodes))
			require.Equal(t, "dc2", reply.Datacenter)
			require.Equal(t, 1, reply.Failovers)
			require.True(t, reply.CrossDCIndex["dc1"] > indexes["dc1"])
			require.True(t, reply.CrossDCIndex["dc2"] > 0)
			indexes = reply.CrossDCIndex
		},
	)

	// deregister dc2's node, we should be notified that no nodes are available anymore
	execShouldUnblock(indexes,
		func() {
			deregisterNode("dc2", "foo-01")
		},
		func(reply structs.PreparedQueryExecuteResponse) {
			require.Equal(t, 0, len(reply.Nodes))
			require.Equal(t, "dc2", reply.Datacenter)
			require.Equal(t, 1, reply.Failovers)
			require.True(t, reply.CrossDCIndex["dc1"] == indexes["dc1"])
			require.True(t, reply.CrossDCIndex["dc2"] > indexes["dc2"])
			indexes = reply.CrossDCIndex
		},
	)

	// registering another service should not unblock the query
	execShouldNotUnblock(indexes, func() {
		registerNode("dc1", "bar", "bar-01")
	})
}

func TestPreparedQuery_Execute_ConnectExact(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Setup 3 services on 3 nodes: one is non-Connect, one is Connect native,
	// and one is a proxy to the non-Connect one.
	for i := 0; i < 3; i++ {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("node%d", i+1),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: "foo",
				Port:    8000,
			},
		}

		switch i {
		case 0:
			// Default do nothing

		case 1:
			// Connect native
			req.Service.Connect.Native = true

		case 2:
			// Connect proxy
			req.Service.Kind = structs.ServiceKindConnectProxy
			req.Service.Proxy.DestinationServiceName = req.Service.Service
			req.Service.Service = "proxy"
		}

		var reply struct{}
		require.NoError(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply))
	}

	// The query, start with connect disabled
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "test",
			Service: structs.ServiceQuery{
				Service: "foo",
			},
			DNS: structs.QueryDNSOptions{
				TTL: "10s",
			},
		},
	}
	require.NoError(msgpackrpc.CallWithCodec(
		codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// In the future we'll run updates
	query.Op = structs.PreparedQueryUpdate

	// Run the registered query.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(msgpackrpc.CallWithCodec(
			codec, "PreparedQuery.Execute", &req, &reply))

		// Result should have two because it omits the proxy whose name
		// doesn't match the query.
		require.Len(reply.Nodes, 2)
		require.Equal(query.Query.Service.Service, reply.Service)
		require.Equal(query.Query.DNS, reply.DNS)
		require.True(reply.QueryMeta.KnownLeader, "queried leader")
	}

	// Run with the Connect setting specified on the request
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			Connect:       true,
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(msgpackrpc.CallWithCodec(
			codec, "PreparedQuery.Execute", &req, &reply))

		// Result should have two because we should get the native AND
		// the proxy (since the destination matches our service name).
		require.Len(reply.Nodes, 2)
		require.Equal(query.Query.Service.Service, reply.Service)
		require.Equal(query.Query.DNS, reply.DNS)
		require.True(reply.QueryMeta.KnownLeader, "queried leader")

		// Make sure the native is the first one
		if !reply.Nodes[0].Service.Connect.Native {
			reply.Nodes[0], reply.Nodes[1] = reply.Nodes[1], reply.Nodes[0]
		}

		require.True(reply.Nodes[0].Service.Connect.Native, "native")
		require.Equal(reply.Service, reply.Nodes[0].Service.Service)

		require.Equal(structs.ServiceKindConnectProxy, reply.Nodes[1].Service.Kind)
		require.Equal(reply.Service, reply.Nodes[1].Service.Proxy.DestinationServiceName)
	}

	// Update the query
	query.Query.Service.Connect = true
	require.NoError(msgpackrpc.CallWithCodec(
		codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Run the registered query.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(msgpackrpc.CallWithCodec(
			codec, "PreparedQuery.Execute", &req, &reply))

		// Result should have two because we should get the native AND
		// the proxy (since the destination matches our service name).
		require.Len(reply.Nodes, 2)
		require.Equal(query.Query.Service.Service, reply.Service)
		require.Equal(query.Query.DNS, reply.DNS)
		require.True(reply.QueryMeta.KnownLeader, "queried leader")

		// Make sure the native is the first one
		if !reply.Nodes[0].Service.Connect.Native {
			reply.Nodes[0], reply.Nodes[1] = reply.Nodes[1], reply.Nodes[0]
		}

		require.True(reply.Nodes[0].Service.Connect.Native, "native")
		require.Equal(reply.Service, reply.Nodes[0].Service.Service)

		require.Equal(structs.ServiceKindConnectProxy, reply.Nodes[1].Service.Kind)
		require.Equal(reply.Service, reply.Nodes[1].Service.Proxy.DestinationServiceName)
	}

	// Unset the query
	query.Query.Service.Connect = false
	require.NoError(msgpackrpc.CallWithCodec(
		codec, "PreparedQuery.Apply", &query, &query.Query.ID))
}

func TestPreparedQuery_tagFilter(t *testing.T) {
	t.Parallel()
	testNodes := func() structs.CheckServiceNodes {
		return structs.CheckServiceNodes{
			structs.CheckServiceNode{
				Node:    &structs.Node{Node: "node1"},
				Service: &structs.NodeService{Tags: []string{"foo"}},
			},
			structs.CheckServiceNode{
				Node:    &structs.Node{Node: "node2"},
				Service: &structs.NodeService{Tags: []string{"foo", "BAR"}},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{Node: "node3"},
			},
			structs.CheckServiceNode{
				Node:    &structs.Node{Node: "node4"},
				Service: &structs.NodeService{Tags: []string{"foo", "baz"}},
			},
			structs.CheckServiceNode{
				Node:    &structs.Node{Node: "node5"},
				Service: &structs.NodeService{Tags: []string{"foo", "zoo"}},
			},
			structs.CheckServiceNode{
				Node:    &structs.Node{Node: "node6"},
				Service: &structs.NodeService{Tags: []string{"bar"}},
			},
		}
	}

	// This always sorts so that it's not annoying to compare after the swap
	// operations that the algorithm performs.
	stringify := func(nodes structs.CheckServiceNodes) string {
		var names []string
		for _, node := range nodes {
			names = append(names, node.Node.Node)
		}
		sort.Strings(names)
		return strings.Join(names, "|")
	}

	ret := stringify(tagFilter([]string{}, testNodes()))
	if ret != "node1|node2|node3|node4|node5|node6" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"foo"}, testNodes()))
	if ret != "node1|node2|node4|node5" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"!foo"}, testNodes()))
	if ret != "node3|node6" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"!foo", "bar"}, testNodes()))
	if ret != "node6" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"!foo", "!bar"}, testNodes()))
	if ret != "node3" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"nope"}, testNodes()))
	if ret != "" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"bar"}, testNodes()))
	if ret != "node2|node6" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"BAR"}, testNodes()))
	if ret != "node2|node6" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{"bAr"}, testNodes()))
	if ret != "node2|node6" {
		t.Fatalf("bad: %s", ret)
	}

	ret = stringify(tagFilter([]string{""}, testNodes()))
	if ret != "" {
		t.Fatalf("bad: %s", ret)
	}
}

func TestPreparedQuery_Wrapper(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Try to WAN join.
	joinWAN(t, s2, s1)

	// Try all the operations on a real server via the wrapper.
	wrapper := &queryServerWrapper{&PreparedQuery{s1}}
	wrapper.GetLogger().Printf("[DEBUG] Test")

	ret, err := wrapper.GetDatacentersByDistance()
	wrapper.GetLogger().Println("Returned value: ", ret)
	require.Nil(t, err)
	require.Equal(t, []string{"dc1", "dc2"}, ret)

	// Since we have no idea when the joinWAN operation completes
	// we keep on querying until the the join operation completes.
	retry.Run(t, func(r *retry.R) {
		r.Check(s1.forwardDC("Status.Ping", "dc2", &struct{}{}, &struct{}{}))
	})
}

type mockQueryServer struct {
	Datacenters      []string
	DatacentersError error
	QueriedDCs       []string
	QueryFn          func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error)
	Logger           *log.Logger
	LogBuffer        *bytes.Buffer
}

func (m *mockQueryServer) GetLogger() *log.Logger {
	if m.Logger == nil {
		m.LogBuffer = new(bytes.Buffer)
		m.Logger = log.New(m.LogBuffer, "", 0)
	}
	return m.Logger
}

func (m *mockQueryServer) GetDatacentersByDistance() ([]string, error) {
	return m.Datacenters, m.DatacentersError
}

func (m *mockQueryServer) LocalDC() string {
	return m.Datacenters[0]
}

func (m *mockQueryServer) Exec(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
	m.QueriedDCs = append(m.QueriedDCs, args.Datacenter)

	if m.QueryFn != nil {
		return m.QueryFn(query, args)
	}

	return &structs.PreparedQueryExecuteResponse{}, nil
}

func TestPreparedQuery_fetchQueryResults(t *testing.T) {
	t.Parallel()
	query := &structs.PreparedQuery{
		Name: "test",
		Service: structs.ServiceQuery{
			Failover: structs.QueryDatacenterOptions{
				NearestN:    0,
				Datacenters: []string{""},
			},
		},
	}

	nodes := func() structs.CheckServiceNodes {
		return structs.CheckServiceNodes{
			structs.CheckServiceNode{
				Node: &structs.Node{Node: "node1"},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{Node: "node2"},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{Node: "node3"},
			},
		}
	}

	// Datacenters are available but the query doesn't use them.
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 || reply.Datacenter != "local" || reply.Failovers != 0 {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Make it fail to get datacenters.
	{
		mock := &mockQueryServer{
			Datacenters:      []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			DatacentersError: fmt.Errorf("XXX"),
		}

		var reply structs.PreparedQueryExecuteResponse
		err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply)
		if err == nil || !strings.Contains(err.Error(), "XXX") {
			t.Fatalf("bad: %v", err)
		}
		if len(reply.Nodes) != 0 || reply.Datacenter != "" || reply.Failovers != 0 {
			t.Fatalf("bad: %v", reply)
		}
	}

	// The query wants to use other datacenters but none are available.
	query.Service.Failover.NearestN = 3
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local"},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 || reply.Datacenter != "local" || reply.Failovers != 0 {
			t.Fatalf("bad: %v", reply)
		}
	}

	// Try the first three nearest datacenters, first one has the data.
	query.Service.Failover.NearestN = 3
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "dc1" {
					ret.Datacenter = "dc1"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc1" || reply.Failovers != 1 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %+v", reply)
		}
		require.Equal(t, []string{"local", "dc1"}, mock.QueriedDCs)
	}

	// Try the first three nearest datacenters, last one has the data.
	query.Service.Failover.NearestN = 3
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "dc3" {
					ret.Datacenter = "dc3"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc3" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		require.Equal(t, []string{"local", "dc1", "dc2", "dc3"}, mock.QueriedDCs)
	}

	// Try the first four nearest datacenters, nobody has the data.
	query.Service.Failover.NearestN = 4
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 ||
			reply.Datacenter != "xxx" || reply.Failovers != 4 {
			t.Fatalf("bad: %v", reply)
		}
		require.Equal(t, []string{"local", "dc1", "dc2", "dc3", "xxx"}, mock.QueriedDCs)
	}

	// Try the first two nearest datacenters, plus a user-specified one that
	// has the data.
	query.Service.Failover.NearestN = 2
	query.Service.Failover.Datacenters = []string{"dc4"}
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "dc4" {
					ret.Datacenter = "dc4"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		require.Equal(t, []string{"local", "dc1", "dc2", "dc4"}, mock.QueriedDCs)
	}

	// Add in a hard-coded value that overlaps with the nearest list.
	query.Service.Failover.NearestN = 2
	query.Service.Failover.Datacenters = []string{"dc4", "dc1"}
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "dc4" {
					ret.Datacenter = "dc4"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		require.Equal(t, []string{"local", "dc1", "dc2", "dc4"}, mock.QueriedDCs)
	}

	// Now add a bogus user-defined one to the mix.
	query.Service.Failover.NearestN = 2
	query.Service.Failover.Datacenters = []string{"nope", "dc4", "dc1"}
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "dc4" {
					ret.Datacenter = "dc4"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		require.Equal(t, []string{"local", "dc1", "dc2", "dc4"}, mock.QueriedDCs)
		if !strings.Contains(mock.LogBuffer.String(), "Skipping unknown datacenter") {
			t.Fatalf("bad: %s", mock.LogBuffer.String())
		}
	}

	// Same setup as before but dc1 is going to return an error and should
	// get skipped over, still yielding data from dc4 which comes later.
	query.Service.Failover.NearestN = 2
	query.Service.Failover.Datacenters = []string{"dc4", "dc1"}
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "dc1" {
					return nil, fmt.Errorf("XXX")
				} else if args.Datacenter == "dc4" {
					ret.Datacenter = "dc4"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		require.Equal(t, []string{"local", "dc1", "dc2", "dc4"}, mock.QueriedDCs)
		if !strings.Contains(mock.LogBuffer.String(), "Failed querying") {
			t.Fatalf("bad: %s", mock.LogBuffer.String())
		}
	}

	// Just use a hard-coded list and now xxx has the data.
	query.Service.Failover.NearestN = 0
	query.Service.Failover.Datacenters = []string{"dc3", "xxx"}
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "xxx" {
					ret.Datacenter = "xxx"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "xxx" || reply.Failovers != 2 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %+v", reply)
		}
		require.Equal(t, []string{"local", "dc3", "xxx"}, mock.QueriedDCs)
	}
	// Make sure the limit and query options are plumbed through.
	query.Service.Failover.NearestN = 0
	query.Service.Failover.Datacenters = []string{"xxx"}
	{
		mock := &mockQueryServer{
			Datacenters: []string{"local", "dc1", "dc2", "xxx", "dc3", "dc4"},
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				ret := &structs.PreparedQueryExecuteResponse{}
				if args.Datacenter == "xxx" {
					if args.Limit != 5 {
						t.Fatalf("bad: %d", args.Limit)
					}
					if args.RequireConsistent != true {
						t.Fatalf("bad: %v", args.RequireConsistent)
					}
					ret.Datacenter = "xxx"
					ret.Nodes = nodes()
				}
				return ret, nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{
			Limit:        5,
			QueryOptions: structs.QueryOptions{RequireConsistent: true},
		}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "xxx" || reply.Failovers != 1 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		require.Equal(t, []string{"local", "xxx"}, mock.QueriedDCs)
	}
}

func TestPreparedQuery_fetchQueryResults_Blocking(t *testing.T) {
	t.Parallel()
	query := &structs.PreparedQuery{
		Name: "test",
		Service: structs.ServiceQuery{
			Failover: structs.QueryDatacenterOptions{
				NearestN:    1,
				Datacenters: []string{""},
			},
		},
	}

	type execFunc func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error)

	makeMock := func(dcsFns map[string][]execFunc) *mockQueryServer {
		dcs := []string{}
		for dc := range dcsFns {
			dcs = append(dcs, dc)
		}
		sort.Strings(dcs)

		runs := map[string]int{}

		return &mockQueryServer{
			Datacenters: dcs,
			QueryFn: func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
				dc := args.Datacenter
				_, ok := dcsFns[dc]
				if !ok {
					t.Fatalf("unexpected call to dc %s", dc)
				}

				if runs[dc] >= len(dcsFns[dc]) {
					t.Fatalf("dc %s should not have been called more than %d time(s)", dc, len(dcsFns[dc]))
				}

				defer func() {
					runs[dc]++
				}()
				return dcsFns[dc][runs[dc]](query, args)
			},
		}
	}

	// DC1 has the results, DC2 is not queried
	{
		mock := makeMock(map[string][]execFunc{
			"dc1": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(10), args.MinQueryIndex)
					time.Sleep(100 * time.Millisecond)
					return &structs.PreparedQueryExecuteResponse{
						Nodes: structs.CheckServiceNodes{
							structs.CheckServiceNode{
								Node: &structs.Node{Node: "node1"},
							},
						},
						Datacenter: "dc1",
						QueryMeta: structs.QueryMeta{
							Index: 11,
						},
					}, nil
				},
			},
			"dc2": nil,
		})

		var reply structs.PreparedQueryExecuteResponse
		err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryCrossDCIndex: map[string]uint64{
					"dc1": 10,
				},
				MaxQueryTime: 200 * time.Millisecond,
			},
		}, &reply)
		require.Nil(t, err)
		require.Equal(t, 1, len(reply.Nodes))
		require.Equal(t, uint64(11), reply.CrossDCIndex["dc1"])
		require.Equal(t, uint64(0), reply.CrossDCIndex["dc2"])
	}

	// DC1 Fails, still failing
	{
		mock := makeMock(map[string][]execFunc{
			"dc1": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(10), args.MinQueryIndex)
					time.Sleep(100 * time.Millisecond)
					return &structs.PreparedQueryExecuteResponse{
						Datacenter: "dc1",
						QueryMeta: structs.QueryMeta{
							Index: 11,
						},
					}, nil
				},
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					return &structs.PreparedQueryExecuteResponse{}, nil
				},
			},
			"dc2": nil,
		})

		var reply structs.PreparedQueryExecuteResponse
		err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryCrossDCIndex: map[string]uint64{
					"dc1": 10,
				},
				MaxQueryTime: 100 * time.Millisecond,
			},
		}, &reply)
		require.Nil(t, err)
		require.Equal(t, 0, len(reply.Nodes))
		require.Equal(t, uint64(11), reply.CrossDCIndex["dc1"])
	}

	// DC1 Fails, Fallback to DC2
	{
		mock := makeMock(map[string][]execFunc{
			"dc1": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(10), args.MinQueryIndex)
					time.Sleep(50 * time.Millisecond)
					return &structs.PreparedQueryExecuteResponse{
						Datacenter: "dc1",
						QueryMeta: structs.QueryMeta{
							Index: 11,
						},
					}, nil
				},
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					return &structs.PreparedQueryExecuteResponse{}, nil
				},
			},
			"dc2": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(0), args.MinQueryIndex)
					return &structs.PreparedQueryExecuteResponse{
						Nodes: structs.CheckServiceNodes{
							structs.CheckServiceNode{
								Node: &structs.Node{Node: "node1"},
							},
						},
						Datacenter: "dc2",
						QueryMeta: structs.QueryMeta{
							Index: 13,
						},
					}, nil
				},
			},
		})

		var reply structs.PreparedQueryExecuteResponse
		err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryCrossDCIndex: map[string]uint64{
					"dc1": 10,
				},
				MaxQueryTime: 100 * time.Millisecond,
			},
		}, &reply)
		require.Nil(t, err)
		require.Equal(t, 1, len(reply.Nodes))
		require.Equal(t, "dc2", reply.Datacenter)
		require.Equal(t, uint64(11), reply.CrossDCIndex["dc1"])
		require.Equal(t, uint64(13), reply.CrossDCIndex["dc2"])
	}

	// A DC fails and comes back
	{
		mock := makeMock(map[string][]execFunc{
			"dc1": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(10), args.MinQueryIndex)
					time.Sleep(100 * time.Millisecond)
					return &structs.PreparedQueryExecuteResponse{
						Datacenter: "dc1",
						QueryMeta: structs.QueryMeta{
							Index: 11,
						},
					}, nil
				},
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					time.Sleep(50 * time.Millisecond)
					assert.Equal(t, uint64(11), args.MinQueryIndex)
					return &structs.PreparedQueryExecuteResponse{
						Datacenter: "dc1",
						Nodes: structs.CheckServiceNodes{
							structs.CheckServiceNode{
								Node: &structs.Node{Node: "node1"},
							},
						},
						QueryMeta: structs.QueryMeta{
							Index: 12,
						},
					}, nil
				},
			},
			"dc2": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(15), args.MinQueryIndex)

					time.Sleep(200 * time.Millisecond)
					return &structs.PreparedQueryExecuteResponse{
						Datacenter: "dc2",
						QueryMeta: structs.QueryMeta{
							Index: 15,
						},
					}, nil
				},
			},
		})

		var reply structs.PreparedQueryExecuteResponse
		err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryCrossDCIndex: map[string]uint64{
					"dc1": 10,
					"dc2": 15,
				},
				MaxQueryTime: 200 * time.Millisecond,
			},
		}, &reply)
		require.Nil(t, err)
		require.Equal(t, 1, len(reply.Nodes))
		require.Equal(t, uint64(12), reply.CrossDCIndex["dc1"])
		require.Equal(t, uint64(0), reply.CrossDCIndex["dc2"])
	}

	// DC2 has results, DC1 comes back
	{
		mock := makeMock(map[string][]execFunc{
			"dc1": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(10), args.MinQueryIndex)
					time.Sleep(50 * time.Millisecond)
					return &structs.PreparedQueryExecuteResponse{
						Datacenter: "dc1",
						QueryMeta: structs.QueryMeta{
							Index: 11,
						},
					}, nil
				},
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(11), args.MinQueryIndex)
					return &structs.PreparedQueryExecuteResponse{
						Datacenter: "dc1",
						Nodes: structs.CheckServiceNodes{
							structs.CheckServiceNode{
								Node: &structs.Node{Node: "node1"},
							},
						},
						QueryMeta: structs.QueryMeta{
							Index: 12,
						},
					}, nil
				},
			},
			"dc2": []execFunc{
				func(query *structs.PreparedQuery, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
					assert.Equal(t, uint64(12), args.MinQueryIndex)
					time.Sleep(100 * time.Millisecond)
					return &structs.PreparedQueryExecuteResponse{
						Nodes: structs.CheckServiceNodes{
							structs.CheckServiceNode{
								Node: &structs.Node{Node: "node1"},
							},
						},
						Datacenter: "dc2",
						QueryMeta: structs.QueryMeta{
							Index: 13,
						},
					}, nil
				},
			},
		})

		var reply structs.PreparedQueryExecuteResponse
		err := fetchQueryResults(mock, query, &structs.PreparedQueryExecuteRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryCrossDCIndex: map[string]uint64{
					"dc1": 10,
					"dc2": 12,
				},
				MaxQueryTime: 200 * time.Millisecond,
			},
		}, &reply)
		require.Nil(t, err)
		require.Equal(t, 1, len(reply.Nodes))
		require.Equal(t, "dc1", reply.Datacenter)
		require.Equal(t, uint64(12), reply.CrossDCIndex["dc1"])
		require.Equal(t, uint64(0), reply.CrossDCIndex["dc2"])
	}
}
