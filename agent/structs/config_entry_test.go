package structs

import (
	"bytes"
	"testing"

	"github.com/hashicorp/go-msgpack/codec"
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
				//Connect:  ConnectConfiguration{SidecarProxy: true},
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
				//Connect:  ConnectConfiguration{SidecarProxy: true},
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

func TestServiceConfigResponse_MsgPack(t *testing.T) {
	// TODO(banks) lib.MapWalker doesn't actually fix the map[interface{}] issue
	// it claims to in docs yet. When it does uncomment those cases below.
	a := ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"string": "foo",
			// "map": map[string]interface{}{
			// 	"baz": "bar",
			// },
		},
		UpstreamConfigs: map[string]map[string]interface{}{
			"a": map[string]interface{}{
				"string": "aaaa",
				// "map": map[string]interface{}{
				// 	"baz": "aa",
				// },
			},
			"b": map[string]interface{}{
				"string": "bbbb",
				// "map": map[string]interface{}{
				// 	"baz": "bb",
				// },
			},
		},
	}

	var buf bytes.Buffer

	// Encode as msgPack using a regular handle i.e. NOT one with RawAsString
	// since our RPC codec doesn't use that.
	enc := codec.NewEncoder(&buf, msgpackHandle)
	require.NoError(t, enc.Encode(&a))

	var b ServiceConfigResponse

	dec := codec.NewDecoder(&buf, msgpackHandle)
	require.NoError(t, dec.Decode(&b))

	require.Equal(t, a, b)
}

func TestConfigEntryResponseMarshalling(t *testing.T) {
	t.Parallel()

	cases := map[string]ConfigEntryResponse{
		"nil entry": ConfigEntryResponse{},
		"proxy-default entry": ConfigEntryResponse{
			Entry: &ProxyConfigEntry{
				Kind: ProxyDefaults,
				Name: ProxyConfigGlobal,
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		"service-default entry": ConfigEntryResponse{
			Entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "foo",
				Protocol: "tcp",
				// Connect:  ConnectConfiguration{SideCarProxy: true},
			},
		},
	}

	for name, tcase := range cases {
		name := name
		tcase := tcase
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := tcase.MarshalBinary()
			require.NoError(t, err)
			require.NotEmpty(t, data)

			var resp ConfigEntryResponse
			require.NoError(t, resp.UnmarshalBinary(data))

			require.Equal(t, tcase, resp)
		})
	}
}
