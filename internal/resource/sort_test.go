// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestLessReference(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	parseTenancy := func(s string) *pbresource.Tenancy {
		// format is: <partition>.<namespace>
		parts := strings.Split(s, ".")
		if len(parts) != 2 {
			panic("bad tenancy")
		}
		return &pbresource.Tenancy{
			Partition: parts[0],
			Namespace: parts[1],
		}
	}

	makeRef := func(s string) *pbresource.Reference {
		// format is:
		// - <type>/<tenancy>/<name>@<section>
		// - <type>/<tenancy>/<name>
		//
		// type = (gvk style)
		// tenancy = <partition>.<namespace>

		parts := strings.Split(s, "/")
		require.Len(t, parts, 3)

		name, section, _ := strings.Cut(parts[2], "@")

		return &pbresource.Reference{
			Type:    GVKToType(parts[0]),
			Tenancy: parseTenancy(parts[1]),
			Name:    name,
			Section: section,
		}
	}

	var inputs []*pbresource.Reference

	stringify := func(all []*pbresource.Reference) []string {
		var out []string
		for _, ref := range all {
			out = append(out, ReferenceToString(ref))
		}
		return out
	}

	// We generate pre-sorted data.
	vals := []string{"a", "aa", "b", "bb"}
	sectionVals := append([]string{""}, vals...)
	for _, group := range vals {
		for _, version := range vals {
			for _, kind := range vals {
				for _, partition := range vals {
					for _, namespace := range vals {
						for _, name := range vals {
							for _, section := range sectionVals {
								if section != "" {
									section = "@" + section
								}
								inputs = append(inputs, makeRef(
									fmt.Sprintf(
										"%s.%s.%s/%s.%s/%s%s",
										group, version, kind,
										partition, namespace,
										name, section,
									),
								))
							}
						}
					}
				}
			}
		}
	}

	require.True(t, sort.IsSorted(sortedReferences(inputs)))

	const randomTrials = 5

	mixed := protoSliceClone(inputs)
	for i := 0; i < randomTrials; i++ {
		rand.Shuffle(len(mixed), func(i, j int) {
			mixed[i], mixed[j] = mixed[j], mixed[i]
		})

		// We actually got a permuted list out of this.
		require.NotEqual(t, stringify(inputs), stringify(mixed))

		sort.Slice(mixed, func(i, j int) bool {
			return LessReference(mixed[i], mixed[j])
		})

		// And it sorted.
		require.Equal(t, stringify(inputs), stringify(mixed))
	}

}

type sortedReferences []*pbresource.Reference

func (r sortedReferences) Len() int {
	return len(r)
}

func (r sortedReferences) Less(i, j int) bool {
	return LessReference(r[i], r[j])
}

func (r sortedReferences) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func protoClone[T proto.Message](v T) T {
	return proto.Clone(v).(T)
}

func protoSliceClone[T proto.Message](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, 0, len(in))
	for _, v := range in {
		out = append(out, protoClone[T](v))
	}
	return out
}
