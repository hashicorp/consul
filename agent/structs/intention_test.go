package structs

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

func TestIntention_ACLs(t *testing.T) {
	type testCase struct {
		intention Intention
		rules     string
		read      bool
		write     bool
	}

	cases := map[string]testCase{
		"all-denied": {
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "web",
				DestinationNS:   "default",
				DestinationName: "api",
			},
			read:  false,
			write: false,
		},
		"deny-write-read-dest": {
			rules: `service "api" { policy = "deny" intentions = "read" }`,
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "web",
				DestinationNS:   "default",
				DestinationName: "api",
			},
			read:  true,
			write: false,
		},
		"deny-write-read-source": {
			rules: `service "web" { policy = "deny" intentions = "read" }`,
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "web",
				DestinationNS:   "default",
				DestinationName: "api",
			},
			read:  true,
			write: false,
		},
		"allow-write-with-dest-write": {
			rules: `service "api" { policy = "deny" intentions = "write" }`,
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "web",
				DestinationNS:   "default",
				DestinationName: "api",
			},
			read:  true,
			write: true,
		},
		"deny-write-with-source-write": {
			rules: `service "web" { policy = "deny" intentions = "write" }`,
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "web",
				DestinationNS:   "default",
				DestinationName: "api",
			},
			read:  true,
			write: false,
		},
		"deny-wildcard-write-allow-read": {
			rules: `service "*" { policy = "deny" intentions = "write" }`,
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "*",
			},
			// technically having been granted read/write on any intention will allow
			// read access for this rule
			read:  true,
			write: false,
		},
		"allow-wildcard-write": {
			rules: `service_prefix "" { policy = "deny" intentions = "write" }`,
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "*",
			},
			read:  true,
			write: true,
		},
		"allow-wildcard-read": {
			rules: `service "foo" { policy = "deny" intentions = "read" }`,
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: "*",
			},
			read:  true,
			write: false,
		},
	}

	config := acl.Config{
		WildcardName: WildcardSpecifier,
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			authz, err := acl.NewAuthorizerFromRules(tcase.rules, acl.SyntaxCurrent, &config, nil)
			require.NoError(t, err)

			require.Equal(t, tcase.read, tcase.intention.CanRead(authz))
			require.Equal(t, tcase.write, tcase.intention.CanWrite(authz))
		})
	}
}

func TestIntentionValidate(t *testing.T) {
	cases := []struct {
		Name   string
		Modify func(*Intention)
		Err    string
	}{
		{
			"long description",
			func(x *Intention) {
				x.Description = strings.Repeat("x", metaValueMaxLength+1)
			},
			"description exceeds",
		},

		{
			"no action set",
			func(x *Intention) { x.Action = "" },
			"action must be set",
		},

		{
			"no SourceNS",
			func(x *Intention) { x.SourceNS = "" },
			"SourceNS must be set",
		},

		{
			"no SourceName",
			func(x *Intention) { x.SourceName = "" },
			"SourceName must be set",
		},

		{
			"no DestinationNS",
			func(x *Intention) { x.DestinationNS = "" },
			"DestinationNS must be set",
		},

		{
			"no DestinationName",
			func(x *Intention) { x.DestinationName = "" },
			"DestinationName must be set",
		},

		{
			"SourceNS partial wildcard",
			func(x *Intention) { x.SourceNS = "foo*" },
			"partial value",
		},

		{
			"SourceName partial wildcard",
			func(x *Intention) { x.SourceName = "foo*" },
			"partial value",
		},

		{
			"SourceName exact following wildcard",
			func(x *Intention) {
				x.SourceNS = "*"
				x.SourceName = "foo"
			},
			"follow wildcard",
		},

		{
			"DestinationNS partial wildcard",
			func(x *Intention) { x.DestinationNS = "foo*" },
			"partial value",
		},

		{
			"DestinationName partial wildcard",
			func(x *Intention) { x.DestinationName = "foo*" },
			"partial value",
		},

		{
			"DestinationName exact following wildcard",
			func(x *Intention) {
				x.DestinationNS = "*"
				x.DestinationName = "foo"
			},
			"follow wildcard",
		},

		{
			"SourceType is not set",
			func(x *Intention) { x.SourceType = "" },
			"SourceType must",
		},

		{
			"SourceType is other",
			func(x *Intention) { x.SourceType = IntentionSourceType("other") },
			"SourceType must",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)
			ixn := TestIntention(t)
			tc.Modify(ixn)

			err := ixn.Validate()
			assert.Equal(err != nil, tc.Err != "", err)
			if err == nil {
				return
			}

			assert.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.Err))
		})
	}
}

