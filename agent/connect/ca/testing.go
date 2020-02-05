package ca

import (
	"fmt"
	"io/ioutil"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-testing-interface"
)

// KeyTestCases is a list of the important CA key types that we should test
// against when signing. For now leaf keys are always EC P256 but CA can be EC
// (any NIST curve) or RSA (2048, 4096). Providers must be able to complete all
// signing operations with both types that includes:
//   - Sign must be able to sign EC P256 leaf with all these types of CA key
//   - CrossSignCA must be able to sign all these types of new CA key with all
//     these types of old CA key.
//   - SignIntermediate muse bt able to sign all the types of secondary
//     intermediate CA key with all these types of primary CA key
var KeyTestCases = []struct {
	Desc    string
	KeyType string
	KeyBits int
}{
	{
		Desc:    "Default Key Type (EC 256)",
		KeyType: connect.DefaultPrivateKeyType,
		KeyBits: connect.DefaultPrivateKeyBits,
	},
	{
		Desc:    "RSA 2048",
		KeyType: "rsa",
		KeyBits: 2048,
	},
}

// CASigningKeyTypes is a struct with params for tests that sign one CA CSR with
// another CA key.
type CASigningKeyTypes struct {
	Desc           string
	SigningKeyType string
	SigningKeyBits int
	CSRKeyType     string
	CSRKeyBits     int
}

// CASigningKeyTypeCases returns the cross-product of the important supported CA
// key types for generating table tests for CA signing tests (CrossSignCA and
// SignIntermediate).
func CASigningKeyTypeCases() []CASigningKeyTypes {
	cases := make([]CASigningKeyTypes, 0, len(KeyTestCases)*len(KeyTestCases))
	for _, outer := range KeyTestCases {
		for _, inner := range KeyTestCases {
			cases = append(cases, CASigningKeyTypes{
				Desc: fmt.Sprintf("%s-%d signing %s-%d", outer.KeyType, outer.KeyBits,
					inner.KeyType, inner.KeyBits),
				SigningKeyType: outer.KeyType,
				SigningKeyBits: outer.KeyBits,
				CSRKeyType:     inner.KeyType,
				CSRKeyBits:     inner.KeyBits,
			})
		}
	}
	return cases
}

// TestConsulProvider creates a new ConsulProvider, taking care to stub out it's
// Logger so that logging calls don't panic. If logging output is important
// SetLogger can be called again with another logger to capture logs.
func TestConsulProvider(t testing.T, d ConsulProviderStateDelegate) *ConsulProvider {
	provider := &ConsulProvider{Delegate: d}
	logger := hclog.New(&hclog.LoggerOptions{
		Output: ioutil.Discard,
	})
	provider.SetLogger(logger)
	return provider
}
