package mock

import "fmt"
import "github.com/hashicorp/consul/agent/structs"

// PreparedQuery is a fake endpoint that we inject into the Consul server
// in order to observe the RPC calls made by these HTTP endpoints. This lets
// us make sure that the request is being formed properly without having to
// set up a realistic environment for prepared queries, which is a huge task and
// already done in detail inside the prepared query endpoint's unit tests. If we
// can prove this formats proper requests into that then we should be good to
// go. We will do a single set of end-to-end tests in here to make sure that the
// server is wired up to the right endpoint when not "injected".
type PreparedQuery struct {
	ApplyFn   func(*structs.PreparedQueryRequest, *string) error
	GetFn     func(*structs.PreparedQuerySpecificRequest, *structs.IndexedPreparedQueries) error
	ListFn    func(*structs.DCSpecificRequest, *structs.IndexedPreparedQueries) error
	ExecuteFn func(*structs.PreparedQueryExecuteRequest, *structs.PreparedQueryExecuteResponse) error
	ExplainFn func(*structs.PreparedQueryExecuteRequest, *structs.PreparedQueryExplainResponse) error
}

func (m *PreparedQuery) Apply(args *structs.PreparedQueryRequest, reply *string) (err error) {
	if m.ApplyFn != nil {
		return m.ApplyFn(args, reply)
	}
	return fmt.Errorf("should not have called Apply")
}

func (m *PreparedQuery) Get(args *structs.PreparedQuerySpecificRequest, reply *structs.IndexedPreparedQueries) error {
	if m.GetFn != nil {
		return m.GetFn(args, reply)
	}
	return fmt.Errorf("should not have called Get")
}

func (m *PreparedQuery) List(args *structs.DCSpecificRequest, reply *structs.IndexedPreparedQueries) error {
	if m.ListFn != nil {
		return m.ListFn(args, reply)
	}
	return fmt.Errorf("should not have called List")
}

func (m *PreparedQuery) Execute(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
	if m.ExecuteFn != nil {
		return m.ExecuteFn(args, reply)
	}
	return fmt.Errorf("should not have called Execute")
}

func (m *PreparedQuery) Explain(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExplainResponse) error {
	if m.ExplainFn != nil {
		return m.ExplainFn(args, reply)
	}
	return fmt.Errorf("should not have called Explain")
}
