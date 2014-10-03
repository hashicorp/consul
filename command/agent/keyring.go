package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

const (
	SerfLANKeyring = "serf/local.keyring"
	SerfWANKeyring = "serf/remote.keyring"
)

// loadKeyringFile will load a gossip encryption keyring out of a file. The file
// must be in JSON format and contain a list of encryption key strings.
func loadKeyringFile(c *serf.Config) error {
	if c.KeyringFile == "" {
		return nil
	}

	if _, err := os.Stat(c.KeyringFile); err != nil {
		return err
	}

	// Read in the keyring file data
	keyringData, err := ioutil.ReadFile(c.KeyringFile)
	if err != nil {
		return err
	}

	// Decode keyring JSON
	keys := make([]string, 0)
	if err := json.Unmarshal(keyringData, &keys); err != nil {
		return err
	}

	// Decode base64 values
	keysDecoded := make([][]byte, len(keys))
	for i, key := range keys {
		keyBytes, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return err
		}
		keysDecoded[i] = keyBytes
	}

	// Create the keyring
	keyring, err := memberlist.NewKeyring(keysDecoded, keysDecoded[0])
	if err != nil {
		return err
	}

	c.MemberlistConfig.Keyring = keyring

	// Success!
	return nil
}

// keyringProcess is used to abstract away the semantic similarities in
// performing various operations on the encryption keyring.
func (a *Agent) keyringProcess(
	method string,
	args *structs.KeyringRequest) (*structs.KeyringResponses, error) {

	var reply structs.KeyringResponses
	if a.server == nil {
		return nil, fmt.Errorf("keyring operations must run against a server node")
	}
	if err := a.RPC(method, args, &reply); err != nil {
		return &reply, err
	}

	return &reply, nil
}

// ListKeys lists out all keys installed on the collective Consul cluster. This
// includes both servers and clients in all DC's.
func (a *Agent) ListKeys() (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{}
	args.AllowStale = true
	args.Operation = structs.KeyringList
	return a.keyringProcess("Internal.KeyringOperation", &args)
}

// InstallKey installs a new gossip encryption key
func (a *Agent) InstallKey(key string) (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{Key: key}
	args.AllowStale = true
	args.Operation = structs.KeyringInstall
	return a.keyringProcess("Internal.KeyringOperation", &args)
}

// UseKey changes the primary encryption key used to encrypt messages
func (a *Agent) UseKey(key string) (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{Key: key}
	args.AllowStale = true
	args.Operation = structs.KeyringUse
	return a.keyringProcess("Internal.KeyringOperation", &args)
}

// RemoveKey will remove a gossip encryption key from the keyring
func (a *Agent) RemoveKey(key string) (*structs.KeyringResponses, error) {
	args := structs.KeyringRequest{Key: key}
	args.AllowStale = true
	args.Operation = structs.KeyringRemove
	return a.keyringProcess("Internal.KeyringOperation", &args)
}
