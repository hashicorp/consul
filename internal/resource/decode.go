// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
type DecodedResource[T proto.Message] struct {
	Resource *pbresource.Resource
	Data     T
}

func (d *DecodedResource[T]) GetResource() *pbresource.Resource {
	if d == nil {
		return nil
	}

	return d.Resource
}

func (d *DecodedResource[T]) GetData() T {
	if d == nil {
		var zero T
		return zero
	}

	return d.Data
}

// Decode will generically decode the provided resource into a 2-field
// structure that holds onto the original Resource and the decoded contents.
//
// Returns an ErrDataParse on unmarshalling errors.
func Decode[T proto.Message](res *pbresource.Resource) (*DecodedResource[T], error) {
	var zero T
	data := zero.ProtoReflect().New().Interface().(T)

	if err := res.Data.UnmarshalTo(data); err != nil {
		return nil, NewErrDataParse(data, err)
	}
	return &DecodedResource[T]{
		Resource: res,
		Data:     data,
	}, nil
}

// GetDecodedResource will generically read the requested resource using the
// client and either return nil on a NotFound or decode the response value.
func GetDecodedResource[T proto.Message](ctx context.Context, client pbresource.ResourceServiceClient, id *pbresource.ID) (*DecodedResource[T], error) {
	rsp, err := client.Read(ctx, &pbresource.ReadRequest{Id: id})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	return Decode[T](rsp.Resource)
}
