//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

var enterpriseMetaField = "EnterpriseMeta"

func TestServiceID_String(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		sid := NewServiceID("the-id", &acl.EnterpriseMeta{})
		require.Equal(t, "the-id", fmt.Sprintf("%v", sid))
	})
	t.Run("pointer", func(t *testing.T) {
		sid := NewServiceID("the-id", &acl.EnterpriseMeta{})
		require.Equal(t, "the-id", fmt.Sprintf("%v", &sid))
	})
}

func TestCheckID_String(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		cid := NewCheckID("the-id", &acl.EnterpriseMeta{})
		require.Equal(t, "the-id", fmt.Sprintf("%v", cid))
	})
	t.Run("pointer", func(t *testing.T) {
		cid := NewCheckID("the-id", &acl.EnterpriseMeta{})
		require.Equal(t, "the-id", fmt.Sprintf("%v", &cid))
	})
}

func TestServiceName_String(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		sn := NewServiceName("the-id", &acl.EnterpriseMeta{})
		require.Equal(t, "the-id", fmt.Sprintf("%v", sn))
	})
	t.Run("pointer", func(t *testing.T) {
		sn := NewServiceName("the-id", &acl.EnterpriseMeta{})
		require.Equal(t, "the-id", fmt.Sprintf("%v", &sn))
	})
}

func TestIntention_HasWildcardSource(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		ixn := Intention{
			SourceName: WildcardSpecifier,
		}
		require.True(t, ixn.HasWildcardSource())
	})

	t.Run("false", func(t *testing.T) {
		ixn := Intention{
			SourceName: "web",
		}
		require.False(t, ixn.HasWildcardSource())
	})
}

func TestIntention_HasWildcardDestination(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		ixn := Intention{
			DestinationName: WildcardSpecifier,
		}
		require.True(t, ixn.HasWildcardDestination())
	})

	t.Run("false", func(t *testing.T) {
		ixn := Intention{
			DestinationName: "web",
		}
		require.False(t, ixn.HasWildcardDestination())
	})
}
