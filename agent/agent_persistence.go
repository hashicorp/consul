package agent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib/file"
	"github.com/hashicorp/consul/types"
)

const (
	// Path to save agent service definitions
	servicesDir      = "services"
	serviceConfigDir = "services/configs"

	// Path to save local agent checks
	checksDir     = "checks"
	checkStateDir = "checks/state"

	// Name of the file tokens will be persisted within
	tokensPath = "acl-tokens.json"
)

// persistedService is used to wrap a service definition and bundle it
// with an ACL token so we can restore both at a later agent start.
type persistedService struct {
	Token   string
	Service *structs.NodeService
	Source  string

	File string `json:"-"`
}

// persistService saves a service definition to a JSON file in the data dir
func (a *Agent) persistService(service *structs.NodeService, source configSource) error {
	svcPath := filepath.Join(a.config.DataDir, servicesDir, stringHash(service.ID))

	wrapped := persistedService{
		Token:   a.State.ServiceToken(service.ID),
		Service: service,
		Source:  source.String(),
	}
	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	return file.WriteAtomic(svcPath, encoded)
}

// purgeService removes a persisted service definition file from the data dir
func (a *Agent) purgeService(serviceID string) error {
	svcPath := filepath.Join(a.config.DataDir, servicesDir, stringHash(serviceID))
	if _, err := os.Stat(svcPath); err == nil {
		return os.Remove(svcPath)
	}
	return nil
}

func (a *Agent) readPersistedServices() ([]*persistedService, error) {
	var out []*persistedService

	// Load any persisted services
	svcDir := filepath.Join(a.config.DataDir, servicesDir)
	files, err := ioutil.ReadDir(svcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed reading services dir %q: %s", svcDir, err)
	}

	for _, fi := range files {
		// Skip all dirs
		if fi.IsDir() {
			continue
		}

		// Skip all partially written temporary files
		if strings.HasSuffix(fi.Name(), "tmp") {
			a.logger.Printf("[WARN] agent: Ignoring temporary service file %v", fi.Name())
			continue
		}

		// Read the contents into a buffer
		file := filepath.Join(svcDir, fi.Name())
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed reading service file %q: %s", file, err)
		}

		// Try decoding the service definition
		var p persistedService
		if err := json.Unmarshal(buf, &p); err != nil {
			// Backwards-compatibility for pre-0.5.1 persisted services
			if err := json.Unmarshal(buf, &p.Service); err != nil {
				a.logger.Printf("[ERR] agent: Failed decoding service file %q: %s", file, err)
				continue
			}
		}
		p.File = file

		out = append(out, &p)
	}

	return out, nil
}

// persistedCheck is used to serialize a check and write it to disk
// so that it may be restored later on.
type persistedCheck struct {
	Check   *structs.HealthCheck
	ChkType *structs.CheckType
	Token   string
	Source  string

	File string `json:"-"`
}

// persistCheck saves a check definition to the local agent's state directory
func (a *Agent) persistCheck(check *structs.HealthCheck, chkType *structs.CheckType, source configSource) error {
	checkPath := filepath.Join(a.config.DataDir, checksDir, checkIDHash(check.CheckID))

	// Create the persisted check
	wrapped := persistedCheck{
		Check:   check,
		ChkType: chkType,
		Token:   a.State.CheckToken(check.CheckID),
		Source:  source.String(),
	}

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	return file.WriteAtomic(checkPath, encoded)
}

// purgeCheck removes a persisted check definition file from the data dir
func (a *Agent) purgeCheck(checkID types.CheckID) error {
	checkPath := filepath.Join(a.config.DataDir, checksDir, checkIDHash(checkID))
	if _, err := os.Stat(checkPath); err == nil {
		return os.Remove(checkPath)
	}
	return nil
}

// persistedCheckState is used to persist the current state of a given
// check. This is different from the check definition, and includes an
// expiration timestamp which is used to determine staleness on later
// agent restarts.
type persistedCheckState struct {
	CheckID types.CheckID
	Output  string
	Status  string
	Expires int64
}

