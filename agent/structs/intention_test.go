package structs

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntentionGetACLPrefix(t *testing.T) {
	cases := []struct {
		Name     string
		Input    *Intention
		Expected string
	}{
		{
			"unset name",
			&Intention{DestinationName: ""},
			"",
		},

		{
			"set name",
			&Intention{DestinationName: "fo"},
			"fo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual, ok := tc.Input.GetACLPrefix()
			if tc.Expected == "" {
				if !ok {
					return
				}

				t.Fatal("should not be ok")
			}

			if actual != tc.Expected {
				t.Fatalf("bad: %q", actual)
			}
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
			"SourceType must be set to 'consul'",
		},

		{
			"SourceType is other",
			func(x *Intention) { x.SourceType = IntentionSourceType("other") },
			"SourceType must be set to 'consul'",
		},

		{
			"SourceType is external-trust-domain",
			func(x *Intention) { x.SourceType = IntentionSourceType("external-trust-domain") },
			"SourceTypes 'external-trust-domain' and 'external-uri' are only supported in Consul Enterprise",
		},

		{
			"SourceType is external-uri",
			func(x *Intention) { x.SourceType = IntentionSourceType("external-uri") },
			"SourceTypes 'external-trust-domain' and 'external-uri' are only supported in Consul Enterprise",
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
		Input    [][]string // SrcType, SrcNS, SrcN, DstNS, DstN
		Expected [][]string // Same structure as Input
	}{
		{
			"exhaustive list",
			[][]string{
				{"consul", "*", "*", "exact", "*"},
				{"consul", "*", "*", "*", "*"},
				{"consul", "exact", "*", "exact", "exact"},
				{"consul", "*", "*", "exact", "exact"},
				{"consul", "exact", "exact", "*", "*"},
				{"consul", "exact", "exact", "exact", "exact"},
				{"consul", "exact", "exact", "exact", "*"},
				{"consul", "exact", "*", "exact", "*"},
				{"consul", "exact", "*", "*", "*"},
				{"external-trust-domain", "exact", "spiffe://trust.domain", "*", "*"},
				{"external-trust-domain", "exact", "spiffe://trust.domain", "exact", "exact"},
				{"external-trust-domain", "exact", "spiffe://trust.domain", "exact", "*"},
				{"external-uri", "exact", "spiffe://trust.domain/path", "*", "*"},
				{"external-uri", "exact", "spiffe://trust.domain/path", "exact", "exact"},
				{"external-uri", "exact", "spiffe://trust.domain/path", "exact", "*"},
			},
			[][]string{
				{"consul", "exact", "exact", "exact", "exact"},
				{"external-uri", "exact", "spiffe://trust.domain/path", "exact", "exact"},
				{"external-trust-domain", "exact", "spiffe://trust.domain", "exact", "exact"},
				{"consul", "exact", "*", "exact", "exact"},
				{"consul", "*", "*", "exact", "exact"},
				{"consul", "exact", "exact", "exact", "*"},
				{"external-uri", "exact", "spiffe://trust.domain/path", "exact", "*"},
				{"external-trust-domain", "exact", "spiffe://trust.domain", "exact", "*"},
				{"consul", "exact", "*", "exact", "*"},
				{"consul", "*", "*", "exact", "*"},
				{"consul", "exact", "exact", "*", "*"},
				{"external-uri", "exact", "spiffe://trust.domain/path", "*", "*"},
				{"external-trust-domain", "exact", "spiffe://trust.domain", "*", "*"},
				{"consul", "exact", "*", "*", "*"},
				{"consul", "*", "*", "*", "*"},
			},
		},
		{
			"tiebreak deterministically",
			[][]string{
				{"consul", "a", "*", "a", "b"},
				{"consul", "a", "*", "a", "a"},
				{"consul", "b", "a", "a", "a"},
				{"consul", "a", "b", "a", "a"},
				{"consul", "a", "a", "b", "a"},
				{"consul", "a", "a", "a", "b"},
				{"consul", "a", "a", "a", "a"},
				{"external-trust-domain", "a", "spiffe://a", "a", "a"},
				{"external-trust-domain", "a", "spiffe://b", "a", "a"},
				{"external-uri", "a", "spiffe://a/a", "a", "a"},
				{"external-uri", "a", "spiffe://a/b", "a", "a"},
			},
			[][]string{
				// Exact matches first in lexicographical order (arbitrary but
				// deterministic)
				{"consul", "a", "a", "a", "a"},
				{"consul", "a", "a", "a", "b"},
				{"consul", "a", "a", "b", "a"},
				{"consul", "a", "b", "a", "a"},
				{"consul", "b", "a", "a", "a"},
				{"external-uri", "a", "spiffe://a/a", "a", "a"},
				{"external-uri", "a", "spiffe://a/b", "a", "a"},
				{"external-trust-domain", "a", "spiffe://a", "a", "a"},
				{"external-trust-domain", "a", "spiffe://b", "a", "a"},
				// Wildcards next, lexicographical
				{"consul", "a", "*", "a", "a"},
				{"consul", "a", "*", "a", "b"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)

			var input Intentions
			for _, v := range tc.Input {
				input = append(input, &Intention{
					SourceType:      IntentionSourceType(v[0]),
					SourceNS:        v[1],
					SourceName:      v[2],
					DestinationNS:   v[3],
					DestinationName: v[4],
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
					string(v.SourceType),
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
