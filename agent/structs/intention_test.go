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
