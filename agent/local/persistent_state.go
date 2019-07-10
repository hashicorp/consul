package local

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/file"
	"github.com/hashicorp/consul/types"
)

const (
	// Path to save agent service definitions
	servicesDir = "services"

	// Path to save agent proxy definitions
	proxyDir = "proxies"

	// Path to save local agent checks
	checksDir     = "checks"
	checkStateDir = "checks/state"

	retryInterval = time.Minute
)

// PersistedService is used to wrap a service definition and bundle it
// with an ACL token so we can restore both at a later agent start.
type PersistedService struct {
	Token   string
	Service *structs.NodeService
}

// PersistedCheck is used to serialize a check and write it to disk
// so that it may be restored later on.
type PersistedCheck struct {
	Check   *structs.HealthCheck
	ChkType *structs.CheckType
	Token   string
}

// PersistedProxy is used to wrap a proxy definition and bundle it with an Proxy
// token so we can continue to authenticate the running proxy after a restart.
type PersistedProxy struct {
	ProxyToken string
	Proxy      *structs.ConnectManagedProxy

	// Set to true when the proxy information originated from the agents configuration
	// as opposed to API registration.
	FromFile bool
}

// PersistedCheckState is used to persist the current state of a given
// check. This is different from the check definition, and includes an
// expiration timestamp which is used to determine staleness on later
// agent restarts.
type PersistedCheckState struct {
	CheckID types.CheckID
	Output  string
	Status  string
	Expires int64
}

type peristentOp func() error

// stringHash returns a simple md5sum for a string.
func stringHash(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

// PersistentState manages the agent state on the filesystem and ensure
// its data is eventually written even in the case of errors
type PersistentState struct {
	baseDir  string
	lock     sync.Mutex
	ops      map[string]peristentOp
	sync     chan struct{}
	syncDone chan struct{}
	logger   *log.Logger
}

// NewPersistentState created a new PersistentState
func NewPersistentState(logger *log.Logger, baseDir string) *PersistentState {
	s := &PersistentState{
		baseDir:  baseDir,
		ops:      make(map[string]peristentOp),
		sync:     make(chan struct{}, 1),
		syncDone: make(chan struct{}, 1),
		logger:   logger,
	}

	go func() {
		for range s.sync {
			ok := s.doSync()
			if !ok {
				time.AfterFunc(retryInterval, s.triggerSync)
			}
		}
	}()

	return s
}

// WriteService writes a service definition to the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) WriteService(def PersistedService) {
	s.write(filepath.Join(s.baseDir, servicesDir, stringHash(def.Service.ID)), def, true)
}

// RemoveService removes a service definition from the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) RemoveService(id string) {
	s.remove(filepath.Join(s.baseDir, servicesDir, stringHash(id)))
}

// WriteCheck writes a check definition to the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) WriteCheck(def PersistedCheck) {
	s.write(filepath.Join(s.baseDir, checksDir, stringHash((string(def.Check.CheckID)))), def, true)
}

// RemoveCheck removes a check definition from the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) RemoveCheck(id types.CheckID) {
	s.remove(filepath.Join(s.baseDir, checksDir, stringHash((string(id)))))
}

// WriteCheckState writes a check state to the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) WriteCheckState(def PersistedCheckState) {
	s.write(filepath.Join(s.baseDir, checkStateDir, stringHash((string(def.CheckID)))), def, false)
}

// RemoveCheckState removes a check state from the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) RemoveCheckState(id types.CheckID) {
	s.remove(filepath.Join(s.baseDir, checkStateDir, stringHash((string(id)))))
}

// WriteProxy writes a proxy state to the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) WriteProxy(def PersistedProxy) {
	s.write(filepath.Join(s.baseDir, proxyDir, stringHash(def.Proxy.ProxyService.ID)), def, true)
}

// RemoveProxy removes a proxy state from the state. The call returns immeditely and the actual
// disk write will happen in background and retry if an error occurs.
func (s *PersistentState) RemoveProxy(id string) {
	s.remove(filepath.Join(s.baseDir, proxyDir, stringHash(id)))
}

