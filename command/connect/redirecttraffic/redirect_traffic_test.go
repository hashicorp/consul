package redirecttraffic

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestRun_FlagValidation(t *testing.T) {
	t.Parallel()

	ui := cli.NewMockUi()
	c := New(ui)

	code := c.Run(nil)
	require.Equal(t, code, 1)
	require.Contains(t, ui.ErrorWriter.String(), "-proxy-uid is required")
}