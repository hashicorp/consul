// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

type autoConfigBackend struct {
	Server *Server
}

func (b autoConfigBackend) ForwardRPC(method string, info structs.RPCInfo, reply interface{}) (bool, error) {
	return b.Server.ForwardRPC(method, info, reply)
}

func (b autoConfigBackend) SignCertificate(csr *x509.CertificateRequest, id connect.CertURI) (*structs.IssuedCert, error) {
	return b.Server.caManager.SignCertificate(csr, id)
}

// GetCARoots returns the CA roots.
func (b autoConfigBackend) GetCARoots() (*structs.IndexedCARoots, error) {
	return b.Server.getCARoots(nil, b.Server.fsm.State())
}

// DatacenterJoinAddresses will return all the strings suitable for usage in
// retry join operations to connect to the LAN or LAN segment gossip pool.
func (b autoConfigBackend) DatacenterJoinAddresses(partition, segment string) ([]string, error) {
	members, err := b.Server.LANMembers(LANMemberFilter{
		Segment:   segment,
		Partition: partition,
	})
	if err != nil {
		if segment != "" {
			return nil, fmt.Errorf("Failed to retrieve members for segment %s: %w", segment, err)
		}
		return nil, fmt.Errorf("Failed to retrieve members for partition %s: %w", acl.PartitionOrDefault(partition), err)
	}

	var joinAddrs []string
	for _, m := range members {
		if ok, _ := metadata.IsConsulServer(m); ok {
			serfAddr := net.TCPAddr{IP: m.Addr, Port: int(m.Port)}
			joinAddrs = append(joinAddrs, serfAddr.String())
		}
	}

	return joinAddrs, nil
}

// CreateACLToken will create an ACL token from the given template
func (b autoConfigBackend) CreateACLToken(template *structs.ACLToken) (*structs.ACLToken, error) {
	// we have to require local tokens or else it would require having these servers use a token with acl:write to make a
	// token create RPC to the servers in the primary DC.
	if !b.Server.LocalTokensEnabled() {
		return nil, fmt.Errorf("Agent Auto Configuration requires local token usage to be enabled in this datacenter: %s", b.Server.config.Datacenter)
	}

	newToken := *template

	// generate the accessor id
	if newToken.AccessorID == "" {
		accessor, err := lib.GenerateUUID(b.Server.checkTokenUUID)
		if err != nil {
			return nil, err
		}

		newToken.AccessorID = accessor
	}

	// generate the secret id
	if newToken.SecretID == "" {
		secret, err := lib.GenerateUUID(b.Server.checkTokenUUID)
		if err != nil {
			return nil, err
		}

		newToken.SecretID = secret
	}

	newToken.CreateTime = time.Now()

	req := structs.ACLTokenBatchSetRequest{
		Tokens: structs.ACLTokens{&newToken},
		CAS:    false,
	}

	// perform the request to mint the new token
	if _, err := b.Server.raftApplyMsgpack(structs.ACLTokenSetRequestType, &req); err != nil {
		return nil, err
	}

	// return the full token definition from the FSM
	_, token, err := b.Server.fsm.State().ACLTokenGetByAccessor(nil, newToken.AccessorID, &newToken.EnterpriseMeta)
	return token, err
}
