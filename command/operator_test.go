package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestOperator_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorCommand{}
}
