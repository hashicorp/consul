// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package logout

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())

	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	if len(c.flags.Args()) > 0 {
		c.UI.Error("Should have no non-flag arguments.")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if _, err := client.ACL().Logout(nil); err != nil {
		c.UI.Error(fmt.Sprintf("Error destroying token: %v", err))
		return 1
	}

	// Best-effort IdP (front-channel) logout. If the matching `consul login`
	// wrote an IdP logout sidecar file next to the token file, open the
	// provider's RP-initiated logout URL in a browser to terminate the IdP
	// session, then remove the sidecar. Absence of the sidecar preserves the
	// existing behavior of only destroying the Consul token.
	c.maybeIDPLogout()

	return 0
}

// idpLogoutSuffix is appended to the token file path to locate the companion
// file written by `consul login` that stores the OIDC RP-initiated logout URL.
const idpLogoutSuffix = ".oidc-logout"

func (c *cmd) maybeIDPLogout() {
	tokenFile := c.http.TokenFile()
	if tokenFile == "" {
		return
	}

	sinkPath := tokenFile + idpLogoutSuffix
	data, err := os.ReadFile(sinkPath)
	if err != nil {
		return // no sidecar -> plain logout, as before
	}
	// Remove the sidecar regardless of whether the browser opens successfully;
	// the Consul token has already been destroyed.
	defer os.Remove(sinkPath)

	logoutURL := strings.TrimSpace(string(data))
	if logoutURL == "" {
		return
	}

	c.UI.Info("Completing logout at your identity provider via browser:")
	c.UI.Info("    " + logoutURL)
	if err := openURL(logoutURL); err != nil {
		c.UI.Warn(fmt.Sprintf("Unable to open a browser automatically (%s). Visit the URL above to finish signing out of your identity provider.", err))
	}
}

// openURL opens the specified URL in the user's default browser. It is a
// package variable so tests can stub browser launching. Only well-formed
// http(s) URLs are opened; anything else is rejected so that a tampered sidecar
// file cannot pass shell/command metacharacters to the operating system.
var openURL = func(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid logout URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("refusing to open logout URL with non-http(s) scheme %q", u.Scheme)
	}

	switch {
	case runtime.GOOS == "windows":
		// Invoke the URL protocol handler directly rather than via
		// "cmd.exe /c start" so the URL is never parsed by the command
		// interpreter (avoids command injection through metacharacters).
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	case isWSL():
		// Under WSL defer to the Windows shell. The empty title argument keeps
		// "start" from interpreting the URL as the window title.
		return exec.Command("cmd.exe", "/c", "start", "", rawURL).Start()
	case runtime.GOOS == "darwin":
		return exec.Command("open", rawURL).Start()
	default: // "linux", "freebsd", "openbsd", "netbsd"
		return exec.Command("xdg-open", rawURL).Start()
	}
}

// isWSL reports whether the process is running under Windows Subsystem for
// Linux, where the default browser must be launched via the Windows shell.
func isWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(data))
	return strings.Contains(v, "microsoft") || strings.Contains(v, "wsl")
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Destroy a Consul token created with login"

const help = `
Usage: consul logout [options]

  The logout command will destroy the provided token if it was created from
  'consul login'.
`
