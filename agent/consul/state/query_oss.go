// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func prefixIndexFromQuery(arg interface{}) ([]byte, error) {
	var b indexBuilder
	switch v := arg.(type) {
	case *structs.EnterpriseMeta:
		return nil, nil
	case structs.EnterpriseMeta:
		return nil, nil
	case Query:
		b.String(strings.ToLower(v.Value))
		return b.Bytes(), nil
	}

	return nil, fmt.Errorf("unexpected type %T for Query prefix index", arg)
}

func prefixIndexFromQueryNoNamespace(arg interface{}) ([]byte, error) {
	return prefixIndexFromQuery(arg)
}

func prefixIndexFromServiceNameAsString(arg interface{}) ([]byte, error) {
	var b indexBuilder
	switch v := arg.(type) {
	case *structs.EnterpriseMeta:
		return nil, nil
	case structs.EnterpriseMeta:
		return nil, nil
	case structs.ServiceName:
		b.String(strings.ToLower(v.String()))
		return b.Bytes(), nil
	}

	return nil, fmt.Errorf("unexpected type %T for Query prefix index", arg)
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
