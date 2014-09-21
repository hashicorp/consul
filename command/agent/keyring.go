package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

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

// ListKeysLAN returns the keys installed on the LAN gossip pool
func (a *Agent) ListKeysLAN() (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerLAN()
		return km.ListKeys()
	}
	km := a.client.KeyManagerLAN()
	return km.ListKeys()
}

// ListKeysWAN returns the keys installed on the WAN gossip pool
func (a *Agent) ListKeysWAN() (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerWAN()
		return km.ListKeys()
	}
	return nil, fmt.Errorf("WAN keyring not available on client node")
}

// InstallKeyWAN installs a new WAN gossip encryption key on server nodes
func (a *Agent) InstallKeyWAN(key string) (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerWAN()
		return km.InstallKey(key)
	}
	return nil, fmt.Errorf("WAN keyring not available on client node")
}

// InstallKeyLAN installs a new LAN gossip encryption key on all nodes
func (a *Agent) InstallKeyLAN(key string) (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerLAN()
		return km.InstallKey(key)
	}
	km := a.client.KeyManagerLAN()
	return km.InstallKey(key)
}

// UseKeyWAN changes the primary WAN gossip encryption key on server nodes
func (a *Agent) UseKeyWAN(key string) (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerWAN()
		return km.UseKey(key)
	}
	return nil, fmt.Errorf("WAN keyring not available on client node")
}

// UseKeyLAN changes the primary LAN gossip encryption key on all nodes
func (a *Agent) UseKeyLAN(key string) (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerLAN()
		return km.UseKey(key)
	}
	km := a.client.KeyManagerLAN()
	return km.UseKey(key)
}

// RemoveKeyWAN removes a WAN gossip encryption key on server nodes
func (a *Agent) RemoveKeyWAN(key string) (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerWAN()
		return km.RemoveKey(key)
	}
	return nil, fmt.Errorf("WAN keyring not available on client node")
}

// RemoveKeyLAN removes a LAN gossip encryption key on all nodes
func (a *Agent) RemoveKeyLAN(key string) (*serf.KeyResponse, error) {
	if a.server != nil {
		km := a.server.KeyManagerLAN()
		return km.RemoveKey(key)
	}
	km := a.client.KeyManagerLAN()
	return km.RemoveKey(key)
}