// persistCheckState is used to record the check status into the data dir.
// This allows the state to be restored on a later agent start. Currently
// only useful for TTL based checks.
func (a *Agent) persistCheckState(check *checks.CheckTTL, status, output string) error {
	// Create the persisted state
	state := persistedCheckState{
		CheckID: check.CheckID,
		Status:  status,
		Output:  output,
		Expires: time.Now().Add(check.TTL).Unix(),
	}

	// Encode the state
	buf, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Create the state dir if it doesn't exist
	dir := filepath.Join(a.config.DataDir, checkStateDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating check state dir %q: %s", dir, err)
	}

	// Write the state to the file
	file := filepath.Join(dir, checkIDHash(check.CheckID))

	// Create temp file in same dir, to make more likely atomic
	tempFile := file + ".tmp"

	// persistCheckState is called frequently, so don't use writeFileAtomic to avoid calling fsync here
	if err := ioutil.WriteFile(tempFile, buf, 0600); err != nil {
		return fmt.Errorf("failed writing temp file %q: %s", tempFile, err)
	}
	if err := os.Rename(tempFile, file); err != nil {
		return fmt.Errorf("failed to rename temp file from %q to %q: %s", tempFile, file, err)
	}

	return nil
}

// loadCheckState is used to restore the persisted state of a check.
func (a *Agent) loadCheckState(check *structs.HealthCheck) error {
	// Try to read the persisted state for this check
	file := filepath.Join(a.config.DataDir, checkStateDir, checkIDHash(check.CheckID))
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed reading file %q: %s", file, err)
	}

	// Decode the state data
	var p persistedCheckState
	if err := json.Unmarshal(buf, &p); err != nil {
		a.logger.Printf("[ERR] agent: failed decoding check state: %s", err)
		return a.purgeCheckState(check.CheckID)
	}

	// Check if the state has expired
	if time.Now().Unix() >= p.Expires {
		a.logger.Printf("[DEBUG] agent: check state expired for %q, not restoring", check.CheckID)
		return a.purgeCheckState(check.CheckID)
	}

	// Restore the fields from the state
	check.Output = p.Output
	check.Status = p.Status
	return nil
}

// purgeCheckState is used to purge the state of a check from the data dir
func (a *Agent) purgeCheckState(checkID types.CheckID) error {
	file := filepath.Join(a.config.DataDir, checkStateDir, checkIDHash(checkID))
	err := os.Remove(file)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (a *Agent) readPersistedChecks() ([]*persistedCheck, error) {
	var out []*persistedCheck

	// Load any persisted checks
	checkDir := filepath.Join(a.config.DataDir, checksDir)
	files, err := ioutil.ReadDir(checkDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed reading checks dir %q: %s", checkDir, err)
	}

	for _, fi := range files {
		// Ignore dirs - we only care about the check definition files
		if fi.IsDir() {
			continue
		}

		// Skip all partially written temporary files
		if strings.HasSuffix(fi.Name(), "tmp") {
			a.logger.Printf("[WARN] agent: Ignoring temporary check file %v", fi.Name())
			continue
		}

		// Read the contents into a buffer
		file := filepath.Join(checkDir, fi.Name())
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed reading check file %q: %s", file, err)
		}

		// Decode the check
		var p persistedCheck
		if err := json.Unmarshal(buf, &p); err != nil {
			a.logger.Printf("[ERR] agent: Failed decoding check file %q: %s", file, err)
			continue
		}
		p.File = file

		out = append(out, &p)
	}

	return out, nil
}

// persistedServiceConfig is used to serialize the resolved service config that
// feeds into the ServiceManager at registration time so that it may be
// restored later on.
type persistedServiceConfig struct {
	ServiceID string
	Defaults  *structs.ServiceConfigResponse
}

func (a *Agent) persistServiceConfig(serviceID string, defaults *structs.ServiceConfigResponse) error {
	// Create the persisted config.
	wrapped := persistedServiceConfig{
		ServiceID: serviceID,
		Defaults:  defaults,
	}

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	dir := filepath.Join(a.config.DataDir, serviceConfigDir)
	configPath := filepath.Join(dir, stringHash(serviceID))

	// Create the config dir if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating service configs dir %q: %s", dir, err)
	}

	return file.WriteAtomic(configPath, encoded)
}

