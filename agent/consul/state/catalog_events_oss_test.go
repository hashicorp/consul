//go:build !consulent
// +build !consulent

package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestEventPayloadCheckServiceNode_Subject_OSS(t *testing.T) {
	for desc, tc := range map[string]struct {
		evt EventPayloadCheckServiceNode
		sub string
	}{
		"mixed casing": {
			EventPayloadCheckServiceNode{
				Value: &structs.CheckServiceNode{
					Service: &structs.NodeService{
						Service: "FoO",
					},
				},
			},
			"foo",
		},
		"override key": {
			EventPayloadCheckServiceNode{
				Value: &structs.CheckServiceNode{
					Service: &structs.NodeService{
						Service: "foo",
					},
				},
				overrideKey: "bar",
			},
			"bar",
		},
	} {
		t.Run(desc, func(t *testing.T) {
			require.Equal(t, tc.sub, tc.evt.Subject().String())
		})
	}
}
