package pbconnect

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommon"
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
