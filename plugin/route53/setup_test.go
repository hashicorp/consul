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

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
    upstream 10.0.0.1
}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
    upstream
}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
    wat
}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
    aws_access_key ACCESS_KEY_ID SEKRIT_ACCESS_KEY
    upstream 1.2.3.4
}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
    fallthrough
}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
		credentials
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
		credentials default
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
		credentials default credentials
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err != nil {
		t.Fatalf("Unexpected errors: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 {
		credentials default credentials extra-arg
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `route53 example.org:12345678 example.org:12345678 {
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
	c = caddy.NewTestController("dns", `route53 example.org {
 		upstream 1.2.3.4
	}`)
	if err := setup(c, f); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
}
