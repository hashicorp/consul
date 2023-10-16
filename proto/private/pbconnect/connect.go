// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbconnect

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbcommon"
)

func QueryMetaFrom(f structs.QueryMeta) *pbcommon.QueryMeta {
	t := new(pbcommon.QueryMeta)
	pbcommon.QueryMetaFromStructs(&f, t)
	return t
}

func QueryMetaTo(f *pbcommon.QueryMeta) structs.QueryMeta {
	t := new(structs.QueryMeta)
	pbcommon.QueryMetaToStructs(f, t)
	return *t
}

func RaftIndexFrom(f structs.RaftIndex) *pbcommon.RaftIndex {
	t := new(pbcommon.RaftIndex)
	pbcommon.RaftIndexFromStructs(&f, t)
	return t
}

func RaftIndexTo(f *pbcommon.RaftIndex) structs.RaftIndex {
	t := new(structs.RaftIndex)
	pbcommon.RaftIndexToStructs(f, t)
	return *t
}

func EnterpriseMetaFrom(f acl.EnterpriseMeta) *pbcommon.EnterpriseMeta {
	t := new(pbcommon.EnterpriseMeta)
	pbcommon.EnterpriseMetaFromStructs(&f, t)
	return t
}

func EnterpriseMetaTo(f *pbcommon.EnterpriseMeta) acl.EnterpriseMeta {
	t := new(acl.EnterpriseMeta)
	pbcommon.EnterpriseMetaToStructs(f, t)
	return *t
}

func NewIssuedCertFromStructs(in *structs.IssuedCert) (*IssuedCert, error) {
	t := new(IssuedCert)
	IssuedCertFromStructsIssuedCert(in, t)
	return t, nil
}

func NewCARootsFromStructs(in *structs.IndexedCARoots) (*CARoots, error) {
	t := new(CARoots)
	CARootsFromStructsIndexedCARoots(in, t)
	return t, nil
}

func CARootsToStructs(in *CARoots) (*structs.IndexedCARoots, error) {
	t := new(structs.IndexedCARoots)
	CARootsToStructsIndexedCARoots(in, t)
	return t, nil
}

func NewCARootFromStructs(in *structs.CARoot) (*CARoot, error) {
	t := new(CARoot)
	CARootFromStructsCARoot(in, t)
	return t, nil
}

func CARootToStructs(in *CARoot) (*structs.CARoot, error) {
	t := new(structs.CARoot)
	CARootToStructsCARoot(in, t)
	return t, nil
}

func IssuedCertToStructs(in *IssuedCert) (*structs.IssuedCert, error) {
	t := new(structs.IssuedCert)
	IssuedCertToStructsIssuedCert(in, t)
	return t, nil
}
