// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcehcl

import (
	"reflect"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/hashicorp/consul/internal/protohcl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Unmarshal the given HCL source into a resource.
func Unmarshal(src []byte, reg resource.Registry) (*pbresource.Resource, error) {
	return UnmarshalOptions{}.Unmarshal(src, reg)
}

type UnmarshalOptions struct{ SourceFileName string }

// Unmarshal the given HCL source into a resource.
func (u UnmarshalOptions) Unmarshal(src []byte, reg resource.Registry) (*pbresource.Resource, error) {
	var out pbresource.Resource
	err := (protohcl.UnmarshalOptions{
		SourceFileName: u.SourceFileName,
		AnyTypeProvider: anyProvider{
			base: &protohcl.AnyTypeURLProvider{TypeURLFieldName: "Type"},
			reg:  reg,
		},
		FieldNamer: fieldNamer{acroynms: []string{"ID", "TCP", "UDP", "HTTP"}},
		Functions:  map[string]function.Function{"gvk": gvk},
	}).Unmarshal(src, &out)
	return &out, err
}

var (
	typeType = cty.Capsule("type", reflect.TypeOf(pbresource.Type{}))

	gvk = function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "GVK String", Type: cty.String},
		},
		Type: function.StaticReturnType(typeType),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			t, err := resource.ParseGVK(args[0].AsString())
			if err != nil {
				return cty.NilVal, err
			}
			return cty.CapsuleVal(typeType, t), nil
		},
	})
)
