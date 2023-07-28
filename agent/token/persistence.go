package token

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/consul/lib/file"
)

// Logger used by Store.Load to report warnings.
type Logger interface {
	Warn(msg string, args ...interface{})
}

// Config used by Store.Load, which includes tokens and settings for persistence.
type Config struct {
	EnablePersistence              bool
	DataDir                        string
	ACLDefaultToken                string
	ACLAgentToken                  string
	ACLAgentRecoveryToken          string
	ACLReplicationToken            string
	ACLConfigFileRegistrationToken string

	EnterpriseConfig
}

const tokensPath = "acl-tokens.json"

// Load tokens from Config and optionally from a persisted file in the cfg.DataDir.
// If a token exists in both the persisted file and in the Config a warning will
// be logged and the persisted token will be used.
//
// Failures to load the persisted file will result in loading tokens from the
// config before returning the error.
func (t *Store) Load(cfg Config, logger Logger) error {
	t.persistenceLock.RLock()
	if !cfg.EnablePersistence {
		t.persistence = nil
		t.persistenceLock.RUnlock()
		loadTokens(t, cfg, persistedTokens{}, logger)
		return nil
	}

	defer t.persistenceLock.RUnlock()
	t.persistence = &fileStore{
		filename: filepath.Join(cfg.DataDir, tokensPath),
		logger:   logger,
	}
	return t.persistence.load(t, cfg)
}

// WithPersistenceLock executes f while hold a lock. If f returns a nil error,
// the tokens in Store will be persisted to the tokens file. Otherwise no
// tokens will be persisted, and the error from f will be returned.
//
// The lock is held so that the writes are persisted before some other thread
// can change the value.
func (t *Store) WithPersistenceLock(f func() error) error {
	t.persistenceLock.Lock()
	if t.persistence == nil {
		t.persistenceLock.Unlock()
		return f()
	}
	defer t.persistenceLock.Unlock()
	return t.persistence.withPersistenceLock(t, f)
}

type persistedTokens struct {
	Replication            string `json:"replication,omitempty"`
	AgentRecovery          string `json:"agent_recovery,omitempty"`
	Default                string `json:"default,omitempty"`
	Agent                  string `json:"agent,omitempty"`
	ConfigFileRegistration string `json:"config_file_service_registration,omitempty"`
}

type fileStore struct {
	filename string
	logger   Logger
}

func (p *fileStore) load(s *Store, cfg Config) error {
	tokens, err := readPersistedFromFile(p.filename)
	if err != nil {
		p.logger.Warn("unable to load persisted tokens", "error", err)
	}
	loadTokens(s, cfg, tokens, p.logger)
	return err
}

func loadTokens(s *Store, cfg Config, tokens persistedTokens, logger Logger) {
	if tokens.Default != "" {
		s.UpdateUserToken(tokens.Default, TokenSourceAPI)

		if cfg.ACLDefaultToken != "" {
			logger.Warn("\"default\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		s.UpdateUserToken(cfg.ACLDefaultToken, TokenSourceConfig)
	}

	if tokens.Agent != "" {
		s.UpdateAgentToken(tokens.Agent, TokenSourceAPI)

		if cfg.ACLAgentToken != "" {
			logger.Warn("\"agent\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		s.UpdateAgentToken(cfg.ACLAgentToken, TokenSourceConfig)
	}

	if tokens.AgentRecovery != "" {
		s.UpdateAgentRecoveryToken(tokens.AgentRecovery, TokenSourceAPI)

		if cfg.ACLAgentRecoveryToken != "" {
			logger.Warn("\"agent_recovery\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		s.UpdateAgentRecoveryToken(cfg.ACLAgentRecoveryToken, TokenSourceConfig)
	}

	if tokens.Replication != "" {
		s.UpdateReplicationToken(tokens.Replication, TokenSourceAPI)

		if cfg.ACLReplicationToken != "" {
			logger.Warn("\"replication\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		s.UpdateReplicationToken(cfg.ACLReplicationToken, TokenSourceConfig)
	}

	if tokens.ConfigFileRegistration != "" {
		s.UpdateConfigFileRegistrationToken(tokens.ConfigFileRegistration, TokenSourceAPI)

		if cfg.ACLConfigFileRegistrationToken != "" {
			logger.Warn("\"config_file_service_registration\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		s.UpdateConfigFileRegistrationToken(cfg.ACLConfigFileRegistrationToken, TokenSourceConfig)
	}

	loadEnterpriseTokens(s, cfg)
}

func readPersistedFromFile(filename string) (persistedTokens, error) {
	var tokens struct {
		persistedTokens

		// Support reading tokens persisted by versions <1.11, where agent_master was
		// renamed to agent_recovery.
		LegacyAgentMaster string `json:"agent_master"`
	}

	buf, err := os.ReadFile(filename)
	switch {
	case os.IsNotExist(err):
		// non-existence is not an error we care about
		return tokens.persistedTokens, nil
	case err != nil:
		return tokens.persistedTokens, fmt.Errorf("failed reading tokens file %q: %w", filename, err)
	}

	if err := json.Unmarshal(buf, &tokens); err != nil {
		return tokens.persistedTokens, fmt.Errorf("failed to decode tokens file %q: %w", filename, err)
	}

	if tokens.AgentRecovery == "" {
		tokens.AgentRecovery = tokens.LegacyAgentMaster
	}

	return tokens.persistedTokens, nil
}

func (p *fileStore) withPersistenceLock(s *Store, f func() error) error {
	if err := f(); err != nil {
		return err
	}

	return p.saveToFile(s)
}

func (p *fileStore) saveToFile(s *Store) error {
	tokens := persistedTokens{}
	if tok, source := s.UserTokenAndSource(); tok != "" && source == TokenSourceAPI {
		tokens.Default = tok
	}

	if tok, source := s.AgentTokenAndSource(); tok != "" && source == TokenSourceAPI {
		tokens.Agent = tok
	}

	if tok, source := s.AgentRecoveryTokenAndSource(); tok != "" && source == TokenSourceAPI {
		tokens.AgentRecovery = tok
	}

	if tok, source := s.ReplicationTokenAndSource(); tok != "" && source == TokenSourceAPI {
		tokens.Replication = tok
	}

	if tok, source := s.ConfigFileRegistrationTokenAndSource(); tok != "" && source == TokenSourceAPI {
		tokens.ConfigFileRegistration = tok
	}

	data, err := json.Marshal(tokens)
	if err != nil {
		p.logger.Warn("failed to persist tokens", "error", err)
		return fmt.Errorf("Failed to marshal tokens for persistence: %v", err)
	}

	if err := file.WriteAtomicWithPerms(p.filename, data, 0700, 0600); err != nil {
		p.logger.Warn("failed to persist tokens", "error", err)
		return fmt.Errorf("Failed to persist tokens - %v", err)
	}
	return nil
}
