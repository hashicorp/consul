// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package session

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

type shellResult struct {
	libcluster.ExecResult
	err     error
	elapsed time.Duration
}

// shellAsync runs a script in the client agent's container without blocking
// the test, for commands that hold a lock for a while.
func (e *env) shellAsync(script string) <-chan shellResult {
	out := make(chan shellResult, 1)
	go func() {
		start := time.Now()
		res, err := e.run(script)
		out <- shellResult{ExecResult: res, err: err, elapsed: time.Since(start)}
	}()
	return out
}

func awaitShell(t *testing.T, ch <-chan shellResult, within time.Duration) shellResult {
	t.Helper()
	select {
	case res := <-ch:
		require.NoError(t, res.err)
		return res
	case <-time.After(within):
		t.Fatalf("command did not finish within %s", within)
		return shellResult{}
	}
}

// concurrency replays a log of "start-X" and "end-X" lines and returns the
// highest number of workers that were running at the same time, and whether
// the log is well formed (every worker started and ended exactly once).
func concurrency(t *testing.T, log string) (peak int, workers map[string]int) {
	t.Helper()

	running := map[string]bool{}
	workers = map[string]int{}
	for _, line := range strings.Fields(log) {
		switch {
		case strings.HasPrefix(line, "start-"):
			name := strings.TrimPrefix(line, "start-")
			require.False(t, running[name], "worker %s started twice", name)
			running[name] = true
			workers[name]++
			if len(running) > peak {
				peak = len(running)
			}
		case strings.HasPrefix(line, "end-"):
			name := strings.TrimPrefix(line, "end-")
			require.True(t, running[name], "worker %s ended without starting", name)
			delete(running, name)
		}
	}
	require.Empty(t, running, "every worker must finish")
	return peak, workers
}

// runWorkers runs `consul lock` for each worker concurrently. Every worker
// appends start/end lines to logFile around a sleep, so the log shows how many
// were running at once.
func (e *env) runWorkers(t *testing.T, flags, prefix, logFile string, workers []string, hold int) string {
	t.Helper()

	e.shell(t, "rm -f "+logFile)

	var wg sync.WaitGroup
	for _, w := range workers {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			script := fmt.Sprintf(`consul lock %s %s "echo start-%s >> %s; sleep %d; echo end-%s >> %s"`, flags, prefix, w, logFile, hold, w, logFile)
			res, err := e.run(script)
			if err != nil || res.ExitCode != 0 {
				t.Errorf("worker %s: exit=%d err=%v output=%s", w, res.ExitCode, err, res.Output)
			}
		}()
		// Stagger the workers so they queue up in a known order.
		time.Sleep(300 * time.Millisecond)
	}
	wg.Wait()

	return e.shell(t, "cat "+logFile).Output
}

// TestLock_CLI covers `consul lock`, which runs a command while holding a lock
// and is the usual way to elect a single active worker.
func TestLock_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")

	t.Run("workers contending for a lock take turns", func(t *testing.T) {
		log := e.runWorkers(t, "-name=election", "worker-lock", "/tmp/mutex.log", []string{"a", "b", "c"}, 3)

		peak, ran := concurrency(t, log)
		require.Equal(t, 1, peak, "only the lock holder may run:\n%s", log)
		require.Equal(t, map[string]int{"a": 1, "b": 1, "c": 1}, ran, "every worker runs once, one after another")
	})

	t.Run("the lock is gone once the command finishes", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			pair, _, err := e.server.KV().Get("worker-lock/.lock", nil)
			require.NoError(r, err)
			if pair != nil {
				require.Empty(r, pair.Session, "the lock must be released")
			}
		})
	})

	t.Run("a semaphore admits a limited number of holders", func(t *testing.T) {
		log := e.runWorkers(t, "-n 2", "worker-semaphore", "/tmp/semaphore.log", []string{"a", "b", "c", "d"}, 4)

		peak, ran := concurrency(t, log)
		require.Equal(t, 2, peak, "-n 2 allows two holders at a time and no more:\n%s", log)
		require.Len(t, ran, 4, "every worker eventually runs")
	})

	t.Run("while it runs, the lock is a key held by a session", func(t *testing.T) {
		done := e.shellAsync(`consul lock -name=visible visible-lock "sleep 8"`)

		eventually(t, func(r *retry.R) {
			pair, _, err := e.server.KV().Get("visible-lock/.lock", nil)
			require.NoError(r, err)
			require.NotNil(r, pair)
			require.NotEmpty(r, pair.Session)

			session, _, err := e.server.Session().Info(pair.Session, nil)
			require.NoError(r, err)
			require.NotNil(r, session)
			require.Equal(r, "visible", session.Name)
		})

		awaitShell(t, done, time.Minute)
	})

	t.Run("a command that cannot get the lock in time gives up", func(t *testing.T) {
		holder := e.shellAsync(`consul lock try-lock "sleep 15"`)
		eventually(t, func(r *retry.R) {
			pair, _, err := e.server.KV().Get("try-lock/.lock", nil)
			require.NoError(r, err)
			require.NotNil(r, pair)
			require.NotEmpty(r, pair.Session)
		})

		res := e.shell(t, `consul lock -try=2s try-lock "echo must-not-run"`)
		require.Equal(t, 1, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "timeout during lock acquisition")
		require.NotContains(t, res.Output, "must-not-run")

		awaitShell(t, holder, time.Minute)
	})

	t.Run("the child's failure is reported only when asked for", func(t *testing.T) {
		res := e.shell(t, `consul lock exit-quiet "exit 3"`)
		require.Equal(t, 0, res.ExitCode, "without -child-exit-code the lock command itself succeeded")
		require.Contains(t, res.Output, "exit status 3")

		res = e.shell(t, `consul lock -child-exit-code exit-loud "exit 3"`)
		require.Equal(t, 2, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "exit status 3")
	})

	t.Run("losing the lock stops the command", func(t *testing.T) {
		done := e.shellAsync(`consul lock -name=doomed doomed-lock "sleep 120"`)

		var sessionID string
		eventually(t, func(r *retry.R) {
			pair, _, err := e.server.KV().Get("doomed-lock/.lock", nil)
			require.NoError(r, err)
			require.NotNil(r, pair)
			require.NotEmpty(r, pair.Session)
			sessionID = pair.Session
		})

		// Destroying the session takes the lock away from the holder.
		_, err := e.server.Session().Destroy(sessionID, nil)
		require.NoError(t, err)

		res := awaitShell(t, done, 60*time.Second)
		require.Less(t, res.elapsed, 100*time.Second, "the child must be stopped instead of running to completion")
		require.Contains(t, res.Output, "terminated")
	})
}
