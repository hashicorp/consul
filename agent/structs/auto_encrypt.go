// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

type SignedResponse struct {
	IssuedCert           IssuedCert     `json:",omitempty"`
	ConnectCARoots       IndexedCARoots `json:",omitempty"`
	ManualCARoots        []string       `json:",omitempty"`
	GossipKey            string         `json:",omitempty"`
	VerifyServerHostname bool           `json:",omitempty"`
}
