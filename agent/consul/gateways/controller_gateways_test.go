package gateways

import (
	"context"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/gateways/datastore"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"testing"
)

func Test_apiGatewayReconciler_Reconcile(t *testing.T) {
	type fields struct {
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
		{
			name: "happy path - update available",
			fields: fields{
				store: datastoreWithUpdate(t),
			},
			args: args{
				ctx: context.Background(),
				req: controller.Request{
					Kind: structs.APIGateway,
					Name: "test-request",
				},
			},
			wantErr: false,
		},
		{
			name: "delete happy path",
			fields: fields{
				store: datastoreWithDelete(t),
			},
			args: args{
				ctx: context.Background(),
				req: controller.Request{
					Kind: structs.APIGateway,
					Name: "test-request",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := apiGatewayReconciler{
				logger: tt.fields.logger,
				store:  tt.fields.store,
			}
			if err := r.Reconcile(tt.args.ctx, tt.args.req); (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func datastoreWithUpdate(t *testing.T) *datastore.MockDataStore {
	ds := datastore.NewMockDataStore(t)
	ds.On("GetConfigEntry", mock.Anything, mock.Anything, mock.Anything).Return(structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "test",
		Listeners: []structs.APIGatewayListener{
			{
				Name: "test-listener",
			},
		},
	})
	return ds
}

func datastoreWithDelete(t *testing.T) *datastore.MockDataStore {
	ds := datastore.NewMockDataStore(t)
	ds.On("GetConfigEntry", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return ds
}
