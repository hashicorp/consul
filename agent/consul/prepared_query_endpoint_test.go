// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	grpcexternal "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

const localTestDC = "dc1"

func TestPreparedQuery_Apply(t *testing.T) {
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

	// Fix that and ensure Targets and NearestN cannot be set at the same time.
	query.Query.Service.Failover.NearestN = 1
	query.Query.Service.Failover.Targets = []structs.QueryFailoverTarget{{Peer: "peer"}}
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "Targets cannot be populated with") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and ensure Targets and Datacenters cannot be set at the same time.
	query.Query.Service.Failover.NearestN = 0
	query.Query.Service.Failover.Datacenters = []string{"dc2"}
	query.Query.Service.Failover.Targets = []structs.QueryFailoverTarget{{Peer: "peer"}}
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "Targets cannot be populated with") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and make sure it propagates an error from the Raft apply.
	query.Query.Service.Failover.Targets = nil
	query.Query.Session = "nope"
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &query, &reply)
	if err == nil || !strings.Contains(err.Error(), "invalid session") {
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
			if !structs.IsErrQueryNotFound(err) {
				t.Fatalf("err: %v", err)
			}
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestPreparedQuery_Apply_ACLDeny(t *testing.T) {
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
		query_prefix "redis" {
			policy = "write"
		}
	`
	token := createToken(t, codec, rules)

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "redis-primary",
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
			if !structs.IsErrQueryNotFound(err) {
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
			if !structs.IsErrQueryNotFound(err) {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
				Tags:    []string{"primary"},
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

	err := parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Must be bound to a session") {
		t.Fatalf("bad: %v", err)
	}

	query.Session = "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Must provide a Service") {
		t.Fatalf("bad: %v", err)
	}

	query.Session = ""
	query.Template.Type = "some-kind-of-template"
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Must provide a Service") {
		t.Fatalf("bad: %v", err)
	}

	query.Template.Type = ""
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Must be bound to a session") {
		t.Fatalf("bad: %v", err)
	}

	// None of the rest of these care about version 8 ACL enforcement.
	query = &structs.PreparedQuery{}
	query.Session = "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
	query.Service.Service = "foo"
	if err := parseQuery(query); err != nil {
		t.Fatalf("err: %v", err)
	}

	query.Token = aclfilter.RedactedToken
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "Bad Token") {
		t.Fatalf("bad: %v", err)
	}

	query.Token = "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
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

	query.Service.NodeMeta = map[string]string{"": "somevalue"}
	err = parseQuery(query)
	if err == nil || !strings.Contains(err.Error(), "cannot be blank") {
		t.Fatalf("bad: %v", err)
	}

	query.Service.NodeMeta = map[string]string{"somekey": "somevalue"}
	if err := parseQuery(query); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestPreparedQuery_ACLDeny_Catchall_Template(t *testing.T) {
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
		query "" {
			policy = "write"
		}
	`
	token := createToken(t, codec, rules)

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
	query.Query.Token = aclfilter.RedactedToken
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
	query.Query.Token = aclfilter.RedactedToken
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
		query_prefix "redis" {
			policy = "write"
		}
	`
	token := createToken(t, codec, rules)

	// Set up a bare bones query.
	query := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: &structs.PreparedQuery{
			Name: "redis-primary",
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
			WriteRequest: structs.WriteRequest{Token: "root"},
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
	query.Query.Token = aclfilter.RedactedToken
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
			if !structs.IsErrQueryNotFound(err) {
				t.Fatalf("err: %v", err)
			}
		}

		if len(resp.Queries) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestPreparedQuery_List(t *testing.T) {
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
		query_prefix "redis" {
			policy = "write"
		}
	`
	token := createToken(t, codec, rules)

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
			Name:  "redis-primary",
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
	query.Query.Token = aclfilter.RedactedToken
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

	// Same for a token without access to the query.
	{
		token := createTokenWithPolicyName(t, codec, "deny-queries", `
			query_prefix "" {
				policy = "deny"
			}
		`, "root")

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
		if !resp.QueryMeta.ResultsFilteredByACLs {
			t.Fatal("ResultsFilteredByACLs should be true")
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
			WriteRequest: structs.WriteRequest{Token: "root"},
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
		query_prefix "prod-" {
			policy = "write"
		}
	`
	token := createToken(t, codec, rules)

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
	query.Query.Token = aclfilter.RedactedToken
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
			if !structs.IsErrQueryNotFound(err) {
				t.Fatalf("err: %v", err)
			}
		}
	}
}

// This is a beast of a test, but the setup is so extensive it makes sense to
// walk through the different cases once we have it up. This is broken into
// sections so it's still pretty easy to read.
func TestPreparedQuery_Execute(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	es := createExecuteServers(t)

	newSessionDC1 := func(t *testing.T) string {
		t.Helper()
		req := structs.SessionRequest{
			Datacenter: "dc1",
			Op:         structs.SessionCreate,
			Session: structs.Session{
				Node: es.server.server.config.NodeName,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var session string
		if err := msgpackrpc.CallWithCodec(es.server.codec, "Session.Apply", &req, &session); err != nil {
			t.Fatalf("err: %v", err)
		}
		return session
	}

	// Set up some nodes in each DC that host the service.
	{
		for i := 0; i < 10; i++ {
			for _, d := range []struct {
				codec rpc.ClientCodec
				dc    string
			}{
				{es.server.codec, "dc1"},
				{es.wanServer.codec, "dc2"},
				{es.peeringServer.codec, "dc3"},
			} {
				req := structs.RegisterRequest{
					Datacenter: d.dc,
					Node:       fmt.Sprintf("node%d", i+1),
					Address:    fmt.Sprintf("127.0.0.%d", i+1),
					NodeMeta: map[string]string{
						"group":         fmt.Sprintf("%d", i/5),
						"instance_type": "t2.micro",
					},
					Service: &structs.NodeService{
						Service: "foo",
						Port:    8000,
						Tags:    []string{d.dc, fmt.Sprintf("tag%d", i+1)},
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

				var reply struct{}
				if err := msgpackrpc.CallWithCodec(d.codec, "Catalog.Register", &req, &reply); err != nil {
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
	if err := msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Run a query that doesn't exist.
	t.Run("run query that doesn't exist", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: "nope",
		}

		var reply structs.PreparedQueryExecuteResponse
		err := msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply)
		assert.EqualError(t, err, structs.ErrQueryNotFound.Error())
		assert.Len(t, reply.Nodes, 0)
	})

	expectNodes := func(t require.TestingT, query *structs.PreparedQueryRequest, reply *structs.PreparedQueryExecuteResponse, n int) {
		assert.Len(t, reply.Nodes, n)
		assert.Equal(t, "dc1", reply.Datacenter)
		assert.Equal(t, 0, reply.Failovers)
		assert.Equal(t, query.Query.Service.Service, reply.Service)
		assert.Equal(t, query.Query.DNS, reply.DNS)
		assert.True(t, reply.QueryMeta.KnownLeader)
	}
	expectFailoverNodes := func(t require.TestingT, query *structs.PreparedQueryRequest, reply *structs.PreparedQueryExecuteResponse, n int) {
		assert.Len(t, reply.Nodes, n)
		assert.Equal(t, "dc2", reply.Datacenter)
		assert.Equal(t, 1, reply.Failovers)
		assert.Equal(t, query.Query.Service.Service, reply.Service)
		assert.Equal(t, query.Query.DNS, reply.DNS)
		assert.True(t, reply.QueryMeta.KnownLeader)
	}

	expectFailoverPeerNodes := func(t require.TestingT, query *structs.PreparedQueryRequest, reply *structs.PreparedQueryExecuteResponse, n int) {
		assert.Len(t, reply.Nodes, n)
		assert.Equal(t, "", reply.Datacenter)
		assert.Equal(t, es.peeringServer.acceptingPeerName, reply.PeerName)
		assert.Equal(t, 2, reply.Failovers)
		assert.Equal(t, query.Query.Service.Service, reply.Service)
		assert.Equal(t, query.Query.DNS, reply.DNS)
		assert.True(t, reply.QueryMeta.KnownLeader)
	}

	t.Run("run the registered query", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
		expectNodes(t, &query, &reply, 10)
	})

	t.Run("try with a limit", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			Limit:         3,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
		expectNodes(t, &query, &reply, 3)
	})

	// Run various service queries with node metadata filters.
	for name, tc := range map[string]struct {
		filters  map[string]string
		numNodes int
	}{
		"no filter 10 nodes": {
			filters:  map[string]string{},
			numNodes: 10,
		},
		"instance filter 10 nodes": {
			filters:  map[string]string{"instance_type": "t2.micro"},
			numNodes: 10,
		},
		"group filter 5 nodes": {
			filters:  map[string]string{"group": "1"},
			numNodes: 5,
		},
		"group filter unique 1 node": {
			filters:  map[string]string{"group": "0", "unique": "true"},
			numNodes: 1,
		},
	} {
		tc := tc
		t.Run("node metadata - "+name, func(t *testing.T) {
			session := newSessionDC1(t)
			nodeMetaQuery := structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Session: session,
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
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &nodeMetaQuery, &nodeMetaQuery.Query.ID))

			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: nodeMetaQuery.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: es.execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, tc.numNodes)

			for _, node := range reply.Nodes {
				assert.True(t, structs.SatisfiesMetaFilters(node.Node.Meta, tc.filters), "meta: %v", node.Node.Meta)
			}
		})
	}

	// Run various service queries with service metadata filters
	for name, tc := range map[string]struct {
		filters  map[string]string
		numNodes int
	}{
		"no filter 10 nodes": {
			filters:  map[string]string{},
			numNodes: 10,
		},
		"foo filter 10 nodes": {
			filters:  map[string]string{"foo": "true"},
			numNodes: 10,
		},
		"group filter 0 - 5 nodes": {
			filters:  map[string]string{"svc-group": "0"},
			numNodes: 5,
		},
		"group filter 1 - 5 nodes": {
			filters:  map[string]string{"svc-group": "1"},
			numNodes: 5,
		},
		"group filter 0 - unique 1 node": {
			filters:  map[string]string{"svc-group": "0", "unique": "true"},
			numNodes: 1,
		},
	} {
		tc := tc
		require.True(t, t.Run("service metadata - "+name, func(t *testing.T) {
			session := newSessionDC1(t)
			svcMetaQuery := structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Session: session,
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

			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &svcMetaQuery, &svcMetaQuery.Query.ID))

			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: svcMetaQuery.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: es.execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, tc.numNodes)
			for _, node := range reply.Nodes {
				assert.True(t, structs.SatisfiesMetaFilters(node.Service.Meta, tc.filters), "meta: %v", node.Service.Meta)
			}
		}))
	}

	// Push a coordinate for one of the nodes so we can try an RTT sort. We
	// have to sleep a little while for the coordinate batch to get flushed.
	{
		req := structs.CoordinateUpdateRequest{
			Datacenter:   "dc1",
			Node:         "node3",
			Coord:        coordinate.NewCoordinate(coordinate.DefaultConfig()),
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "Coordinate.Update", &req, &out))
		time.Sleep(3 * es.server.server.config.CoordinateUpdatePeriod)
	}

	// Try an RTT sort. We don't have any other coordinates in there but
	// showing that the node with a coordinate is always first proves we
	// call the RTT sorting function, which is tested elsewhere.
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("rtt sort iter %d", i), func(t *testing.T) {
			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: query.Query.ID,
				Source: structs.QuerySource{
					Datacenter: "dc1",
					Node:       "node3",
				},
				QueryOptions: structs.QueryOptions{Token: es.execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

			expectNodes(t, &query, &reply, 10)
			assert.Equal(t, "node3", reply.Nodes[0].Node.Node)
		})
	}

	// Make sure the shuffle looks like it's working.
	uniques := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("shuffle iter %d", i), func(t *testing.T) {
			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: query.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: es.execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

			expectNodes(t, &query, &reply, 10)

			var names []string
			for _, node := range reply.Nodes {
				names = append(names, node.Node.Node)
			}
			key := strings.Join(names, "|")
			uniques[key] = struct{}{}
		})
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
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Now run the query and make sure the sort looks right.
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("run nearest query iter %d", i), func(t *testing.T) {
			req := structs.PreparedQueryExecuteRequest{
				Agent: structs.QuerySource{
					Datacenter: "dc1",
					Node:       "node3",
				},
				Datacenter:    "dc1",
				QueryIDOrName: query.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: es.execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, 10)
			assert.Equal(t, "node3", reply.Nodes[0].Node.Node)
		})
	}

	// Query again, but this time set a client-supplied query source. This
	// proves that we allow overriding the baked-in value with ?near.
	t.Run("nearest fallback to shuffle", func(t *testing.T) {
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
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		shuffled := false
		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, 10)

			if node := reply.Nodes[0].Node.Node; node != "node3" {
				shuffled = true
				break
			}
		}

		require.True(t, shuffled, "expect nodes to be shuffled")
	})

	// If the exact node we are sorting near appears in the list, make sure it
	// gets popped to the front of the result.
	t.Run("nearest bypasses shuffle", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Source: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node1",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, 10)
			assert.Equal(t, "node1", reply.Nodes[0].Node.Node)
		}
	})

	// Bake the magic "_agent" flag into the query.
	query.Query.Service.Near = "_agent"
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Check that we sort the local agent first when the magic flag is set.
	t.Run("local agent is first using _agent on node3", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Agent: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "node3",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, 10)
			assert.Equal(t, "node3", reply.Nodes[0].Node.Node)
		}
	})

	// Check that the query isn't just sorting "node3" first because we
	// provided it in the Agent query source. Proves that we use the
	// Agent source when the magic "_agent" flag is passed.
	t.Run("local agent is first using _agent on foo", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Agent: structs.QuerySource{
				Datacenter: "dc1",
				Node:       "foo",
			},
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		// Expect the set to be shuffled since we have no coordinates
		// on the "foo" node.
		shuffled := false
		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, 10)
			if node := reply.Nodes[0].Node.Node; node != "node3" {
				shuffled = true
				break
			}
		}

		require.True(t, shuffled, "expect nodes to be shuffled")
	})

	// Shuffles if the response comes from a non-local DC. Proves that the
	// agent query source does not interfere with the order.
	t.Run("shuffles if coming from non-local dc", func(t *testing.T) {
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
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		shuffled := false
		for i := 0; i < 10; i++ {
			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))
			assert.Len(t, reply.Nodes, 10)
			if reply.Nodes[0].Node.Node != "node3" {
				shuffled = true
				break
			}
		}

		require.True(t, shuffled, "expect node shuffle for remote results")
	})

	// Un-bake the near parameter.
	query.Query.Service.Near = ""
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Update the health of a node to mark it critical.
	setHealth := func(t *testing.T, codec rpc.ClientCodec, dc string, i int, health string) {
		t.Helper()
		req := structs.RegisterRequest{
			Datacenter: dc,
			Node:       fmt.Sprintf("node%d", i),
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "foo",
				Port:    8000,
				Tags:    []string{dc, fmt.Sprintf("tag%d", i)},
			},
			Check: &structs.HealthCheck{
				Name:      "failing",
				Status:    health,
				ServiceID: "foo",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var reply struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply))
	}
	setHealth(t, es.server.codec, "dc1", 1, api.HealthCritical)

	// The failing node should be filtered.
	t.Run("failing node filtered", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 9)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node1", node.Node.Node)
		}
	})

	// Upgrade it to a warning and re-query, should be 10 nodes again.
	setHealth(t, es.server.codec, "dc1", 1, api.HealthWarning)
	t.Run("warning nodes are included", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 10)
	})

	// Make the query more picky so it excludes warning nodes.
	query.Query.Service.OnlyPassing = true
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// The node in the warning state should be filtered.
	t.Run("warning nodes are omitted with onlypassing=true", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 9)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node1", node.Node.Node)
		}
	})

	// Make the query ignore all our health checks (which have "failing" ID
	// implicitly from their name).
	query.Query.Service.IgnoreCheckIDs = []types.CheckID{"failing"}
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// We should end up with 10 nodes again
	t.Run("all nodes including when ignoring failing checks", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 10)
	})

	// Undo that so all the following tests aren't broken!
	query.Query.Service.IgnoreCheckIDs = nil
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Make the query more picky by adding a tag filter. This just proves we
	// call into the tag filter, it is tested more thoroughly in a separate
	// test.
	query.Query.Service.Tags = []string{"!tag3"}
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// The node in the warning state should be filtered as well as the node
	// with the filtered tag.
	t.Run("filter node in warning state and filtered node", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 8)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node1", node.Node.Node)
			assert.NotEqual(t, "node3", node.Node.Node)
		}
	})

	// Make sure the query gets denied with this token.
	t.Run("query denied with deny token", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 0)
	})

	// Bake the exec token into the query.
	query.Query.Token = es.execToken
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Now even querying with the deny token should work.
	t.Run("query with deny token still works", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 8)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node1", node.Node.Node)
			assert.NotEqual(t, "node3", node.Node.Node)
		}
	})

	// Un-bake the token.
	query.Query.Token = ""
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Make sure the query gets denied again with the deny token.
	t.Run("denied with deny token when no query token", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 0)
	})

	t.Run("filter nodes with exec token without node privileges", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execNoNodesToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 0)
		require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("normal operation again with exec token", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 8)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node1", node.Node.Node)
			assert.NotEqual(t, "node3", node.Node.Node)
		}
	})

	// Now fail everything in dc1 and we should get an empty list back.
	for i := 0; i < 10; i++ {
		setHealth(t, es.server.codec, "dc1", i+1, api.HealthCritical)
	}
	t.Run("everything is failing so should get empty list", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectNodes(t, &query, &reply, 0)
	})

	// Modify the query to have it fail over to a bogus DC and then dc2.
	query.Query.Service.Failover.Datacenters = []string{"bogus", "dc2"}
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Now we should see 9 nodes from dc2 (we have the tag filter still).
	t.Run("see 9 nodes from dc2 using tag filter", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectFailoverNodes(t, &query, &reply, 9)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node3", node.Node.Node)
		}
	})

	// Make sure the limit and query options are forwarded.
	t.Run("forward limit and query options", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			Limit:         3,
			QueryOptions: structs.QueryOptions{
				Token:             es.execToken,
				RequireConsistent: true,
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectFailoverNodes(t, &query, &reply, 3)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node3", node.Node.Node)
		}
	})

	// Make sure the remote shuffle looks like it's working.
	uniques = make(map[string]struct{})
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("remote shuffle iter %d", i), func(t *testing.T) {
			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: query.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: es.execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

			expectFailoverNodes(t, &query, &reply, 9)
			var names []string
			for _, node := range reply.Nodes {
				names = append(names, node.Node.Node)
			}
			key := strings.Join(names, "|")
			uniques[key] = struct{}{}
		})
	}

	// We have to allow for the fact that there won't always be a unique
	// shuffle each pass, so we just look for smell here without the test
	// being flaky.
	if len(uniques) < 50 {
		t.Fatalf("unique shuffle ratio too low: %d/100", len(uniques))
	}

	// Make sure the query response from dc2 gets denied with the deny token.
	t.Run("query from dc2 denied with deny token", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectFailoverNodes(t, &query, &reply, 0)
	})

	t.Run("nodes in response from dc2 are filtered by ACL token", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execNoNodesToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectFailoverNodes(t, &query, &reply, 0)
		require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	// Bake the exec token into the query.
	query.Query.Token = es.execToken
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Now even querying with the deny token should work.
	t.Run("query from dc2 with exec token using deny token works", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.denyToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		expectFailoverNodes(t, &query, &reply, 9)
		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node3", node.Node.Node)
		}
	})

	// Modify the query to have it fail over to a bogus DC and then dc2.
	query.Query.Service.Failover = structs.QueryFailoverOptions{
		Targets: []structs.QueryFailoverTarget{
			{Datacenter: "dc2"},
			{Peer: es.peeringServer.acceptingPeerName},
		},
	}
	require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Ensure the foo service has fully replicated.
	retry.Run(t, func(r *retry.R) {
		_, nodes, err := es.server.server.fsm.State().CheckServiceNodes(nil, "foo", nil, es.peeringServer.acceptingPeerName)
		require.NoError(r, err)
		require.Len(r, nodes, 10)
	})

	// Now we should see 9 nodes from dc2
	t.Run("failing over to cluster peers", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node3", node.Node.Node)
		}
		expectFailoverNodes(t, &query, &reply, 9)
	})

	// Set all checks in dc2 as critical
	for i := 0; i < 10; i++ {
		setHealth(t, es.wanServer.codec, "dc2", i+1, api.HealthCritical)
	}

	// Now we should see 9 nodes from dc3 (we have the tag filter still)
	t.Run("failing over to cluster peers", func(t *testing.T) {
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			QueryOptions:  structs.QueryOptions{Token: es.execToken},
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

		for _, node := range reply.Nodes {
			assert.NotEqual(t, "node3", node.Node.Node)
		}
		expectFailoverPeerNodes(t, &query, &reply, 9)
	})

	// Set all checks in dc1 as passing
	for i := 0; i < 10; i++ {
		setHealth(t, es.server.codec, "dc1", i+1, api.HealthPassing)
	}

	// Nothing is healthy so nothing is returned
	t.Run("un-failing over", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req := structs.PreparedQueryExecuteRequest{
				Datacenter:    "dc1",
				QueryIDOrName: query.Query.ID,
				QueryOptions:  structs.QueryOptions{Token: es.execToken},
			}

			var reply structs.PreparedQueryExecuteResponse
			require.NoError(r, msgpackrpc.CallWithCodec(es.server.codec, "PreparedQuery.Execute", &req, &reply))

			for _, node := range reply.Nodes {
				assert.NotEqual(r, "node3", node.Node.Node)
			}

			expectNodes(r, &query, &reply, 9)
		})
	})
}

func TestPreparedQuery_Execute_ForwardLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
				Tags:    []string{"primary"},
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

func TestPreparedQuery_Execute_ConnectExact(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

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
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply))
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
	require.NoError(t, msgpackrpc.CallWithCodec(
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
		require.NoError(t, msgpackrpc.CallWithCodec(
			codec, "PreparedQuery.Execute", &req, &reply))

		// Result should have two because it omits the proxy whose name
		// doesn't match the query.
		require.Len(t, reply.Nodes, 2)
		require.Equal(t, query.Query.Service.Service, reply.Service)
		require.Equal(t, query.Query.DNS, reply.DNS)
		require.True(t, reply.QueryMeta.KnownLeader, "queried leader")
	}

	// Run with the Connect setting specified on the request
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
			Connect:       true,
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(
			codec, "PreparedQuery.Execute", &req, &reply))

		// Result should have two because we should get the native AND
		// the proxy (since the destination matches our service name).
		require.Len(t, reply.Nodes, 2)
		require.Equal(t, query.Query.Service.Service, reply.Service)
		require.Equal(t, query.Query.DNS, reply.DNS)
		require.True(t, reply.QueryMeta.KnownLeader, "queried leader")

		// Make sure the native is the first one
		if !reply.Nodes[0].Service.Connect.Native {
			reply.Nodes[0], reply.Nodes[1] = reply.Nodes[1], reply.Nodes[0]
		}

		require.True(t, reply.Nodes[0].Service.Connect.Native, "native")
		require.Equal(t, reply.Service, reply.Nodes[0].Service.Service)

		require.Equal(t, structs.ServiceKindConnectProxy, reply.Nodes[1].Service.Kind)
		require.Equal(t, reply.Service, reply.Nodes[1].Service.Proxy.DestinationServiceName)
	}

	// Update the query
	query.Query.Service.Connect = true
	require.NoError(t, msgpackrpc.CallWithCodec(
		codec, "PreparedQuery.Apply", &query, &query.Query.ID))

	// Run the registered query.
	{
		req := structs.PreparedQueryExecuteRequest{
			Datacenter:    "dc1",
			QueryIDOrName: query.Query.ID,
		}

		var reply structs.PreparedQueryExecuteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(
			codec, "PreparedQuery.Execute", &req, &reply))

		// Result should have two because we should get the native AND
		// the proxy (since the destination matches our service name).
		require.Len(t, reply.Nodes, 2)
		require.Equal(t, query.Query.Service.Service, reply.Service)
		require.Equal(t, query.Query.DNS, reply.DNS)
		require.True(t, reply.QueryMeta.KnownLeader, "queried leader")

		// Make sure the native is the first one
		if !reply.Nodes[0].Service.Connect.Native {
			reply.Nodes[0], reply.Nodes[1] = reply.Nodes[1], reply.Nodes[0]
		}

		require.True(t, reply.Nodes[0].Service.Connect.Native, "native")
		require.Equal(t, reply.Service, reply.Nodes[0].Service.Service)

		require.Equal(t, structs.ServiceKindConnectProxy, reply.Nodes[1].Service.Kind)
		require.Equal(t, reply.Service, reply.Nodes[1].Service.Proxy.DestinationServiceName)
	}

	// Unset the query
	query.Query.Service.Connect = false
	require.NoError(t, msgpackrpc.CallWithCodec(
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

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	testrpc.WaitForLeader(t, s2.RPC, "dc2", testrpc.WithToken("root"))

	// Try to WAN join.
	joinWAN(t, s2, s1)

	// Try all the operations on a real server via the wrapper.
	wrapper := &queryServerWrapper{srv: s1, executeRemote: func(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
		return nil
	}}
	wrapper.GetLogger().Debug("Test")

	ret, err := wrapper.GetOtherDatacentersByDistance()
	wrapper.GetLogger().Info("Returned value", "value", ret)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(ret) != 1 || ret[0] != "dc2" {
		t.Fatalf("bad: %v", ret)
	}
	// Since we have no idea when the joinWAN operation completes
	// we keep on querying until the join operation completes.
	retry.Run(t, func(r *retry.R) {
		r.Check(s1.forwardDC("Status.Ping", "dc2", &struct{}{}, &struct{}{}))
	})
}

var _ queryServer = (*mockQueryServer)(nil)

type mockQueryServer struct {
	queryServerWrapper
	Datacenters      []string
	DatacentersError error
	QueryLog         []string
	QueryFn          func(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error
	Logger           hclog.Logger
	LogBuffer        *bytes.Buffer
	SamenessGroup    map[string]*structs.SamenessGroupConfigEntry
}

func (m *mockQueryServer) JoinQueryLog() string {
	return strings.Join(m.QueryLog, "|")
}

func (m *mockQueryServer) GetLogger() hclog.Logger {
	if m.Logger == nil {
		m.LogBuffer = new(bytes.Buffer)

		m.Logger = hclog.New(&hclog.LoggerOptions{
			Name:   "mock_query",
			Output: m.LogBuffer,
			Level:  hclog.Debug,
		})
	}
	return m.Logger
}

func (m *mockQueryServer) GetLocalDC() string {
	return localTestDC
}

func (m *mockQueryServer) GetOtherDatacentersByDistance() ([]string, error) {
	return m.Datacenters, m.DatacentersError
}

func (m *mockQueryServer) ExecuteRemote(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
	peerName := args.Query.Service.Peer
	partitionName := args.Query.Service.PartitionOrEmpty()
	namespaceName := args.Query.Service.NamespaceOrEmpty()
	dc := args.Datacenter
	if peerName != "" {
		m.QueryLog = append(m.QueryLog, fmt.Sprintf("peer:%s", peerName))
	} else if partitionName != "" {
		m.QueryLog = append(m.QueryLog, fmt.Sprintf("partition:%s", partitionName))
	} else if namespaceName != "" {
		m.QueryLog = append(m.QueryLog, fmt.Sprintf("namespace:%s", namespaceName))
	} else {
		m.QueryLog = append(m.QueryLog, fmt.Sprintf("%s:%s", dc, "PreparedQuery.ExecuteRemote"))
	}
	reply.PeerName = peerName
	reply.Datacenter = dc
	reply.EnterpriseMeta = acl.NewEnterpriseMetaWithPartition(partitionName, namespaceName)

	if m.QueryFn != nil {
		return m.QueryFn(args, reply)
	}
	return nil
}

type mockStateLookup struct {
	SamenessGroup map[string]*structs.SamenessGroupConfigEntry
}

func (sl mockStateLookup) samenessGroupLookup(name string, entMeta acl.EnterpriseMeta) (uint64, *structs.SamenessGroupConfigEntry, error) {
	lookup := name
	if ap := entMeta.PartitionOrEmpty(); ap != "" {
		lookup = fmt.Sprintf("%s-%s", lookup, ap)
	} else if ns := entMeta.NamespaceOrEmpty(); ns != "" {
		lookup = fmt.Sprintf("%s-%s", lookup, ns)
	}

	sg, ok := sl.SamenessGroup[lookup]
	if !ok {
		return 0, nil, errors.New("unable to find sameness group")
	}

	return 0, sg, nil
}

func (m *mockQueryServer) GetSamenessGroupFailoverTargets(name string, entMeta acl.EnterpriseMeta) ([]structs.QueryFailoverTarget, error) {
	m.sl = mockStateLookup{
		SamenessGroup: m.SamenessGroup,
	}
	return m.queryServerWrapper.GetSamenessGroupFailoverTargets(name, entMeta)
}

func TestPreparedQuery_queryFailover(t *testing.T) {
	t.Parallel()
	query := structs.PreparedQuery{
		Name: "test",
		Service: structs.ServiceQuery{
			Failover: structs.QueryFailoverOptions{
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
	t.Run("Query no datacenters used", func(t *testing.T) {
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 || reply.Datacenter != "" || reply.Failovers != 0 {
			t.Fatalf("bad: %v", reply)
		}
	})

	// Make it fail to get datacenters.
	t.Run("Fail to get datacenters", func(t *testing.T) {
		mock := &mockQueryServer{
			Datacenters:      []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			DatacentersError: fmt.Errorf("XXX"),
		}

		var reply structs.PreparedQueryExecuteResponse
		err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply)
		if err == nil || !strings.Contains(err.Error(), "XXX") {
			t.Fatalf("bad: %v", err)
		}
		if len(reply.Nodes) != 0 || reply.Datacenter != "" || reply.Failovers != 0 {
			t.Fatalf("bad: %v", reply)
		}
	})

	// The query wants to use other datacenters but none are available.
	t.Run("no datacenters available", func(t *testing.T) {
		query.Service.Failover.NearestN = 3
		mock := &mockQueryServer{
			Datacenters: []string{},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 || reply.Datacenter != "" || reply.Failovers != 0 {
			t.Fatalf("bad: %v", reply)
		}
	})

	// Try the first three nearest datacenters, first one has the data.
	t.Run("first datacenter has data", func(t *testing.T) {
		query.Service.Failover.NearestN = 3
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "dc1" {
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc1" || reply.Failovers != 1 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc1:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
	})

	// Try the first three nearest datacenters, last one has the data.
	t.Run("last datacenter has data", func(t *testing.T) {
		query.Service.Failover.NearestN = 3
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "dc3" {
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc3" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc1:PreparedQuery.ExecuteRemote|dc2:PreparedQuery.ExecuteRemote|dc3:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
	})

	// Try the first four nearest datacenters, nobody has the data.
	t.Run("no datacenters with data", func(t *testing.T) {
		query.Service.Failover.NearestN = 4
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 ||
			reply.Datacenter != "xxx" || reply.Failovers != 4 {
			t.Fatalf("bad: %+v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc1:PreparedQuery.ExecuteRemote|dc2:PreparedQuery.ExecuteRemote|dc3:PreparedQuery.ExecuteRemote|xxx:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
	})

	// Try the first two nearest datacenters, plus a user-specified one that
	// has the data.
	t.Run("user specified datacenter with data", func(t *testing.T) {
		query.Service.Failover.NearestN = 2
		query.Service.Failover.Datacenters = []string{"dc4"}
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "dc4" {
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc1:PreparedQuery.ExecuteRemote|dc2:PreparedQuery.ExecuteRemote|dc4:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
	})

	// Add in a hard-coded value that overlaps with the nearest list.
	t.Run("overlap with nearest list", func(t *testing.T) {
		query.Service.Failover.NearestN = 2
		query.Service.Failover.Datacenters = []string{"dc4", "dc1"}
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "dc4" {
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc1:PreparedQuery.ExecuteRemote|dc2:PreparedQuery.ExecuteRemote|dc4:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
	})

	// Now add a bogus user-defined one to the mix.
	t.Run("bogus user-defined", func(t *testing.T) {
		query.Service.Failover.NearestN = 2
		query.Service.Failover.Datacenters = []string{"nope", "dc4", "dc1"}
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "dc4" {
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc1:PreparedQuery.ExecuteRemote|dc2:PreparedQuery.ExecuteRemote|dc4:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
		require.Contains(t, mock.LogBuffer.String(), "Skipping unknown datacenter")
	})

	// Same setup as before but dc1 is going to return an error and should
	// get skipped over, still yielding data from dc4 which comes later.
	t.Run("dc1 error", func(t *testing.T) {
		query.Service.Failover.NearestN = 2
		query.Service.Failover.Datacenters = []string{"dc4", "dc1"}
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "dc1" {
					return fmt.Errorf("XXX")
				} else if req.Datacenter == "dc4" {
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "dc4" || reply.Failovers != 3 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc1:PreparedQuery.ExecuteRemote|dc2:PreparedQuery.ExecuteRemote|dc4:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
		if !strings.Contains(mock.LogBuffer.String(), "Failed querying") {
			t.Fatalf("bad: %s", mock.LogBuffer.String())
		}
	})

	// Just use a hard-coded list and now xxx has the data.
	t.Run("hard coded list", func(t *testing.T) {
		query.Service.Failover.NearestN = 0
		query.Service.Failover.Datacenters = []string{"dc3", "xxx"}
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "xxx" {
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 3 ||
			reply.Datacenter != "xxx" || reply.Failovers != 2 ||
			!reflect.DeepEqual(reply.Nodes, nodes()) {
			t.Fatalf("bad: %v", reply)
		}
		if queries := mock.JoinQueryLog(); queries != "dc3:PreparedQuery.ExecuteRemote|xxx:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
	})

	// Make sure the limit and query options are plumbed through.
	t.Run("limit and query options used", func(t *testing.T) {
		query.Service.Failover.NearestN = 0
		query.Service.Failover.Datacenters = []string{"xxx"}
		mock := &mockQueryServer{
			Datacenters: []string{"dc1", "dc2", "dc3", "xxx", "dc4"},
			QueryFn: func(req *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if req.Datacenter == "xxx" {
					if req.Limit != 5 {
						t.Fatalf("bad: %d", req.Limit)
					}
					if req.RequireConsistent != true {
						t.Fatalf("bad: %v", req.RequireConsistent)
					}
					reply.Nodes = nodes()
				}
				return nil
			},
		}

		var reply structs.PreparedQueryExecuteResponse
		if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{
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
		if queries := mock.JoinQueryLog(); queries != "xxx:PreparedQuery.ExecuteRemote" {
			t.Fatalf("bad: %s", queries)
		}
	})

	// Failover returns data from the first cluster peer with data.
	t.Run("failover first peer with data", func(t *testing.T) {
		query.Service.Failover.Datacenters = nil
		query.Service.Failover.Targets = []structs.QueryFailoverTarget{
			{Peer: "cluster-01"},
			{Datacenter: "dc44"},
			{Peer: "cluster-02"},
		}
		{
			mock := &mockQueryServer{
				Datacenters: []string{"dc44"},
				QueryFn: func(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
					if args.Query.Service.Peer == "cluster-02" {
						reply.Nodes = nodes()
					}
					return nil
				},
			}

			var reply structs.PreparedQueryExecuteResponse
			if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			require.Equal(t, "cluster-02", reply.PeerName)
			require.Equal(t, 3, reply.Failovers)
			require.Equal(t, nodes(), reply.Nodes)
			require.Equal(t, "peer:cluster-01|dc44:PreparedQuery.ExecuteRemote|peer:cluster-02", mock.JoinQueryLog())
		}
	})

	tests := []struct {
		name               string
		targets            []structs.QueryFailoverTarget
		datacenters        []string
		queryfn            func(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error
		expectedPeer       string
		expectedDatacenter string
		expectedReplies    int
		expectedQuery      string
	}{
		{
			name: "failover first peer with data",
			targets: []structs.QueryFailoverTarget{
				{Peer: "cluster-01"},
				{Datacenter: "dc44"},
				{Peer: "cluster-02"},
			},
			queryfn: func(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if args.Query.Service.Peer == "cluster-02" {
					reply.Nodes = nodes()
				}
				return nil
			},
			datacenters:        []string{"dc44"},
			expectedPeer:       "cluster-02",
			expectedDatacenter: "",
			expectedReplies:    3,
			expectedQuery:      "peer:cluster-01|dc44:PreparedQuery.ExecuteRemote|peer:cluster-02",
		},
		{
			name: "failover datacenter with data",
			targets: []structs.QueryFailoverTarget{
				{Peer: "cluster-01"},
				{Datacenter: "dc44"},
				{Peer: "cluster-02"},
			},
			queryfn: func(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if args.Datacenter == "dc44" {
					reply.Nodes = nodes()
				}
				return nil
			},
			datacenters:        []string{"dc44"},
			expectedPeer:       "",
			expectedDatacenter: "dc44",
			expectedReplies:    2,
			expectedQuery:      "peer:cluster-01|dc44:PreparedQuery.ExecuteRemote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query.Service.Failover.Datacenters = nil
			query.Service.Failover.Targets = tt.targets

			mock := &mockQueryServer{
				Datacenters: tt.datacenters,
				QueryFn:     tt.queryfn,
			}

			var reply structs.PreparedQueryExecuteResponse
			if err := queryFailover(mock, query, &structs.PreparedQueryExecuteRequest{}, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
			require.Equal(t, tt.expectedPeer, reply.PeerName)
			require.Equal(t, tt.expectedReplies, reply.Failovers)
			require.Equal(t, nodes(), reply.Nodes)
			require.Equal(t, tt.expectedQuery, mock.JoinQueryLog())
		})
	}
}

type serverTestMetadata struct {
	server            *Server
	codec             rpc.ClientCodec
	datacenter        string
	acceptingPeerName string
	dialingPeerName   string
}

type executeServers struct {
	server           *serverTestMetadata
	peeringServer    *serverTestMetadata
	wanServer        *serverTestMetadata
	execToken        string
	denyToken        string
	execNoNodesToken string
}

func createExecuteServers(t *testing.T) *executeServers {
	es := newExecuteServers(t)
	es.initWanFed(t)
	es.exportPeeringServices(t)
	es.initTokens(t)

	return es
}

func newExecuteServers(t *testing.T) *executeServers {

	// Setup server
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	t.Cleanup(func() {
		os.RemoveAll(dir1)
	})
	t.Cleanup(func() {
		s1.Shutdown()
	})
	waitForLeaderEstablishment(t, s1)
	codec1 := rpcClient(t, s1)
	t.Cleanup(func() {
		codec1.Close()
	})

	ca := connect.TestCA(t, nil)
	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc3"
		c.PrimaryDatacenter = "dc3"
		c.NodeName = "acceptingServer.dc3"
		c.GRPCTLSPort = freeport.GetOne(t)
		c.CAConfig = &structs.CAConfiguration{
			ClusterID: connect.TestClusterID,
			Provider:  structs.ConsulCAProvider,
			Config: map[string]interface{}{
				"PrivateKey": ca.SigningKey,
				"RootCert":   ca.RootCert,
			},
		}
	})
	t.Cleanup(func() {
		os.RemoveAll(dir3)
	})
	t.Cleanup(func() {
		s3.Shutdown()
	})
	waitForLeaderEstablishment(t, s3)
	codec3 := rpcClient(t, s3)
	t.Cleanup(func() {
		codec3.Close()
	})

	// check for RPC forwarding
	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	testrpc.WaitForLeader(t, s3.RPC, "dc3")

	acceptingPeerName := "my-peer-accepting-server"
	dialingPeerName := "my-peer-dialing-server"

	// Set up peering between dc1 (dialing) and dc3 (accepting) and export the foo service
	{
		// Create a peering by generating a token.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: "root"}
		ctx, err := grpcexternal.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)

		conn, err := grpc.DialContext(ctx, s3.config.RPCAddr.String(),
			grpc.WithContextDialer(newServerDialer(s3.config.RPCAddr.String())),
			//nolint:staticcheck
			grpc.WithInsecure(),
			grpc.WithBlock())
		require.NoError(t, err)
		t.Cleanup(func() {
			conn.Close()
		})

		peeringClient := pbpeering.NewPeeringServiceClient(conn)
		req := pbpeering.GenerateTokenRequest{
			PeerName: dialingPeerName,
		}
		resp, err := peeringClient.GenerateToken(ctx, &req)
		require.NoError(t, err)

		conn, err = grpc.DialContext(ctx, s1.config.RPCAddr.String(),
			grpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
			//nolint:staticcheck
			grpc.WithInsecure(),
			grpc.WithBlock())
		require.NoError(t, err)
		t.Cleanup(func() {
			conn.Close()
		})

		peeringClient = pbpeering.NewPeeringServiceClient(conn)
		establishReq := pbpeering.EstablishRequest{
			PeerName:     acceptingPeerName,
			PeeringToken: resp.PeeringToken,
		}
		establishResp, err := peeringClient.Establish(ctx, &establishReq)
		require.NoError(t, err)
		require.NotNil(t, establishResp)

		readResp, err := peeringClient.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: acceptingPeerName})
		require.NoError(t, err)
		require.NotNil(t, readResp)

		// Wait for the stream to be connected.
		retry.Run(t, func(r *retry.R) {
			status, found := s1.peerStreamServer.StreamStatus(readResp.GetPeering().GetID())
			require.True(r, found)
			require.True(r, status.Connected)
		})
	}

	es := executeServers{
		server: &serverTestMetadata{
			server:     s1,
			codec:      codec1,
			datacenter: "dc1",
		},
		peeringServer: &serverTestMetadata{
			server:            s3,
			codec:             codec3,
			datacenter:        "dc3",
			dialingPeerName:   dialingPeerName,
			acceptingPeerName: acceptingPeerName,
		},
	}

	return &es
}

func (es *executeServers) initTokens(t *testing.T) {
	es.execNoNodesToken = createTokenWithPolicyName(t, es.server.codec, "no-nodes", `service_prefix "foo" { policy = "read" }`, "root")
	rules := `
		service_prefix "" { policy = "read" }
		node_prefix "" { policy = "read" }
	`
	es.execToken = createTokenWithPolicyName(t, es.server.codec, "with-read", rules, "root")
	es.denyToken = createTokenWithPolicyName(t, es.server.codec, "with-deny", `service_prefix "foo" { policy = "deny" }`, "root")
}

func (es *executeServers) exportPeeringServices(t *testing.T) {
	exportedServices := structs.ConfigEntryRequest{
		Op:         structs.ConfigEntryUpsert,
		Datacenter: "dc3",
		Entry: &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name:      "foo",
					Consumers: []structs.ServiceConsumer{{Peer: es.peeringServer.dialingPeerName}},
				},
			},
		},
	}
	var configOutput bool
	require.NoError(t, msgpackrpc.CallWithCodec(es.peeringServer.codec, "ConfigEntry.Apply", &exportedServices, &configOutput))
	require.True(t, configOutput)
}

func (es *executeServers) initWanFed(t *testing.T) {
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	t.Cleanup(func() {
		os.RemoveAll(dir2)
	})
	t.Cleanup(func() {
		s2.Shutdown()
	})
	waitForLeaderEstablishment(t, s2)
	codec2 := rpcClient(t, s2)
	t.Cleanup(func() {
		codec2.Close()
	})

	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)

	// Try to WAN join.
	joinWAN(t, s2, es.server.server)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(es.server.server.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members want %d", got, want)
		}
		if got, want := len(s2.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members want %d", got, want)
		}
	})
	testrpc.WaitForLeader(t, es.server.server.RPC, "dc2", testrpc.WithToken("root"))
	es.wanServer = &serverTestMetadata{
		server:     s2,
		codec:      codec2,
		datacenter: "dc2",
	}
}
