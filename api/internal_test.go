package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAPI_Internal_AssignServiceVirtualIP(t *testing.T) {
	t.Parallel()
	doTest_Internal_AssignServiceVirtualIP(t, &WriteOptions{
		Namespace: defaultNamespace,
		Partition: defaultPartition,
	})
}

func doTest_Internal_AssignServiceVirtualIP(t *testing.T, writeOpts *WriteOptions) {
	c, s := makeClient(t)
	defer s.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if writeOpts.Partition != "" {
		_, _, err := c.Partitions().Create(ctx, &Partition{Name: writeOpts.Partition}, nil)
		require.NoError(t, err)
	}
	if writeOpts.Namespace != "" {
		_, _, err := c.Namespaces().Create(&Namespace{Name: writeOpts.Namespace, Partition: writeOpts.Partition}, nil)
		require.NoError(t, err)
	}

	// Create resolvers that we can attach VIPs to.
	for _, name := range []string{"one", "two", "three"} {
		ok, _, err := c.ConfigEntries().Set(&ServiceResolverConfigEntry{
			Kind:      ServiceResolver,
			Name:      name,
			Namespace: writeOpts.Namespace,
			Partition: writeOpts.Partition,
		}, writeOpts)
		require.NoError(t, err)
		require.True(t, ok)
	}

	tests := []struct {
		tName                string
		svcName              string
		vips                 []string
		expectFound          bool
		expectUnassignedFrom []PeeredServiceName
	}{
		{
			tName:                "missing service is no-op",
			svcName:              "missing",
			vips:                 []string{"1.1.1.1", "2.2.2.2"},
			expectFound:          false,
			expectUnassignedFrom: nil,
		},
		{
			tName:                "set vips for one",
			svcName:              "one",
			vips:                 []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"},
			expectFound:          true,
			expectUnassignedFrom: nil,
		},
		{
			tName:       "move vip to two",
			svcName:     "two",
			vips:        []string{"2.2.2.2"},
			expectFound: true,
			expectUnassignedFrom: []PeeredServiceName{
				{ServiceName: CompoundServiceName{Name: "one", Namespace: writeOpts.Namespace, Partition: writeOpts.Partition}},
			},
		},
		{
			tName:       "move vip to three",
			svcName:     "three",
			vips:        []string{"3.3.3.3"},
			expectFound: true,
			expectUnassignedFrom: []PeeredServiceName{
				{ServiceName: CompoundServiceName{Name: "one", Namespace: writeOpts.Namespace, Partition: writeOpts.Partition}},
			},
		},
		{
			tName:                "no-op try move vips to missing",
			svcName:              "missing",
			vips:                 []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"},
			expectFound:          false,
			expectUnassignedFrom: nil,
		},
		{
			tName:       "move all vips back to one",
			svcName:     "one",
			vips:        []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"},
			expectFound: true,
			expectUnassignedFrom: []PeeredServiceName{
				{ServiceName: CompoundServiceName{Name: "two", Namespace: writeOpts.Namespace, Partition: writeOpts.Partition}},
				{ServiceName: CompoundServiceName{Name: "three", Namespace: writeOpts.Namespace, Partition: writeOpts.Partition}},
			},
		},
	}

	internal := c.Internal()
	for _, tc := range tests {
		t.Run(tc.tName, func(t *testing.T) {
			resp, _, err := internal.AssignServiceVirtualIP(ctx, tc.svcName, tc.vips, writeOpts)
			require.NoError(t, err)
			require.Equal(t, tc.expectFound, resp.ServiceFound)
			require.ElementsMatch(t, tc.expectUnassignedFrom, resp.UnassignedFrom)
		})
	}
}
