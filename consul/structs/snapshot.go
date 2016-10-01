package structs

type SnapshotOp int

const (
	SnapshotSave SnapshotOp = iota
	SnapshotRestore
)

type SnapshotRequest struct {
	Datacenter string
	Token      string
	Op         SnapshotOp
}

type SnapshotResponse struct {
	Error string
}
