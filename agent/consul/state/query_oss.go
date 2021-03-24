// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// indexFromQuery builds an index key where Query.Value is lowercase, and is
// a required value.
func indexFromQuery(arg interface{}) ([]byte, error) {
	q, ok := arg.(Query)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for Query index", arg)
	}

	var b indexBuilder
	b.String(strings.ToLower(q.Value))
	return b.Bytes(), nil
}

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

func indexFromUUIDQuery(raw interface{}) ([]byte, error) {
	q, ok := raw.(Query)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for UUIDQuery index", raw)
	}
	return uuidStringToBytes(q.Value)
}
