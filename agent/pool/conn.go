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

	// NOTE: Currently we use values between 0 and 7 for the different
	// "protocols" that we may ride over our "rpc" port. We had an idea of
	// using TLS + ALPN for negotiating the protocol instead of our own
	// bytes as it could provide other benefits. Currently our 0-7 values
	// are mutually exclusive with any valid first byte of a TLS header
	// The first TLS header byte will content a TLS content type and the
	// values 0-19 are all explicitly unassigned and marked as
	// requiring coordination. RFC 7983 does the marking and goes into
	// some details about multiplexing connections and identifying TLS.
)
