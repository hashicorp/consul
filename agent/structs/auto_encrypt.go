// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

type SignedResponse struct {
	IssuedCert           IssuedCert     `json:",omitempty"`
	ConnectCARoots       IndexedCARoots `json:",omitempty"`
	ManualCARoots        []string       `json:",omitempty"`
	GossipKey            string         `json:",omitempty"`
	VerifyServerHostname bool           `json:",omitempty"`
}
