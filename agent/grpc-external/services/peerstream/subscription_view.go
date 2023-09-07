// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"fmt"
	"sort"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

type Subscriber interface {
	Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error)
}

type exportedServiceRequest struct {
	logger hclog.Logger
	req    structs.ServiceSpecificRequest
	sub    Subscriber
}

func newExportedStandardServiceRequest(logger hclog.Logger, svc structs.ServiceName, sub Subscriber) *exportedServiceRequest {
	req := structs.ServiceSpecificRequest{
		ServiceName:    svc.Name,
		Connect:        false,
		EnterpriseMeta: svc.EnterpriseMeta,
	}
	return &exportedServiceRequest{
		logger: logger,
		req:    req,
		sub:    sub,
	}
}

// CacheInfo implements submatview.Request
func (e *exportedServiceRequest) CacheInfo() cache.RequestInfo {
	return e.req.CacheInfo()
}

func (e *exportedServiceRequest) getTopic() pbsubscribe.Topic {
	if e.req.Connect {
		return pbsubscribe.Topic_ServiceHealthConnect
	}
	// Using the Topic_ServiceHealth will ignore proxies unless the ServiceName is a proxy name.
	return pbsubscribe.Topic_ServiceHealth
}

// NewMaterializer implements submatview.Request
func (e *exportedServiceRequest) NewMaterializer() (submatview.Materializer, error) {
	// TODO(peering): reinstate this
	// if e.req.Connect {
	// 	return nil, fmt.Errorf("connect views are not supported")
	// }
	reqFn := func(index uint64) *pbsubscribe.SubscribeRequest {
		return &pbsubscribe.SubscribeRequest{
			Topic: e.getTopic(),
			Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
				NamedSubject: &pbsubscribe.NamedSubject{
					Key:       e.req.ServiceName,
					Namespace: e.req.EnterpriseMeta.NamespaceOrEmpty(),
					Partition: e.req.EnterpriseMeta.PartitionOrEmpty(),
				},
			},
			Token:      e.req.Token,
			Datacenter: e.req.Datacenter,
			Index:      index,
		}
	}
	deps := submatview.LocalMaterializerDeps{
		Backend:     e.sub,
		ACLResolver: resolver.DANGER_NO_AUTH{},
		Deps: submatview.Deps{
			View:    newExportedServicesView(),
			Logger:  e.logger,
			Request: reqFn,
		},
	}
	return submatview.NewLocalMaterializer(deps), nil
}

// Type implements submatview.Request
func (e *exportedServiceRequest) Type() string {
	return "leader.peering.stream.exportedServiceRequest"
}

// exportedServicesView implements submatview.View for storing the view state
// of an exported service's health result. We store it as a map to make updates and
// deletions a little easier but we could just store a result type
// (IndexedCheckServiceNodes) and update it in place for each event - that
// involves re-sorting each time etc. though.
//
// Unlike rpcclient.healthView, there is no need for a filter because for exported services
// we export all instances unconditionally.
type exportedServicesView struct {
	state map[string]*pbservice.CheckServiceNode
}

func newExportedServicesView() *exportedServicesView {
	return &exportedServicesView{
		state: make(map[string]*pbservice.CheckServiceNode),
	}
}

// Reset implements submatview.View
func (s *exportedServicesView) Reset() {
	s.state = make(map[string]*pbservice.CheckServiceNode)
}

// Update implements submatview.View
func (s *exportedServicesView) Update(events []*pbsubscribe.Event) error {
	for _, event := range events {
		serviceHealth := event.GetServiceHealth()
		if serviceHealth == nil {
			return fmt.Errorf("unexpected event type for service health view: %T",
				event.GetPayload())
		}

		id := serviceHealth.CheckServiceNode.UniqueID()
		switch serviceHealth.Op {
		case pbsubscribe.CatalogOp_Register:
			s.state[id] = serviceHealth.CheckServiceNode

		case pbsubscribe.CatalogOp_Deregister:
			delete(s.state, id)
		}
	}
	return nil
}

// Result returns the CheckServiceNodes stored by this view.
// Result implements submatview.View
func (s *exportedServicesView) Result(index uint64) interface{} {
	result := pbservice.IndexedCheckServiceNodes{
		Nodes: make([]*pbservice.CheckServiceNode, 0, len(s.state)),
		Index: index,
	}
	for _, node := range s.state {
		result.Nodes = append(result.Nodes, node)
	}
	sortCheckServiceNodes(&result)

	return &result
}

// sortCheckServiceNodes stable sorts the results to match memdb semantics.
func sortCheckServiceNodes(n *pbservice.IndexedCheckServiceNodes) {
	sort.SliceStable(n.Nodes, func(i, j int) bool {
		return n.Nodes[i].UniqueID() < n.Nodes[j].UniqueID()
	})
}
