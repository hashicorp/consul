// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package oidcauth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidRedirect(t *testing.T) {
	tests := []struct {
		uri      string
		allowed  []string
		expected bool
	}{
		// valid
		{"https://example.com", []string{"https://example.com"}, true},
		{"https://example.com:5000", []string{"a", "b", "https://example.com:5000"}, true},
		{"https://example.com/a/b/c", []string{"a", "b", "https://example.com/a/b/c"}, true},
		{"https://localhost:9000", []string{"a", "b", "https://localhost:5000"}, true},
		{"https://127.0.0.1:9000", []string{"a", "b", "https://127.0.0.1:5000"}, true},
		{"https://[::1]:9000", []string{"a", "b", "https://[::1]:5000"}, true},
		{"https://[::1]:9000/x/y?r=42", []string{"a", "b", "https://[::1]:5000/x/y?r=42"}, true},

		// invalid
		{"https://example.com", []string{}, false},
		{"http://example.com", []string{"a", "b", "https://example.com"}, false},
		{"https://example.com:9000", []string{"a", "b", "https://example.com:5000"}, false},
		{"https://[::2]:9000", []string{"a", "b", "https://[::2]:5000"}, false},
		{"https://localhost:5000", []string{"a", "b", "https://127.0.0.1:5000"}, false},
		{"https://localhost:5000", []string{"a", "b", "https://127.0.0.1:5000"}, false},
		{"https://localhost:5000", []string{"a", "b", "http://localhost:5000"}, false},
		{"https://[::1]:5000/x/y?r=42", []string{"a", "b", "https://[::1]:5000/x/y?r=43"}, false},

		// extra invalid
		{"%%%%%%%%%%%", []string{}, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("uri=%q allowed=%#v", tc.uri, tc.allowed), func(t *testing.T) {
			require.Equal(t, tc.expected, validRedirect(tc.uri, tc.allowed))
		})
	}
}
