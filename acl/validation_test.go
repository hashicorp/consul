// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ValidatePolicyName(t *testing.T) {
	for _, tc := range []struct {
		description string
		name        string
		valid       bool
	}{
		{
			description: "valid policy",
			name:        "this-is-valid",
			valid:       true,
		},
		{
			description: "empty policy",
			name:        "",
			valid:       false,
		},
		{
			description: "with slash",
			name:        "policy/with-slash",
			valid:       true,
		},
		{
			description: "leading slash",
			name:        "/no-leading-slash",
			valid:       false,
		},
		{
			description: "too many slashes",
			name:        "too/many/slashes",
			valid:       false,
		},
		{
			description: "no double-slash",
			name:        "no//double-slash",
			valid:       false,
		},
		{
			description: "builtin prefix",
			name:        "builtin/prefix-cannot-be-used",
			valid:       false,
		},
		{
			description: "long",
			name:        "this-policy-name-is-very-very-long-but-it-is-okay-because-it-is-the-max-length-that-we-allow-here-in-a-policy-name-which-is-good",
			valid:       true,
		},
		{
			description: "too long",
			name:        "this-is-a-policy-that-has-one-character-too-many-it-is-way-too-long-for-a-policy-we-do-not-want-a-policy-of-this-length-because-1",
			valid:       false,
		},
		{
			description: "invalid start character",
			name:        "!foo",
			valid:       false,
		},
		{
			description: "invalid character",
			name:        "this%is%bad",
			valid:       false,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.valid, ValidatePolicyName(tc.name) == nil)
		})
	}
}
