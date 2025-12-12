// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package fsm

import (
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func decodeRegistrationReq(buf []byte, req *structs.RegisterRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeRegistration(buf, req)
}

func decodeDeregistrationReq(buf []byte, req *structs.DeregisterRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeDeregistration(buf, req)
}

func decodeKVSRequest(buf []byte, req *structs.KVSRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeKVS(buf, req)
}

func decodeSessionRequest(buf []byte, req *structs.SessionRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}

	return decodeSession(buf, req)
}

func decodePreparedQueryRequest(buf []byte, req *structs.PreparedQueryRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodePreparedQuery(buf, req)
}

func decodeTxnRequest(buf []byte, req *structs.TxnRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeTxn(buf, req)
}

func decodeACLTokenBatchSetRequest(buf []byte, req *structs.ACLTokenBatchSetRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeACLTokenBatchSet(buf, req)

}

func decodeACLPolicyBatchSetRequest(buf []byte, req *structs.ACLPolicyBatchSetRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeACLPolicyBatchSet(buf, req)

}

func decodeACLRoleBatchSetRequest(buf []byte, req *structs.ACLRoleBatchSetRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeACLRoleBatchSet(buf, req)
}

func decodeACLBindingRuleBatchSetRequest(buf []byte, req *structs.ACLBindingRuleBatchSetRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeACLBindingRuleBatchSet(buf, req)
}

func decodeACLAuthMethodBatchSetRequest(buf []byte, req *structs.ACLAuthMethodBatchSetRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeACLAuthMethodBatchSet(buf, req)
}

func decodeACLAuthMethodBatchDeleteRequest(buf []byte, req *structs.ACLAuthMethodBatchDeleteRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}

	return decodeACLAuthMethodBatchDelete(buf, req)
}

func decodeServiceVirtualIPRequest(buf []byte, req *state.ServiceVirtualIP) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}
	return decodeServiceVirtualIP(buf, req)
}

func decodePeeringWriteRequest(buf []byte, req *pbpeering.PeeringWriteRequest) error {
	if !structs.CEDowngrade {
		return structs.DecodeProto(buf, req)
	}
	return decodePeeringWrite(buf, req)
}

func decodePeeringDeleteRequest(buf []byte, req *pbpeering.PeeringDeleteRequest) error {
	if !structs.CEDowngrade {
		return structs.DecodeProto(buf, req)
	}

	return decodePeeringDelete(buf, req)
}

func decodePeeringTrustBundleWriteRequest(buf []byte, req *pbpeering.PeeringTrustBundleWriteRequest) error {
	if !structs.CEDowngrade {
		return structs.DecodeProto(buf, req)
	}
	return decodePeeringTrustBundleWrite(buf, req)
}

func decodePeeringTrustBundleDeleteRequest(buf []byte, req *pbpeering.PeeringTrustBundleDeleteRequest) error {
	if !structs.CEDowngrade {
		return structs.DecodeProto(buf, req)
	}
	return decodePeeringTrustBundleDelete(buf, req)
}

func decodeConfigEntryOperationRequest(buf []byte, req *structs.ConfigEntryRequest) error {
	if !structs.CEDowngrade {
		return structs.Decode(buf, req)
	}

	return decodeConfigEntryOperation(buf, req)
}
