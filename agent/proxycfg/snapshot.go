package proxycfg

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/copystructure"
)

// ConfigSnapshot captures all the resulting config needed for a proxy instance.
// It is meant to be point-in-time coherent and is used to deliver the current
// config state to observers who need it to be pushed in (e.g. XDS server).
type ConfigSnapshot struct {
	ProxyID           string
	Address           string
	Port              int
	Proxy             structs.ConnectProxyConfig
	Roots             *structs.IndexedCARoots
	Leaf              *structs.IssuedCert
	UpstreamEndpoints map[string]structs.CheckServiceNodes

	// Skip intentions for now as we don't push those down yet, just pre-warm them.
}

// Valid returns whether or not the snapshot has all required fields filled yet.
func (s *ConfigSnapshot) Valid() bool {
	return s.Roots != nil && s.Leaf != nil
}

// Clone makes a deep copy of the snapshot we can send to other goroutines
// without worrying that they will racily read or mutate shared maps etc.
func (s *ConfigSnapshot) Clone() (*ConfigSnapshot, error) {
	snapCopy, err := copystructure.Copy(s)
	if err != nil {
		return nil, err
	}
	return snapCopy.(*ConfigSnapshot), nil
}
