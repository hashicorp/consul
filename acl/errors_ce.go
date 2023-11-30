// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package acl

// In some sense we really want this to contain an EnterpriseMeta, but
// this turns out to be a convenient place to hang helper functions off of.
type ResourceDescriptor struct {
	Name string
}

func NewResourceDescriptor(name string, _ *AuthorizerContext) ResourceDescriptor {
	return ResourceDescriptor{Name: name}
}

func (od *ResourceDescriptor) ToString() string {
	return "\"" + od.Name + "\""
}
