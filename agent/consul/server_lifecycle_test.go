// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/lifecycle"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestServer_RegistersLifecycleComponents(t *testing.T) {
	t.Parallel()

	ex := lifecycle.NewExchange()
	t.Cleanup(ex.Close)

	dir, s := testServerWithDepsAndConfig(t, func(deps *Deps) { deps.Lifecycle = ex })
	defer os.RemoveAll(dir)
	defer s.Shutdown()

	retry.Run(t, func(r *retry.R) {
		for _, name := range []lifecycle.ComponentName{ACLResolverComponent, "server/state-store"} {
			st, ok := ex.Status(name)
			if !ok {
				r.Fatalf("%s is not registered", name)
			}
			if st.Phase != lifecycle.PhaseReady {
				r.Fatalf("%s is not ready: %+v", name, st)
			}
		}
	})

	// The state store serves from construction; restores bump its generation.
	st, _ := ex.Status("server/state-store")
	require.GreaterOrEqual(t, st.Generation, uint64(1))
}
