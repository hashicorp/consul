// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pool

type RPCType byte

func (t RPCType) ALPNString() string {
	switch t {
	case RPCConsul:
		return ALPN_RPCConsul
	case RPCRaft:
		return ALPN_RPCRaft
	case RPCMultiplex:
		return "" // unsupported
	case RPCTLS:
		return "" // unsupported
	case RPCMultiplexV2:
		return ALPN_RPCMultiplexV2
	case RPCSnapshot:
		return ALPN_RPCSnapshot
	case RPCGossip:
		return ALPN_RPCGossip
	case RPCTLSInsecure:
		return "" // unsupported
	case RPCGRPC:
		return ALPN_RPCGRPC
	default:
		return "" // unsupported
	}
}

const (
	// keep numbers unique.
	RPCConsul      RPCType = 0
	RPCRaft        RPCType = 1
	RPCMultiplex   RPCType = 2 // Old Muxado byte, no longer supported.
	RPCTLS         RPCType = 3
	RPCMultiplexV2 RPCType = 4
	RPCSnapshot    RPCType = 5
	RPCGossip      RPCType = 6
	// RPCTLSInsecure is used to flag RPC calls that require verify
	// incoming to be disabled, even when it is turned on in the
	// configuration. At the time of writing there is only AutoEncrypt.Sign
	// that is supported and it might be the only one there
	// ever is.
	RPCTLSInsecure    RPCType = 7
	RPCGRPC           RPCType = 8
	RPCRaftForwarding RPCType = 9

	// RPCMaxTypeValue is the maximum rpc type byte value currently used for the
	// various protocols riding over our "rpc" port.
	//
	// Currently our 0-9 values are mutually exclusive with any valid first byte
	// of a TLS header.  The first TLS header byte will begin with a TLS content
	// type and the values 0-19 are all explicitly unassigned and marked as
	// requiring coordination. RFC 7983 does the marking and goes into some
	// details about multiplexing connections and identifying TLS.
	//
	// We use this value to determine if the incoming request is actual real
	// native TLS (where we can de-multiplex based on ALPN protocol) or our older
	// type-byte system when new connections are established.
	//
	// NOTE: if you add new RPCTypes beyond this value, you must similarly bump
	// this value.
	RPCMaxTypeValue = 9
)

const (
	// regular old rpc (note there is no equivalent of RPCMultiplex, RPCTLS, or RPCTLSInsecure)
	ALPN_RPCConsul         = "consul/rpc-single"      // RPCConsul
	ALPN_RPCRaft           = "consul/raft"            // RPCRaft
	ALPN_RPCMultiplexV2    = "consul/rpc-multi"       // RPCMultiplexV2
	ALPN_RPCSnapshot       = "consul/rpc-snapshot"    // RPCSnapshot
	ALPN_RPCGossip         = "consul/rpc-gossip"      // RPCGossip
	ALPN_RPCGRPC           = "consul/rpc-grpc"        // RPCGRPC
	ALPN_RPCRaftForwarding = "consul/raft-forwarding" // RPCRaftForwarding
	// wan federation additions
	ALPN_WANGossipPacket = "consul/wan-gossip/packet"
	ALPN_WANGossipStream = "consul/wan-gossip/stream"
)

var RPCNextProtos = []string{
	ALPN_RPCConsul,
	ALPN_RPCRaft,
	ALPN_RPCMultiplexV2,
	ALPN_RPCSnapshot,
	ALPN_RPCGossip,
	ALPN_RPCGRPC,
	ALPN_RPCRaftForwarding,
	ALPN_WANGossipPacket,
	ALPN_WANGossipStream,
}
