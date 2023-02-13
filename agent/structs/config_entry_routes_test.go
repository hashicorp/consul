package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTCPRoute(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"multiple services": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-one",
				Services: []TCPService{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
			validateErr: "tcp-route currently only supports one service",
		},
		"normalize parent kind": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Services: []TCPService{{
					Name: "foo",
				}},
			},
			normalizeOnly: true,
			check: func(t *testing.T, entry ConfigEntry) {
				expectedParent := ResourceReference{
					Kind: APIGateway,
					Name: "gateway",
				}
				route := entry.(*TCPRouteConfigEntry)
				require.Len(t, route.Parents, 1)
				require.Equal(t, expectedParent, route.Parents[0])
			},
		},
		"invalid parent kind": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Kind: "route",
					Name: "gateway",
				}},
			},
			validateErr: "unsupported parent kind",
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}
