package consul

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestACLReplication_Sorter(t *testing.T) {
	t.Parallel()
	acls := structs.ACLs{
		&structs.ACL{ID: "a"},
		&structs.ACL{ID: "b"},
		&structs.ACL{ID: "c"},
	}

	sorter := &aclIterator{acls, 0}
	if len := sorter.Len(); len != 3 {
		t.Fatalf("bad: %d", len)
	}
	if !sorter.Less(0, 1) {
		t.Fatalf("should be less")
	}
	if sorter.Less(1, 0) {
		t.Fatalf("should not be less")
	}
	if !sort.IsSorted(sorter) {
		t.Fatalf("should be sorted")
	}

	expected := structs.ACLs{
		&structs.ACL{ID: "b"},
		&structs.ACL{ID: "a"},
		&structs.ACL{ID: "c"},
	}
	sorter.Swap(0, 1)
	if !reflect.DeepEqual(acls, expected) {
		t.Fatalf("bad: %v", acls)
	}
	if sort.IsSorted(sorter) {
		t.Fatalf("should not be sorted")
	}
	sort.Sort(sorter)
	if !sort.IsSorted(sorter) {
		t.Fatalf("should be sorted")
	}
}

func TestACLReplication_Iterator(t *testing.T) {
	t.Parallel()
	acls := structs.ACLs{}

	iter := newACLIterator(acls)
	if front := iter.Front(); front != nil {
		t.Fatalf("bad: %v", front)
	}
	iter.Next()
	if front := iter.Front(); front != nil {
		t.Fatalf("bad: %v", front)
	}

	acls = structs.ACLs{
		&structs.ACL{ID: "a"},
		&structs.ACL{ID: "b"},
		&structs.ACL{ID: "c"},
	}
	iter = newACLIterator(acls)
	if front := iter.Front(); front != acls[0] {
		t.Fatalf("bad: %v", front)
	}
	iter.Next()
	if front := iter.Front(); front != acls[1] {
		t.Fatalf("bad: %v", front)
	}
	iter.Next()
	if front := iter.Front(); front != acls[2] {
		t.Fatalf("bad: %v", front)
	}
	iter.Next()
	if front := iter.Front(); front != nil {
		t.Fatalf("bad: %v", front)
	}
}

func TestACLReplication_reconcileACLs(t *testing.T) {
	t.Parallel()
	parseACLs := func(raw string) structs.ACLs {
		var acls structs.ACLs
		for _, key := range strings.Split(raw, "|") {
			if len(key) == 0 {
				continue
			}

			tuple := strings.Split(key, ":")
			index, err := strconv.Atoi(tuple[1])
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			acl := &structs.ACL{
				ID:    tuple[0],
				Rules: tuple[2],
				RaftIndex: structs.RaftIndex{
					ModifyIndex: uint64(index),
				},
			}
			acls = append(acls, acl)
		}
		return acls
	}

	parseChanges := func(changes structs.ACLRequests) string {
		var ret string
		for i, change := range changes {
			if i > 0 {
				ret += "|"
			}
			ret += fmt.Sprintf("%s:%s:%s", change.Op, change.ACL.ID, change.ACL.Rules)
		}
		return ret
	}

	tests := []struct {
		local           string
		remote          string
		lastRemoteIndex uint64
		expected        string
	}{
		// Everything empty.
		{
			local:           "",
			remote:          "",
			lastRemoteIndex: 0,
			expected:        "",
		},
		// First time with empty local.
		{
			local:           "",
			remote:          "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			lastRemoteIndex: 0,
			expected:        "set:bbb:X|set:ccc:X|set:ddd:X|set:eee:X",
		},
		// Remote not sorted.
		{
			local:           "",
			remote:          "ddd:2:X|bbb:3:X|ccc:9:X|eee:11:X",
			lastRemoteIndex: 0,
			expected:        "set:bbb:X|set:ccc:X|set:ddd:X|set:eee:X",
		},
		// Neither side sorted.
		{
			local:           "ddd:2:X|bbb:3:X|ccc:9:X|eee:11:X",
			remote:          "ccc:9:X|bbb:3:X|ddd:2:X|eee:11:X",
			lastRemoteIndex: 0,
			expected:        "",
		},
		// Fully replicated, nothing to do.
		{
			local:           "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			remote:          "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			lastRemoteIndex: 0,
			expected:        "",
		},
		// Change an ACL.
		{
			local:           "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			remote:          "bbb:3:X|ccc:33:Y|ddd:2:X|eee:11:X",
			lastRemoteIndex: 0,
			expected:        "set:ccc:Y",
		},
		// Change an ACL, but mask the change by the last replicated
		// index. This isn't how things work normally, but it proves
		// we are skipping the full compare based on the index.
		{
			local:           "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			remote:          "bbb:3:X|ccc:33:Y|ddd:2:X|eee:11:X",
			lastRemoteIndex: 33,
			expected:        "",
		},
		// Empty everything out.
		{
			local:           "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			remote:          "",
			lastRemoteIndex: 0,
			expected:        "delete:bbb:X|delete:ccc:X|delete:ddd:X|delete:eee:X",
		},
		// Adds on the ends and in the middle.
		{
			local:           "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			remote:          "aaa:99:X|bbb:3:X|ccc:9:X|ccx:101:X|ddd:2:X|eee:11:X|fff:102:X",
			lastRemoteIndex: 0,
			expected:        "set:aaa:X|set:ccx:X|set:fff:X",
		},
		// Deletes on the ends and in the middle.
		{
			local:           "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			remote:          "ccc:9:X",
			lastRemoteIndex: 0,
			expected:        "delete:bbb:X|delete:ddd:X|delete:eee:X",
		},
		// Everything.
		{
			local:           "bbb:3:X|ccc:9:X|ddd:2:X|eee:11:X",
			remote:          "aaa:99:X|bbb:3:X|ccx:101:X|ddd:103:Y|eee:11:X|fff:102:X",
			lastRemoteIndex: 11,
			expected:        "set:aaa:X|delete:ccc:X|set:ccx:X|set:ddd:Y|set:fff:X",
		},
	}
	for i, test := range tests {
		local, remote := parseACLs(test.local), parseACLs(test.remote)
		changes := reconcileLegacyACLs(local, remote, test.lastRemoteIndex)
		if actual := parseChanges(changes); actual != test.expected {
			t.Errorf("test case %d failed: %s", i, actual)
		}
	}
}

