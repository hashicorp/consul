// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package loginutil

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// OpenURL opens the specified http(s) URL in the user's default browser. It
// starts the platform opener and returns without waiting for the browser to
// exit.
//
// The URL is parsed and re-serialized before it is handed to the OS so that
// only a normalized, percent-encoded http(s) URL is ever passed on. On Windows
// and WSL the URL is opened via the url.dll protocol handler rather than
// `cmd.exe /c start`, so it is never interpreted by the cmd shell (where
// characters such as &, |, < and > would otherwise be treated as operators).
func OpenURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("refusing to open non-http(s) URL")
	}
	name, args := openCommand(runtime.GOOS, isWSL(), u.String())
	return exec.Command(name, args...).Start()
}

// openCommand returns the platform command used to open target.
func openCommand(goos string, wsl bool, target string) (string, []string) {
	switch {
	case goos == "windows" || wsl:
		return "rundll32.exe", []string{"url.dll,FileProtocolHandler", target}
	case goos == "darwin":
		return "open", []string{target}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		return "xdg-open", []string{target}
	}
}

// isWSL reports whether the binary is running in Windows Subsystem for Linux.
func isWSL() bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return false
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}
