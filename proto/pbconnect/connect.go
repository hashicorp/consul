package pbconnect

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommongogo"
)

func QueryMetaFrom(f structs.QueryMeta) *pbcommongogo.QueryMeta {
	t := new(pbcommongogo.QueryMeta)
	pbcommongogo.QueryMetaFromStructs(&f, t)
	return t
}

func QueryMetaTo(f *pbcommongogo.QueryMeta) structs.QueryMeta {
	t := new(structs.QueryMeta)
	pbcommongogo.QueryMetaToStructs(f, t)
	return *t
}

func RaftIndexFrom(f structs.RaftIndex) *pbcommongogo.RaftIndex {
	t := new(pbcommongogo.RaftIndex)
	pbcommongogo.RaftIndexFromStructs(&f, t)
	return t
}

func RaftIndexTo(f *pbcommongogo.RaftIndex) structs.RaftIndex {
	t := new(structs.RaftIndex)
	pbcommongogo.RaftIndexToStructs(f, t)
	return *t
}

func EnterpriseMetaFrom(f structs.EnterpriseMeta) *pbcommongogo.EnterpriseMeta {
	t := new(pbcommongogo.EnterpriseMeta)
	pbcommongogo.EnterpriseMetaFromStructs(&f, t)
	return t
}

func EnterpriseMetaTo(f *pbcommongogo.EnterpriseMeta) structs.EnterpriseMeta {
	t := new(structs.EnterpriseMeta)
	pbcommongogo.EnterpriseMetaToStructs(f, t)
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
