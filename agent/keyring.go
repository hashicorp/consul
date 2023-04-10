// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

const (
	SerfLANKeyring = "serf/local.keyring"
	SerfWANKeyring = "serf/remote.keyring"
)

// setupKeyrings in config.SerfLANConfig and config.SerfWANConfig.
func setupKeyrings(config *consul.Config, rtConfig *config.RuntimeConfig, logger hclog.Logger) error {
	// First set up the LAN and WAN keyrings.
	if err := setupBaseKeyrings(config, rtConfig, logger); err != nil {
		return err
	}

	// If there's no LAN keyring then there's nothing else to set up for
	// any segments.
	lanKeyring := config.SerfLANConfig.MemberlistConfig.Keyring
	if lanKeyring == nil {
		return nil
	}

	// Copy the initial state of the LAN keyring into each segment config.
	// Segments don't have their own keyring file, they rely on the LAN
	// holding the state so things can't get out of sync.
	k, pk := lanKeyring.GetKeys(), lanKeyring.GetPrimaryKey()
	for _, segment := range config.Segments {
		keyring, err := memberlist.NewKeyring(k, pk)
		if err != nil {
			return err
		}
		segment.SerfConfig.MemberlistConfig.Keyring = keyring
	}
	return nil
}

// setupBaseKeyrings configures the LAN and WAN keyrings.
func setupBaseKeyrings(config *consul.Config, rtConfig *config.RuntimeConfig, logger hclog.Logger) error {
	// If the keyring file is disabled then just poke the provided key
	// into the in-memory keyring.
	federationEnabled := config.SerfWANConfig != nil
	if rtConfig.DisableKeyringFile {
		if rtConfig.EncryptKey == "" {
			return nil
		}

		keys := []string{rtConfig.EncryptKey}
		if err := loadKeyring(config.SerfLANConfig, keys); err != nil {
			return err
		}
		if rtConfig.ServerMode && federationEnabled {
			if err := loadKeyring(config.SerfWANConfig, keys); err != nil {
				return err
			}
		}
		return nil
	}

	// Otherwise, we need to deal with the keyring files.
	fileLAN := filepath.Join(rtConfig.DataDir, SerfLANKeyring)
	fileWAN := filepath.Join(rtConfig.DataDir, SerfWANKeyring)

	var existingLANKeyring, existingWANKeyring bool
	if rtConfig.EncryptKey == "" {
		goto LOAD
	}
	if _, err := os.Stat(fileLAN); err != nil {
		if err := initKeyring(fileLAN, rtConfig.EncryptKey); err != nil {
			return err
		}
	} else {
		existingLANKeyring = true
	}
	if rtConfig.ServerMode && federationEnabled {
		if _, err := os.Stat(fileWAN); err != nil {
			if err := initKeyring(fileWAN, rtConfig.EncryptKey); err != nil {
				return err
			}
		} else {
			existingWANKeyring = true
		}
	}

LOAD:
	if _, err := os.Stat(fileLAN); err == nil {
		config.SerfLANConfig.KeyringFile = fileLAN
	}
	if err := loadKeyringFile(config.SerfLANConfig); err != nil {
		return err
	}
	if rtConfig.ServerMode && federationEnabled {
		if _, err := os.Stat(fileWAN); err == nil {
			config.SerfWANConfig.KeyringFile = fileWAN
		}
		if err := loadKeyringFile(config.SerfWANConfig); err != nil {
			return err
		}
	}

	// Only perform the following checks if there was an encrypt_key
	// provided in the configuration.
	if rtConfig.EncryptKey != "" {
		msg := " keyring doesn't include key provided with -encrypt, using keyring"
		if existingLANKeyring &&
			keyringIsMissingKey(
				config.SerfLANConfig.MemberlistConfig.Keyring,
				rtConfig.EncryptKey,
			) {
			logger.Warn(msg, "keyring", "LAN")
		}
		if existingWANKeyring &&
			keyringIsMissingKey(
				config.SerfWANConfig.MemberlistConfig.Keyring,
				rtConfig.EncryptKey,
			) {
			logger.Warn(msg, "keyring", "WAN")
		}
	}

	return nil
}

