// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"sort"
	"strings"
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
	tlsVersions = map[TLSVersion]struct{}{
		TLSVersionAuto: {},
		TLSv1_0:        {},
		TLSv1_1:        {},
		TLSv1_2:        {},
		TLSv1_3:        {},
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
	TLSVersionsWithConfigurableCipherSuites = map[TLSVersion]struct{}{
		// NOTE: these two are implementation-dependent, but it is not expected that
		// either Go or Envoy would default to TLS 1.3 as a minimum version in the
		// near future
		TLSVersionUnspecified: {},
		TLSVersionAuto:        {},

		TLSv1_0: {},
		TLSv1_1: {},
		TLSv1_2: {},
	}

	TLSVersionsWithConfigurableECDHCurves = map[TLSVersion]struct{}{
		TLSVersionUnspecified: {},
		TLSVersionAuto:        {},
		TLSv1_2:               {},
		TLSv1_3:               {},
	}
)

const (
	// EnvoyMinSupportedTLSVersion is the minimum supported TLS version for Envoy proxies.
	EnvoyMinSupportedTLSVersion = TLSv1_2

	// AgentMinSupportedTLSVersion is the minimum supported TLS version for Consul Agent RPC/HTTPS/gRPC.
	AgentMinSupportedTLSVersion = TLSv1_2
)


func (v *TLSVersion) String() string {
	return string(*v)
}

var tlsVersionComparison = map[TLSVersion]uint{
	TLSv1_0: 1,
	TLSv1_1: 2,
	TLSv1_2: 3,
	TLSv1_3: 4,
}

// Will only return true for concrete versions and won't catch
// implementation-dependent conflicts with TLSVersionAuto or unspecified values
func (a TLSVersion) LessThan(b TLSVersion) (error, bool) {
	for _, v := range []TLSVersion{a, b} {
		if _, ok := tlsVersionComparison[v]; !ok {
			return fmt.Errorf("can't compare implementation-dependent values"), false
		}
	}

	return nil, tlsVersionComparison[a] < tlsVersionComparison[b]
}

func TLSVersions() string {
	versions := []string{}
	for v := range tlsVersions {
		versions = append(versions, string(v))
	}
	sort.Strings(versions)

	return strings.Join(versions, ", ")
}

