package structs

type SnapshotOp int

const (
	SnapshotSave SnapshotOp = iota
	SnapshotRestore
)

// SnapshotRequest is used as a header for a snapshot RPC request. This will
// precede any streaming data that's part of the request and is JSON-encoded on
// the wire.
type SnapshotRequest struct {
	// Datacenter is the target datacenter for this request. The request
	// will be forwarded if necessary.
	Datacenter string

	// Token is the ACL token to use for the operation. If ACLs are enabled
	// then all operations require a management token.
	Token string

	// Op is the operation code for the RPC.
	Op SnapshotOp
}

// SnapshotResponse is used header for a snapshot RPC response. This will
// precede any streaming data that's part of the request and is JSON-encoded on
// the wire.
type SnapshotResponse struct {
	// Error is the overall error status of the RPC request.
	Error string
}
