package route53

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/mholt/caddy"
)

func TestSetupRoute53(t *testing.T) {
	f := func(credential *credentials.Credentials) route53iface.Route53API {
		return fakeRoute53{}
	}

	tests := []struct {
		body          string
		expectedError bool
	}{
		{`route53`, false},
		{`route53 :`, true},
		{`route53 example.org:12345678`, false},
		{`route53 example.org:12345678 {
    aws_access_key
}`, true},
		{`route53 example.org:12345678 {
    upstream 10.0.0.1
}`, false},

		{`route53 example.org:12345678 {
    upstream
}`, false},
		{`route53 example.org:12345678 {
    wat
}`, true},
		{`route53 example.org:12345678 {
    aws_access_key ACCESS_KEY_ID SEKRIT_ACCESS_KEY
    upstream 1.2.3.4
}`, false},

		{`route53 example.org:12345678 {
    fallthrough
}`, false},
		{`route53 example.org:12345678 {
		credentials
 		upstream 1.2.3.4
	}`, true},

		{`route53 example.org:12345678 {
		credentials default
 		upstream 1.2.3.4
	}`, false},
		{`route53 example.org:12345678 {
		credentials default credentials
 		upstream 1.2.3.4
	}`, false},
		{`route53 example.org:12345678 {
		credentials default credentials extra-arg
 		upstream 1.2.3.4
	}`, true},
		{`route53 example.org:12345678 example.org:12345678 {
 		upstream 1.2.3.4
	}`, true},

		{`route53 example.org {
 		upstream 1.2.3.4
	}`, true},
	}

	for _, test := range tests {
		c := caddy.NewTestController("dns", test.body)
		if err := setup(c, f); (err == nil) == test.expectedError {
			t.Errorf("Unexpected errors: %v", err)
		}
	}
}