func TestIntentionPrecedenceSorter(t *testing.T) {
	cases := []struct {
		Name     string
		Input    [][]string // SrcNS, SrcN, DstNS, DstN
		Expected [][]string // Same structure as Input
	}{
		{
			"exhaustive list",
			[][]string{
				{"*", "*", "exact", "*"},
				{"*", "*", "*", "*"},
				{"exact", "*", "exact", "exact"},
				{"*", "*", "exact", "exact"},
				{"exact", "exact", "*", "*"},
				{"exact", "exact", "exact", "exact"},
				{"exact", "exact", "exact", "*"},
				{"exact", "*", "exact", "*"},
				{"exact", "*", "*", "*"},
			},
			[][]string{
				{"exact", "exact", "exact", "exact"},
				{"exact", "*", "exact", "exact"},
				{"*", "*", "exact", "exact"},
				{"exact", "exact", "exact", "*"},
				{"exact", "*", "exact", "*"},
				{"*", "*", "exact", "*"},
				{"exact", "exact", "*", "*"},
				{"exact", "*", "*", "*"},
				{"*", "*", "*", "*"},
			},
		},
		{
			"tiebreak deterministically",
			[][]string{
				{"a", "*", "a", "b"},
				{"a", "*", "a", "a"},
				{"b", "a", "a", "a"},
				{"a", "b", "a", "a"},
				{"a", "a", "b", "a"},
				{"a", "a", "a", "b"},
				{"a", "a", "a", "a"},
			},
			[][]string{
				// Exact matches first in lexicographical order (arbitrary but
				// deterministic)
				{"a", "a", "a", "a"},
				{"a", "a", "a", "b"},
				{"a", "a", "b", "a"},
				{"a", "b", "a", "a"},
				{"b", "a", "a", "a"},
				// Wildcards next, lexicographical
				{"a", "*", "a", "a"},
				{"a", "*", "a", "b"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)

			var input Intentions
			for _, v := range tc.Input {
				input = append(input, &Intention{
					SourceNS:        v[0],
					SourceName:      v[1],
					DestinationNS:   v[2],
					DestinationName: v[3],
				})
			}

			// Set all the precedence values
			for _, ixn := range input {
				ixn.UpdatePrecedence()
			}

			// Sort
			sort.Sort(IntentionPrecedenceSorter(input))

			// Get back into a comparable form
			var actual [][]string
			for _, v := range input {
				actual = append(actual, []string{
					v.SourceNS,
					v.SourceName,
					v.DestinationNS,
					v.DestinationName,
				})
			}
			assert.Equal(tc.Expected, actual)
		})
	}
}

func TestIntention_SetHash(t *testing.T) {
	i := Intention{
		ID:              "the-id",
		Description:     "the-description",
		SourceNS:        "source-ns",
		SourceName:      "source-name",
		DestinationNS:   "dest-ns",
		DestinationName: "dest-name",
		SourceType:      "source-type",
		Action:          "action",
		Precedence:      123,
		Meta: map[string]string{
			"meta1": "one",
			"meta2": "two",
		},
	}
	i.SetHash()
	expected := []byte{
		0x20, 0x89, 0x55, 0xdb, 0x69, 0x34, 0xce, 0x89, 0xd8, 0xb9, 0x2e, 0x3a,
		0x85, 0xb6, 0xea, 0x43, 0xb2, 0x23, 0x16, 0x93, 0x94, 0x13, 0x2a, 0xe4,
		0x81, 0xfe, 0xe, 0x34, 0x91, 0x99, 0xe9, 0x8d,
	}
	require.Equal(t, expected, i.Hash)
}

func TestIntention_String(t *testing.T) {
	type testcase struct {
		ixn    *Intention
		expect string
	}

	testID := generateUUID()

	partitionPrefix := DefaultEnterpriseMetaInDefaultPartition().PartitionOrEmpty()
	if partitionPrefix != "" {
		partitionPrefix += "/"
	}

	cases := map[string]testcase{
		"legacy allow": {
			&Intention{
				ID:              testID,
				SourceName:      "foo",
				DestinationName: "bar",
				Action:          IntentionActionAllow,
			},
			partitionPrefix + `default/foo => ` + partitionPrefix + `default/bar (ID: ` + testID + `, Precedence: 9, Action: ALLOW)`,
		},
		"legacy deny": {
			&Intention{
				ID:              testID,
				SourceName:      "foo",
				DestinationName: "bar",
				Action:          IntentionActionDeny,
			},
			partitionPrefix + `default/foo => ` + partitionPrefix + `default/bar (ID: ` + testID + `, Precedence: 9, Action: DENY)`,
		},
		"L4 allow": {
			&Intention{
				SourceName:      "foo",
				DestinationName: "bar",
				Action:          IntentionActionAllow,
			},
			partitionPrefix + `default/foo => ` + partitionPrefix + `default/bar (Precedence: 9, Action: ALLOW)`,
		},
		"L4 deny": {
			&Intention{
				SourceName:      "foo",
				DestinationName: "bar",
				Action:          IntentionActionDeny,
			},
			partitionPrefix + `default/foo => ` + partitionPrefix + `default/bar (Precedence: 9, Action: DENY)`,
		},
		"L7 one perm": {
			&Intention{
				SourceName:      "foo",
				DestinationName: "bar",
				Permissions: []*IntentionPermission{
					{
						Action: IntentionActionAllow,
						HTTP: &IntentionHTTPPermission{
							PathPrefix: "/foo",
						},
					},
				},
			},
			partitionPrefix + `default/foo => ` + partitionPrefix + `default/bar (Precedence: 9, Permissions: 1)`,
		},
		"L7 two perms": {
			&Intention{
				SourceName:      "foo",
				DestinationName: "bar",
				Permissions: []*IntentionPermission{
					{
						Action: IntentionActionDeny,
						HTTP: &IntentionHTTPPermission{
							PathExact: "/foo/admin",
						},
					},
					{
						Action: IntentionActionAllow,
						HTTP: &IntentionHTTPPermission{
							PathPrefix: "/foo",
						},
					},
				},
			},
			partitionPrefix + `default/foo => ` + partitionPrefix + `default/bar (Precedence: 9, Permissions: 2)`,
		},
	}

	for name, tc := range cases {
		tc := tc
		// Add a bunch of required fields.
		tc.ixn.FillPartitionAndNamespace(DefaultEnterpriseMetaInDefaultPartition(), true)
		tc.ixn.UpdatePrecedence()

		t.Run(name, func(t *testing.T) {
			got := tc.ixn.String()
			require.Equal(t, tc.expect, got)
		})
	}
}

func TestIntentionQueryRequest_CacheInfoKey(t *testing.T) {
	assertCacheInfoKeyIsComplete(t, &IntentionQueryRequest{})
}
