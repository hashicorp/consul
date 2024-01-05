// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package leafcert

import (
	"context"

	"github.com/hashicorp/consul/agent/structs"
)

// NetRPC is an interface that an NetRPC client must implement. This is a helper
// interface that is implemented by the agent delegate so that Type
// implementations can request NetRPC access.
type NetRPC interface {
	RPC(ctx context.Context, method string, args any, reply any) error
}

// NewNetRPCCertSigner returns a CertSigner that uses net-rpc to sign certs.
func NewNetRPCCertSigner(netRPC NetRPC) CertSigner {
	return &netRPCCertSigner{netRPC: netRPC}
}

type netRPCCertSigner struct {
	// NetRPC is an RPC client for remote cert signing requests.
	netRPC NetRPC
}

var _ CertSigner = (*netRPCCertSigner)(nil)

func (s *netRPCCertSigner) SignCert(ctx context.Context, args *structs.CASignRequest) (*structs.IssuedCert, error) {
	var reply structs.IssuedCert
	err := s.netRPC.RPC(ctx, "ConnectCA.Sign", args, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}
