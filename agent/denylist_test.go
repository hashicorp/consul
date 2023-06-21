// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"testing"
)

func TestDenylist(t *testing.T) {
	t.Parallel()

	complex := []string{
		"/a",
		"/b/c",
	}

	tests := []struct {
		desc     string
		prefixes []string
		path     string
		block    bool
	}{
		{"nothing blocked root", nil, "/", false},
		{"nothing blocked path", nil, "/a", false},
		{"exact match 1", complex, "/a", true},
		{"exact match 2", complex, "/b/c", true},
		{"subpath", complex, "/a/b", true},
		{"longer prefix", complex, "/apple", true},
		{"longer subpath", complex, "/b/c/d", true},
		{"partial prefix", complex, "/b/d", false},
		{"no match", complex, "/c", false},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			denylist := NewDenylist(tt.prefixes)
			if got, want := denylist.Block(tt.path), tt.block; got != want {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
}
