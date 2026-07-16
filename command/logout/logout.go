// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package logout

import (
	"flag"
	"fmt"
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

// openURL opens the specified URL in the user's default browser.
func openURL(url string) error {
	var name string
	var args []string

	switch runtime.GOOS {
	case "windows":
		name = "cmd.exe"
		args = []string{"/c", "start"}
		url = strings.ReplaceAll(url, "&", "^&")
	case "darwin":
		name = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		name = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(name, args...).Start()
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
