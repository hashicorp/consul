// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import "github.com/hashicorp/consul/sdk/testutil"

// TestSpiffeIDService returns a SPIFFE ID representing a service.
func TestSpiffeIDService(t testutil.TestingTB, service string) *SpiffeIDService {
	return TestSpiffeIDServiceWithHost(t, service, TestClusterID+".consul")
}

// TestSpiffeIDServiceWithHost returns a SPIFFE ID representing a service with
// the specified trust domain.
func TestSpiffeIDServiceWithHost(t testutil.TestingTB, service, host string) *SpiffeIDService {
	return TestSpiffeIDServiceWithHostDC(t, service, host, "dc1")
}

// TestSpiffeIDServiceWithHostDC returns a SPIFFE ID representing a service with
// the specified trust domain for the given datacenter.
func TestSpiffeIDServiceWithHostDC(t testutil.TestingTB, service, host, datacenter string) *SpiffeIDService {
	return &SpiffeIDService{
		Host:       host,
		Namespace:  "default",
		Datacenter: datacenter,
		Service:    service,
	}
}