func TestACLReplication_updateLocalACLs_RateLimit(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLReplicationApplyLimit = 1
	})
	s1.tokens.UpdateACLReplicationToken("secret")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	changes := structs.ACLRequests{
		&structs.ACLRequest{
			Op: structs.ACLSet,
			ACL: structs.ACL{
				ID:   "secret",
				Type: "client",
			},
		},
	}

	// Should be throttled to 1 Hz.
	start := time.Now()
	if _, err := s1.updateLocalLegacyACLs(changes, context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if dur := time.Since(start); dur < time.Second {
		t.Fatalf("too slow: %9.6f", dur.Seconds())
	}

	changes = append(changes,
		&structs.ACLRequest{
			Op: structs.ACLSet,
			ACL: structs.ACL{
				ID:   "secret",
				Type: "client",
			},
		})

	// Should be throttled to 1 Hz.
	start = time.Now()
	if _, err := s1.updateLocalLegacyACLs(changes, context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if dur := time.Since(start); dur < 2*time.Second {
		t.Fatalf("too fast: %9.6f", dur.Seconds())
	}
}

func TestACLReplication_IsACLReplicationEnabled(t *testing.T) {
	t.Parallel()
	// ACLs not enabled.
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = ""
		c.ACLsEnabled = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	if s1.IsACLReplicationEnabled() {
		t.Fatalf("should not be enabled")
	}

	// ACLs enabled but not replication.
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	if s2.IsACLReplicationEnabled() {
		t.Fatalf("should not be enabled")
	}

	// ACLs enabled with replication.
	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()
	testrpc.WaitForLeader(t, s3.RPC, "dc2")
	if !s3.IsACLReplicationEnabled() {
		t.Fatalf("should be enabled")
	}

	// ACLs enabled with replication, but inside the ACL datacenter
	// so replication should be disabled.
	dir4, s4 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
	})
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()
	testrpc.WaitForLeader(t, s4.RPC, "dc1")
	if s4.IsACLReplicationEnabled() {
		t.Fatalf("should not be enabled")
	}
}

func TestACLReplication(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
		c.ACLReplicationApplyLimit = 1000000
	})
	s2.tokens.UpdateACLReplicationToken("root")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Create a bunch of new tokens.
	var id string
	for i := 0; i < 50; i++ {
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: testACLPolicy,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := s1.RPC("ACL.Apply", &arg, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	checkSame := func() error {
		index, remote, err := s1.fsm.State().ACLTokenList(nil, true, true, "")
		if err != nil {
			return err
		}
		_, local, err := s2.fsm.State().ACLTokenList(nil, true, true, "")
		if err != nil {
			return err
		}
		if got, want := len(remote), len(local); got != want {
			return fmt.Errorf("got %d remote ACLs want %d", got, want)
		}
		for i, token := range remote {
			if !bytes.Equal(token.Hash, local[i].Hash) {
				return fmt.Errorf("ACLs differ")
			}
		}

		var status structs.ACLReplicationStatus
		s2.aclReplicationStatusLock.RLock()
		status = s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()
		if !status.Enabled || !status.Running ||
			status.ReplicatedTokenIndex != index ||
			status.SourceDatacenter != "dc1" {
			return fmt.Errorf("ACL replication status differs")
		}

		return nil
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		if err := checkSame(); err != nil {
			r.Fatal(err)
		}
	})

	// Create more new tokens.
	for i := 0; i < 50; i++ {
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: testACLPolicy,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var dontCare string
		if err := s1.RPC("ACL.Apply", &arg, &dontCare); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		if err := checkSame(); err != nil {
			r.Fatal(err)
		}
	})

	// Delete a token.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLDelete,
		ACL: structs.ACL{
			ID: id,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var dontCare string
	if err := s1.RPC("ACL.Apply", &arg, &dontCare); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		if err := checkSame(); err != nil {
			r.Fatal(err)
		}
	})
}

