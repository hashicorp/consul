package token

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

// golden reads and optionally writes the expected data to the golden file,
// returning the contents as a string.
func golden(t *testing.T, name, got string) string {
	t.Helper()

	golden := filepath.Join("testdata", name+".golden")
	if *update && got != "" {
		err := ioutil.WriteFile(golden, []byte(got), 0644)
		require.NoError(t, err)
	}

	expected, err := ioutil.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func TestFormatToken(t *testing.T) {
	type testCase struct {
		token              api.ACLToken
		overrideGoldenName string
	}

	timeRef := func(in time.Time) *time.Time {
		return &in
	}

	cases := map[string]testCase{
		"basic": {
			token: api.ACLToken{
				AccessorID:  "fbd2447f-7479-4329-ad13-b021d74f86ba",
				SecretID:    "869c6e91-4de9-4dab-b56e-87548435f9c6",
				Description: "test token",
				Local:       false,
				CreateTime:  time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
				Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex: 42,
				ModifyIndex: 100,
			},
		},
		"legacy": {
			token: api.ACLToken{
				AccessorID:  "8acc7486-ca54-4d3c-9aed-5cd85651b0ee",
				SecretID:    "legacy-secret",
				Description: "legacy",
				Rules:       `operator = "read"`,
			},
		},
		"complex": {
			token: api.ACLToken{
				AccessorID:     "fbd2447f-7479-4329-ad13-b021d74f86ba",
				SecretID:       "869c6e91-4de9-4dab-b56e-87548435f9c6",
				Namespace:      "foo",
				Description:    "test token",
				Local:          false,
				AuthMethod:     "bar",
				CreateTime:     time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
				ExpirationTime: timeRef(time.Date(2020, 5, 22, 19, 52, 31, 0, time.UTC)),
				Hash:           []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex:    5,
				ModifyIndex:    10,
				Policies: []*api.ACLLink{
					&api.ACLLink{
						ID:   "beb04680-815b-4d7c-9e33-3d707c24672c",
						Name: "hobbiton",
					},
					&api.ACLLink{
						ID:   "18788457-584c-4812-80d3-23d403148a90",
						Name: "bywater",
					},
				},
				Roles: []*api.ACLLink{
					&api.ACLLink{
						ID:   "3b0a78fe-b9c3-40de-b8ea-7d4d6674b366",
						Name: "shire",
					},
					&api.ACLLink{
						ID:   "6c9d1e1d-34bc-4d55-80f3-add0890ad791",
						Name: "west-farthing",
					},
				},
				ServiceIdentities: []*api.ACLServiceIdentity{
					&api.ACLServiceIdentity{
						ServiceName: "gardener",
						Datacenters: []string{"middleearth-northwest"},
					},
				},
				NodeIdentities: []*api.ACLNodeIdentity{
					&api.ACLNodeIdentity{
						NodeName:   "bagend",
						Datacenter: "middleearth-northwest",
					},
				},
			},
		},
	}

	formatters := map[string]Formatter{
		"pretty":      newPrettyFormatter(false),
		"pretty-meta": newPrettyFormatter(true),
		// the JSON formatter ignores the showMeta
		"json": newJSONFormatter(false),
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			for fmtName, formatter := range formatters {
				t.Run(fmtName, func(t *testing.T) {
					actual, err := formatter.FormatToken(&tcase.token)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)
					if tcase.overrideGoldenName != "" {
						gName = tcase.overrideGoldenName
					}

					expected := golden(t, path.Join("FormatToken", gName), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}

func TestFormatTokenList(t *testing.T) {
	type testCase struct {
		tokens             []*api.ACLTokenListEntry
		overrideGoldenName string
	}

	timeRef := func(in time.Time) *time.Time {
		return &in
	}

	cases := map[string]testCase{
		"basic": {
			tokens: []*api.ACLTokenListEntry{
				&api.ACLTokenListEntry{
					AccessorID:  "fbd2447f-7479-4329-ad13-b021d74f86ba",
					Description: "test token",
					Local:       false,
					CreateTime:  time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
					Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
					CreateIndex: 42,
					ModifyIndex: 100,
				},
			},
		},
		"legacy": {
			tokens: []*api.ACLTokenListEntry{
				&api.ACLTokenListEntry{
					AccessorID:  "8acc7486-ca54-4d3c-9aed-5cd85651b0ee",
					Description: "legacy",
					Legacy:      true,
				},
			},
		},
		"complex": {
			tokens: []*api.ACLTokenListEntry{
				&api.ACLTokenListEntry{
					AccessorID:     "fbd2447f-7479-4329-ad13-b021d74f86ba",
					Namespace:      "foo",
					Description:    "test token",
					Local:          false,
					AuthMethod:     "bar",
					CreateTime:     time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
					ExpirationTime: timeRef(time.Date(2020, 5, 22, 19, 52, 31, 0, time.UTC)),
					Hash:           []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
					CreateIndex:    5,
					ModifyIndex:    10,
					Policies: []*api.ACLLink{
						&api.ACLLink{
							ID:   "beb04680-815b-4d7c-9e33-3d707c24672c",
							Name: "hobbiton",
						},
						&api.ACLLink{
							ID:   "18788457-584c-4812-80d3-23d403148a90",
							Name: "bywater",
						},
					},
					Roles: []*api.ACLLink{
						&api.ACLLink{
							ID:   "3b0a78fe-b9c3-40de-b8ea-7d4d6674b366",
							Name: "shire",
						},
						&api.ACLLink{
							ID:   "6c9d1e1d-34bc-4d55-80f3-add0890ad791",
							Name: "west-farthing",
						},
					},
					ServiceIdentities: []*api.ACLServiceIdentity{
						&api.ACLServiceIdentity{
							ServiceName: "gardener",
							Datacenters: []string{"middleearth-northwest"},
						},
					},
					NodeIdentities: []*api.ACLNodeIdentity{
						&api.ACLNodeIdentity{
							NodeName:   "bagend",
							Datacenter: "middleearth-northwest",
						},
					},
				},
			},
		},
	}

	formatters := map[string]Formatter{
		"pretty":      newPrettyFormatter(false),
		"pretty-meta": newPrettyFormatter(true),
		// the JSON formatter ignores the showMeta
		"json": newJSONFormatter(false),
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			for fmtName, formatter := range formatters {
				t.Run(fmtName, func(t *testing.T) {
					actual, err := formatter.FormatTokenList(tcase.tokens)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)
					if tcase.overrideGoldenName != "" {
						gName = tcase.overrideGoldenName
					}

					expected := golden(t, path.Join("FormatTokenList", gName), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}
