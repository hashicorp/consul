//go:build !consulent
// +build !consulent

package gateways

import (
	"testing"

	"github.com/hashicorp/consul/api"
)

func getOrCreateNamespace(_ *testing.T, _ *api.Client) string {
	return ""
}
