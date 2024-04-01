// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tlsutil

import "crypto/tls"

var _ ConfiguratorIface = (*MockConfigurator)(nil)

// MockConfigurator is used for mocking the ConfiguratorIface in testing
type MockConfigurator struct {
	BaseConfig               Config
	TlsCert                  *tls.Certificate
	ManualCAPemsArr          []string
	VerifyIncomingRPCBool    bool
	VerifyServerHostnameBool bool
}

func (m MockConfigurator) Base() Config {
	return m.BaseConfig
}

func (m MockConfigurator) Cert() *tls.Certificate {
	return m.TlsCert
}

func (m MockConfigurator) ManualCAPems() []string {
	return m.ManualCAPemsArr
}

func (m MockConfigurator) VerifyIncomingRPC() bool {
	return m.VerifyIncomingRPCBool
}

func (m MockConfigurator) VerifyServerHostname() bool {
	return m.VerifyServerHostnameBool
}
