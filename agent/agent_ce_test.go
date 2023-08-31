// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgent_consulConfig_Reporting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	hcl := `
		reporting {
			license {
				enabled = true
			}
		}
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	require.Equal(t, false, a.consulConfig().Reporting.License.Enabled)
}

func TestAgent_consulConfig_Reporting_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	hcl := `
		reporting {
		}
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	require.Equal(t, false, a.consulConfig().Reporting.License.Enabled)
}
