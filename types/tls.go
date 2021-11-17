package types

// TLSVersion is a strongly-typed int used for relative comparison
// (minimum, maximum, greater than, less than) of TLS versions
type TLSVersion int

const (
	// Error value, excluded from lookup maps
	TLSVersionInvalid TLSVersion = iota - 1

	// Explictly allow implementation to select TLS version
	// May be useful to supercede defaults specified at a higher layer
	TLSVersionAuto

	_ // Placeholder for SSLv3, hopefully we won't have to add this

	// TLS versions
	TLSv1_0
	TLSv1_1
	TLSv1_2
	TLSv1_3
)

var (
	TLSVersions = map[string]TLSVersion{
		"TLS_AUTO": TLSVersionAuto,
		"TLSv1_0":  TLSv1_0,
		"TLSv1_1":  TLSv1_1,
		"TLSv1_2":  TLSv1_2,
		"TLSv1_3":  TLSv1_3,
	}
	// NOTE: This interface is deprecated in favor of TLSVersions
	// and should be eventually removed in a future release.
	DeprecatedAgentTLSVersions = map[string]TLSVersion{
		"":      TLSVersionAuto,
		"tls10": TLSv1_0,
		"tls11": TLSv1_1,
		"tls12": TLSv1_2,
		"tls13": TLSv1_3,
	}
	HumanTLSVersionStrings = map[TLSVersion]string{
		TLSVersionAuto: "Allow implementation to select TLS version",
		TLSv1_0:        "TLS 1.0",
		TLSv1_1:        "TLS 1.1",
		TLSv1_2:        "TLS 1.2",
		TLSv1_3:        "TLS 1.3",
	}
	EnvoyTLSVersionStrings = map[TLSVersion]string{
		TLSVersionAuto: "TLS_AUTO",
		TLSv1_0:        "TLSv1_0",
		TLSv1_1:        "TLSv1_1",
		TLSv1_2:        "TLSv1_2",
		TLSv1_3:        "TLSv1_3",
	}
)

func (v TLSVersion) String() string {
	return HumanTLSVersionStrings[v]
}

func (v TLSVersion) EnvoyString() string {
	return EnvoyTLSVersionStrings[v]
}

// The naming convention for cipher suites changed in TLS 1.3
// but constant values should still be globally unqiue
// Handling validation on a subset of TLSCipherSuite constants
// would be a future exercise if cipher suites for TLS 1.3 ever
// become configurable in BoringSSL, Envoy, or other implementation
type TLSCipherSuite string

// IANA cipher suite constants
// NOTE: This is the total list of TLS 1.2-style cipher suites
// which are currently supported by Envoy 1.21 and may change
// as some older suites are removed in future Envoy releases
// and Consul drops support for older Envoy versions
// TODO: Is there any better/less verbose way to handle this mapping?
const (
	ECDHE_ECDSA_AES128_GCM_SHA256 TLSCipherSuite = "ECDHE-ECDSA-AES128-GCM-SHA256"
	ECDHE_ECDSA_CHACHA20_POLY1305                = "ECDHE-ECDSA-CHACHA20-POLY1305"
	ECDHE_RSA_AES128_GCM_SHA256                  = "ECDHE-RSA-AES128-GCM-SHA256"
	ECDHE_RSA_CHACHA20_POLY1305                  = "ECDHE-RSA-CHACHA20-POLY1305"
	ECDHE_ECDSA_AES128_SHA                       = "ECDHE-ECDSA-AES128-SHA"
	ECDHE_RSA_AES128_SHA                         = "ECDHE-RSA-AES128-SHA"
	AES128_GCM_SHA256                            = "AES128-GCM-SHA256"
	AES128_SHA                                   = "AES128-SHA"
	ECDHE_ECDSA_AES256_GCM_SHA384                = "ECDHE-ECDSA-AES256-GCM-SHA384"
	ECDHE_RSA_AES256_GCM_SHA384                  = "ECDHE-RSA-AES256-GCM-SHA384"
	ECDHE_ECDSA_AES256_SHA                       = "ECDHE-ECDSA-AES256-SHA"
	ECDHE_RSA_AES256_SHA                         = "ECDHE-RSA-AES256-SHA"
	AES256_GCM_SHA384                            = "AES256-GCM-SHA384"
	AES256_SHA                                   = "AES256-SHA"
)
