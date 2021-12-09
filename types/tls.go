package types

import (
	"fmt"
)

// TLSVersion is a strongly-typed string for TLS versions
type TLSVersion string

const (
	// Error value, excluded from lookup maps
	TLSVersionInvalid TLSVersion = "TLS_INVALID"

	// Explicit unspecified zero-value to avoid overwriting parent defaults
	TLSVersionUnspecified TLSVersion = ""

	// Explictly allow implementation to select TLS version
	// May be useful to supercede defaults specified at a higher layer
	TLSVersionAuto TLSVersion = "TLS_AUTO"

	_ // Placeholder for SSLv3, hopefully we won't have to add this

	// TLS versions
	TLSv1_0 TLSVersion = "TLSv1_0"
	TLSv1_1 TLSVersion = "TLSv1_1"
	TLSv1_2 TLSVersion = "TLSv1_2"
	TLSv1_3 TLSVersion = "TLSv1_3"
)

var (
	tlsVersions = map[string]TLSVersion{
		"TLS_AUTO": TLSVersionAuto,
		"TLSv1_0":  TLSv1_0,
		"TLSv1_1":  TLSv1_1,
		"TLSv1_2":  TLSv1_2,
		"TLSv1_3":  TLSv1_3,
	}
	// NOTE: This interface is deprecated in favor of tlsVersions
	// and should be eventually removed in a future release.
	DeprecatedConsulAgentTLSVersions = map[string]TLSVersion{
		"":      TLSVersionAuto,
		"tls10": TLSv1_0,
		"tls11": TLSv1_1,
		"tls12": TLSv1_2,
		"tls13": TLSv1_3,
	}
	// NOTE: these currently map to the deprecated config strings to support the
	// deployment pattern of upgrading servers first. This map should eventually
	// be removed and any lookups updated to instead use the TLSVersion string
	// values directly in a future release.
	ConsulAutoConfigTLSVersionStrings = map[TLSVersion]string{
		TLSVersionAuto: "",
		TLSv1_0:        "tls10",
		TLSv1_1:        "tls11",
		TLSv1_2:        "tls12",
		TLSv1_3:        "tls13",
	}
)

func (v *TLSVersion) String() string {
	return fmt.Sprintf("%s", *v)
}

func ValidateTLSVersion(v TLSVersion) error {
	if _, ok := tlsVersions[v.String()]; !ok {
		return fmt.Errorf("no matching TLS version found for %s", v.String())
	}

	return nil
}

// IANA cipher suite string constants as defined at
// https://www.iana.org/assignments/tls-parameters/tls-parameters.xhtml
// This is the total list of TLS 1.2-style cipher suites
// which are currently supported by either Envoy 1.21 or the Consul agent
// via Go, and may change as some older suites are removed in future
// Envoy releases and Consul drops support for older Envoy versions,
// and as supported cipher suites in the Go runtime change.
//
// The naming convention for cipher suites changed in TLS 1.3
// but constant values should still be globally unqiue.
//
// Handling validation on distinct sets of TLS 1.3 and TLS 1.2 TLSCipherSuite
// constants would be a future exercise if cipher suites for TLS 1.3 ever
// become configurable in BoringSSL, Envoy, or other implementation.
type TLSCipherSuite string

const (
	// Cipher suites used by Envoy but not used by Consul agent yet
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 = "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256"
	TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256   = "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"

	// Cipher suites used by both Envoy and Consul agent
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 = "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256   = "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA    = "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA"
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA      = "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA"
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 = "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384   = "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA    = "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA"
	TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA      = "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA"

	// Older cipher suites not supported for Consul agent TLS,
	// will eventually be removed from Envoy defaults
	TLS_RSA_WITH_AES_128_GCM_SHA256 = "TLS_RSA_WITH_AES_128_GCM_SHA256"
	TLS_RSA_WITH_AES_128_CBC_SHA    = "TLS_RSA_WITH_AES_128_CBC_SHA"
	TLS_RSA_WITH_AES_256_GCM_SHA384 = "TLS_RSA_WITH_AES_256_GCM_SHA384"
	TLS_RSA_WITH_AES_256_CBC_SHA    = "TLS_RSA_WITH_AES_256_CBC_SHA"

	// Additional cipher suites used by Consul agent but not Envoy
	// TODO: these are both explicitly listed as insecure and disabled in the Go source, should they be removed?
	// https://cs.opensource.google/go/go/+/refs/tags/go1.17.3:src/crypto/tls/cipher_suites.go;l=329-330
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256 = "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256"
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256   = "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256"
)

var (
	ConsulAgentTLSCipherSuites = map[string]TLSCipherSuite{
		// "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,

		// "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256": TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":    TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256": TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":    TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384": TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,

		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	}
	envoyTLSCipherSuites = map[string]TLSCipherSuite{
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":          TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":          TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,

		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256": TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":          TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":       TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":          TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":       TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,

		"TLS_RSA_WITH_AES_128_GCM_SHA256": TLS_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_RSA_WITH_AES_128_CBC_SHA":    TLS_RSA_WITH_AES_128_CBC_SHA,
		"TLS_RSA_WITH_AES_256_GCM_SHA384": TLS_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_RSA_WITH_AES_256_CBC_SHA":    TLS_RSA_WITH_AES_256_CBC_SHA,
	}
	envoyTLSCipherSuiteStrings = map[TLSCipherSuite]string{
		TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:       "ECDHE-ECDSA-AES128-GCM-SHA256",
		TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256: "ECDHE-ECDSA-CHACHA20-POLY1305",
		TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:         "ECDHE-RSA-AES128-GCM-SHA256",
		TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256:   "ECDHE-RSA-CHACHA20-POLY1305",
		TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:          "ECDHE-ECDSA-AES128-SHA",
		TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:            "ECDHE-RSA-AES128-SHA",
		TLS_RSA_WITH_AES_128_GCM_SHA256:               "AES128-GCM-SHA256",
		TLS_RSA_WITH_AES_128_CBC_SHA:                  "AES128-SHA",
		TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:       "ECDHE-ECDSA-AES256-GCM-SHA384",
		TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:         "ECDHE-RSA-AES256-GCM-SHA384",
		TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:          "ECDHE-ECDSA-AES256-SHA",
		TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:            "ECDHE-RSA-AES256-SHA",
		TLS_RSA_WITH_AES_256_GCM_SHA384:               "AES256-GCM-SHA384",
		TLS_RSA_WITH_AES_256_CBC_SHA:                  "AES256-SHA",
	}
)

func (c *TLSCipherSuite) String() string {
	return fmt.Sprintf("%s", *c)
}

// TODO: Should this return an errgroup instead of only the first error?
func ValidateEnvoyCipherSuites(cipherSuites []TLSCipherSuite) error {
	for _, c := range cipherSuites {
		if _, ok := envoyTLSCipherSuiteStrings[c]; !ok {
			return fmt.Errorf("no matching Envoy TLS cipher suite found for %s", c.String())
		}
	}

	return nil
}

func MarshalEnvoyTLSCipherSuiteStrings(cipherSuites []TLSCipherSuite) []string {
	cipherSuiteStrings := []string{}
	for _, c := range cipherSuites {
		cipherSuiteStrings = append(cipherSuiteStrings, envoyTLSCipherSuiteStrings[c])
	}
	return cipherSuiteStrings
}
