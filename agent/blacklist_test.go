package agent

import (
	"testing"
)

func TestBlacklist(t *testing.T) {
	type pathCase struct {
		path string
		want bool
	}

	tests := []struct {
		desc     string
		prefixes []string
		paths    []pathCase
	}{
		{
			"nothing disallowed",
			nil,
			[]pathCase{
				{"/", false},
				{"/v1/agent/self", false},
			},
		},
		{
			"prefix match",
			[]string{
				"/v1/acl",
				"/v1/agent/self",
			},
			[]pathCase{
				{"/", false},
				{"/v1/acl/foo", true},
				{"/v1/acl/bar", true},
				{"/v1/agent/self", true},
				{"/v1/agent/selfish", true},
				{"/v1/agent/self/sub", true},
				{"/v1/agent/other", false},
			},
		},
	}
	for _, tt := range tests {
		blacklist := NewBlacklist(tt.prefixes)
		for _, p := range tt.paths {
			if got := blacklist.IsDisallowed(p.path); got != p.want {
				t.Fatalf("case %q: %q got %v want %v",
					tt.desc, p.path, got, p.want)
			}
		}
	}
}
