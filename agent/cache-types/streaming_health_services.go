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

func NewStreamingHealthServices(client StreamingClient, logger *log.Logger) *StreamingHealthServices {
	return &StreamingHealthServices{
		client: client,
		logger: logger,
	}
}

func (c *StreamingHealthServices) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	// The request should be a ServiceSpecificRequest.
	reqReal, ok := req.(*structs.ServiceSpecificRequest)
	if !ok {
		return cache.FetchResult{}, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	subscribeReq := stream.SubscribeRequest{
		Topic:      stream.Topic_ServiceHealth,
		Key:        reqReal.ServiceName,
		Token:      reqReal.Token,
		Index:      reqReal.MinQueryIndex,
		Filter:     reqReal.Filter,
		Datacenter: reqReal.Datacenter,
	}

	// Switch the topic if Connect is enabled.
	if reqReal.Connect {
		subscribeReq.Topic = stream.Topic_ServiceHealthConnect
	}

	handler := healthServicesHandler{
		state: make(map[string]structs.CheckServiceNode),
	}
	indexFunc := func(v interface{}) uint64 {
		if v, ok := v.(*structs.IndexedCheckServiceNodes); ok {
			return v.Index
		}
		return 0
	}

	return watchSubscriber(c.client, c.logger, opts, subscribeReq, &handler, indexFunc)
}

func (c *StreamingHealthServices) SupportsBlocking() bool {
	return true
}

// healthServicesHandler maintains a view of the health of a service
// based on incoming events from a Subscriber.
type healthServicesHandler struct {
	state map[string]structs.CheckServiceNode
}

// HandleEvent updates the handler's state based on register/deregister events.
func (h *healthServicesHandler) HandleEvent(event *stream.Event) {
	serviceHealth := event.GetServiceHealth()
	node := serviceHealth.CheckServiceNode
	id := fmt.Sprintf("%s/%s", node.Node.Node, node.Service.ID)

	switch serviceHealth.Op {
	case stream.CatalogOp_Register:
		checkServiceNode := stream.FromCheckServiceNode(serviceHealth.CheckServiceNode)
		h.state[id] = checkServiceNode
	case stream.CatalogOp_Deregister:
		delete(h.state, id)
	}
}

// State returns the current view of the state based on the events seen.
func (h *healthServicesHandler) State(idx uint64) interface{} {
	var result structs.IndexedCheckServiceNodes
	for _, node := range h.state {
		result.Nodes = append(result.Nodes, node)
	}
	result.Nodes.Sort()
	result.Index = idx
	return &result
}

func (h *healthServicesHandler) Reset() {
	h.state = make(map[string]structs.CheckServiceNode)
}
