// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// DecodedResource is a generic holder to contain an original Resource and its
// decoded contents.
type DecodedResource[V any, PV interface {
	proto.Message
	*V
}] struct {
	Resource *pbresource.Resource
	Data     PV
}

// Decode will generically decode the provided resource into a 2-field
// structure that holds onto the original Resource and the decoded contents.
//
// Returns an ErrDataParse on unmarshalling errors.
func Decode[V any, PV interface {
	proto.Message
	*V
}](res *pbresource.Resource) (*DecodedResource[V, PV], error) {
	data := PV(new(V))
	if err := res.Data.UnmarshalTo(data); err != nil {
		return nil, NewErrDataParse(data, err)
	}
	return &DecodedResource[V, PV]{
		Resource: res,
		Data:     data,
	}, nil
}

// GetDecodedResource will generically read the requested resource using the
// client and either return nil on a NotFound or decode the response value.
func GetDecodedResource[V any, PV interface {
	proto.Message
	*V
}](ctx context.Context, client pbresource.ResourceServiceClient, id *pbresource.ID) (*DecodedResource[V, PV], error) {
	rsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: id})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	return Decode[V, PV](rsp.Resource)
}
