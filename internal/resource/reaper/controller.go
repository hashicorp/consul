// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package reaper

import (
	"context"
	"time"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	statusKeyReaperController = "consul.io/reaper-controller"
	secondPassDelay           = 30 * time.Second
)

// RegisterControllers registers controllers for the tombstone type.
func RegisterControllers(mgr *controller.Manager) {
	mgr.Register(reaperController())
}

func reaperController() controller.Controller {
	return controller.ForType(resource.TypeV1Tombstone).
		WithReconciler(&tombstoneReconciler{})
}

type tombstoneReconciler struct{}

// Given successful completion of a pass, figures out
// what the next step is and updates the resource
// conditions accordingly.
//
// Returns nil if 1st and 2nd pass completed.
// Returns RequeueAfterError if 1st pass completed.
func nextStep(ctx context.Context, rt controller.Runtime, tombstone *pbresource.Resource) error {

	status, ok := tombstone.Status[statusKeyReaperController]
	if !ok {
		status = &pbresource.Status{
			ObservedGeneration: tombstone.Generation,
			Conditions:         make([]*pbresource.Condition, 0),
		}
	}

	_ = status

	if len(status.Conditions) == 0 {
		// We just completed the first pass, queue up the second pass
		status.Conditions = append(status.Conditions, &pbresource.Condition{
			Type:    "FirstPassCompleted",
			State:   pbresource.Condition_STATE_TRUE,
			Reason:  "Success",
			Message: "Reaper first pass completed",
		})

		return controller.RequeueAfterError(secondPassDelay)
	} else {
		// We completed the second pass and we're all done
		return nil
	}
}

// Deletes all owned (child) resources of an owner (parent) resource.
//
// The reconciliation for tombstones is split into two passes.
// The first pass attempts to delete child resources created before the tombstone was created.
// The second pass is run after a reasonable delay to delete child resources that may have been
// created during or after the completion of the first pass.
func (r *tombstoneReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil
	case err != nil:
		return err
	}
	res := rsp.Resource

	var tombstone pbresource.Tombstone
	if err := res.Data.UnmarshalTo(&tombstone); err != nil {
		return err
	}

	// Retrieve owner's children
	listRsp, err := rt.Client.ListByOwner(ctx, &pbresource.ListByOwnerRequest{Owner: tombstone.Owner})
	if err != nil {
		return err
	}

	// Delete each child
	success := true
	for _, child := range listRsp.Resources {
		_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: child.Id})
		if err != nil {
			success = success && false
		}
	}

	// All successful, delete the tombstone resource
	if success {
		_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: res.Id})
		if err != nil {
			return err
		}
		return nil
	} else {
		return controller.RequeueAfter(time.Second * 30)
	}
	// What conditions should a tombstone have?
	// What is a must vs nice to have?

	// Case 1: Tombstone we've never encountered
	// Case 2: Tombstone that we've seen before
	// conditions := []*pbresource.Condition{
	// 	{
	// 		Type:    "Accepted",
	// 		State:   pbresource.Condition_STATE_TRUE,
	// 		Reason:  "Accepted",
	// 		Message: fmt.Sprintf("Artist '%s' accepted", artist.Name),
	// 	},
	// }

	// numAlbums := 3
	// if artist.Genre == pbdemov2.Genre_GENRE_BLUES {
	// 	numAlbums = 10
	// }

	// desiredAlbums, err := generateV2AlbumsDeterministic(res.Id, numAlbums)
	// if err != nil {
	// 	return err
	// }

	// actualAlbums, err := rt.Client.List(ctx, &pbresource.ListRequest{
	// 	Type:       TypeV2Album,
	// 	Tenancy:    res.Id.Tenancy,
	// 	NamePrefix: fmt.Sprintf("%s/", res.Id.Name),
	// })
	// if err != nil {
	// 	return err
	// }

	// writes, deletions, err := diffAlbums(desiredAlbums, actualAlbums.Resources)
	// if err != nil {
	// 	return err
	// }
	// for _, w := range writes {
	// 	if _, err := rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: w}); err != nil {
	// 		return err
	// 	}
	// }
	// for _, d := range deletions {
	// 	if _, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: d}); err != nil {
	// 		return err
	// 	}
	// }

	// for _, want := range desiredAlbums {
	// 	var album pbdemov2.Album
	// 	if err := want.Data.UnmarshalTo(&album); err != nil {
	// 		return err
	// 	}
	// 	conditions = append(conditions, &pbresource.Condition{
	// 		Type:     "AlbumCreated",
	// 		State:    pbresource.Condition_STATE_TRUE,
	// 		Reason:   "AlbumCreated",
	// 		Message:  fmt.Sprintf("Album '%s' created for artist '%s'", album.Title, artist.Name),
	// 		Resource: resource.Reference(want.Id, ""),
	// 	})
	// }

	// newStatus := &pbresource.Status{
	// 	ObservedGeneration: res.Generation,
	// 	Conditions:         conditions,
	// }

	// if proto.Equal(res.Status[statusKeyArtistController], newStatus) {
	// 	return nil
	// }

	// _, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
	// 	Id:     res.Id,
	// 	Key:    statusKeyArtistController,
	// 	Status: newStatus,
	// })
	// return err
}

// func diffAlbums(want, have []*pbresource.Resource) ([]*pbresource.Resource, []*pbresource.ID, error) {
// 	haveMap := make(map[string]*pbresource.Resource, len(have))
// 	for _, r := range have {
// 		haveMap[r.Id.Name] = r
// 	}

// 	wantMap := make(map[string]struct{}, len(want))
// 	for _, r := range want {
// 		wantMap[r.Id.Name] = struct{}{}
// 	}

// 	writes := make([]*pbresource.Resource, 0)
// 	for _, w := range want {
// 		h, ok := haveMap[w.Id.Name]
// 		if ok {
// 			var wd, hd pbdemov2.Album
// 			if err := w.Data.UnmarshalTo(&wd); err != nil {
// 				return nil, nil, err
// 			}
// 			if err := h.Data.UnmarshalTo(&hd); err != nil {
// 				return nil, nil, err
// 			}
// 			if proto.Equal(&wd, &hd) {
// 				continue
// 			}
// 		}

// 		writes = append(writes, w)
// 	}

// 	deletions := make([]*pbresource.ID, 0)
// 	for _, h := range have {
// 		if _, ok := wantMap[h.Id.Name]; ok {
// 			continue
// 		}
// 		deletions = append(deletions, h.Id)
// 	}

// 	return writes, deletions, nil
// }

// func generateV2AlbumsDeterministic(artistID *pbresource.ID, count int) ([]*pbresource.Resource, error) {
// 	uid, err := ulid.Parse(artistID.Uid)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to parse Uid: %w", err)
// 	}
// 	rand := rand.New(rand.NewSource(int64(uid.Time())))

// 	albums := make([]*pbresource.Resource, count)
// 	for i := 0; i < count; i++ {
// 		album, err := generateV2Album(artistID, rand)
// 		if err != nil {
// 			return nil, err
// 		}
// 		// Add suffix to avoid collisions.
// 		album.Id.Name = fmt.Sprintf("%s-%d", album.Id.Name, i)
// 		albums[i] = album
// 	}
// 	return albums, nil
// }
