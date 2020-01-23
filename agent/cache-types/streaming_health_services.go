package cachetype

import (
	"fmt"
	"log"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// Recommended name for registration.
	StreamingHealthServicesName = "streaming-health-services"
)

// StreamingHealthServices supports fetching discovering service instances via the
// catalog using the streaming gRPC endpoint.
type StreamingHealthServices struct {
	client StreamingClient
	logger *log.Logger
}

// NewStreamingHealthServices creates a cache-type for watching for service
// health results via streaming updates.
func NewStreamingHealthServices(client StreamingClient, logger *log.Logger) *StreamingHealthServices {
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

	r := stream.SubscribeRequest{
		Topic:      stream.Topic_ServiceHealth,
		Key:        reqReal.ServiceName,
		Token:      reqReal.Token,
		Index:      reqReal.MinQueryIndex,
		Filter:     reqReal.Filter,
		Datacenter: reqReal.Datacenter,
	}

	// Connect requests need a different topic
	if reqReal.Connect {
		r.Topic = stream.Topic_ServiceHealthConnect
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
	return make(healthViewState)
}

// StreamingClient implements StreamingCacheType
func (c *StreamingHealthServices) StreamingClient() StreamingClient {
	return c.client
}

// Logger implements StreamingCacheType
func (c *StreamingHealthServices) Logger() *log.Logger {
	return c.logger
}

// healthViewState implements MaterializedViewState for storing the view state
// of a service health result. We store it as a map to make updates and
// deletions a little easier but we could just store a result type
// (IndexedCheckServiceNodes) and update it in place for each event - that
// involves re-sorting each time etc. though.
type healthViewState map[string]structs.CheckServiceNode

// Update implements MaterializedViewState
func (s healthViewState) Update(events []*stream.Event) error {
	for _, event := range events {
		serviceHealth := event.GetServiceHealth()
		if serviceHealth == nil {
			return fmt.Errorf("unexpected event type for service health view: %T",
				event.GetPayload())
		}
		node := serviceHealth.CheckServiceNode
		id := fmt.Sprintf("%s/%s", node.Node.Node, node.Service.ID)

		switch serviceHealth.Op {
		case stream.CatalogOp_Register:
			checkServiceNode := stream.FromCheckServiceNode(serviceHealth.CheckServiceNode)
			s[id] = checkServiceNode
		case stream.CatalogOp_Deregister:
			delete(s, id)
		}
	}
	return nil
}

// Result implements MaterializedViewState
func (s healthViewState) Result(index uint64) (interface{}, error) {
	var result structs.IndexedCheckServiceNodes
	// Avoid a nil slice if there are no results in the view
	result.Nodes = structs.CheckServiceNodes{}
	for _, node := range s {
		result.Nodes = append(result.Nodes, node)
	}
	result.Nodes.Sort()
	result.Index = index
	return &result, nil
}
