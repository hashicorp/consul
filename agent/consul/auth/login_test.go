// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestBuildTokenDescription(t *testing.T) {
	t.Parallel()

	t.Run("empty meta", func(t *testing.T) {
		desc, err := BuildTokenDescription("my-prefix", nil)
		require.NoError(t, err)
		require.Equal(t, "my-prefix", desc)

		desc, err = BuildTokenDescription("my-prefix", map[string]string{})
		require.NoError(t, err)
		require.Equal(t, "my-prefix", desc)
	})

	t.Run("with meta", func(t *testing.T) {
		meta := map[string]string{
			"user": "alice",
			"role": "admin",
		}
		desc, err := BuildTokenDescription("my-prefix", meta)
		require.NoError(t, err)

		// Maps are unordered, so checking exact JSON string match can be flaky.
		// Instead, we verify the prefix and that both keys/values are present in the JSON.
		require.True(t, strings.HasPrefix(desc, "my-prefix: {"))
		require.Contains(t, desc, `"user":"alice"`)
		require.Contains(t, desc, `"role":"admin"`)
		require.True(t, strings.HasSuffix(desc, "}"))
	})
	t.Run("empty prefix with meta", func(t *testing.T) {
		meta := map[string]string{"env": "prod"}
		desc, err := BuildTokenDescription("", meta)
		require.NoError(t, err)

		// It should format as ": {"env":"prod"}" based on the fmt.Sprintf("%s: %s") logic
		require.True(t, strings.HasPrefix(desc, ": {"))
		require.Contains(t, desc, `"env":"prod"`)
	})

	t.Run("meta with special characters", func(t *testing.T) {
		meta := map[string]string{
			"complex": "value \"with quotes\" \n and newlines",
			"unicode": "💥 symbols",
		}
		desc, err := BuildTokenDescription("prefix", meta)
		require.NoError(t, err)

		// Ensure the JSON marshal correctly escaped the quotes and newlines
		require.Contains(t, desc, `"value \"with quotes\" \n and newlines"`)
		require.Contains(t, desc, `"💥 symbols"`)
	})
}

