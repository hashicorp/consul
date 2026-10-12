// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package loginutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenCommands(t *testing.T) {
	const target = "https://idp.example.com/api/oidc/logout?id_token_hint=a.b.c&state=x"
	rundll32 := openCmd{"rundll32.exe", []string{"url.dll,FileProtocolHandler", target}}
	xdgOpen := openCmd{"xdg-open", []string{target}}

	cases := map[string]struct {
		goos string
		wsl  bool
		want []openCmd
	}{
		"darwin":  {goos: "darwin", want: []openCmd{{"open", []string{target}}}},
		"linux":   {goos: "linux", want: []openCmd{xdgOpen}},
		"windows": {goos: "windows", want: []openCmd{rundll32}},
		"wsl falls back to linux openers": {goos: "linux", wsl: true,
			want: []openCmd{rundll32, {"wslview", []string{target}}, xdgOpen}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, openCommands(tc.goos, tc.wsl, target))
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
