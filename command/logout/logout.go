// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package logout

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/loginutil"
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

	openBrowser bool
	// openURL launches the browser; overridable in tests.
	openURL func(string) error
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.openBrowser, "open-browser", false,
		"Also open the identity provider's logout URL in the default browser to "+
			"end the identity provider session. Only applies to tokens created by "+
			"an OIDC login whose provider supports RP-Initiated logout. The URL is "+
			"always printed. Do not use in non-interactive environments.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())

	c.openURL = loginutil.OpenURL
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

	// Best-effort IdP (RP-Initiated) logout. If the matching `consul login`
	// wrote an IdP logout sidecar file next to the token file, print the
	// provider's RP-Initiated logout URL (and open it in a browser when
	// -open-browser is set) to terminate the IdP session, then remove the
	// sidecar. Absence of the sidecar preserves the existing behavior of only
	// destroying the Consul token.
	c.maybeIDPLogout()

	return 0
}

func (c *cmd) maybeIDPLogout() {
	tokenFile := c.http.TokenFile()
	if tokenFile == "" {
		return
	}

	sinkPath := tokenFile + loginutil.IDPLogoutSuffix
	data, err := os.ReadFile(sinkPath)
	if err != nil {
		return // no sidecar -> plain logout, as before
	}
	// Remove the sidecar regardless of the outcome below; the Consul token has
	// already been destroyed.
	defer os.Remove(sinkPath)

	logoutURL := strings.TrimSpace(string(data))
	if logoutURL == "" {
		return
	}

	// Only surface well-formed http(s) URLs so a tampered sidecar file cannot
	// emit arbitrary content (e.g. terminal escape sequences) to the console.
	u, err := url.Parse(logoutURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return
	}

	// Print the URL unconditionally so the user can always complete IdP logout
	// manually. A browser is launched only when explicitly requested with
	// -open-browser: by default `consul logout` must remain safe for
	// non-interactive contexts (CI pipelines, cron jobs, remote SSH sessions)
	// where no browser is available.
	c.UI.Info("To also end your identity provider session, open the following URL in a browser:")
	c.UI.Info("    " + logoutURL)

	if !c.openBrowser {
		return
	}
	if err := c.openURL(logoutURL); err != nil {
		c.UI.Warn(fmt.Sprintf("Error attempting to automatically open browser: %s. Please open the URL above manually.", err))
		return
	}
	c.UI.Info("Opened the identity provider logout URL in your default browser.")
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

  If the token was created by an OIDC login and the identity provider supports
  RP-Initiated logout, the provider's logout URL is printed so the identity
  provider session can also be ended. Use -open-browser to open it in the
  default browser automatically.
`