func TestFormatTokenName(t *testing.T) {
	t.Parallel()

	t.Run("nil auth method", func(t *testing.T) {
		name, err := FormatTokenName(nil, nil)
		require.NoError(t, err)
		require.Empty(t, name)
	})

	t.Run("empty token name format", func(t *testing.T) {
		authMethod := &structs.ACLAuthMethod{
			Name: "test-method",
			Type: "testing",
		}
		name, err := FormatTokenName(authMethod, nil)
		require.NoError(t, err)
		require.Empty(t, name)
	})

	t.Run("valid format with standard fields", func(t *testing.T) {
		authMethod := &structs.ACLAuthMethod{
			Name:            "k8s-dev",
			Type:            "kubernetes",
			TokenNameFormat: "${auth_method_type}-${auth_method_name}",
		}

		name, err := FormatTokenName(authMethod, nil)
		require.NoError(t, err)

		// Expect the interpolated prefix followed by a random uint64 suffix
		require.True(t, strings.HasPrefix(name, "kubernetes-k8s-dev-"))
		require.NotEqual(t, "kubernetes-k8s-dev-", name) // Ensure suffix exists
	})

	t.Run("valid format with claims", func(t *testing.T) {
		authMethod := &structs.ACLAuthMethod{
			Name:            "jwt-prod",
			Type:            "jwt",
			TokenNameFormat: "${auth_method_name}-${value.app}-${value.env}",
		}
		claims := map[string]string{
			"app": "frontend",
			"env": "production",
		}

		name, err := FormatTokenName(authMethod, claims)
		require.NoError(t, err)

		require.True(t, strings.HasPrefix(name, "jwt-prod-frontend-production-"))
	})
	t.Run("static token name format (no interpolation)", func(t *testing.T) {
		authMethod := &structs.ACLAuthMethod{
			Name:            "static-method",
			Type:            "testing",
			TokenNameFormat: "my-hardcoded-prefix",
		}

		name, err := FormatTokenName(authMethod, nil)
		require.NoError(t, err)

		// Expect the literal string followed by a random uint64 suffix
		require.True(t, strings.HasPrefix(name, "my-hardcoded-prefix-"))
		require.NotEqual(t, "my-hardcoded-prefix-", name)
	})

	t.Run("missing claim variable in format", func(t *testing.T) {
		authMethod := &structs.ACLAuthMethod{
			Name: "jwt-test",
			Type: "jwt",
			// Format expects value.namespace, but we won't provide it
			TokenNameFormat: "${auth_method_name}-${value.namespace}",
		}

		// Passing nil claims means "value.namespace" won't exist in the valueMap
		name, err := FormatTokenName(authMethod, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown variable accessed: value.namespace")
		require.Empty(t, name)
	})

	t.Run("unknown root variable in format", func(t *testing.T) {
		authMethod := &structs.ACLAuthMethod{
			Name: "test",
			Type: "testing",
			// Format accesses a variable that FormatTokenName never populates
			TokenNameFormat: "${random_unsupported_var}",
		}

		name, err := FormatTokenName(authMethod, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown variable accessed: random_unsupported_var")
		require.Empty(t, name)
	})
}

func TestParseTokenName(t *testing.T) {
	t.Parallel()

	authMethod := &structs.ACLAuthMethod{
		TokenNameFormat: "${namespace}-${role}",
	}

	t.Run("successful interpolation without random suffix", func(t *testing.T) {
		values := map[string]string{
			"namespace": "default",
			"role":      "web",
		}

		name, err := ParseTokenName(authMethod, values, false)
		require.NoError(t, err)
		require.Equal(t, "default-web", name)
	})

	t.Run("successful interpolation with random suffix", func(t *testing.T) {
		values := map[string]string{
			"namespace": "default",
			"role":      "web",
		}

		name, err := ParseTokenName(authMethod, values, true)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(name, "default-web-"))

		// Suffix should be a numeric string
		parts := strings.Split(name, "-")
		require.Len(t, parts, 3)
		require.NotEmpty(t, parts[2])
	})

	t.Run("invalid HIL syntax", func(t *testing.T) {
		invalidAuthMethod := &structs.ACLAuthMethod{
			TokenNameFormat: "${namespace}-${something-wrong}",
		}
		values := map[string]string{
			"namespace": "default",
		}

		name, err := ParseTokenName(invalidAuthMethod, values, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to generate ACL token name")
		require.Empty(t, name)
	})
	t.Run("empty format string", func(t *testing.T) {
		emptyAuthMethod := &structs.ACLAuthMethod{
			TokenNameFormat: "",
		}
		values := map[string]string{"foo": "bar"}

		name, err := ParseTokenName(emptyAuthMethod, values, true)
		require.NoError(t, err)

		// An empty format string should interpolate to "",
		// but the suffixRandomInt logic appends "-[random]"
		require.True(t, strings.HasPrefix(name, "-"))
		require.NotEqual(t, "-", name)
	})

	t.Run("literal format without random suffix", func(t *testing.T) {
		literalAuthMethod := &structs.ACLAuthMethod{
			TokenNameFormat: "just-a-string",
		}

		name, err := ParseTokenName(literalAuthMethod, nil, false)
		require.NoError(t, err)
		require.Equal(t, "just-a-string", name)
	})

	t.Run("missing variable in value map", func(t *testing.T) {
		method := &structs.ACLAuthMethod{
			TokenNameFormat: "${missing_var}",
		}

		// Empty map, so "missing_var" cannot be resolved
		name, err := ParseTokenName(method, map[string]string{}, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to generate ACL token name")
		require.Contains(t, err.Error(), "unknown variable accessed: missing_var")
		require.Empty(t, name)
	})

	t.Run("malformed HIL expression (unclosed brace)", func(t *testing.T) {
		method := &structs.ACLAuthMethod{
			TokenNameFormat: "${unclosed_var",
		}

		name, err := ParseTokenName(method, map[string]string{"unclosed_var": "val"}, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to generate ACL token name")
		require.Empty(t, name)
	})
}
