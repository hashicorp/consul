package cachetype

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/lib/retry"
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

// RegisterOptions returns options with a much shorter LastGetTTL than the default.
// Unlike other cache-types, StreamingHealthServices runs a materialized view in
// the background which will receive streamed events from a server. If the cache
// is not being used, that stream uses memory on the server and network transfer
// between the client and the server.
// The materialize view and the stream are stopped when the cache entry expires,
// so using a shorter TTL ensures the cache entry expires sooner.
func (c *StreamingHealthServices) RegisterOptions() cache.RegisterOptions {
	opts := c.RegisterOptionsBlockingRefresh.RegisterOptions()
	opts.LastGetTTL = 10 * time.Minute
	return opts
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

// Fetch service health from the materialized view. If no materialized view
// exists, create one and start it running in a goroutine. The goroutine will
// exit when the cache entry storing the result is expired, the cache will call
// Close on the result.State.
//
// Fetch implements part of the cache.Type interface, and assumes that the
// caller ensures that only a single call to Fetch is running at any time.
func (c *StreamingHealthServices) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	if opts.LastResult != nil && opts.LastResult.State != nil {
		return opts.LastResult.State.(*streamingHealthState).Fetch(opts)
	}

	srvReq := req.(*structs.ServiceSpecificRequest)
	newReqFn := func(index uint64) pbsubscribe.SubscribeRequest {
		req := pbsubscribe.SubscribeRequest{
			Topic:      pbsubscribe.Topic_ServiceHealth,
			Key:        srvReq.ServiceName,
			Token:      srvReq.Token,
			Datacenter: srvReq.Datacenter,
			Index:      index,
			Namespace:  srvReq.EnterpriseMeta.GetNamespace(),
		}
		if srvReq.Connect {
			req.Topic = pbsubscribe.Topic_ServiceHealthConnect
		}
		return req
	}

	materializer, err := newMaterializer(c.deps, newReqFn, srvReq.Filter)
	if err != nil {
		return cache.FetchResult{}, err
	}
	ctx, cancel := context.WithCancel(context.TODO())
	go materializer.Run(ctx)

	state := &streamingHealthState{
		materializer: materializer,
		done:         ctx.Done(),
		cancel:       cancel,
	}
	return state.Fetch(opts)
}

func newMaterializer(
	deps MaterializerDeps,
	newRequestFn func(uint64) pbsubscribe.SubscribeRequest,
	filter string,
) (*submatview.Materializer, error) {
	view, err := newHealthView(filter)
	if err != nil {
		return nil, err
	}
	return submatview.NewMaterializer(submatview.Deps{
		View:   view,
		Client: deps.Client,
		Logger: deps.Logger,
		Waiter: &retry.Waiter{
			MinFailures: 1,
			MinWait:     0,
			MaxWait:     60 * time.Second,
			Jitter:      retry.NewJitter(100),
		},
		Request: newRequestFn,
	}), nil
}

// streamingHealthState wraps a Materializer to manage its lifecycle, and to
// add itself to the FetchResult.State.
type streamingHealthState struct {
	materializer *submatview.Materializer
	done         <-chan struct{}
	cancel       func()
}

func (s *streamingHealthState) Close() error {
	s.cancel()
	return nil
}

func (s *streamingHealthState) Fetch(opts cache.FetchOptions) (cache.FetchResult, error) {
	result, err := s.materializer.Fetch(s.done, opts)
	result.State = s
	return result, err
}

func newHealthView(filterExpr string) (*healthView, error) {
	fe, err := newFilterEvaluator(filterExpr)
	if err != nil {
		return nil, err
	}
	return &healthView{
		state:  make(map[string]structs.CheckServiceNode),
		filter: fe,
	}, nil
}

// healthView implements submatview.View for storing the view state
// of a service health result. We store it as a map to make updates and
// deletions a little easier but we could just store a result type
// (IndexedCheckServiceNodes) and update it in place for each event - that
// involves re-sorting each time etc. though.
type healthView struct {
	state  map[string]structs.CheckServiceNode
	filter filterEvaluator
}

// Update implements View
func (s *healthView) Update(events []*pbsubscribe.Event) error {
	for _, event := range events {
		serviceHealth := event.GetServiceHealth()
		if serviceHealth == nil {
			return fmt.Errorf("unexpected event type for service health view: %T",
				event.GetPayload())
		}

		id := serviceHealth.CheckServiceNode.UniqueID()
		switch serviceHealth.Op {
		case pbsubscribe.CatalogOp_Register:
			csn := *pbservice.CheckServiceNodeToStructs(serviceHealth.CheckServiceNode)
			passed, err := s.filter.Evaluate(csn)
			switch {
			case err != nil:
				return err
			case passed:
				s.state[id] = csn
			}

		case pbsubscribe.CatalogOp_Deregister:
			delete(s.state, id)
		}
	}
	return nil
}

type filterEvaluator interface {
	Evaluate(datum interface{}) (bool, error)
}

func newFilterEvaluator(expr string) (filterEvaluator, error) {
	if expr == "" {
		return noopFilterEvaluator{}, nil
	}
	return bexpr.CreateEvaluatorForType(expr, nil, reflect.TypeOf(structs.CheckServiceNode{}))
}

// noopFilterEvaluator may be used in place of a bexpr.Evaluator. The Evaluate
// method always return true, so no items will be filtered out.
type noopFilterEvaluator struct{}

func (noopFilterEvaluator) Evaluate(_ interface{}) (bool, error) {
	return true, nil
}

// Result returns the structs.IndexedCheckServiceNodes stored by this view.
func (s *healthView) Result(index uint64) (interface{}, error) {
	result := structs.IndexedCheckServiceNodes{
		Nodes: make(structs.CheckServiceNodes, 0, len(s.state)),
		QueryMeta: structs.QueryMeta{
			Index: index,
		},
	}
	for _, node := range s.state {
		result.Nodes = append(result.Nodes, node)
	}
	return &result, nil
}

func (s *healthView) Reset() {
	s.state = make(map[string]structs.CheckServiceNode)
}
