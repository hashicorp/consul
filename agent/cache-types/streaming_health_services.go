package cachetype

import (
	"fmt"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

const (
	// Recommended name for registration.
	StreamingHealthServicesName = "streaming-health-services"
)

// StreamingHealthServices supports fetching discovering service instances via the
// catalog using the streaming gRPC endpoint.
type StreamingHealthServices struct {
	client StreamingClient
	logger hclog.Logger
}

// NewStreamingHealthServices creates a cache-type for watching for service
// health results via streaming updates.
func NewStreamingHealthServices(client StreamingClient, logger hclog.Logger) *StreamingHealthServices {
	return &StreamingHealthServices{
		client: client,
		logger: logger,
	}
}

// Fetch implements cache.Type
func (c *StreamingHealthServices) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	// The request should be a ServiceSpecificRequest.
	reqReal, ok := req.(*structs.ServiceSpecificRequest)
	if !ok {
		return cache.FetchResult{}, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	r := Request{
		SubscribeRequest: pbsubscribe.SubscribeRequest{
			Topic:      pbsubscribe.Topic_ServiceHealth,
			Key:        reqReal.ServiceName,
			Token:      reqReal.Token,
			Index:      reqReal.MinQueryIndex,
			Datacenter: reqReal.Datacenter,
		},
		Filter: reqReal.Filter,
	}

	// Connect requests need a different topic
	if reqReal.Connect {
		r.Topic = pbsubscribe.Topic_ServiceHealthConnect
	}

	view := MaterializedViewFromFetch(c, opts, r)
	return view.Fetch(opts)
}

// SupportsBlocking implements cache.Type
func (c *StreamingHealthServices) SupportsBlocking() bool {
	return true
}

// NewMaterializedView implements StreamingCacheType
func (c *StreamingHealthServices) NewMaterializedViewState() MaterializedViewState {
	return &healthViewState{
		state: make(map[string]structs.CheckServiceNode),
	}
}

// StreamingClient implements StreamingCacheType
func (c *StreamingHealthServices) StreamingClient() StreamingClient {
	return c.client
}

// Logger implements StreamingCacheType
func (c *StreamingHealthServices) Logger() hclog.Logger {
	return c.logger
}

// healthViewState implements MaterializedViewState for storing the view state
// of a service health result. We store it as a map to make updates and
// deletions a little easier but we could just store a result type
// (IndexedCheckServiceNodes) and update it in place for each event - that
// involves re-sorting each time etc. though.
type healthViewState struct {
	state  map[string]structs.CheckServiceNode
	filter *bexpr.Filter
}

// InitFilter implements MaterializedViewState
func (s *healthViewState) InitFilter(expression string) error {
	// We apply filtering to the raw CheckServiceNodes before we are done mutating
	// state in Update to save from storing stuff in memory we'll only filter
	// later. Because the state is just a map of those types, we can simply run
	// that map through filter and it will remove any entries that don't match.
	filter, err := bexpr.CreateFilter(expression, nil, s.state)
	if err != nil {
		return err
	}
	s.filter = filter
	return nil
}

// Update implements MaterializedViewState
func (s *healthViewState) Update(events []*pbsubscribe.Event) error {
	for _, event := range events {
		serviceHealth := event.GetServiceHealth()
		if serviceHealth == nil {
			return fmt.Errorf("unexpected event type for service health view: %T",
				event.GetPayload())
		}
		node := serviceHealth.CheckServiceNode
		id := fmt.Sprintf("%s/%s", node.Node.Node, node.Service.ID)

		switch serviceHealth.Op {
		case pbsubscribe.CatalogOp_Register:
			checkServiceNode := pbservice.CheckServiceNodeToStructs(serviceHealth.CheckServiceNode)
			s.state[id] = *checkServiceNode
		case pbsubscribe.CatalogOp_Deregister:
			delete(s.state, id)
		}
	}
	if s.filter != nil {
		filtered, err := s.filter.Execute(s.state)
		if err != nil {
			return err
		}
		s.state = filtered.(map[string]structs.CheckServiceNode)
	}
	return nil
}

// Result implements MaterializedViewState
func (s *healthViewState) Result(index uint64) (interface{}, error) {
	var result structs.IndexedCheckServiceNodes
	// Avoid a nil slice if there are no results in the view
	result.Nodes = structs.CheckServiceNodes{}
	for _, node := range s.state {
		result.Nodes = append(result.Nodes, node)
	}
	result.Index = index
	return &result, nil
}
