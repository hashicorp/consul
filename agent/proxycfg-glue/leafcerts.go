// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"

	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/proxycfg"
)

// LocalLeafCerts satisfies the proxycfg.LeafCertificate interface by sourcing data from
// the given leafcert.Manager.
func LocalLeafCerts(m *leafcert.Manager) proxycfg.LeafCertificate {
	return &localLeafCerts{m}
}

type localLeafCerts struct {
	leafCertManager *leafcert.Manager
}

func (c *localLeafCerts) Notify(ctx context.Context, req *leafcert.ConnectCALeafRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return c.leafCertManager.NotifyCallback(ctx, req, correlationID, dispatchCacheUpdate(ch))
}
