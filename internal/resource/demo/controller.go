package demo

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
)

const statusKeyArtistController = "consul.io/artist-controller"

// RegisterControllers registers controllers for the demo types. Should only be
// called in dev mode.
func RegisterControllers(mgr *controller.Manager) {
	mgr.Register(artistController())
}

func artistController() controller.Controller {
	return controller.ForType(TypeV2Artist).
		WithWatch(TypeV2Album, controller.MapOwner).
		WithReconciler(&artistReconciler{})
}

type artistReconciler struct{}

func (r *artistReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil
	case err != nil:
		return err
	}

	res := rsp.Resource

	var artist pbdemov2.Artist
	if err := res.Data.UnmarshalTo(&artist); err != nil {
		return err
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions: []*pbresource.Condition{
			{
				Type:    "Accepted",
				State:   pbresource.Condition_STATE_TRUE,
				Reason:  "Accepted",
				Message: fmt.Sprintf("Artist '%s' accepted", artist.Name),
			},
		},
	}

	if proto.Equal(res.Status[statusKeyArtistController], newStatus) {
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    statusKeyArtistController,
		Status: newStatus,
	})
	return err
}
