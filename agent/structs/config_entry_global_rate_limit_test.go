// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build consulent

package structs

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func float64Ptr(f float64) *float64 {
	return &f
}

func TestGlobalRateLimitConfigEntry_Validate(t *testing.T) {
	tests := map[string]struct {
		entry     *GlobalRateLimitConfigEntry
		expectErr string
	}{
		"nil entry": {
			entry:     nil,
			expectErr: "config entry is nil",
		},
		"empty name": {
			entry: &GlobalRateLimitConfigEntry{
				Name:   "",
				Config: &GlobalRateLimitConfig{},
			},
			expectErr: "Name is required",
		},
		"name not global": {
			entry: &GlobalRateLimitConfigEntry{
				Name:   "other",
				Config: &GlobalRateLimitConfig{},
			},
			expectErr: "Name for rate-limit config entry must be 'global'",
		},
		"nil config": {
			entry: &GlobalRateLimitConfigEntry{
				Name:   "global",
				Config: nil,
			},
			expectErr: "Config is required",
		},
		"negative read rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(-10),
					WriteRate: float64Ptr(100),
				},
			},
			expectErr: "readRate must be non-negative",
		},
		"negative write rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(100),
					WriteRate: float64Ptr(-5),
				},
			},
			expectErr: "writeRate must be non-negative",
		},
		"both rates negative": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(-1),
					WriteRate: float64Ptr(-2),
				},
			},
			// readRate is validated first
			expectErr: "readRate must be non-negative",
		},
		"NaN read rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(math.NaN()),
					WriteRate: float64Ptr(100),
				},
			},
			expectErr: "readRate must be a valid number, got NaN",
		},
		"NaN write rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(100),
					WriteRate: float64Ptr(math.NaN()),
				},
			},
			expectErr: "writeRate must be a valid number, got NaN",
		},
		"+Inf read rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(math.Inf(1)),
					WriteRate: float64Ptr(100),
				},
			},
			expectErr: "readRate must be a finite number",
		},
		"-Inf read rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(math.Inf(-1)),
					WriteRate: float64Ptr(100),
				},
			},
			expectErr: "readRate must be a finite number",
		},
		"+Inf write rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(100),
					WriteRate: float64Ptr(math.Inf(1)),
				},
			},
			expectErr: "writeRate must be a finite number",
		},
		"empty exclude endpoint": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:         float64Ptr(100),
					WriteRate:        float64Ptr(200),
					ExcludeEndpoints: []string{"Health.Check", ""},
				},
			},
			expectErr: "excludeEndpoints[1] cannot be empty",
		},
		"valid with both rates set": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(100),
					WriteRate: float64Ptr(200),
				},
			},
			expectErr: "",
		},
		"valid with nil rates (unlimited)": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  nil,
					WriteRate: nil,
				},
			},
			expectErr: "",
		},
		"valid with zero rates": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(0),
					WriteRate: float64Ptr(0),
				},
			},
			expectErr: "",
		},
		"valid with fractional rates": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(0.5),
					WriteRate: float64Ptr(1.5),
				},
			},
			expectErr: "",
		},
		"valid with exclude endpoints": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:         float64Ptr(100),
					WriteRate:        float64Ptr(200),
					ExcludeEndpoints: []string{"Health.Check", "Status.Leader"},
				},
			},
			expectErr: "",
		},
		"valid with priority enabled": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(50),
					WriteRate: float64Ptr(100),
					Priority:  true,
				},
			},
			expectErr: "",
		},
		"valid with only read rate set": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(100),
					WriteRate: nil,
				},
			},
			expectErr: "",
		},
		"valid with only write rate set": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  nil,
					WriteRate: float64Ptr(200),
				},
			},
			expectErr: "",
		},
		"valid with very small decimal rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(0.001),
					WriteRate: float64Ptr(0.001),
				},
			},
			expectErr: "",
		},
		"valid with large decimal rate": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(999999.99),
					WriteRate: float64Ptr(999999.99),
				},
			},
			expectErr: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGlobalRateLimitConfigEntry_Normalize(t *testing.T) {
	tests := map[string]struct {
		entry     *GlobalRateLimitConfigEntry
		expectErr string
		checkFunc func(t *testing.T, e *GlobalRateLimitConfigEntry)
	}{
		"nil entry": {
			entry:     nil,
			expectErr: "config entry is nil",
		},
		"sets kind to RateLimit": {
			entry: &GlobalRateLimitConfigEntry{
				Name:   "global",
				Config: &GlobalRateLimitConfig{},
			},
			checkFunc: func(t *testing.T, e *GlobalRateLimitConfigEntry) {
				require.Equal(t, RateLimit, e.Kind)
			},
		},
		"computes hash": {
			entry: &GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &GlobalRateLimitConfig{
					ReadRate:  float64Ptr(100),
					WriteRate: float64Ptr(200),
				},
			},
			checkFunc: func(t *testing.T, e *GlobalRateLimitConfigEntry) {
				require.NotZero(t, e.Hash)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
				if tc.checkFunc != nil {
					tc.checkFunc(t, tc.entry)
				}
			}
		})
	}
}

