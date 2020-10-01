package cachetype

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/submatview"

	"github.com/hashicorp/consul/lib/retry"

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
	client submatview.StreamingClient
	logger hclog.Logger
}

// NewStreamingHealthServices creates a cache-type for watching for service
// health results via streaming updates.
func NewStreamingHealthServices(client submatview.StreamingClient, logger hclog.Logger) *StreamingHealthServices {
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

	r := submatview.Request{
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

	view, err := c.getMaterializedView(opts, r)
	if err != nil {
		return cache.FetchResult{}, err
	}
	return view.Fetch(opts)
}

func (c *StreamingHealthServices) getMaterializedView(opts cache.FetchOptions, r submatview.Request) (*submatview.Materializer, error) {
	if opts.LastResult != nil && opts.LastResult.State != nil {
		return opts.LastResult.State.(*submatview.Materializer), nil
	}

	state, err := newHealthViewState(r.Filter)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.TODO())
	view := submatview.NewMaterializer(submatview.ViewDeps{
		State:  state,
		Client: c.client,
		Logger: c.logger,
		Waiter: &retry.Waiter{
			MinFailures: 1,
			MinWait:     0,
			MaxWait:     60 * time.Second,
			Jitter:      retry.NewJitter(100),
		},
		Request: r,
		Stop:    cancel,
		Done:    ctx.Done(),
	})
	go view.Run(ctx)
	return view, nil
}

// SupportsBlocking implements cache.Type
func (c *StreamingHealthServices) SupportsBlocking() bool {
	return true
}

func newHealthViewState(filterExpr string) (submatview.View, error) {
	s := &healthViewState{state: make(map[string]structs.CheckServiceNode)}

	// We apply filtering to the raw CheckServiceNodes before we are done mutating
	// state in Update to save from storing stuff in memory we'll only filter
	// later. Because the state is just a map of those types, we can simply run
	// that map through filter and it will remove any entries that don't match.
	var err error
	s.filter, err = bexpr.CreateFilter(filterExpr, nil, s.state)
	return s, err
}

// StreamingClient implements StreamingCacheType
func (c *StreamingHealthServices) StreamingClient() submatview.StreamingClient {
	return c.client
}

// Logger implements StreamingCacheType
func (c *StreamingHealthServices) Logger() hclog.Logger {
	return c.logger
}

// healthViewState implements View for storing the view state
// of a service health result. We store it as a map to make updates and
// deletions a little easier but we could just store a result type
// (IndexedCheckServiceNodes) and update it in place for each event - that
// involves re-sorting each time etc. though.
type healthViewState struct {
	state map[string]structs.CheckServiceNode
	// TODO: test case with filter
	filter *bexpr.Filter
}

// Update implements View
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
	// TODO: replace with a no-op filter instead of a conditional
	if s.filter != nil {
		filtered, err := s.filter.Execute(s.state)
		if err != nil {
			return err
		}
		s.state = filtered.(map[string]structs.CheckServiceNode)
	}
	return nil
}

// Result implements View
func (s *healthViewState) Result(index uint64) (interface{}, error) {
	var result structs.IndexedCheckServiceNodes
	// Avoid a nil slice if there are no results in the view
	// TODO: why this ^
	result.Nodes = structs.CheckServiceNodes{}
	for _, node := range s.state {
		result.Nodes = append(result.Nodes, node)
	}
	result.Index = index
	return &result, nil
}

func (s *healthViewState) Reset() {
	s.state = make(map[string]structs.CheckServiceNode)
}
