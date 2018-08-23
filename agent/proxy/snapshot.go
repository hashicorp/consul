package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/file"
)

// snapshot is the structure of the snapshot file. This is unexported because
// we don't want this being a public API.
//
// The snapshot doesn't contain any configuration for the manager. We only
// want to restore the proxies that we're managing, and we use the config
// set at runtime to sync and reconcile what proxies we should start,
// restart, stop, or have already running.
type snapshot struct {
	// Version is the version of the snapshot format and can be used
	// to safely update the format in the future if necessary.
	Version int

	// Proxies are the set of proxies that the manager has.
	Proxies map[string]snapshotProxy
}

// snapshotProxy represents a single proxy.
type snapshotProxy struct {
	// Mode corresponds to the type of proxy running.
	Mode structs.ProxyExecMode

	// Config is an opaque mapping of primitive values that the proxy
	// implementation uses to restore state.
	Config map[string]interface{}
}

// snapshotVersion is the current version to encode within the snapshot.
const snapshotVersion = 1

// SnapshotPath returns the default snapshot path for this manager. This
// will return empty if DataDir is not set. This file may not exist yet.
func (m *Manager) SnapshotPath() string {
	if m.DataDir == "" {
		return ""
	}

	return filepath.Join(m.DataDir, "snapshot.json")
}

// Snapshot will persist a snapshot of the proxy manager state that
// can be restored with Restore.
//
// If DataDir is non-empty, then the Manager will automatically snapshot
// whenever the set of managed proxies changes. This method generally doesn't
// need to be called manually.
func (m *Manager) Snapshot(path string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.snapshot(path, false)
}

// snapshot is the internal function analogous to Snapshot but expects
// a lock to already be held.
//
// checkDup when set will store the snapshot on lastSnapshot and use
// reflect.DeepEqual to verify that its not writing an identical snapshot.
func (m *Manager) snapshot(path string, checkDup bool) error {
	// Build the snapshot
	s := snapshot{
		Version: snapshotVersion,
		Proxies: make(map[string]snapshotProxy, len(m.proxies)),
	}
	for id, p := range m.proxies {
		// Get the snapshot configuration. If the configuration is nil or
		// empty then we don't persist this proxy.
		config := p.MarshalSnapshot()
		if len(config) == 0 {
			continue
		}

		s.Proxies[id] = snapshotProxy{
			Mode:   proxyExecMode(p),
			Config: config,
		}
	}

	// Dup detection, if the snapshot is identical to the last, do nothing
	if checkDup && reflect.DeepEqual(m.lastSnapshot, &s) {
		return nil
	}

	// Encode as JSON
	encoded, err := json.Marshal(&s)
	if err != nil {
		return err
	}

	// Write the file
	err = file.WriteAtomic(path, encoded)

	// If we are checking for dups and we had a successful write, store
	// it so we don't rewrite the same value.
	if checkDup && err == nil {
		m.lastSnapshot = &s
	}
	return err
}

// Restore restores the manager state from a snapshot at path. If path
// doesn't exist, this does nothing and no error is returned.
//
// This restores proxy state but does not restore any Manager configuration
// such as DataDir, Logger, etc. All of those should be set _before_ Restore
// is called.
//
// Restore must be called before Run. Restore will immediately start
// supervising the restored processes but will not sync with the local
// state store until Run is called.
//
// If an error is returned the manager state is left untouched.
func (m *Manager) Restore(path string) error {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	var s snapshot
	if err := json.Unmarshal(buf, &s); err != nil {
		return err
	}

	// Verify the version matches so we can be more confident that we're
	// decoding a structure that we expect.
	if s.Version != snapshotVersion {
		return fmt.Errorf("unknown snapshot version, expecting %d", snapshotVersion)
	}

	// Build the proxies from the snapshot
	proxies := make(map[string]Proxy, len(s.Proxies))
	for id, sp := range s.Proxies {
		p, err := m.newProxyFromMode(sp.Mode, id)
		if err != nil {
			return err
		}

		// Unmarshal the proxy. If there is an error we just continue on and
		// ignore it. Errors restoring proxies should be exceptionally rare
		// and only under scenarios where the proxy isn't running anymore or
		// we won't have permission to access it. We log and continue.
		if err := p.UnmarshalSnapshot(sp.Config); err != nil {
			m.Logger.Printf("[WARN] agent/proxy: error restoring proxy %q: %s", id, err)
			continue
		}

		proxies[id] = p
	}

	// Overwrite the proxies. The documentation notes that this will happen.
	m.lock.Lock()
	defer m.lock.Unlock()
	m.proxies = proxies
	return nil
}