func (a *Agent) purgeServiceConfig(serviceID string) error {
	configPath := filepath.Join(a.config.DataDir, serviceConfigDir, stringHash(serviceID))
	if _, err := os.Stat(configPath); err == nil {
		return os.Remove(configPath)
	}
	return nil
}

func (a *Agent) readPersistedServiceConfigs() (map[string]*structs.ServiceConfigResponse, error) {
	out := make(map[string]*structs.ServiceConfigResponse)

	configDir := filepath.Join(a.config.DataDir, serviceConfigDir)
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed reading service configs dir %q: %s", configDir, err)
	}

	for _, fi := range files {
		// Skip all dirs
		if fi.IsDir() {
			continue
		}

		// Skip all partially written temporary files
		if strings.HasSuffix(fi.Name(), "tmp") {
			a.logger.Printf("[WARN] agent: Ignoring temporary service config file %v", fi.Name())
			continue
		}

		// Read the contents into a buffer
		file := filepath.Join(configDir, fi.Name())
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed reading service config file %q: %s", file, err)
		}

		// Try decoding the service config definition
		var p persistedServiceConfig
		if err := json.Unmarshal(buf, &p); err != nil {
			a.logger.Printf("[ERR] agent: Failed decoding service config file %q: %s", file, err)
			continue
		}
		out[p.ServiceID] = p.Defaults
	}

	return out, nil
}

type persistedTokens struct {
	Replication string `json:"replication,omitempty"`
	AgentMaster string `json:"agent_master,omitempty"`
	Default     string `json:"default,omitempty"`
	Agent       string `json:"agent,omitempty"`
}

func (a *Agent) getPersistedTokens() (*persistedTokens, error) {
	persistedTokens := &persistedTokens{}
	if !a.config.ACLEnableTokenPersistence {
		return persistedTokens, nil
	}

	a.persistedTokensLock.RLock()
	defer a.persistedTokensLock.RUnlock()

	tokensFullPath := filepath.Join(a.config.DataDir, tokensPath)

	buf, err := ioutil.ReadFile(tokensFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// non-existence is not an error we care about
			return persistedTokens, nil
		}
		return persistedTokens, fmt.Errorf("failed reading tokens file %q: %s", tokensFullPath, err)
	}

	if err := json.Unmarshal(buf, persistedTokens); err != nil {
		return persistedTokens, fmt.Errorf("failed to decode tokens file %q: %s", tokensFullPath, err)
	}

	return persistedTokens, nil
}

func (a *Agent) loadTokens(conf *config.RuntimeConfig) error {
	persistedTokens, persistenceErr := a.getPersistedTokens()

	if persistenceErr != nil {
		a.logger.Printf("[WARN] unable to load persisted tokens: %v", persistenceErr)
	}

	if persistedTokens.Default != "" {
		a.tokens.UpdateUserToken(persistedTokens.Default, token.TokenSourceAPI)

		if conf.ACLToken != "" {
			a.logger.Printf("[WARN] \"default\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateUserToken(conf.ACLToken, token.TokenSourceConfig)
	}

	if persistedTokens.Agent != "" {
		a.tokens.UpdateAgentToken(persistedTokens.Agent, token.TokenSourceAPI)

		if conf.ACLAgentToken != "" {
			a.logger.Printf("[WARN] \"agent\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateAgentToken(conf.ACLAgentToken, token.TokenSourceConfig)
	}

	if persistedTokens.AgentMaster != "" {
		a.tokens.UpdateAgentMasterToken(persistedTokens.AgentMaster, token.TokenSourceAPI)

		if conf.ACLAgentMasterToken != "" {
			a.logger.Printf("[WARN] \"agent_master\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateAgentMasterToken(conf.ACLAgentMasterToken, token.TokenSourceConfig)
	}

	if persistedTokens.Replication != "" {
		a.tokens.UpdateReplicationToken(persistedTokens.Replication, token.TokenSourceAPI)

		if conf.ACLReplicationToken != "" {
			a.logger.Printf("[WARN] \"replication\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateReplicationToken(conf.ACLReplicationToken, token.TokenSourceConfig)
	}

	return persistenceErr
}
