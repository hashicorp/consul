package agent

import (
	"github.com/mitchellh/cli"
	"testing"
)

func TestCommand_implements(t *testing.T) {
	var _ cli.Command = new(Command)
}

func TestValidDatacenter(t *testing.T) {
	shouldMatch := []string{
		"dc1",
		"east-aws-001",
		"PROD_aws01-small",
	}
	noMatch := []string{
		"east.aws",
		"east!aws",
		"first,second",
	}
	for _, m := range shouldMatch {
		if !validDatacenter.MatchString(m) {
			t.Fatalf("expected match: %s", m)
		}
	}
	for _, m := range noMatch {
		if validDatacenter.MatchString(m) {
			t.Fatalf("expected no match: %s", m)
		}
	}
}
