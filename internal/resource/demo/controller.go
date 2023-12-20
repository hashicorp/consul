// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package demo

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
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
	conditions := []*pbresource.Condition{
		{
			Type:    "Accepted",
			State:   pbresource.Condition_STATE_TRUE,
			Reason:  "Accepted",
			Message: fmt.Sprintf("Artist '%s' accepted", artist.Name),
		},
	}

	numAlbums := 3
	if artist.Genre == pbdemov2.Genre_GENRE_BLUES {
		numAlbums = 10
	}

	desiredAlbums, err := generateV2AlbumsDeterministic(res.Id, numAlbums)
	if err != nil {
		return err
	}

	actualAlbums, err := rt.Client.List(ctx, &pbresource.ListRequest{
		Type:       TypeV2Album,
		Tenancy:    res.Id.Tenancy,
		NamePrefix: fmt.Sprintf("%s/", res.Id.Name),
	})
	if err != nil {
		return err
	}

	writes, deletions, err := diffAlbums(desiredAlbums, actualAlbums.Resources)
	if err != nil {
		return err
	}
	for _, w := range writes {
		if _, err := rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: w}); err != nil {
			return err
		}
	}
	for _, d := range deletions {
		if _, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: d}); err != nil {
			return err
		}
	}

	for _, want := range desiredAlbums {
		var album pbdemov2.Album
		if err := want.Data.UnmarshalTo(&album); err != nil {
			return err
		}
		conditions = append(conditions, &pbresource.Condition{
			Type:     "AlbumCreated",
			State:    pbresource.Condition_STATE_TRUE,
			Reason:   "AlbumCreated",
			Message:  fmt.Sprintf("Album '%s' created for artist '%s'", album.Title, artist.Name),
			Resource: resource.Reference(want.Id, ""),
		})
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions:         conditions,
	}

	if resource.EqualStatus(res.Status[statusKeyArtistController], newStatus, false) {
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    statusKeyArtistController,
		Status: newStatus,
	})
	return err
}

func diffAlbums(want, have []*pbresource.Resource) ([]*pbresource.Resource, []*pbresource.ID, error) {
	haveMap := make(map[string]*pbresource.Resource, len(have))
	for _, r := range have {
		haveMap[r.Id.Name] = r
	}

	wantMap := make(map[string]struct{}, len(want))
	for _, r := range want {
		wantMap[r.Id.Name] = struct{}{}
	}

	writes := make([]*pbresource.Resource, 0)
	for _, w := range want {
		h, ok := haveMap[w.Id.Name]
		if ok {
			var wd, hd pbdemov2.Album
			if err := w.Data.UnmarshalTo(&wd); err != nil {
				return nil, nil, err
			}
			if err := h.Data.UnmarshalTo(&hd); err != nil {
				return nil, nil, err
			}
			if proto.Equal(&wd, &hd) {
				continue
			}
		}

		writes = append(writes, w)
	}

	deletions := make([]*pbresource.ID, 0)
	for _, h := range have {
		if _, ok := wantMap[h.Id.Name]; ok {
			continue
		}
		deletions = append(deletions, h.Id)
	}

	return writes, deletions, nil
}

func generateV2AlbumsDeterministic(artistID *pbresource.ID, count int) ([]*pbresource.Resource, error) {
	uid, err := ulid.Parse(artistID.Uid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Uid: %w", err)
	}
	rand := rand.New(rand.NewSource(int64(uid.Time())))

	albums := make([]*pbresource.Resource, count)
	for i := 0; i < count; i++ {
		album, err := generateV2Album(artistID, rand)
		if err != nil {
			return nil, err
		}
		// Add suffix to avoid collisions.
		album.Id.Name = fmt.Sprintf("%s-%d", album.Id.Name, i)
		albums[i] = album
	}
	return albums, nil
}
