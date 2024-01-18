package telemetrystate

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	if resource.EqualID(req.ID, globalID) {

	}

	// First get the link resource in order to build a hcp client. If the link resource
	// doesn't exist then the telemetry-state should not exist either.
	res, err := getLinkResource(ctx, rt)
	if err != nil {
		rt.Logger.Error("failed to lookup Link resource", "error", err)
		return err
	}
	if res == nil {
		return r.ensureStateDeleted(ctx, rt)
	}

	// Check that the link resource indicates the cluster is linked
	// If the cluster is not linked, the telemetry-state resource should not exist
	if linked, reason := link.IsLinked(res.GetResource()); !linked {
		rt.Logger.Trace("cluster is not linked", "reason", reason)
		return r.ensureStateDeleted(ctx, rt)
	}

	hcpClient, err := r.hcpClientFn(res.Data)
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

	state := &pbhcp.TelemetryState{
		ResourceId:   res.GetData().ResourceId,
		ClientId:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     tCfg.Endpoint,
		Labels:       tCfg.MetricsConfig.Labels,
		Metrics: &pbhcp.MetricsState{
			Disabled: tCfg.MetricsConfig.Disabled,
		},
	}

	if tCfg.MetricsConfig.Endpoint != nil {
		state.Metrics.Endpoint = tCfg.MetricsConfig.Endpoint.String()
	}
	if tCfg.MetricsConfig.Filters != nil {
		state.Metrics.IncludeList = []string{tCfg.MetricsConfig.Filters.String()}
	}

	stateData, err := anypb.New(state)
	if err != nil {
		rt.Logger.Error("error marshalling telemetry-state data", "error", err)
		return err
	}

	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: &pbresource.Resource{
		Id: &pbresource.ID{
			Name: "global",
			Type: pbhcp.TelemetryStateType,
		},
		Data: stateData,
	}})
	if err != nil {
		rt.Logger.Error("error updating telemetry-state", "error", err)
		return err
	}

	return nil
}

func (r *telemetryStateReconciler) ensureStateDeleted(ctx context.Context, rt controller.Runtime) error {
	resp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: &pbresource.ID{Name: "global", Type: pbhcp.TelemetryStateType}})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}

	rt.Logger.Trace("deleting telemetry-state")
	if _, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: resp.GetResource().GetId()}); err != nil {
		rt.Logger.Error("error deleting telemetry-state resource", "error", err)
		return err
	}
	return nil
}

// getLinkResource returns the cluster scoped pbhcp.Link resource. If the resource is not found a nil
// pointer and no error will be returned.
func getLinkResource(ctx context.Context, rt controller.Runtime) (*types.DecodedLink, error) {
	resp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: &pbresource.ID{Name: "global", Type: pbhcp.LinkType}})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	return resource.Decode[*pbhcp.Link](resp.GetResource())
}
