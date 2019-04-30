package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeConfigEntry(t *testing.T) {
	t.Parallel()
	type tcase struct {
		input     map[string]interface{}
		expected  ConfigEntry
		expectErr bool
	}

	cases := map[string]tcase{
		"proxy-defaults": tcase{
			input: map[string]interface{}{
				"Kind": ProxyDefaults,
				"Name": ProxyConfigGlobal,
				"Config": map[string]interface{}{
					"foo": "bar",
				},
			},
			expected: &ProxyConfigEntry{
				Kind: ProxyDefaults,
				Name: ProxyConfigGlobal,
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		"proxy-defaults translations": tcase{
			input: map[string]interface{}{
				"kind": ProxyDefaults,
				"name": ProxyConfigGlobal,
				"config": map[string]interface{}{
					"foo":           "bar",
					"sidecar_proxy": true,
				},
			},
			expected: &ProxyConfigEntry{
				Kind: ProxyDefaults,
				Name: ProxyConfigGlobal,
				Config: map[string]interface{}{
					"foo":           "bar",
					"sidecar_proxy": true,
				},
			},
		},
		"service-defaults": tcase{
			input: map[string]interface{}{
				"Kind":     ServiceDefaults,
				"Name":     "foo",
				"Protocol": "tcp",
				"Connect": map[string]interface{}{
					"SidecarProxy": true,
				},
			},
			expected: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "foo",
				Protocol: "tcp",
				Connect:  ConnectConfiguration{SidecarProxy: true},
			},
		},
		"service-defaults translations": tcase{
			input: map[string]interface{}{
				"kind":     ServiceDefaults,
				"name":     "foo",
				"protocol": "tcp",
				"connect": map[string]interface{}{
					"sidecar_proxy": true,
				},
			},
			expected: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "foo",
				Protocol: "tcp",
				Connect:  ConnectConfiguration{SidecarProxy: true},
			},
		},
	}

	for name, tcase := range cases {
		name := name
		tcase := tcase

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual, err := DecodeConfigEntry(tcase.input)
			if tcase.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tcase.expected, actual)
			}
		})
	}
}
