package consul

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

func TestPreparedQuery_Apply(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testutil.WaitForLeader(t, s1.RPC, "dc1")

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
	// part of another test.
	query.Op = structs.PreparedQueryCreate
	query.Query.ID = ""
	query.Query.Service.Failover.NearestN = -1
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "Bad NearestN") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and make sure it propagates an error from the Raft apply.
	query.Query.Service.Failover.NearestN = 0
	query.Query.Service.Service = "nope"
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "invalid service") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and make sure the apply goes through.
	query.Query.Service.Service = "redis"
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and read the query back to verify.
	query.Query.ID = reply
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestPreparedQuery_Apply_ACLDeny(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testutil.WaitForLeader(t, s1.RPC, "dc1")

	// Create two ACLs with read permission to the service.
	var token1, token2 string
	{
		var rules = `
                    service "redis" {
                        policy = "read"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var reply string

		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		token1 = reply

		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		token2 = reply
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
			WriteRequest: structs.WriteRequest{Token: "root"},
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
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
	}
	var reply string

	// Creating without a token should fail since the default policy is to
	// deny.
	err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), permissionDenied) {
		t.Fatalf("bad: %v", err)
	}

	// Now add the token and try again.
	query.WriteRequest.Token = token1
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and set the token, then read back the query to verify.
	query.Query.ID = reply
	query.Query.Token = token1
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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

	// Try to do an update with a different token that does have access to
	// the service, but isn't the one that was used to create the query.
	query.Op = structs.PreparedQueryUpdate
	query.WriteRequest.Token = token2
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), permissionDenied) {
		t.Fatalf("bad: %v", err)
	}

	// Try again with no token.
	query.WriteRequest.Token = ""
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), permissionDenied) {
		t.Fatalf("bad: %v", err)
	}

	// Try again with the original token. This should go through.
	query.WriteRequest.Token = token1
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try to do a delete with a different token that does have access to
	// the service, but isn't the one that was used to create the query.
	query.Op = structs.PreparedQueryDelete
	query.WriteRequest.Token = token2
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), permissionDenied) {
		t.Fatalf("bad: %v", err)
	}

	// Try again with no token.
	query.WriteRequest.Token = ""
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), permissionDenied) {
		t.Fatalf("bad: %v", err)
	}

	// Try again with the original token. This should go through.
	query.WriteRequest.Token = token1
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the query got deleted.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Make the query again.
	query.Op = structs.PreparedQueryCreate
	query.Query.ID = ""
	query.Query.Token = ""
	query.WriteRequest.Token = token1
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that it's there.
	query.Query.ID = reply
	query.Query.Token = token1
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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

	// That last update should have changed the token to the management one.
	query.Query.Token = "root"
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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

	// Make another query.
	query.Op = structs.PreparedQueryCreate
	query.Query.ID = ""
	query.Query.Token = ""
	query.WriteRequest.Token = token1
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that it's there.
	query.Query.ID = reply
	query.Query.Token = token1
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestPreparedQuery_parseQuery(t *testing.T) {
	query := &structs.PreparedQuery{}

	err := parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Must provide a service") {
		t.Fatalf("bad: %v", err)
	}

	query.Service.Service = "foo"
	if err := parseQuery(query); err != nil {
		t.Fatalf("err: %v", err)
	}

	query.Service.Failover.NearestN = -1
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Bad NearestN") {
		t.Fatalf("bad: %v", err)
	}

	query.Service.Failover.NearestN = 3
	if err := parseQuery(query); err != nil {
		t.Fatalf("err: %v", err)
	}

	query.DNS.TTL = "two fortnights"
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Bad DNS TTL") {
		t.Fatalf("bad: %v", err)
	}

	query.DNS.TTL = "-3s"
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "must be >=0") {
		t.Fatalf("bad: %v", err)
	}

	query.DNS.TTL = "3s"
	if err := parseQuery(query); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestPreparedQuery_Lookup(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testutil.WaitForLeader(t, s1.RPC, "dc1")

	// Create two ACLs with read permission to the service.
	var token1, token2 string
	{
		var rules = `
                    service "redis" {
                        policy = "read"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var reply string

		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		token1 = reply

		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		token2 = reply
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
			WriteRequest: structs.WriteRequest{Token: "root"},
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
			Name: "my-query",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
		WriteRequest: structs.WriteRequest{Token: token1},
	}
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and set the token, then read back the query to verify.
	query.Query.ID = reply
	query.Query.Token = token1
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: token1},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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

	// Now try to read it with a token that has read access to the
	// service but isn't the token used to create the query. This should
	// be denied.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: token2},
		}
		var resp structs.IndexedPreparedQueries
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp)
		if err == nil || !strings.Contains(err.Error(), permissionDenied) {
			t.Fatalf("bad: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Try again with no token, which should also be denied.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: ""},
		}
		var resp structs.IndexedPreparedQueries
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp)
		if err == nil || !strings.Contains(err.Error(), permissionDenied) {
			t.Fatalf("bad: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// A management token should be able to read no matter what.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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

	// Try a lookup by name instead of ID.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.Name,
			QueryOptions:  structs.QueryOptions{Token: token1},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
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

	// Try to lookup an unknown ID.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: generateUUID(),
			QueryOptions:  structs.QueryOptions{Token: token1},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Try to lookup an unknown name.
	{
		req := &structs.PreparedQuerySpecificRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "nope",
			QueryOptions:  structs.QueryOptions{Token: token1},
		}
		var resp structs.IndexedPreparedQueries
		if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Lookup", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestPreparedQuery_List(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testutil.WaitForLeader(t, s1.RPC, "dc1")

	// Create an ACL with read permission to the service.
	var token string
	{
		var rules = `
                    service "redis" {
                        policy = "read"
                    }
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var reply string

		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		token = reply
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
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var reply struct{}
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Query with a legit management token but no queries.
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
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
			Name: "my-query",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
		WriteRequest: structs.WriteRequest{Token: token},
	}
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the ID and set the token, then try to list all the queries.
	// A management token is required so this should be denied.
	query.Query.ID = reply
	query.Query.Token = token
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var resp structs.IndexedPreparedQueries
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp)
		if err == nil || !strings.Contains(err.Error(), permissionDenied) {
			t.Fatalf("bad: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// An empty token should fail in a similar way.
	{
		req := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: ""},
		}
		var resp structs.IndexedPreparedQueries
		err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.List", req, &resp)
		if err == nil || !strings.Contains(err.Error(), permissionDenied) {
			t.Fatalf("bad: %v", err)
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}

	// Now try a legit management token.
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
