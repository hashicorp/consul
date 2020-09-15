package grpc

import (
	"context"
	"time"

	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
)

type simple struct {
	name string
	dc   string
}

func (s *simple) Flow(_ *testservice.Req, flow testservice.Simple_FlowServer) error {
	for flow.Context().Err() == nil {
		resp := &testservice.Resp{ServerName: "one", Datacenter: s.dc}
		if err := flow.Send(resp); err != nil {
			return err
		}
		time.Sleep(time.Millisecond)
	}
	return nil
}

func (s *simple) Something(_ context.Context, _ *testservice.Req) (*testservice.Resp, error) {
	return &testservice.Resp{ServerName: s.name, Datacenter: s.dc}, nil
}
