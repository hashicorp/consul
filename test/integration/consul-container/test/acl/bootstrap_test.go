// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

var resetIndexRE = regexp.MustCompile(`reset index: (\d+)`)

// TestACL_Bootstrap verifies that ACLs can be bootstrapped only once, and that
// the documented reset procedure allows a second bootstrap.
func TestACL_Bootstrap(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")

	t.Run("the bootstrap token is the global management token", func(t *testing.T) {
		self, _, err := e.adminClient.ACL().TokenReadSelf(nil)
		require.NoError(t, err)

		require.Equal(t, e.cluster.TokenBootstrap, self.SecretID)
		require.Equal(t, "Bootstrap Token (Global Management)", self.Description)
		require.Len(t, self.Policies, 1)
		require.Equal(t, "global-management", self.Policies[0].Name)
		require.Equal(t, "00000000-0000-0000-0000-000000000001", self.Policies[0].ID)
	})

	t.Run("a second bootstrap is refused and reports the reset index", func(t *testing.T) {
		_, _, err := e.admin.ACL().Bootstrap()

		require.Error(t, err)
		require.Contains(t, err.Error(), "ACL bootstrap no longer allowed")
		require.Regexp(t, resetIndexRE, err.Error())
	})

	t.Run("writing the reset index to the data directory allows another bootstrap", func(t *testing.T) {
		_, _, err := e.admin.ACL().Bootstrap()
		require.Error(t, err)
		index, err := strconv.Atoi(resetIndexRE.FindStringSubmatch(err.Error())[1])
		require.NoError(t, err)

		e.writeBootstrapReset(t, index)

		second, _, err := e.admin.ACL().Bootstrap()
		require.NoError(t, err)
		require.NotEqual(t, e.cluster.TokenBootstrap, second.SecretID, "a new token is issued")
		require.Equal(t, "global-management", second.Policies[0].Name)

		// Both tokens are valid: a reset adds a bootstrap token, it does not revoke one.
		for name, secret := range map[string]string{"original": e.cluster.TokenBootstrap, "new": second.SecretID} {
			self, _, err := e.tokenClient(t, secret).ACL().TokenReadSelf(nil)
			require.NoError(t, err, name)
			require.Equal(t, "global-management", self.Policies[0].Name, name)
		}
	})

	t.Run("the reset is single use", func(t *testing.T) {
		_, _, err := e.admin.ACL().Bootstrap()

		require.Error(t, err)
		require.Contains(t, err.Error(), "ACL bootstrap no longer allowed")
	})

	t.Run("an unauthenticated client cannot read ACL data", func(t *testing.T) {
		anonymous := e.tokenClient(t, "")

		_, _, err := anonymous.ACL().PolicyList(nil)
		requireDenied(t, err, "anonymous policy list")
		_, _, err = anonymous.ACL().TokenList(nil)
		requireDenied(t, err, "anonymous token list")
	})

}
