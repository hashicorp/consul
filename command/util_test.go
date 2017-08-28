package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func assertNoTabs(t *testing.T, c cli.Command) {
	if strings.ContainsRune(c.Help(), '\t') {
		t.Errorf("%#v help output contains tabs", c)
	}
}