// initKeyring will create a keyring file at a given path.
func initKeyring(path, key string) error {
	var keys []string

	if keyBytes, err := decodeStringKey(key); err != nil {
		return fmt.Errorf("Invalid key: %s", err)
	} else if err := memberlist.ValidateKey(keyBytes); err != nil {
		return fmt.Errorf("Invalid key: %s", err)
	}

	// Just exit if the file already exists.
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	keys = append(keys, key)
	keyringBytes, err := json.Marshal(keys)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer fh.Close()

	if _, err := fh.Write(keyringBytes); err != nil {
		os.Remove(path)
		return err
	}

	return nil
}

// loadKeyringFile will load a gossip encryption keyring out of a file. The file
// must be in JSON format and contain a list of encryption key strings.
func loadKeyringFile(c *serf.Config) error {
	if c.KeyringFile == "" {
		return nil
	}

	if _, err := os.Stat(c.KeyringFile); err != nil {
		return err
	}

	keyringData, err := os.ReadFile(c.KeyringFile)
	if err != nil {
		return err
	}

	keys := make([]string, 0)
	if err := json.Unmarshal(keyringData, &keys); err != nil {
		return err
	}

	return loadKeyring(c, keys)
}

// loadKeyring takes a list of base64-encoded strings and installs them in the
// given Serf's keyring.
func loadKeyring(c *serf.Config, keys []string) error {
	keysDecoded := make([][]byte, len(keys))
	for i, key := range keys {
		keyBytes, err := decodeStringKey(key)
		if err != nil {
			return err
		}
		keysDecoded[i] = keyBytes
	}

	if len(keysDecoded) == 0 {
		return fmt.Errorf("no keys present in keyring: %s", c.KeyringFile)
	}

	keyring, err := memberlist.NewKeyring(keysDecoded, keysDecoded[0])
	if err != nil {
		return err
	}

	c.MemberlistConfig.Keyring = keyring
	return nil
}

func decodeStringKey(key string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(key)
}

// keyringProcess is used to abstract away the semantic similarities in
// performing various operations on the encryption keyring.
func (a *Agent) keyringProcess(args *structs.KeyringRequest) (*structs.KeyringResponses, error) {
	var reply structs.KeyringResponses

	if err := a.RPC(context.Background(), "Internal.KeyringOperation", args, &reply); err != nil {
		return &reply, err
	}

	return &reply, nil
}

// ParseRelayFactor validates and converts the given relay factor to uint8
func ParseRelayFactor(n int) (uint8, error) {
	if n < 0 || n > 5 {
		return 0, fmt.Errorf("Relay factor must be in range: [0, 5]")
	}
	return uint8(n), nil
}

// ValidateLocalOnly validates the local-only flag, requiring that it only be
// set for list requests.
func ValidateLocalOnly(local bool, list bool) error {
	if local && !list {
		return fmt.Errorf("local-only can only be set for list requests")
	}
	return nil
}

// ListKeys lists out all keys installed on the collective Consul cluster. This
// includes both servers and clients in all DC's.
func (a *Agent) ListKeys(token string, localOnly bool, relayFactor uint8) (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{Operation: structs.KeyringList, LocalOnly: localOnly}
	parseKeyringRequest(&args, token, relayFactor)
	return a.keyringProcess(&args)
}

// InstallKey installs a new gossip encryption key
func (a *Agent) InstallKey(key, token string, relayFactor uint8) (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{Key: key, Operation: structs.KeyringInstall}
	parseKeyringRequest(&args, token, relayFactor)
	return a.keyringProcess(&args)
}

// UseKey changes the primary encryption key used to encrypt messages
func (a *Agent) UseKey(key, token string, relayFactor uint8) (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{Key: key, Operation: structs.KeyringUse}
	parseKeyringRequest(&args, token, relayFactor)
	return a.keyringProcess(&args)
}

// RemoveKey will remove a gossip encryption key from the keyring
func (a *Agent) RemoveKey(key, token string, relayFactor uint8) (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{Key: key, Operation: structs.KeyringRemove}
	parseKeyringRequest(&args, token, relayFactor)
	return a.keyringProcess(&args)
}

func parseKeyringRequest(req *structs.KeyringRequest, token string, relayFactor uint8) {
	req.Token = token
	req.RelayFactor = relayFactor
}

// keyringIsMissingKey checks whether a key is part of a keyring. Returns true
// if it is not included.
func keyringIsMissingKey(keyring *memberlist.Keyring, key string) bool {
	k1, err := decodeStringKey(key)
	if err != nil {
		return true
	}
	for _, k2 := range keyring.GetKeys() {
		if bytes.Equal(k1, k2) {
			return false
		}
	}
	return true
}
