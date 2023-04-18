package cachetype

// RPC is an interface that an RPC client must implement. This is a helper
// interface that is implemented by the agent delegate so that Type
// implementations can request RPC access.
//
//go:generate mockery --name RPC --inpackage
type RPC interface {
	RPC(method string, args interface{}, reply interface{}) error
}
