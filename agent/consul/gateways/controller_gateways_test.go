package gateways

import (
	"context"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"testing"
)

func Test_apiGatewayReconciler_Reconcile(t *testing.T) {
	type fields struct {
		logger hclog.Logger
		store  DataStore
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
				store:  datastoreWithUpdate(t),
				logger: hclog.Default(),
			},
			args: args{
				ctx: context.Background(),
				req: controller.Request{
					Kind: structs.APIGateway,
					Name: "test-gateway",
					Meta: acl.DefaultEnterpriseMeta(),
				},
			},
			wantErr: false,
		},
		{
			name: "delete happy path",
			fields: fields{
				store:  datastoreWithDelete(t),
				logger: hclog.Default(),
			},
			args: args{
				ctx: context.Background(),
				req: controller.Request{
					Kind: structs.APIGateway,
					Name: "test-gateway",
					Meta: acl.DefaultEnterpriseMeta(),
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

func datastoreWithUpdate(t *testing.T) *MockDataStore {
	ds := NewMockDataStore(t)
	ds.On("GetConfigEntry", structs.APIGateway, mock.Anything, mock.Anything).Return(&structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "test-gateway",
		Listeners: []structs.APIGatewayListener{
			{
				Name:     "test-listener",
				Protocol: "tcp",
				Port:     8080,
			},
		},
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}, nil)
	ds.On("GetConfigEntry", structs.BoundAPIGateway, mock.Anything, mock.Anything).Return(
		&structs.BoundAPIGatewayConfigEntry{
			Kind:           structs.BoundAPIGateway,
			Name:           "test-gateway",
			Listeners:      []structs.BoundAPIGatewayListener{},
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}, nil)

	ds.On("GetConfigEntriesByKind", structs.TCPRoute).Return([]structs.ConfigEntry{
		&structs.TCPRouteConfigEntry{
			Kind: structs.TCPRoute,
			Name: "test-route",
			Parents: []structs.ResourceReference{
				{
					Kind:           structs.APIGateway,
					Name:           "test-gateway",
					SectionName:    "test-listener",
					EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
				},
			},
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		},
	}, nil)

	ds.On("Update", mock.Anything).Return(nil)
	ds.On("UpdateStatus", mock.Anything, mock.Anything).Return(nil)
	return ds
}

func datastoreWithDelete(t *testing.T) *MockDataStore {
	ds := NewMockDataStore(t)
	ds.On("GetConfigEntry", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	ds.On("Delete", mock.Anything).Return(nil)
	return ds
}
