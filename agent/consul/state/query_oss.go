//go:build !consulent
// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func prefixIndexFromQuery(arg interface{}) ([]byte, error) {
	var b indexBuilder
	switch v := arg.(type) {
	case *acl.EnterpriseMeta:
		return nil, nil
	case acl.EnterpriseMeta:
		return nil, nil
	case Query:
		if v.Value == "" {
			return nil, nil
		}
		b.String(strings.ToLower(v.Value))
		return b.Bytes(), nil
	}

	return nil, fmt.Errorf("unexpected type %T for Query prefix index", arg)
}

func prefixIndexFromQueryWithPeer(arg interface{}) ([]byte, error) {
	var b indexBuilder
	switch v := arg.(type) {
	case *acl.EnterpriseMeta:
		return nil, nil
	case acl.EnterpriseMeta:
		return nil, nil
	case Query:
		if v.PeerOrEmpty() == "" {
			b.String(structs.LocalPeerKeyword)
		} else {
			b.String(strings.ToLower(v.PeerOrEmpty()))
		}
		if v.Value == "" {
			return b.Bytes(), nil
		}
		b.String(strings.ToLower(v.Value))
		return b.Bytes(), nil
	}

	return nil, fmt.Errorf("unexpected type %T for Query prefix index", arg)
}

func prefixIndexFromQueryNoNamespace(arg interface{}) ([]byte, error) {
	return prefixIndexFromQuery(arg)
}

// indexFromAuthMethodQuery builds an index key where Query.Value is lowercase, and is
// a required value.
func indexFromAuthMethodQuery(arg interface{}) ([]byte, error) {
	q, ok := arg.(AuthMethodQuery)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for Query index", arg)
	}

	var b indexBuilder
	b.String(strings.ToLower(q.Value))
	return b.Bytes(), nil
}
