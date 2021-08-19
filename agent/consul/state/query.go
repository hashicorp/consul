package state

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// Query is a type used to query any single value index that may include an
// enterprise identifier.
type Query struct {
	Value string
	structs.EnterpriseMeta
}

// NamespaceOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q Query) NamespaceOrDefault() string {
	return q.EnterpriseMeta.NamespaceOrDefault()
}

// PartitionOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q Query) PartitionOrDefault() string {
	return q.EnterpriseMeta.PartitionOrDefault()
}

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

func indexFromServiceNameAsString(arg interface{}) ([]byte, error) {
	sn, ok := arg.(structs.ServiceName)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for ServiceName index", arg)
	}

	var b indexBuilder
	b.String(strings.ToLower(sn.String()))
	return b.Bytes(), nil
}

func uuidStringToBytes(uuid string) ([]byte, error) {
	// Verify the length
	if l := len(uuid); l != 36 {
		return nil, fmt.Errorf("UUID must be 36 characters")
	}
	return parseUUIDString(uuid)
}

func variableLengthUUIDStringToBytes(uuid string) ([]byte, error) {
	// Verify the length
	if l := len(uuid); l > 36 {
		return nil, fmt.Errorf("Invalid UUID length. UUID have 36 characters; got %d", l)
	}
	return parseUUIDString(uuid)
}

// parseUUIDString is a modified version of memdb.UUIDFieldIndex.parseString.
// Callers should verify the length.
func parseUUIDString(uuid string) ([]byte, error) {
	hyphens := strings.Count(uuid, "-")
	if hyphens > 4 {
		return nil, fmt.Errorf(`UUID should have maximum of 4 "-"; got %d`, hyphens)
	}

	// The sanitized length is the length of the original string without the "-".
	sanitized := strings.Replace(uuid, "-", "", -1)
	sanitizedLength := len(sanitized)
	if sanitizedLength%2 != 0 {
		return nil, fmt.Errorf("UUID (without hyphens) must be even length")
	}

	dec, err := hex.DecodeString(sanitized)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}
	return dec, nil
}

// BoolQuery is a type used to query a boolean condition that may include an
// enterprise identifier.
type BoolQuery struct {
	Value bool
	structs.EnterpriseMeta
}

// KeyValueQuery is a type used to query for both a key and a value that may
// include an enterprise identifier.
type KeyValueQuery struct {
	Key   string
	Value string
	structs.EnterpriseMeta
}

// NamespaceOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q KeyValueQuery) NamespaceOrDefault() string {
	return q.EnterpriseMeta.NamespaceOrDefault()
}

// PartitionOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q KeyValueQuery) PartitionOrDefault() string {
	return q.EnterpriseMeta.PartitionOrDefault()
}

func indexFromKeyValueQuery(arg interface{}) ([]byte, error) {
	// NOTE: this is case-sensitive!
	q, ok := arg.(KeyValueQuery)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for Query index", arg)
	}

	var b indexBuilder
	b.String(q.Key)
	b.String(q.Value)
	return b.Bytes(), nil
}
