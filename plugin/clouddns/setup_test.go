package clouddns

import (
	"context"
	"testing"

	"github.com/caddyserver/caddy"
	"google.golang.org/api/option"
)

func TestSetupCloudDNS(t *testing.T) {
	f := func(ctx context.Context, opt option.ClientOption) (gcpDNS, error) {
		return fakeGCPClient{}, nil
	}

	tests := []struct {
		body          string
		expectedError bool
	}{
		{`clouddns`, false},
		{`clouddns :`, true},
		{`clouddns ::`, true},
		{`clouddns example.org.:example-project:zone-name`, false},
		{`clouddns example.org.:example-project:zone-name { }`, false},
		{`clouddns example.org.:example-project: { }`, true},
		{`clouddns example.org.:example-project:zone-name { }`, false},
		{`clouddns example.org.:example-project:zone-name { wat
}`, true},
		{`clouddns example.org.:example-project:zone-name {
    fallthrough
}`, false},
		{`clouddns example.org.:example-project:zone-name {
    credentials
}`, true},
		{`clouddns example.org.:example-project:zone-name example.org.:example-project:zone-name {
	}`, true},

		{`clouddns example.org {
	}`, true},
	}

	for _, test := range tests {
		c := caddy.NewTestController("dns", test.body)
		if err := setup(c, f); (err == nil) == test.expectedError {
			t.Errorf("Unexpected errors: %v", err)
		}
	}
}
