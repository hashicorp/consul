package connect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// testCertURICases contains the test cases for parsing and encoding
// the SPIFFE IDs. This is a global since it is used in multiple test functions.
var testCertURICases = []struct {
	Name       string
	URI        string
	Struct     interface{}
	ParseError string
}{
	{
		"invalid scheme",
		"http://google.com/",
		nil,
		"scheme",
	},

	{
		"basic service ID",
		"spiffe://1234.consul/ns/default/dc/dc01/svc/web",
		&SpiffeIDService{
			Host:       "1234.consul",
			Namespace:  "default",
			Datacenter: "dc01",
			Service:    "web",
		},
		"",
	},

	{
		"basic agent ID",
		"spiffe://1234.consul/agent/client/dc/dc1/id/uuid",
		&SpiffeIDAgent{
			Host:       "1234.consul",
			Datacenter: "dc1",
			Agent:      "uuid",
		},
		"",
	},

	{
		"service with URL-encoded values",
		"spiffe://1234.consul/ns/foo%2Fbar/dc/bar%2Fbaz/svc/baz%2Fqux",
		&SpiffeIDService{
			Host:       "1234.consul",
			Namespace:  "foo/bar",
			Datacenter: "bar/baz",
			Service:    "baz/qux",
		},
		"",
	},

	{
		"signing ID",
		"spiffe://1234.consul",
		&SpiffeIDSigning{
			ClusterID: "1234",
			Domain:    "consul",
		},
		"",
	},
}

func TestParseCertURIFromString(t *testing.T) {
	for _, tc := range testCertURICases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)

			// Parse the ID and check the error/return value
			actual, err := ParseCertURIFromString(tc.URI)
			if err != nil {
				t.Logf("parse error: %s", err.Error())
			}
			assert.Equal(tc.ParseError != "", err != nil, "error value")
			if err != nil {
				assert.Contains(err.Error(), tc.ParseError)
				return
			}
			assert.Equal(tc.Struct, actual)
		})
	}
}
