package structs

import (
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntention_ACLs(t *testing.T) {
	t.Parallel()
	type testCase struct {
		intention Intention
		rules     string
		read      bool
		write     bool
	}

	cases := map[string]testCase{
		"all-denied": testCase{
			intention: Intention{
				SourceNS:        "default",
				SourceName:      "web",
				DestinationNS:   "default",
				DestinationName: "api",
			},
			read:  false,
			write: false,
		},
		"deny-write-read-dest": testCase{
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
		"deny-write-read-source": testCase{
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
		"allow-write-with-dest-write": testCase{
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
		"deny-write-with-source-write": testCase{
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
		"deny-wildcard-write-allow-read": testCase{
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
		"allow-wildcard-write": testCase{
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
		"allow-wildcard-read": testCase{
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
			authz, err := acl.NewAuthorizerFromRules("", 0, tcase.rules, acl.SyntaxCurrent, &config, nil)
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
