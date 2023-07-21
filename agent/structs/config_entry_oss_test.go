//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/hcl"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDecodeConfigEntry_OSS(t *testing.T) {

	for _, tc := range []struct {
		name      string
		camel     string
		snake     string
		expect    ConfigEntry
		expectErr string
	}{
		{
			name: "namespaces invalid top level",
			snake: `
				kind = "terminating-gateway"
				name = "terminating-gateway"
				namespace = "foo"
			`,
			camel: `
				Kind = "terminating-gateway"
				Name = "terminating-gateway"
				Namespace = "foo"
			`,
			expectErr: `invalid config key "namespace", namespaces are a consul enterprise feature`,
		},
		{
			name: "namespaces invalid deep",
			snake: `
				kind = "ingress-gateway"
				name = "ingress-web"
				listeners = [
					{
						port = 8080
						protocol = "http"
						services = [
							{
								name = "web"
								hosts = ["test.example.com", "test2.example.com"]
								namespace = "frontend"
							},
						]
					}
				]
			`,
			camel: `
				Kind = "ingress-gateway"
				Name = "ingress-web"
				Namespace = "blah"
				Listeners = [
					{
						Port = 8080
						Protocol = "http"
						Services = [
							{
								Name = "web"
								Hosts = ["test.example.com", "test2.example.com"]
								Namespace = "frontend"
							},
						]
					},
				]
			`,
			expectErr: `* invalid config key "listeners[0].services[0].namespace", namespaces are a consul enterprise feature`,
		},
	} {
		tc := tc

		testbody := func(t *testing.T, body string) {
			var raw map[string]interface{}
			err := hcl.Decode(&raw, body)
			require.NoError(t, err)

			got, err := DecodeConfigEntry(raw)
			if tc.expectErr != "" {
				require.Nil(t, got)
				require.Error(t, err)
				requireContainsLower(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			}
		}

		t.Run(tc.name+" (snake case)", func(t *testing.T) {
			testbody(t, tc.snake)
		})
		t.Run(tc.name+" (camel case)", func(t *testing.T) {
			testbody(t, tc.camel)
		})
	}
}
