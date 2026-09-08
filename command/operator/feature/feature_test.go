// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package feature

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func TestFormat(t *testing.T) {
	features := []api.FeatureGate{{
		Name:             "test-feature",
		Stage:            "experimental",
		DesiredEnabled:   true,
		EffectiveEnabled: false,
		Eligible:         false,
		Reason:           "below-minimum-version",
		MinVersion:       "2.1.0",
		PolicyIndex:      10,
		StatusIndex:      11,
	}}

	pretty, err := Format(features, PrettyFormat)
	require.NoError(t, err)
	require.Contains(t, pretty, "test-feature")
	require.Contains(t, pretty, "below-minimum-version")

	jsonOutput, err := Format(features, JSONFormat)
	require.NoError(t, err)
	require.JSONEq(t, `[{"Name":"test-feature","Stage":"experimental","MinVersion":"2.1.0","DefaultEnabled":false,"BeforeMinimumVersion":"","Description":"","Owner":"","DesiredEnabled":true,"EffectiveEnabled":false,"Eligible":false,"Source":"","Reason":"below-minimum-version","PolicyIndex":10,"StatusIndex":11}]`, jsonOutput)

	_, err = Format(features, "yaml")
	require.Error(t, err)
}
