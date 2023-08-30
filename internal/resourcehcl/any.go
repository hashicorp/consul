// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcehcl

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/hashicorp/consul/internal/protohcl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// anyProvider implements protohcl.AnyTypeProvider to infer the `Data` block
// type from `ID.Type`.
type anyProvider struct {
	base protohcl.AnyTypeProvider
	reg  resource.Registry
}

func (p anyProvider) AnyType(ctx *protohcl.UnmarshalContext, decoder protohcl.MessageDecoder) (protoreflect.FullName, protohcl.MessageDecoder, error) {
	if ctx.Name != "Data" {
		return p.base.AnyType(ctx, decoder)
	}

	if ctx.Parent == nil || ctx.Parent.Message == nil {
		return p.base.AnyType(ctx, decoder)
	}

	res, isResource := ctx.Parent.Message.Interface().(*pbresource.Resource)
	if !isResource {
		return p.base.AnyType(ctx, decoder)
	}

	resourceType := res.GetId().GetType()
	if res == nil {
		return "", nil, errors.New("ID.Type not found")
	}

	reg, ok := p.reg.Resolve(resourceType)
	if !ok {
		return "", nil, fmt.Errorf("unknown resource type: %s", resource.ToGVK(resourceType))
	}

	return reg.Proto.ProtoReflect().Descriptor().FullName(), decoder, nil
}
