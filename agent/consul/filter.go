package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type dirEntFilter struct {
	authorizer acl.Authorizer
	ent        structs.DirEntries
}

func (d *dirEntFilter) Len() int {
	return len(d.ent)
}
func (d *dirEntFilter) Filter(i int) bool {
	var entCtx acl.AuthorizerContext
	d.ent[i].FillAuthzContext(&entCtx)

	return d.authorizer.KeyRead(d.ent[i].Key, &entCtx) != acl.Allow
}
func (d *dirEntFilter) Move(dst, src, span int) {
	copy(d.ent[dst:dst+span], d.ent[src:src+span])
}

// FilterDirEnt is used to filter a list of directory entries
// by applying an ACL policy
func FilterDirEnt(authorizer acl.Authorizer, ent structs.DirEntries) structs.DirEntries {
	df := dirEntFilter{authorizer: authorizer, ent: ent}
	return ent[:FilterEntries(&df)]
}

type txnResultsFilter struct {
	authorizer acl.Authorizer
	results    structs.TxnResults
}

func (t *txnResultsFilter) Len() int {
	return len(t.results)
}

func (t *txnResultsFilter) Filter(i int) bool {
	result := t.results[i]
	var authzContext acl.AuthorizerContext
	switch {
	case result.KV != nil:
		result.KV.EnterpriseMeta.FillAuthzContext(&authzContext)
		return t.authorizer.KeyRead(result.KV.Key, &authzContext) != acl.Allow
	case result.Node != nil:
		(*structs.Node)(result.Node).FillAuthzContext(&authzContext)
		return t.authorizer.NodeRead(result.Node.Node, &authzContext) != acl.Allow
	case result.Service != nil:
		result.Service.EnterpriseMeta.FillAuthzContext(&authzContext)
		return t.authorizer.ServiceRead(result.Service.Service, &authzContext) != acl.Allow
	case result.Check != nil:
		result.Check.EnterpriseMeta.FillAuthzContext(&authzContext)
		if result.Check.ServiceName != "" {
			return t.authorizer.ServiceRead(result.Check.ServiceName, &authzContext) != acl.Allow
		}
		return t.authorizer.NodeRead(result.Check.Node, &authzContext) != acl.Allow
	}
	return false
}

func (t *txnResultsFilter) Move(dst, src, span int) {
	copy(t.results[dst:dst+span], t.results[src:src+span])
}

// FilterTxnResults is used to filter a list of transaction results by
// applying an ACL policy.
func FilterTxnResults(authorizer acl.Authorizer, results structs.TxnResults) structs.TxnResults {
	rf := txnResultsFilter{authorizer: authorizer, results: results}
	return results[:FilterEntries(&rf)]
}

// Filter interface is used with FilterEntries to do an
// in-place filter of a slice.
type Filter interface {
	Len() int
	Filter(int) bool
	Move(dst, src, span int)
}

// FilterEntries is used to do an inplace filter of
// a slice. This has cost proportional to the list length.
func FilterEntries(f Filter) int {
	// Compact the list
	dst := 0
	src := 0
	n := f.Len()
	for dst < n {
		for src < n && f.Filter(src) {
			src++
		}
		if src == n {
			break
		}
		end := src + 1
		for end < n && !f.Filter(end) {
			end++
		}
		span := end - src
		if span > 0 {
			f.Move(dst, src, span)
			dst += span
			src += span
		}
	}

	// Return the size of the slice
	return dst
}
