package pool

type RPCType byte

const (
	// keep numbers unique.
	RPCConsul      RPCType = 0
	RPCRaft                = 1
	RPCMultiplex           = 2 // Old Muxado byte, no longer supported.
	RPCTLS                 = 3
	RPCMultiplexV2         = 4
	RPCSnapshot            = 5
	RPCGossip              = 6
	// RPCTLSInsecure is used to flag RPC calls that require verify
	// incoming to be disabled, even when it is turned on in the
	// configuration. At the time of writing there is only AutoEncryt.Sign
	// that is supported and it might be the only one there
	// ever is.
	RPCTLSInsecure = 7
)
