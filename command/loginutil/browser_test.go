// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package loginutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenCommand(t *testing.T) {
	const target = "https://idp.example.com/api/oidc/logout?id_token_hint=a.b.c&state=x"

	cases := map[string]struct {
		goos     string
		wsl      bool
		wantName string
		wantArgs []string
	}{
		"darwin":  {goos: "darwin", wantName: "open", wantArgs: []string{target}},
		"linux":   {goos: "linux", wantName: "xdg-open", wantArgs: []string{target}},
		"windows": {goos: "windows", wantName: "rundll32.exe", wantArgs: []string{"url.dll,FileProtocolHandler", target}},
		"wsl":     {goos: "linux", wsl: true, wantName: "rundll32.exe", wantArgs: []string{"url.dll,FileProtocolHandler", target}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotName, gotArgs := openCommand(tc.goos, tc.wsl, target)
			require.Equal(t, tc.wantName, gotName)
			require.Equal(t, tc.wantArgs, gotArgs)
		})
	}
}

func TestOpenURL_RejectsNonHTTP(t *testing.T) {
	for _, bad := range []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"ftp://example.com/x",
		"https:///no-host",
		"%%%",
	} {
		require.Error(t, OpenURL(bad), "must refuse %q", bad)
	}
}