func ValidateTLSVersion(v TLSVersion) error {
	if _, ok := tlsVersions[v]; !ok {
		return fmt.Errorf("no matching TLS version found for %s, please specify one of [%s]", v.String(), TLSVersions())
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
	// Cipher suites used by both Envoy and Consul agent
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 = "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256"
	TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256   = "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256       = "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256         = "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA          = "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA"
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA            = "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA"
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384       = "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384         = "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA          = "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA"
	TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA            = "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA"

	// Older cipher suites not supported for Consul agent TLS,
	// will eventually be removed from Envoy defaults
	TLS_RSA_WITH_AES_128_GCM_SHA256 = "TLS_RSA_WITH_AES_128_GCM_SHA256"
	TLS_RSA_WITH_AES_128_CBC_SHA    = "TLS_RSA_WITH_AES_128_CBC_SHA"
	TLS_RSA_WITH_AES_256_GCM_SHA384 = "TLS_RSA_WITH_AES_256_GCM_SHA384"
	TLS_RSA_WITH_AES_256_CBC_SHA    = "TLS_RSA_WITH_AES_256_CBC_SHA"
)

var (
	consulAgentTLSCipherSuites = map[TLSCipherSuite]struct{}{
		TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256: {},
		TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:          {},
		TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:       {},
		TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:          {},
		TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:       {},

		TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256: {},
		TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:          {},
		TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:       {},
		TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:          {},
		TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:       {},
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
	return string(*c)
}

func ValidateConsulAgentCipherSuites(cipherSuites []TLSCipherSuite) error {
	var unmatched []string

	for _, c := range cipherSuites {
		if _, ok := consulAgentTLSCipherSuites[c]; !ok {
			unmatched = append(unmatched, c.String())
		}
	}

	if len(unmatched) > 0 {
		return fmt.Errorf("no matching Consul Agent TLS cipher suite found for %s", strings.Join(unmatched, ","))
	}
	return nil
}

func ValidateEnvoyCipherSuites(cipherSuites []TLSCipherSuite) error {
	var unmatched []string

	for _, c := range cipherSuites {
		if _, ok := envoyTLSCipherSuiteStrings[c]; !ok {
			unmatched = append(unmatched, c.String())
		}
	}

	if len(unmatched) > 0 {
		return fmt.Errorf("no matching Envoy TLS cipher suite found for %s", strings.Join(unmatched, ","))
	}

	return nil
}

func MarshalEnvoyTLSCipherSuiteStrings(cipherSuites []TLSCipherSuite) []string {
	cipherSuiteStrings := []string{}

	for _, c := range cipherSuites {
		if s, ok := envoyTLSCipherSuiteStrings[c]; ok {
			cipherSuiteStrings = append(cipherSuiteStrings, s)
		}
	}

	return cipherSuiteStrings
}

// TLSECDHCurve is a strongly-typed string identifier for ECDH and post-quantum KEM curves
// accepted by Envoy and Consul agent (backed by Go crypto/tls).
type TLSECDHCurve string

const (
	CurveX25519MLKEM768 TLSECDHCurve = "X25519MLKEM768"
	CurveX25519         TLSECDHCurve = "X25519"
	CurveP256           TLSECDHCurve = "P-256"
	CurveP384           TLSECDHCurve = "P-384"
	CurveP521           TLSECDHCurve = "P-521"
)

var (
	// DefaultPQCECDHCurves is the curve list injected automatically for Envoy when
	// TLSMinVersion >= TLSv1_3 is set and the operator has not provided an explicit curve list.
	DefaultPQCECDHCurves = []string{string(CurveX25519MLKEM768), string(CurveX25519)}

	// DefaultConsulAgentPQCECDHCurves is the curve list injected automatically for Consul Agent
	// when TLSMinVersion >= TLSv1_3 is set and the operator has not provided an explicit curve list.
	DefaultConsulAgentPQCECDHCurves = []TLSECDHCurve{CurveX25519MLKEM768, CurveX25519}

	consulAgentTLSECDHCurves = map[TLSECDHCurve]struct{}{
		CurveX25519MLKEM768: {},
		CurveX25519:         {},
		CurveP256:           {},
		CurveP384:           {},
		CurveP521:           {},
	}

	envoyECDHCurves = map[string]struct{}{
		string(CurveX25519MLKEM768): {},
		string(CurveX25519):         {},
		string(CurveP256):           {},
		string(CurveP384):           {},
		string(CurveP521):           {},
	}
)

func (c *TLSECDHCurve) String() string {
	return string(*c)
}

func ValidateConsulAgentECDHCurves(curves []TLSECDHCurve) error {
	var unmatched []string
	for _, c := range curves {
		if _, ok := consulAgentTLSECDHCurves[c]; !ok {
			unmatched = append(unmatched, c.String())
		}
	}
	if len(unmatched) > 0 {
		return fmt.Errorf("no matching Consul Agent TLS curve found for %s; must be one of [%s]",
			strings.Join(unmatched, ", "), strings.Join(SortedConsulAgentECDHCurves(), ", "))
	}
	return nil
}

func ValidateEnvoyECDHCurves(curves []string) error {
	for _, c := range curves {
		if _, ok := envoyECDHCurves[c]; !ok {
			return fmt.Errorf("unsupported ecdh_curve %q; must be one of [%s]",
				c, strings.Join(SortedEnvoyECDHCurves(), ", "))
		}
	}
	return nil
}

func SortedEnvoyECDHCurves() []string {
	var list []string
	for c := range envoyECDHCurves {
		list = append(list, c)
	}
	sort.Strings(list)
	return list
}

func SortedConsulAgentECDHCurves() []string {
	var list []string
	for c := range consulAgentTLSECDHCurves {
		list = append(list, string(c))
	}
	sort.Strings(list)
	return list
}

// ValidateTLSVersionECDHCurvesCompat checks that the specified TLS version supports
// specifying ECDH/KEM curves. Curves are supported for TLS 1.2 and TLS 1.3 (and TLS_AUTO).
func ValidateTLSVersionECDHCurvesCompat(tlsMinVersion TLSVersion) error {
	if _, ok := TLSVersionsWithConfigurableECDHCurves[tlsMinVersion]; !ok {
		return fmt.Errorf("ecdh_curves can only be configured when tls_min_version is 'TLSv1_2' or higher, TLSMinVersion is set to %s", tlsMinVersion)
	}
	return nil
}

