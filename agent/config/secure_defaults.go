package config

import "fmt"

// TODO: auto-encrypt may set some of these, so make an exception for things set
// by auto-encrypt when it is enabled.
func applyAndValidateSecureDefaults(rt *RuntimeConfig) error {
	// Agent TLS
	rt.VerifyOutgoing = true
	rt.VerifyIncoming = true
	rt.VerifyServerHostname = true
	if rt.CAPath == "" && rt.CAFile == "" {
		return fmt.Errorf("one of ca_file or ca_path must be specified")
	}
	if rt.CertFile == "" || rt.KeyFile == "" {
		return fmt.Errorf("both cert_file and key_file must be specified")
	}
	// TODO: test cases for this
	if rt.TLSMinVersion < "tls12" {
		return fmt.Errorf("TLS minimum version must be at least tls1.2")
	}

	// Gossip
	rt.EncryptVerifyIncoming = true
	rt.EncryptVerifyOutgoing = true
	// TODO: technically should not required, because it gets saved by the keyring
	// TODO: when not specified, set some other field that can be checked later
	// to ensure a key is set in the keyring.
	if rt.EncryptKey == "" {
		return fmt.Errorf("encrypt is required to secure gossip communication")
	}

	// ACLs
	rt.ACLsEnabled = true
	rt.ACLDefaultPolicy = "deny"
	if rt.ACLDownPolicy == "allow" {
		return fmt.Errorf(`acl.down_policy must not be set to "allow"`)
	}
	// TODO: rt.ACLEnableKeyListPolicy ?
	// TODO: acl.tokens

	// Ports
	// TODO: allow HTTPPort if address is localhost
	rt.HTTPPort = -1
	if rt.HTTPSPort <= 0 {
		return fmt.Errorf("ports.https is required")
	}

	// Misc
	rt.EnableRemoteScriptChecks = false
	rt.DisableRemoteExec = true

	// TODO: require telemetry

	// TODO: any additional validation when PrimaryDatacenter != Datacenter?
	return nil
}
