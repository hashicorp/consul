package structs

import (
	"reflect"
	"sort"
	"testing"
)

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
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var input Intentions
			for _, v := range tc.Input {
				input = append(input, &Intention{
					SourceNS:        v[0],
					SourceName:      v[1],
					DestinationNS:   v[2],
					DestinationName: v[3],
				})
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
			if !reflect.DeepEqual(actual, tc.Expected) {
				t.Fatalf("bad (got, wanted):\n\n%#v\n\n%#v", actual, tc.Expected)
			}
		})
	}
}
