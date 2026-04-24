// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package register

import (
	"os"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
)

func TestValidateMultiPortWithConnectSidecarInCE(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)

	f := testFile(t, "json")
	defer os.Remove(f.Name())
	_, err := f.WriteString(`{ "service": { "name": "web", "ports": [ { "name": "test", "port": 8080, "default": true } ], "connect": { "sidecar_service": {} } } }`)
	require.NoError(t, err)

	exitCode := c.Run([]string{"-http-addr=" + a.HTTPAddr(), f.Name()})
	require.Equal(t, 1, exitCode, "expected error but got success")
}
