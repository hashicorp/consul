package authv2beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrafficPermissions_PortsOnly(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var dr *DestinationRule
		require.False(t, dr.PortsOnly())
	})
	t.Run("empty", func(t *testing.T) {
		dr := &DestinationRule{}
		require.False(t, dr.PortsOnly())
	})
	t.Run("ports", func(t *testing.T) {
		dr := &DestinationRule{
			PortNames: []string{"foo"},
		}
		require.True(t, dr.PortsOnly())
	})
	t.Run("excl-ports", func(t *testing.T) {
		dr := &DestinationRule{
			Exclude: []*ExcludePermissionRule{{PortNames: []string{"foo"}}},
		}
		require.True(t, dr.PortsOnly())
	})
	t.Run("ports-and-excl-ports", func(t *testing.T) {
		dr := &DestinationRule{
			PortNames: []string{"foo", "bar"},
			Exclude:   []*ExcludePermissionRule{{PortNames: []string{"foo"}}},
		}
		require.True(t, dr.PortsOnly())
	})
	t.Run("methods", func(t *testing.T) {
		dr := &DestinationRule{
			Methods: []string{"put"},
		}
		require.False(t, dr.PortsOnly())
	})
	t.Run("path", func(t *testing.T) {
		dr := &DestinationRule{
			PathRegex: "*",
			PortNames: []string{"foo"},
		}
		require.False(t, dr.PortsOnly())
	})
	t.Run("headers", func(t *testing.T) {
		dr := &DestinationRule{
			Exclude: []*ExcludePermissionRule{{Headers: []*DestinationRuleHeader{{Name: "Authorization"}}, PortNames: []string{"foo"}}},
		}
		require.False(t, dr.PortsOnly())
	})
	t.Run("path-and-exclports", func(t *testing.T) {
		dr := &DestinationRule{
			PathExact: "/",
			Exclude:   []*ExcludePermissionRule{{PortNames: []string{"foo"}}},
		}
		require.False(t, dr.PortsOnly())
	})
}
