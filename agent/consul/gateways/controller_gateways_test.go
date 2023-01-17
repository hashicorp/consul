package gateways

import (
	"context"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/gateways/datastore"
	"github.com/hashicorp/go-hclog"
	"testing"
)

func Test_apiGatewayReconciler_Reconcile(t *testing.T) {
	type fields struct {
		fsm    *fsm.FSM
		logger hclog.Logger
		store  datastore.DataStore
	}
	type args struct {
		ctx context.Context
		req controller.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := apiGatewayReconciler{
				fsm:    tt.fields.fsm,
				logger: tt.fields.logger,
				store:  tt.fields.store,
			}
			if err := r.Reconcile(tt.args.ctx, tt.args.req); (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
