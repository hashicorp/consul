// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-version"
)

func TestRegistry_DefineDefaultsAndSorts(t *testing.T) {
	r := newRegistry()
	r.define(Definition{
		Name:        "z-feature",
		MinVersion:  version.Must(version.NewVersion("2.1.0")),
		Description: "z",
		Owner:       "test",
	})
	r.define(Definition{
		Name:        "a-feature",
		MinVersion:  version.Must(version.NewVersion("2.1.0")),
		Description: "a",
		Owner:       "test",
	})

	definitions := r.Definitions()
	require.Equal(t, []string{"a-feature", "z-feature"}, []string{definitions[0].Name, definitions[1].Name})
	require.Equal(t, StageExperimental, definitions[0].Stage)
	require.False(t, definitions[0].DefaultEnabled)
	require.Equal(t, BeforeMinimumVersionDisabled, definitions[0].BeforeMinimumVersion)
}

func TestRegistry_DefineRejectsInvalidDefinitions(t *testing.T) {
	valid := Definition{
		Name:        "valid-feature",
		MinVersion:  version.Must(version.NewVersion("2.1.0")),
		Description: "valid",
		Owner:       "test",
	}

	tests := map[string]func(Definition) Definition{
		"invalid name":        func(d Definition) Definition { d.Name = "Invalid_Name"; return d },
		"invalid stage":       func(d Definition) Definition { d.Stage = "beta"; return d },
		"missing version":     func(d Definition) Definition { d.MinVersion = nil; return d },
		"invalid fallback":    func(d Definition) Definition { d.BeforeMinimumVersion = "enabled"; return d },
		"missing description": func(d Definition) Definition { d.Description = ""; return d },
		"missing owner":       func(d Definition) Definition { d.Owner = ""; return d },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			r := newRegistry()
			require.Panics(t, func() { r.define(mutate(valid)) })
		})
	}
}

func TestRegistry_DefineRejectsDuplicate(t *testing.T) {
	r := newRegistry()
	definition := Definition{
		Name:        "duplicate-feature",
		MinVersion:  version.Must(version.NewVersion("2.1.0")),
		Description: "duplicate",
		Owner:       "test",
	}
	r.define(definition)
	require.Panics(t, func() { r.define(definition) })
}

func TestDefinitionsAndDigest(t *testing.T) {
	registry := DefaultRegistry()
	definitions := registry.Definitions()
	require.NotEmpty(t, definitions)
	require.Equal(t, "api-gateway-upstream-routing", definitions[0].Name)
	require.Len(t, registry.Digest(), 64)

	definition, ok := registry.DefinitionForName(APIGatewayUpstreamRouting.String())
	require.True(t, ok)
	require.Equal(t, definitions[0], definition)
}
