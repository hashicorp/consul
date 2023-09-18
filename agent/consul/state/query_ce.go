//go:build !consulent
// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func prefixIndexFromQuery(arg any) ([]byte, error) {
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

func prefixIndexFromQueryWithPeer(arg any) ([]byte, error) {
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

// prefixIndexFromQueryWithPeerWildcardable allows for a wildcard "*" peerName
// to query for all peers _excluding_ structs.LocalPeerKeyword.
// Assumes that non-local peers are prefixed with "peer:".
func prefixIndexFromQueryWithPeerWildcardable(v Query) ([]byte, error) {
	var b indexBuilder

	peername := v.PeerOrEmpty()
	if peername == "" {
		b.String(strings.ToLower(structs.LocalPeerKeyword))
	} else if peername == "*" {
		// use b.Raw so we don't add null terminator to prefix
		b.Raw([]byte("peer:"))
		return b.Bytes(), nil
	} else {
		b.String(strings.ToLower("peer:" + peername))
	}

	if v.Value != "" {
		b.String(strings.ToLower(v.Value))
	}
	return b.Bytes(), nil
}

func prefixIndexFromQueryNoNamespace(arg interface{}) ([]byte, error) {
	return prefixIndexFromQuery(arg)
}

// indexFromAuthMethodQuery builds an index key where Query.Value is lowercase, and is
// a required value.
func indexFromAuthMethodQuery(q AuthMethodQuery) ([]byte, error) {
	var b indexBuilder
	b.String(strings.ToLower(q.Value))
	return b.Bytes(), nil
}
