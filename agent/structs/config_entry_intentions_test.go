package structs

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateUUID() (ret string) {
	var err error
	if ret, err = uuid.GenerateUUID(); err != nil {
		panic(fmt.Sprintf("Unable to generate a UUID, %v", err))
	}
	return ret
}

func TestServiceIntentionsConfigEntry(t *testing.T) {
	type testcase struct {
		entry        *ServiceIntentionsConfigEntry
		legacy       bool
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceIntentionsConfigEntry)
	}

	legacyIDs := []string{
		generateUUID(),
		generateUUID(),
		generateUUID(),
	}

	defaultMeta := DefaultEnterpriseMeta()

	fooName := NewServiceName("foo", defaultMeta)

	cases := map[string]testcase{
		"nil": {
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		"no name": {
			entry:       &ServiceIntentionsConfigEntry{},
			validateErr: "Name is required",
		},
		"dest name has partial wildcard": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test*",
			},
			validateErr: "Name: wildcard character '*' cannot be used with partial values",
		},
		"empty": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
			},
			validateErr: "At least one source is required",
		},
		"source specified twice": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:   "foo",
						Action: IntentionActionAllow,
					},
					{
						Name:   "foo",
						Action: IntentionActionDeny,
					},
				},
			},
			validateErr: `Sources[1] defines "` + fooName.String() + `" more than once`,
		},
		"no source name": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Action: IntentionActionAllow,
					},
				},
			},
			validateErr: `Sources[0].Name is required`,
		},
		"source name has partial wildcard": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:   "foo*",
						Action: IntentionActionAllow,
					},
				},
			},
			validateErr: `Sources[0].Name: wildcard character '*' cannot be used with partial values`,
		},
		"description too long": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:        "foo",
						Action:      IntentionActionAllow,
						Description: strings.Repeat("x", 513),
					},
				},
			},
			validateErr: `Sources[0].Description exceeds maximum length 512`,
		},
		"config entry meta not allowed on legacy writes": {
			legacy: true,
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID: legacyIDs[0],
						Name:     "foo",
						Action:   IntentionActionAllow,
					},
				},
				Meta: map[string]string{
					"key1": "val1",
				},
			},
			validateErr: `Meta must be omitted for legacy intention writes`,
		},
		"config entry meta too many keys": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:   "foo",
						Action: IntentionActionAllow,
					},
				},
				Meta: makeStringMap(65, 5, 5),
			},
			validateErr: `Meta exceeds maximum element count 64`,
		},
		"config entry meta key too large": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:   "foo",
						Action: IntentionActionAllow,
					},
				},
				Meta: makeStringMap(64, 129, 5),
			},
			validateErr: `exceeds maximum length 128`,
		},
		"config entry meta value too large": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:   "foo",
						Action: IntentionActionAllow,
					},
				},
				Meta: makeStringMap(64, 128, 513),
			},
			validateErr: `exceeds maximum length 512`,
		},
		"config entry meta value just big enough": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:   "foo",
						Action: IntentionActionAllow,
					},
				},
				Meta: makeStringMap(64, 128, 512),
			},
		},
		"legacy meta not allowed": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID:    legacyIDs[0],
						Name:        "foo",
						Action:      IntentionActionAllow,
						Description: strings.Repeat("x", 512),
						LegacyMeta: map[string]string{ // stray Meta will be dropped
							"old": "data",
						},
					},
				},
			},
			validateErr: "Sources[0].LegacyMeta must be omitted",
		},
		"legacy meta too many keys": {
			legacy: true,
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID:   legacyIDs[0],
						Name:       "foo",
						Action:     IntentionActionAllow,
						LegacyMeta: makeStringMap(65, 5, 5),
					},
				},
			},
			validateErr: `Sources[0].Meta exceeds maximum element count 64`,
		},
		"legacy meta key too large": {
			legacy: true,
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID:   legacyIDs[0],
						Name:       "foo",
						Action:     IntentionActionAllow,
						LegacyMeta: makeStringMap(64, 129, 5),
					},
				},
			},
			validateErr: `exceeds maximum length 128`,
		},
		"legacy meta value too large": {
			legacy: true,
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID:   legacyIDs[0],
						Name:       "foo",
						Action:     IntentionActionAllow,
						LegacyMeta: makeStringMap(64, 128, 513),
					},
				},
			},
			validateErr: `exceeds maximum length 512`,
		},
		"legacy meta value just big enough": {
			legacy: true,
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID:   legacyIDs[0],
						Name:       "foo",
						Action:     IntentionActionAllow,
						LegacyMeta: makeStringMap(64, 128, 512),
					},
				},
			},
		},
		"legacy ID is required in legacy mode": {
			legacy: true,
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:        "foo",
						Action:      IntentionActionAllow,
						Description: strings.Repeat("x", 512),
					},
				},
			},
			validateErr: "Sources[0].LegacyID must be set",
		},
		"action required for L4": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:        "foo",
						Description: strings.Repeat("x", 512),
					},
				},
			},
			validateErr: `Sources[0].Action must be set to 'allow' or 'deny'`,
		},
		"action must be allow or deny for L4": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:        "foo",
						Action:      "blah",
						Description: strings.Repeat("x", 512),
					},
				},
			},
			validateErr: `Sources[0].Action must be set to 'allow' or 'deny'`,
		},
		"L4 normalize": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID: legacyIDs[0], // stray ID will be dropped
						Name:     WildcardSpecifier,
						Action:   IntentionActionDeny,
					},
					{
						Name:   "foo",
						Action: IntentionActionAllow,
					},
					{
						Name:   "bar",
						Action: IntentionActionDeny,
					},
				},
				Meta: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
			},
			check: func(t *testing.T, entry *ServiceIntentionsConfigEntry) {
				// Note the stable precedence sort has been applied here.
				assert.Equal(t, []*SourceIntention{
					{
						Name:           "foo",
						EnterpriseMeta: *defaultMeta,
						Action:         IntentionActionAllow,
						Precedence:     9,
						Type:           IntentionSourceConsul,
					},
					{
						Name:           "bar",
						EnterpriseMeta: *defaultMeta,
						Action:         IntentionActionDeny,
						Precedence:     9,
						Type:           IntentionSourceConsul,
					},
					{
						Name:           WildcardSpecifier,
						EnterpriseMeta: *defaultMeta,
						Action:         IntentionActionDeny,
						Precedence:     8,
						Type:           IntentionSourceConsul,
					},
				}, entry.Sources)
				assert.Equal(t, map[string]string{
					"key1": "val1",
					"key2": "val2",
				}, entry.Meta)
			},
		},
		"L4 legacy normalize": {
			legacy: true,
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						Name:     WildcardSpecifier,
						Action:   IntentionActionDeny,
						LegacyID: legacyIDs[0],
					},
					{
						Name:     "foo",
						Action:   IntentionActionAllow,
						LegacyID: legacyIDs[1],
						LegacyMeta: map[string]string{
							"key1": "val1",
							"key2": "val2",
						},
					},
					{
						Name:     "bar",
						Action:   IntentionActionDeny,
						LegacyID: legacyIDs[2],
					},
				},
			},
			check: func(t *testing.T, entry *ServiceIntentionsConfigEntry) {
				require.Len(t, entry.Sources, 3)

				assert.False(t, entry.Sources[0].LegacyCreateTime.IsZero())
				assert.False(t, entry.Sources[0].LegacyUpdateTime.IsZero())
				assert.False(t, entry.Sources[1].LegacyCreateTime.IsZero())
				assert.False(t, entry.Sources[1].LegacyUpdateTime.IsZero())
				assert.False(t, entry.Sources[2].LegacyCreateTime.IsZero())
				assert.False(t, entry.Sources[2].LegacyUpdateTime.IsZero())

				assert.Equal(t, []*SourceIntention{
					{
						LegacyID:       legacyIDs[1],
						Name:           "foo",
						EnterpriseMeta: *defaultMeta,
						Action:         IntentionActionAllow,
						Precedence:     9,
						Type:           IntentionSourceConsul,
						LegacyMeta: map[string]string{
							"key1": "val1",
							"key2": "val2",
						},
						LegacyCreateTime: entry.Sources[0].LegacyCreateTime,
						LegacyUpdateTime: entry.Sources[0].LegacyUpdateTime,
					},
					{
						LegacyID:         legacyIDs[2],
						Name:             "bar",
						EnterpriseMeta:   *defaultMeta,
						Action:           IntentionActionDeny,
						Precedence:       9,
						Type:             IntentionSourceConsul,
						LegacyMeta:       map[string]string{},
						LegacyCreateTime: entry.Sources[1].LegacyCreateTime,
						LegacyUpdateTime: entry.Sources[1].LegacyUpdateTime,
					},
					{
						LegacyID:         legacyIDs[0],
						Name:             WildcardSpecifier,
						EnterpriseMeta:   *defaultMeta,
						Action:           IntentionActionDeny,
						Precedence:       8,
						Type:             IntentionSourceConsul,
						LegacyMeta:       map[string]string{},
						LegacyCreateTime: entry.Sources[2].LegacyCreateTime,
						LegacyUpdateTime: entry.Sources[2].LegacyUpdateTime,
					},
				}, entry.Sources)
			},
		},
		"L4 validate": {
			entry: &ServiceIntentionsConfigEntry{
				Kind: ServiceIntentions,
				Name: "test",
				Sources: []*SourceIntention{
					{
						LegacyID: legacyIDs[0], // stray ID will be dropped
						Name:     WildcardSpecifier,
						Action:   IntentionActionDeny,
					},
					{
						Name:   "foo",
						Action: IntentionActionAllow,
					},
					{
						Name:   "bar",
						Action: IntentionActionDeny,
					},
				},
				Meta: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
			},
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
				// require.Error(t, err)
				// require.Contains(t, err.Error(), tc.normalizeErr)
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
				// require.Error(t, err)
				// require.Contains(t, err.Error(), tc.validateErr)
				testutil.RequireErrorContains(t, err, tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func makeStringMap(keys, keySize, valSize int) map[string]string {
	m := make(map[string]string)
	for i := 0; i < keys; i++ {
		base := fmt.Sprintf("%d:", i)
		if len(base) > keySize || len(base) > valSize {
			panic("makeStringMap called with incompatible inputs")
		}
		// this is not performant
		if keySize > valSize {
			base = strings.Repeat(base, keySize)
		} else {
			base = strings.Repeat(base, valSize)
		}

		m[base[0:keySize]] = base[0:valSize]
	}
	return m
}

func TestMigrateIntentions(t *testing.T) {
	type testcase struct {
		in     Intentions
		expect []*ServiceIntentionsConfigEntry
	}

	legacyIDs := []string{
		generateUUID(),
		generateUUID(),
		generateUUID(),
	}

	anyTime := time.Now().UTC()

	cases := map[string]testcase{
		"nil": {},
		"one": {
			in: Intentions{
				{
					ID:              legacyIDs[0],
					Description:     "desc",
					SourceName:      "foo",
					DestinationName: "bar",
					SourceType:      IntentionSourceConsul,
					Action:          IntentionActionAllow,
					Meta: map[string]string{
						"key1": "val1",
					},
					Precedence: 9,
					CreatedAt:  anyTime,
					UpdatedAt:  anyTime,
				},
			},
			expect: []*ServiceIntentionsConfigEntry{
				{
					Kind: ServiceIntentions,
					Name: "bar",
					Sources: []*SourceIntention{
						{
							LegacyID:    legacyIDs[0],
							Description: "desc",
							Name:        "foo",
							Type:        IntentionSourceConsul,
							Action:      IntentionActionAllow,
							LegacyMeta: map[string]string{
								"key1": "val1",
							},
						},
					},
				},
			},
		},
		"two in same": {
			in: Intentions{
				{
					ID:              legacyIDs[0],
					Description:     "desc",
					SourceName:      "foo",
					DestinationName: "bar",
					SourceType:      IntentionSourceConsul,
					Action:          IntentionActionAllow,
					Meta: map[string]string{
						"key1": "val1",
					},
					Precedence: 9,
					CreatedAt:  anyTime,
					UpdatedAt:  anyTime,
				},
				{
					ID:              legacyIDs[1],
					Description:     "desc2",
					SourceName:      "*",
					DestinationName: "bar",
					SourceType:      IntentionSourceConsul,
					Action:          IntentionActionDeny,
					Meta: map[string]string{
						"key2": "val2",
					},
					Precedence: 9,
					CreatedAt:  anyTime,
					UpdatedAt:  anyTime,
				},
			},
			expect: []*ServiceIntentionsConfigEntry{
				{
					Kind: ServiceIntentions,
					Name: "bar",
					Sources: []*SourceIntention{
						{
							LegacyID:    legacyIDs[0],
							Description: "desc",
							Name:        "foo",
							Type:        IntentionSourceConsul,
							Action:      IntentionActionAllow,
							LegacyMeta: map[string]string{
								"key1": "val1",
							},
						},
						{
							LegacyID:    legacyIDs[1],
							Description: "desc2",
							Name:        "*",
							Type:        IntentionSourceConsul,
							Action:      IntentionActionDeny,
							LegacyMeta: map[string]string{
								"key2": "val2",
							},
						},
					},
				},
			},
		},
		"two in different": {
			in: Intentions{
				{
					ID:              legacyIDs[0],
					Description:     "desc",
					SourceName:      "foo",
					DestinationName: "bar",
					SourceType:      IntentionSourceConsul,
					Action:          IntentionActionAllow,
					Meta: map[string]string{
						"key1": "val1",
					},
					Precedence: 9,
					CreatedAt:  anyTime,
					UpdatedAt:  anyTime,
				},
				{
					ID:              legacyIDs[1],
					Description:     "desc2",
					SourceName:      "*",
					DestinationName: "bar2",
					SourceType:      IntentionSourceConsul,
					Action:          IntentionActionDeny,
					Meta: map[string]string{
						"key2": "val2",
					},
					Precedence: 9,
					CreatedAt:  anyTime,
					UpdatedAt:  anyTime,
				},
			},
			expect: []*ServiceIntentionsConfigEntry{
				{
					Kind: ServiceIntentions,
					Name: "bar",
					Sources: []*SourceIntention{
						{
							LegacyID:    legacyIDs[0],
							Description: "desc",
							Name:        "foo",
							Type:        IntentionSourceConsul,
							Action:      IntentionActionAllow,
							LegacyMeta: map[string]string{
								"key1": "val1",
							},
						},
					},
				},
				{
					Kind: ServiceIntentions,
					Name: "bar2",
					Sources: []*SourceIntention{
						{
							LegacyID:    legacyIDs[1],
							Description: "desc2",
							Name:        "*",
							Type:        IntentionSourceConsul,
							Action:      IntentionActionDeny,
							LegacyMeta: map[string]string{
								"key2": "val2",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got := MigrateIntentions(tc.in)
			require.ElementsMatch(t, tc.expect, got)
		})
	}
}