// LoadCheckState load a check state from disk
func (s *PersistentState) LoadCheckState(id types.CheckID) (*PersistedCheckState, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	file := filepath.Join(s.baseDir, checkStateDir, stringHash(string(id)))
	fh, err := os.Open(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	// Decode the state data
	var p PersistedCheckState
	err = json.NewDecoder(fh).Decode(&p)
	if err != nil {
		s.RemoveCheckState(id)
		return nil, fmt.Errorf("failed decoding check state: %s", err)
	}

	return &p, nil
}

// LoadServices loads all services definitions from disk. If corrupted definitions
// are found they will be deleted and ignored.
func (s *PersistentState) LoadServices() ([]PersistedService, error) {
	entries := []PersistedService{}

	err := s.load(filepath.Join(s.baseDir, servicesDir), func(f io.Reader) error {
		buf, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		// Try decoding the service definition
		var p PersistedService
		if err := json.Unmarshal(buf, &p); err != nil {
			// Backwards-compatibility for pre-0.5.1 persisted services
			if err := json.Unmarshal(buf, &p.Service); err != nil {
				return err
			}
		}
		entries = append(entries, p)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// LoadChecks loads all check definitions from disk. If corrupted definitions
// are found they will be deleted and ignored.
func (s *PersistentState) LoadChecks() ([]PersistedCheck, error) {
	entries := []PersistedCheck{}

	err := s.load(filepath.Join(s.baseDir, checksDir), func(f io.Reader) error {
		var p PersistedCheck
		err := json.NewDecoder(f).Decode(&p)
		if err != nil {
			return err
		}
		entries = append(entries, p)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// LoadProxies loads all proxy definitions from disk. If corrupted definitions
// are found they will be deleted and ignored.
func (s *PersistentState) LoadProxies() ([]PersistedProxy, error) {
	entries := []PersistedProxy{}

	err := s.load(filepath.Join(s.baseDir, proxyDir), func(f io.Reader) error {
		var p PersistedProxy
		err := json.NewDecoder(f).Decode(&p)
		if err != nil {
			return err
		}
		entries = append(entries, p)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (s *PersistentState) waitSync() {
	<-s.syncDone
}

func (s *PersistentState) load(path string, cb func(io.Reader) error) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	files, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed reading dir: %s", err)
	}

	for _, fi := range files {
		// Skip all dirs
		if fi.IsDir() {
			continue
		}

		// Skip all partially written temporary files
		if strings.HasSuffix(fi.Name(), "tmp") {
			continue
		}

		// Open the file for reading
		file := filepath.Join(path, fi.Name())
		fh, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed opening file %q: %s", file, err)
		}
		defer fh.Close()

		err = cb(fh)
		if err != nil {
			s.logger.Printf("[ERR] agent: persistent state: failed decoding file %q, removing: %s", file, err)
			err := os.Remove(file)
			if err != nil {
				s.logger.Printf("[ERR] agent: persistent state: failed removing file %q: %s", file, err)
			}
			continue
		}
	}

	return nil
}

func (s *PersistentState) write(path string, data interface{}, atomic bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.clearWaitSync()

	if _, ok := s.ops[path]; ok {
		s.logger.Printf("[WARN] agent: persistent state: dropping peristent state op for path %s", path)
	}

	s.ops[path] = s.writeOp(path, data, atomic)
	s.triggerSync()
}

func (s *PersistentState) remove(path string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.clearWaitSync()

	if _, ok := s.ops[path]; ok {
		s.logger.Printf("[WARN] agent: persistent state: dropping peristent state op for path %s", path)
	}

	s.ops[path] = s.removeOp(path)
	s.triggerSync()
}

func (s *PersistentState) clearWaitSync() {
	select {
	case <-s.syncDone:
	default:
	}
}

func (s *PersistentState) triggerSync() {
	select {
	case s.sync <- struct{}{}:
	default:
	}
}

func (s *PersistentState) doSync() bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.ops) == 0 {
		return true
	}

	ok := true
	for k, op := range s.ops {
		err := op()
		if err != nil {
			s.logger.Printf("[ERR] agent: persistent state: error during sync: %s, will retry", err)
			ok = false
			continue
		}
		delete(s.ops, k)
	}

	select {
	case s.syncDone <- struct{}{}:
	default:
	}

	return ok
}

func (s *PersistentState) removeOp(path string) peristentOp {
	return func() error {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		s.logger.Printf("[DEBUG] agent: persistent state: removed %s", path)
		return nil
	}
}

func (s *PersistentState) writeOp(path string, data interface{}, atomic bool) peristentOp {
	return func() error {
		enc, err := json.Marshal(data)
		if err != nil {
			return err
		}

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed creating state dir %q: %s", dir, err)
		}

		if atomic {
			err := file.WriteAtomicWithPerms(path, enc, 0600)
			if err != nil {
				return err
			}
		} else {
			tempFile := path + ".tmp"
			// persistCheckState is called frequently, so don't use writeFileAtomic to avoid calling fsync here
			if err := ioutil.WriteFile(tempFile, enc, 0600); err != nil {
				return fmt.Errorf("failed writing temp file %q: %s", tempFile, err)
			}
			if err := os.Rename(tempFile, path); err != nil {
				return fmt.Errorf("failed to rename temp file from %q to %q: %s", tempFile, path, err)
			}
		}

		s.logger.Printf("[DEBUG] agent: persistent state: wrote %s", path)

		return nil
	}
}
