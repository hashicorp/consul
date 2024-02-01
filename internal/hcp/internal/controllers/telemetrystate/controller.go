// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package telemetrystate

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/link"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	globalID = &pbresource.ID{
		Name:    "global",
		Type:    pbhcp.TelemetryStateType,
		Tenancy: &pbresource.Tenancy{},
	}
)

const MetaKeyDebugSkipDeletion = StatusKey + "/debug/skip-deletion"

func TelemetryStateController(hcpClientFn link.HCPClientFn) *controller.Controller {
	return controller.NewController(StatusKey, pbhcp.TelemetryStateType).
		WithWatch(pbhcp.LinkType, dependency.ReplaceType(pbhcp.TelemetryStateType)).
		WithReconciler(&telemetryStateReconciler{
			hcpClientFn: hcpClientFn,
		})
}

type telemetryStateReconciler struct {
	hcpClientFn link.HCPClientFn
}

func (r *telemetryStateReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// The runtime is passed by value so replacing it here for the remainder of this
	// reconciliation request processing will not affect future invocations.
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)

	rt.Logger.Trace("reconciling telemetry-state")

	// First get the link resource in order to build a hcp client. If the link resource
	// doesn't exist then the telemetry-state should not exist either.
	res, err := getLinkResource(ctx, rt)
	if err != nil {
		rt.Logger.Error("failed to lookup Link resource", "error", err)
		return err
	}
	if res == nil {
		return ensureTelemetryStateDeleted(ctx, rt)
	}

	// Check that the link resource indicates the cluster is linked
	// If the cluster is not linked, the telemetry-state resource should not exist
	if linked, reason := link.IsLinked(res.GetResource()); !linked {
		rt.Logger.Trace("cluster is not linked", "reason", reason)
		return ensureTelemetryStateDeleted(ctx, rt)
	}

	hcpClient, err := r.hcpClientFn(link.CloudConfigFromLink(res.GetData()))
	if err != nil {
		rt.Logger.Error("error creating HCP Client", "error", err)
		return err
	}

	// Get the telemetry configuration and observability scoped credentials from hcp
	tCfg, err := hcpClient.FetchTelemetryConfig(ctx)
	if err != nil {
		rt.Logger.Error("error requesting telemetry config", "error", err)
		return err
	}
	clientID, clientSecret, err := hcpClient.GetObservabilitySecret(ctx)
	if err != nil {
		rt.Logger.Error("error requesting telemetry credentials", "error", err)
		return nil
	}

	// TODO allow hcp client config override from hcp TelemetryConfig
	hcpCfg := res.GetData().GetHcpConfig()

	// TODO implement proxy options from hcp
	proxyCfg := &pbhcp.ProxyConfig{}

	state := &pbhcp.TelemetryState{
		ResourceId:   res.GetData().ResourceId,
		ClientId:     clientID,
		ClientSecret: clientSecret,
		HcpConfig:    hcpCfg,
		Proxy:        proxyCfg,
		Metrics: &pbhcp.MetricsConfig{
			Labels:   tCfg.MetricsConfig.Labels,
			Disabled: tCfg.MetricsConfig.Disabled,
		},
	}

	if tCfg.MetricsConfig.Endpoint != nil {
		state.Metrics.Endpoint = tCfg.MetricsConfig.Endpoint.String()
	}
	if tCfg.MetricsConfig.Filters != nil {
		state.Metrics.IncludeList = []string{tCfg.MetricsConfig.Filters.String()}
	}

	if err := writeTelemetryStateIfUpdated(ctx, rt, state); err != nil {
		rt.Logger.Error("error updating telemetry-state", "error", err)
		return err
	}

	return nil
}

func ensureTelemetryStateDeleted(ctx context.Context, rt controller.Runtime) error {
	resp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: &pbresource.ID{Name: "global", Type: pbhcp.TelemetryStateType}})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}

	rt.Logger.Trace("deleting telemetry-state")
	if _, ok := resp.GetResource().Metadata[MetaKeyDebugSkipDeletion]; ok {
		rt.Logger.Debug("skip-deletion metadata key found, skipping deletion of telemetry-state resource")
		return nil
	}

	if _, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: resp.GetResource().GetId()}); err != nil {
		rt.Logger.Error("error deleting telemetry-state resource", "error", err)
		return err
	}
	return nil
}

func writeTelemetryStateIfUpdated(ctx context.Context, rt controller.Runtime, state *pbhcp.TelemetryState) error {
	currentState, err := getTelemetryStateResource(ctx, rt)
	if err != nil {
		return err
	}

	if currentState != nil && proto.Equal(currentState.GetData(), state) {
		return nil
	}

	stateData, err := anypb.New(state)
	if err != nil {
		return err
	}

	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: &pbresource.Resource{
		Id: &pbresource.ID{
			Name: "global",
			Type: pbhcp.TelemetryStateType,
		},
		Data: stateData,
	}})
	return err
}

func getGlobalResource(ctx context.Context, rt controller.Runtime, t *pbresource.Type) (*pbresource.Resource, error) {
	resp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: &pbresource.ID{Name: "global", Type: t}})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	return resp.GetResource(), nil
}

// getLinkResource returns the cluster scoped pbhcp.Link resource. If the resource is not found a nil
// pointer and no error will be returned.
func getLinkResource(ctx context.Context, rt controller.Runtime) (*types.DecodedLink, error) {
	res, err := getGlobalResource(ctx, rt, pbhcp.LinkType)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return resource.Decode[*pbhcp.Link](res)
}

func getTelemetryStateResource(ctx context.Context, rt controller.Runtime) (*types.DecodedTelemetryState, error) {
	res, err := getGlobalResource(ctx, rt, pbhcp.TelemetryStateType)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return resource.Decode[*pbhcp.TelemetryState](res)
}
