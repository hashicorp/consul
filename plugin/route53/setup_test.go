package route53

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/mholt/caddy"
)

func TestSetupRoute53(t *testing.T) {
	f := func(credential *credentials.Credentials) route53iface.Route53API {
		return mockedRoute53{}
	}

	c := caddy.NewTestController("dns", `route53`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 :`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
    aws_access_key
}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
}
