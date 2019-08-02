package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryTarget_TextMarshal(t *testing.T) {
	for _, tc := range []struct {
		target DiscoveryTarget
		enc    string
	}{
		{
			target: DiscoveryTarget{"svc", "", "default", "dc1"},
			enc:    "svc.default.dc1",
		},
		{
			target: DiscoveryTarget{"svc", "sub", "other", "dc2"},
			enc:    "sub.svc.other.dc2",
		},
		{
			target: DiscoveryTarget{"one", "two", "three", "four"},
			enc:    "two.one.three.four",
		},
	} {
		tc := tc
		t.Run(tc.target.String(), func(t *testing.T) {
			out, err := tc.target.MarshalText()
			require.NoError(t, err)
			require.Equal(t, tc.enc, string(out))

			var dec DiscoveryTarget
			require.NoError(t, dec.UnmarshalText(out))
			require.Equal(t, tc.target, dec)
		})
	}
}

func TestDiscoveryTarget_CopyAndModify(t *testing.T) {
	type fields = DiscoveryTarget // abbreviation

	for _, tc := range []struct {
		name   string
		in     fields
		mod    fields // this is semantically wrong, but the shape of the struct is still what we want
		expect fields
	}{
		{
			name:   "service with no subset and no mod",
			in:     fields{"foo", "", "default", "dc1"},
			mod:    fields{},
			expect: fields{"foo", "", "default", "dc1"},
		},
		{
			name:   "service with subset and no mod",
			in:     fields{"foo", "v2", "default", "dc1"},
			mod:    fields{},
			expect: fields{"foo", "v2", "default", "dc1"},
		},
		{
			name:   "service with no subset and service mod",
			in:     fields{"foo", "", "default", "dc1"},
			mod:    fields{"bar", "", "", ""},
			expect: fields{"bar", "", "default", "dc1"},
		},
		{
			name:   "service with subset and service mod",
			in:     fields{"foo", "v2", "default", "dc1"},
			mod:    fields{"bar", "", "", ""},
			expect: fields{"bar", "", "default", "dc1"},
		},
		{
			name:   "service with subset and noop service mod with dc mod",
			in:     fields{"foo", "v2", "default", "dc1"},
			mod:    fields{"foo", "", "", "dc9"},
			expect: fields{"foo", "v2", "default", "dc9"},
		},
		{
			name:   "service with subset and namespace mod",
			in:     fields{"foo", "v2", "default", "dc1"},
			mod:    fields{"", "", "fancy", ""},
			expect: fields{"foo", "v2", "fancy", "dc1"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out := tc.in.CopyAndModify(
				tc.mod.Service,
				tc.mod.ServiceSubset,
				tc.mod.Namespace,
				tc.mod.Datacenter,
			)
			require.Equal(t, tc.expect, out)
		})
	}

}
