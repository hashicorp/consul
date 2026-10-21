// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsEnvoyEscapeHatchKey(t *testing.T) {
	cases := map[string]bool{
		"envoy_listener_json": true,
		"envoy_cluster_json":  true,
		"ENVOY_LISTENER_JSON": true,
		"EnvoyListenerJSON":   true,
		"ENVOYLISTENERJSON":   true,
		"EnvoyClusterJSON":    true,
		"ENVOYCLUSTERJSON":    true,
		// U+017F (long s) simple-folds to "s", which mapstructure's EqualFold matching honors.
		"envoy_liſtener_json":  true,
		"EnvoyLiſtenerJSON":    true,
		"EnvoyCluſterJSON":     true,
		"envoy_cluſter_json":   true,
		"envoy_listener_jsonx": false,
		"Envoy_ListenerJSON":   false,
		"unrelated":            false,
	}
	for key, want := range cases {
		t.Run(key, func(t *testing.T) {
			require.Equal(t, want, IsEnvoyEscapeHatchKey(key))
		})
	}
}
