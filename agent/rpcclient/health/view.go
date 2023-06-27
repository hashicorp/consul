// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package health

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/go-bexpr"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

func NewMaterializerRequest(srvReq structs.ServiceSpecificRequest) func(index uint64) *pbsubscribe.SubscribeRequest {
	return func(index uint64) *pbsubscribe.SubscribeRequest {
		req := &pbsubscribe.SubscribeRequest{
			Topic: pbsubscribe.Topic_ServiceHealth,
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key:       srvReq.ServiceName,
					Namespace: srvReq.EnterpriseMeta.NamespaceOrEmpty(),
					Partition: srvReq.EnterpriseMeta.PartitionOrEmpty(),
					PeerName:  srvReq.PeerName,
				},
			},
			Token:      srvReq.Token,
			Datacenter: srvReq.Datacenter,
			Index:      index,
		}
		if srvReq.Connect {
			req.Topic = pbsubscribe.Topic_ServiceHealthConnect
		}
		return req
	}
}

func NewHealthView(req structs.ServiceSpecificRequest) (*HealthView, error) {
	fe, err := newFilterEvaluator(req)
	if err != nil {
		return nil, err
	}
	return &HealthView{
		state:   make(map[string]structs.CheckServiceNode),
		filter:  fe,
		connect: req.Connect,
		kind:    req.ServiceKind,
	}, nil
}

var _ submatview.View = (*HealthView)(nil)

// HealthView implements submatview.View for storing the view state
// of a service health result. We store it as a map to make updates and
// deletions a little easier but we could just store a result type
// (IndexedCheckServiceNodes) and update it in place for each event - that
// involves re-sorting each time etc. though.
type HealthView struct {
	connect bool
	kind    structs.ServiceKind
	state   map[string]structs.CheckServiceNode
	filter  filterEvaluator
}

// Update implements View
func (s *HealthView) Update(events []*pbsubscribe.Event) error {
	for _, event := range events {
		serviceHealth := event.GetServiceHealth()
		if serviceHealth == nil {
			return fmt.Errorf("unexpected event type for service health view: %T",
				event.GetPayload())
		}

		id := serviceHealth.CheckServiceNode.UniqueID()
		switch serviceHealth.Op {
		case pbsubscribe.CatalogOp_Register:
			csn, err := pbservice.CheckServiceNodeToStructs(serviceHealth.CheckServiceNode)
			if err != nil {
				return err
			}
			if csn == nil {
				return errors.New("check service node was unexpectedly nil")
			}

			// check if we intentionally need to skip the filter
			if s.skipFilter(csn) {
				s.state[id] = *csn
				continue
			}

			passed, err := s.filter.Evaluate(*csn)
			if err != nil {
				return err
			} else if passed {
				s.state[id] = *csn
			} else {
				delete(s.state, id)
			}

		case pbsubscribe.CatalogOp_Deregister:
			delete(s.state, id)
		}
	}
	return nil
}

func (s *HealthView) skipFilter(csn *structs.CheckServiceNode) bool {
	// we only do this for connect-enabled services that need to be routed through a terminating gateway
	return s.kind == "" && s.connect && csn.Service.Kind == structs.ServiceKindTerminatingGateway
}

type filterEvaluator interface {
	Evaluate(datum interface{}) (bool, error)
}

func newFilterEvaluator(req structs.ServiceSpecificRequest) (filterEvaluator, error) {
	var evaluators []filterEvaluator

	typ := reflect.TypeOf(structs.CheckServiceNode{})
	if req.Filter != "" {
		e, err := bexpr.CreateEvaluatorForType(req.Filter, nil, typ)
		if err != nil {
			return nil, err
		}
		evaluators = append(evaluators, e)
	}

	if req.ServiceTag != "" {
		// Handle backwards compat with old field
		req.ServiceTags = []string{req.ServiceTag}
	}

	if req.TagFilter && len(req.ServiceTags) > 0 {
		evaluators = append(evaluators, serviceTagEvaluator{tags: req.ServiceTags})
	}

	for key, value := range req.NodeMetaFilters {
		expr := fmt.Sprintf(`"%s" in Node.Meta.%s`, value, key)
		e, err := bexpr.CreateEvaluatorForType(expr, nil, typ)
		if err != nil {
			return nil, err
		}
		evaluators = append(evaluators, e)
	}

	switch len(evaluators) {
	case 0:
		return noopFilterEvaluator{}, nil
	case 1:
		return evaluators[0], nil
	default:
		return &multiFilterEvaluator{evaluators: evaluators}, nil
	}
}

// noopFilterEvaluator may be used in place of a bexpr.Evaluator. The Evaluate
// method always return true, so no items will be filtered out.
type noopFilterEvaluator struct{}

func (noopFilterEvaluator) Evaluate(_ interface{}) (bool, error) {
	return true, nil
}

type multiFilterEvaluator struct {
	evaluators []filterEvaluator
}

func (m multiFilterEvaluator) Evaluate(data interface{}) (bool, error) {
	for _, e := range m.evaluators {
		match, err := e.Evaluate(data)
		if !match || err != nil {
			return match, err
		}
	}
	return true, nil
}

// sortCheckServiceNodes sorts the results to match memdb semantics
// Sort results by Node.Node, if 2 instances match, order by Service.ID
// Will allow result to be stable sorted and match queries without cache
func sortCheckServiceNodes(serviceNodes *structs.IndexedCheckServiceNodes) {
	sort.SliceStable(serviceNodes.Nodes, func(i, j int) bool {
		left := serviceNodes.Nodes[i]
		right := serviceNodes.Nodes[j]
		if left.Node.Node == right.Node.Node {
			return left.Service.ID < right.Service.ID
		}
		return left.Node.Node < right.Node.Node
	})
}

// Result returns the structs.IndexedCheckServiceNodes stored by this view.
func (s *HealthView) Result(index uint64) interface{} {
	result := structs.IndexedCheckServiceNodes{
		Nodes: make(structs.CheckServiceNodes, 0, len(s.state)),
		QueryMeta: structs.QueryMeta{
			Index:   index,
			Backend: structs.QueryBackendStreaming,
		},
	}
	for _, node := range s.state {
		result.Nodes = append(result.Nodes, node)
	}
	sortCheckServiceNodes(&result)

	return &result
}

func (s *HealthView) Reset() {
	s.state = make(map[string]structs.CheckServiceNode)
}

// serviceTagEvaluator implements the filterEvaluator to perform filtering
// by service tags. bexpr can not be used at this time, because the filtering
// must be case insensitive for backwards compatibility. In the future this
// may be replaced with bexpr once case insensitive support is added.
type serviceTagEvaluator struct {
	tags []string
}

func (m serviceTagEvaluator) Evaluate(data interface{}) (bool, error) {
	csn, ok := data.(structs.CheckServiceNode)
	if !ok {
		return false, fmt.Errorf("unexpected type %T for structs.CheckServiceNode filter", data)
	}
	for _, tag := range m.tags {
		if !serviceHasTag(csn.Service, tag) {
			// If any one of the expected tags was not found, filter the service
			return false, nil
		}
	}
	return true, nil
}

func serviceHasTag(sn *structs.NodeService, tag string) bool {
	for _, t := range sn.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}