func TestACLReplication_diffACLPolicies(t *testing.T) {
	local := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:          "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Name:        "policy1",
			Description: "policy1 - already in sync",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLPolicy{
			ID:          "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Name:        "policy2",
			Description: "policy2 - updated but not changed",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLPolicy{
			ID:          "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Name:        "policy3",
			Description: "policy3 - updated and changed",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLPolicy{
			ID:          "e9d33298-6490-4466-99cb-ba93af64fa76",
			Name:        "policy4",
			Description: "policy4 - needs deleting",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
	}

	remote := structs.ACLPolicyListStubs{
		&structs.ACLPolicyListStub{
			ID:          "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Name:        "policy1",
			Description: "policy1 - already in sync",
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 2,
		},
		&structs.ACLPolicyListStub{
			ID:          "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Name:        "policy2",
			Description: "policy2 - updated but not changed",
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLPolicyListStub{
			ID:          "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Name:        "policy3",
			Description: "policy3 - updated and changed",
			Datacenters: nil,
			Hash:        []byte{5, 6, 7, 8},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLPolicyListStub{
			ID:          "c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
			Name:        "policy5",
			Description: "policy5 - needs adding",
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
	}

	// Do the full diff. This full exercises the main body of the loop
	deletions, updates := diffACLPolicies(local, remote, 28)
	require.Len(t, updates, 2)
	require.ElementsMatch(t, updates, []string{
		"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2"})

	require.Len(t, deletions, 1)
	require.Equal(t, "e9d33298-6490-4466-99cb-ba93af64fa76", deletions[0])

	deletions, updates = diffACLPolicies(local, nil, 28)
	require.Len(t, updates, 0)
	require.Len(t, deletions, 4)
	require.ElementsMatch(t, deletions, []string{
		"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
		"8ea41efb-8519-4091-bc91-c42da0cda9ae",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
		"e9d33298-6490-4466-99cb-ba93af64fa76"})

	deletions, updates = diffACLPolicies(nil, remote, 28)
	require.Len(t, deletions, 0)
	require.Len(t, updates, 4)
	require.ElementsMatch(t, updates, []string{
		"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
		"8ea41efb-8519-4091-bc91-c42da0cda9ae",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
		"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926"})
}

func TestACLReplication_diffACLTokens(t *testing.T) {
	local := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID:  "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			SecretID:    "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Description: "token1 - already in sync",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLToken{
			AccessorID:  "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			SecretID:    "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Description: "token2 - updated but not changed",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLToken{
			AccessorID:  "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			SecretID:    "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Description: "token3 - updated and changed",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLToken{
			AccessorID:  "e9d33298-6490-4466-99cb-ba93af64fa76",
			SecretID:    "e9d33298-6490-4466-99cb-ba93af64fa76",
			Description: "token4 - needs deleting",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
	}

	remote := structs.ACLTokenListStubs{
		&structs.ACLTokenListStub{
			AccessorID:  "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Description: "token1 - already in sync",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 2,
		},
		&structs.ACLTokenListStub{
			AccessorID:  "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Description: "token2 - updated but not changed",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLTokenListStub{
			AccessorID:  "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Description: "token3 - updated and changed",
			Hash:        []byte{5, 6, 7, 8},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLTokenListStub{
			AccessorID:  "c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
			Description: "token5 - needs adding",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
	}

	// Do the full diff. This full exercises the main body of the loop
	deletions, updates := diffACLTokens(local, remote, 28)
	require.Len(t, updates, 2)
	require.ElementsMatch(t, updates, []string{
		"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2"})

	require.Len(t, deletions, 1)
	require.Equal(t, "e9d33298-6490-4466-99cb-ba93af64fa76", deletions[0])

	deletions, updates = diffACLTokens(local, nil, 28)
	require.Len(t, updates, 0)
	require.Len(t, deletions, 4)
	require.ElementsMatch(t, deletions, []string{
		"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
		"8ea41efb-8519-4091-bc91-c42da0cda9ae",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
		"e9d33298-6490-4466-99cb-ba93af64fa76"})

	deletions, updates = diffACLTokens(nil, remote, 28)
	require.Len(t, deletions, 0)
	require.Len(t, updates, 4)
	require.ElementsMatch(t, updates, []string{
		"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
		"8ea41efb-8519-4091-bc91-c42da0cda9ae",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
		"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926"})
}
