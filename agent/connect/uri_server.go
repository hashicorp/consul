// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"fmt"
	"net/url"
)

type SpiffeIDServer struct {
	Host       string
	Datacenter string
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDServer) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = fmt.Sprintf("/agent/server/dc/%s", id.Datacenter)
	return &result
}

// PeeringServerSAN returns the DNS SAN to attach to server certificates
// for control-plane peering traffic.
func PeeringServerSAN(dc, trustDomain string) string {
	return fmt.Sprintf("server.%s.peering.%s", dc, trustDomain)
}
