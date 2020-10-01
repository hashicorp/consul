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
	RegisterOptionsBlockingRefresh
	deps MaterializerDeps
}

// NewStreamingHealthServices creates a cache-type for watching for service
// health results via streaming updates.
func NewStreamingHealthServices(deps MaterializerDeps) *StreamingHealthServices {
	return &StreamingHealthServices{deps: deps}
}

type MaterializerDeps struct {
	Client submatview.StreamClient
	Logger hclog.Logger
}

// Fetch implements cache.Type
func (c *StreamingHealthServices) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	if opts.LastResult != nil && opts.LastResult.State != nil {
		state := opts.LastResult.State.(*streamingHealthState)
		return state.materializer.Fetch(state.done, opts)
	}

	srvReq := req.(*structs.ServiceSpecificRequest)
	newReqFn := func(index uint64) pbsubscribe.SubscribeRequest {
		req := pbsubscribe.SubscribeRequest{
			Topic:      pbsubscribe.Topic_ServiceHealth,
			Key:        srvReq.ServiceName,
			Token:      srvReq.Token,
			Datacenter: srvReq.Datacenter,
			Index:      index,
		}
		if srvReq.Connect {
			req.Topic = pbsubscribe.Topic_ServiceHealthConnect
		}
		return req
	}

	m, err := newMaterializer(c.deps, newReqFn, srvReq.Filter)
	if err != nil {
		return cache.FetchResult{}, err
	}
	ctx, cancel := context.WithCancel(context.TODO())
	go m.Run(ctx)

	result, err := m.Fetch(ctx.Done(), opts)
	result.State = &streamingHealthState{
		materializer: m,
		done:         ctx.Done(),
		cancel:       cancel,
	}
	return result, err
}

func newMaterializer(
	d MaterializerDeps,
	r func(uint64) pbsubscribe.SubscribeRequest,
	filter string,
) (*submatview.Materializer, error) {
	view, err := newHealthViewState(filter)
	if err != nil {
		return nil, err
	}
	return submatview.NewMaterializer(submatview.Deps{
		View:   view,
		Client: d.Client,
		Logger: d.Logger,
		Waiter: &retry.Waiter{
			MinFailures: 1,
			MinWait:     0,
			MaxWait:     60 * time.Second,
			Jitter:      retry.NewJitter(100),
		},
		Request: r,
	}), nil
}

type streamingHealthState struct {
	materializer *submatview.Materializer
	done         <-chan struct{}
	cancel       func()
}

func (c *streamingHealthState) Close() error {
	c.cancel()
	return nil
}

func newHealthViewState(filterExpr string) (submatview.View, error) {
	s := &healthView{state: make(map[string]structs.CheckServiceNode)}

	// We apply filtering to the raw CheckServiceNodes before we are done mutating
	// state in Update to save from storing stuff in memory we'll only filter
	// later. Because the state is just a map of those types, we can simply run
	// that map through filter and it will remove any entries that don't match.
	var err error
	s.filter, err = bexpr.CreateFilter(filterExpr, nil, s.state)
	return s, err
}

// healthView implements submatview.View for storing the view state
// of a service health result. We store it as a map to make updates and
// deletions a little easier but we could just store a result type
// (IndexedCheckServiceNodes) and update it in place for each event - that
// involves re-sorting each time etc. though.
type healthView struct {
	state map[string]structs.CheckServiceNode
	// TODO: test case with filter
	filter *bexpr.Filter
}

// Update implements View
func (s *healthView) Update(events []*pbsubscribe.Event) error {
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
func (s *healthView) Result(index uint64) (interface{}, error) {
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

func (s *healthView) Reset() {
	s.state = make(map[string]structs.CheckServiceNode)
}
