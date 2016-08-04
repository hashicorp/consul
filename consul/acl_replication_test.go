package consul

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

func TestACLReplication_Sorter(t *testing.T) {
	acls := structs.ACLs{
		&structs.ACL{ID: "a"},
		&structs.ACL{ID: "b"},
		&structs.ACL{ID: "c"},
	}

	sorter := &aclIDSorter{acls}
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
			lastRemoteIndex: 0,
			expected:        "set:aaa:X|delete:ccc:X|set:ccx:X|set:ddd:Y|set:fff:X",
		},
	}
	for i, test := range tests {
		local, remote := parseACLs(test.local), parseACLs(test.remote)
		changes := reconcileACLs(local, remote, test.lastRemoteIndex)
		if actual := parseChanges(changes); actual != test.expected {
			t.Errorf("test case %d failed: %s", i, actual)
		}
	}
}