func TestGlobalRateLimitConfigEntry_GetKind(t *testing.T) {
	e := &GlobalRateLimitConfigEntry{}
	require.Equal(t, RateLimit, e.GetKind())
}

func TestGlobalRateLimitConfigEntry_GetName(t *testing.T) {
	t.Run("nil entry", func(t *testing.T) {
		var e *GlobalRateLimitConfigEntry
		require.Equal(t, "", e.GetName())
	})
	t.Run("with name", func(t *testing.T) {
		e := &GlobalRateLimitConfigEntry{Name: "global"}
		require.Equal(t, "global", e.GetName())
	})
}

func TestGlobalRateLimitConfigEntry_GetMeta(t *testing.T) {
	t.Run("nil entry", func(t *testing.T) {
		var e *GlobalRateLimitConfigEntry
		require.Nil(t, e.GetMeta())
	})
	t.Run("with meta", func(t *testing.T) {
		meta := map[string]string{"key": "value"}
		e := &GlobalRateLimitConfigEntry{Meta: meta}
		require.Equal(t, meta, e.GetMeta())
	})
}

func TestGlobalRateLimitConfigEntry_GetEnterpriseMeta(t *testing.T) {
	t.Run("nil entry", func(t *testing.T) {
		var e *GlobalRateLimitConfigEntry
		require.Nil(t, e.GetEnterpriseMeta())
	})
	t.Run("non-nil entry", func(t *testing.T) {
		e := &GlobalRateLimitConfigEntry{}
		require.NotNil(t, e.GetEnterpriseMeta())
	})
}

func TestGlobalRateLimitConfigEntry_GetRaftIndex(t *testing.T) {
	t.Run("nil entry", func(t *testing.T) {
		var e *GlobalRateLimitConfigEntry
		require.Nil(t, e.GetRaftIndex())
	})
	t.Run("non-nil entry", func(t *testing.T) {
		e := &GlobalRateLimitConfigEntry{}
		require.NotNil(t, e.GetRaftIndex())
	})
}

func TestGlobalRateLimitConfigEntry_GetSetHash(t *testing.T) {
	t.Run("nil entry get", func(t *testing.T) {
		var e *GlobalRateLimitConfigEntry
		require.Equal(t, uint64(0), e.GetHash())
	})
	t.Run("set and get hash", func(t *testing.T) {
		e := &GlobalRateLimitConfigEntry{}
		e.SetHash(42)
		require.Equal(t, uint64(42), e.GetHash())
	})
	t.Run("set hash on nil is no-op", func(t *testing.T) {
		var e *GlobalRateLimitConfigEntry
		e.SetHash(42) // should not panic
	})
}

// TestGlobalRateLimitConfigEntry_DecodeAndValidate tests the full pipeline:
// JSON decode → DecodeConfigEntry (mapstructure) → Validate.
// This covers the "what if someone passes a string or decimal" scenarios.
func TestGlobalRateLimitConfigEntry_DecodeAndValidate(t *testing.T) {
	tests := map[string]struct {
		jsonBody       string
		expectDecodeOK bool
		expectValidErr string
	}{
		"numeric decimal values are accepted": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": 50.5,
					"writeRate": 100.25
				}
			}`,
			expectDecodeOK: true,
			expectValidErr: "",
		},
		"integer values are accepted": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": 100,
					"writeRate": 200
				}
			}`,
			expectDecodeOK: true,
			expectValidErr: "",
		},
		"string numeric values are coerced by mapstructure WeaklyTypedInput": {
			// mapstructure with WeaklyTypedInput=true converts "100" → float64(100)
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": "100",
					"writeRate": "200"
				}
			}`,
			expectDecodeOK: true,
			expectValidErr: "",
		},
		"string decimal values are coerced by mapstructure WeaklyTypedInput": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": "50.5",
					"writeRate": "100.25"
				}
			}`,
			expectDecodeOK: true,
			expectValidErr: "",
		},
		"non-numeric string value fails decode": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": "abc",
					"writeRate": 100
				}
			}`,
			expectDecodeOK: false,
		},
		"zero values are valid": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": 0,
					"writeRate": 0
				}
			}`,
			expectDecodeOK: true,
			expectValidErr: "",
		},
		"negative values fail validation": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": -10,
					"writeRate": 100
				}
			}`,
			expectDecodeOK: true,
			expectValidErr: "readRate must be non-negative",
		},
		"null rates are decoded as nil (unlimited)": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {
					"readRate": null,
					"writeRate": null
				}
			}`,
			expectDecodeOK: true,
			expectValidErr: "",
		},
		"missing rates are decoded as nil (unlimited)": {
			jsonBody: `{
				"Kind": "rate-limit",
				"Name": "global",
				"config": {}
			}`,
			expectDecodeOK: true,
			expectValidErr: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var raw map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(tc.jsonBody), &raw))

			entry, err := DecodeConfigEntry(raw)
			if !tc.expectDecodeOK {
				require.Error(t, err, "expected decode to fail")
				return
			}
			require.NoError(t, err, "expected decode to succeed")
			require.NotNil(t, entry)

			// Normalize first (as the Apply RPC does)
			require.NoError(t, entry.Normalize())

			err = entry.Validate()
			if tc.expectValidErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectValidErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
