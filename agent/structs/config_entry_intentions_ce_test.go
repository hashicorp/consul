// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestEnterprise_ServiceIntentionsConfigEntry(t *testing.T) {
	type testcase struct {
		entry        *ServiceIntentionsConfigEntry
		legacy       bool
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceIntentionsConfigEntry)
	}

	cases := map[string]testcase{
		"No sameness groups": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:          "foo",
						SamenessGroup: "blah",
						Action:        IntentionActionAllow,
					},
				},
			},
			validateErr: `Sources[0].SamenessGroup: Sameness groups are a Consul Enterprise feature.`,
		},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var err error
			if tc.legacy {
				err = tc.entry.LegacyNormalize()
			} else {
				err = tc.entry.Normalize()
			}
			if tc.normalizeErr != "" {
				testutil.RequireErrorContains(t, err, tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			if tc.legacy {
				err = tc.entry.LegacyValidate()
			} else {
				err = tc.entry.Validate()
			}
			if tc.validateErr != "" {
				testutil.RequireErrorContains(t, err, tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
