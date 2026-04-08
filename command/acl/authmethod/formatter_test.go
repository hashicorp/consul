// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authmethod

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestGetSupportedFormats(t *testing.T) {
	t.Parallel()

	formats := GetSupportedFormats()
	require.Len(t, formats, 2)
	require.Contains(t, formats, PrettyFormat)
	require.Contains(t, formats, JSONFormat)
}

func TestNewFormatter(t *testing.T) {
	t.Parallel()

	t.Run("pretty format", func(t *testing.T) {
		formatter, err := NewFormatter(PrettyFormat, false)
		require.NoError(t, err)
		require.IsType(t, &prettyFormatter{}, formatter)
		require.False(t, formatter.(*prettyFormatter).showMeta)
	})

	t.Run("json format with meta", func(t *testing.T) {
		formatter, err := NewFormatter(JSONFormat, true)
		require.NoError(t, err)
		require.IsType(t, &jsonFormatter{}, formatter)
		require.True(t, formatter.(*jsonFormatter).showMeta)
	})

	t.Run("unknown format", func(t *testing.T) {
		formatter, err := NewFormatter("xml", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Unknown format: xml")
		require.Nil(t, formatter)
	})
}

func TestPrettyFormatter_FormatAuthMethod(t *testing.T) {
	t.Parallel()

	t.Run("minimal fields", func(t *testing.T) {
		formatter := newPrettyFormatter(false)
		method := &api.ACLAuthMethod{
			Name:        "min-method",
			Type:        "jwt",
			Description: "Minimal auth method",
			Config:      map[string]interface{}{"key": "value"},
		}

		out, err := formatter.FormatAuthMethod(method)
		require.NoError(t, err)

		expectedLines := []string{
			"Name:          min-method",
			"Type:          jwt",
			"Description:   Minimal auth method",
			"Config:",
			"{",
			`  "key": "value"`,
			"}",
		}
		for _, line := range expectedLines {
			require.Contains(t, out, line)
		}

		// Ensure optional fields are omitted
		require.NotContains(t, out, "Partition:")
		require.NotContains(t, out, "Namespace:")
		require.NotContains(t, out, "TokenNameFormat:")
		require.NotContains(t, out, "Create Index:")
	})

	t.Run("all fields with meta", func(t *testing.T) {
		formatter := newPrettyFormatter(true)
		method := &api.ACLAuthMethod{
			Name:            "full-method",
			Type:            "kubernetes",
			Partition:       "my-partition",
			Namespace:       "my-namespace",
			DisplayName:     "K8s Auth",
			Description:     "Full auth method",
			MaxTokenTTL:     10 * time.Minute,
			TokenLocality:   "global",
			TokenNameFormat: "${auth_method_type}-{foo}",
			NamespaceRules: []*api.ACLAuthMethodNamespaceRule{
				{
					Selector:      "service=web",
					BindNamespace: "web-ns",
				},
			},
			CreateIndex: 100,
			ModifyIndex: 200,
			Config:      map[string]interface{}{"url": "https://k8s.io"},
		}

		out, err := formatter.FormatAuthMethod(method)
		require.NoError(t, err)

		expectedLines := []string{
			"Name:          full-method",
			"Type:          kubernetes",
			"Partition:     my-partition",
			"Namespace:     my-namespace",
			"DisplayName:   K8s Auth",
			"Description:   Full auth method",
			"MaxTokenTTL:   10m0s",
			"TokenLocality: global",
			"TokenNameFormat: ${auth_method_type}-{foo}",
			"NamespaceRules:",
			"   Selector:      service=web",
			"   BindNamespace: web-ns",
			"Create Index:  100",
			"Modify Index:  200",
			"Config:",
			`  "url": "https://k8s.io"`,
		}
		for _, line := range expectedLines {
			require.Contains(t, out, line)
		}
	})

	t.Run("config formatting error", func(t *testing.T) {
		formatter := newPrettyFormatter(false)

		// Create a map with a channel to force json.MarshalIndent to fail
		unsupportedMap := map[string]interface{}{
			"bad_field": make(chan int),
		}

		method := &api.ACLAuthMethod{
			Name:   "bad-config",
			Config: unsupportedMap,
		}

		out, err := formatter.FormatAuthMethod(method)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Error formatting auth method configuration")
		require.Empty(t, out)
	})
}

func TestPrettyFormatter_FormatAuthMethodList(t *testing.T) {
	t.Parallel()

	t.Run("list without meta", func(t *testing.T) {
		formatter := newPrettyFormatter(false)
		methods := []*api.ACLAuthMethodListEntry{
			{
				Name:        "method-1",
				Type:        "jwt",
				Description: "First method",
			},
			{
				Name:        "method-2",
				Type:        "kubernetes",
				Partition:   "default",
				Namespace:   "default",
				DisplayName: "K8s",
				Description: "Second method",
			},
		}

		out, err := formatter.FormatAuthMethodList(methods)
		require.NoError(t, err)

		require.Contains(t, out, "method-1:\n")
		require.Contains(t, out, "   Type:         jwt\n")
		require.Contains(t, out, "   Description:  First method\n")

		require.Contains(t, out, "method-2:\n")
		require.Contains(t, out, "   Type:         kubernetes\n")
		require.Contains(t, out, "   Partition:    default\n")
		require.Contains(t, out, "   Namespace:    default\n")
		require.Contains(t, out, "   DisplayName:  K8s\n")

		// Ensure meta is not shown
		require.NotContains(t, out, "Create Index:")
	})

	t.Run("list with meta", func(t *testing.T) {
		formatter := newPrettyFormatter(true)
		methods := []*api.ACLAuthMethodListEntry{
			{
				Name:        "method-meta",
				Type:        "jwt",
				Description: "Method with meta",
				CreateIndex: 42,
				ModifyIndex: 84,
			},
		}

		out, err := formatter.FormatAuthMethodList(methods)
		require.NoError(t, err)

		require.Contains(t, out, "   Create Index: 42\n")
		require.Contains(t, out, "   Modify Index: 84\n")
	})
}

func TestJSONFormatter_FormatAuthMethod(t *testing.T) {
	t.Parallel()

	formatter := newJSONFormatter(false) // JSON formatter ignores showMeta internally, it just marshals the whole struct

	method := &api.ACLAuthMethod{
		Name:            "json-method",
		Type:            "jwt",
		TokenNameFormat: "${auth_method_type}-test",
		Config:          map[string]interface{}{"key": "value"},
	}

	out, err := formatter.FormatAuthMethod(method)
	require.NoError(t, err)

	// Validate it's proper JSON
	var parsed api.ACLAuthMethod
	err = json.Unmarshal([]byte(out), &parsed)
	require.NoError(t, err)

	require.Equal(t, method.Name, parsed.Name)
	require.Equal(t, method.Type, parsed.Type)
	require.Equal(t, method.TokenNameFormat, parsed.TokenNameFormat)
	require.Equal(t, "value", parsed.Config["key"])

	// Verify it used indentation
	require.True(t, strings.Contains(out, "    \"Name\": \"json-method\""))
}

func TestJSONFormatter_FormatAuthMethodList(t *testing.T) {
	t.Parallel()

	formatter := newJSONFormatter(false)

	methods := []*api.ACLAuthMethodListEntry{
		{
			Name: "list-1",
			Type: "jwt",
		},
		{
			Name: "list-2",
			Type: "kubernetes",
		},
	}

	out, err := formatter.FormatAuthMethodList(methods)
	require.NoError(t, err)

	var parsed []*api.ACLAuthMethodListEntry
	err = json.Unmarshal([]byte(out), &parsed)
	require.NoError(t, err)

	require.Len(t, parsed, 2)
	require.Equal(t, "list-1", parsed[0].Name)
	require.Equal(t, "list-2", parsed[1].Name)
}