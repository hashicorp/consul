// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableACLTokens       = "acl-tokens"
	tableACLPolicies     = "acl-policies"
	tableACLRoles        = "acl-roles"
	tableACLBindingRules = "acl-binding-rules"
	tableACLAuthMethods  = "acl-auth-methods"

	indexAccessor      = "accessor"
	indexPolicies      = "policies"
	indexRoles         = "roles"
	indexServiceName   = "service-name"
	indexAuthMethod    = "authmethod"
	indexLocality      = "locality"
	indexName          = "name"
	indexExpiresGlobal = "expires-global"
	indexExpiresLocal  = "expires-local"
)

func tokensTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLTokens,
		Indexes: map[string]*memdb.IndexSchema{
			indexAccessor: {
				Name:         indexAccessor,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[string, *structs.ACLToken]{
					readIndex:  indexFromUUIDString,
					writeIndex: indexAccessorIDFromACLToken,
				},
			},
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[string, *structs.ACLToken]{
					readIndex:  indexFromStringCaseSensitive,
					writeIndex: indexSecretIDFromACLToken,
				},
			},
			indexPolicies: {
				Name: indexPolicies,
				// Need to allow missing for the anonymous token
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerMulti[Query, *structs.ACLToken]{
					readIndex:       indexFromUUIDQuery,
					writeIndexMulti: indexPoliciesFromACLToken,
				},
			},
			indexRoles: {
				Name:         indexRoles,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerMulti[Query, *structs.ACLToken]{
					readIndex:       indexFromUUIDQuery,
					writeIndexMulti: indexRolesFromACLToken,
				},
			},
			indexAuthMethod: {
				Name:         indexAuthMethod,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[AuthMethodQuery, *structs.ACLToken]{
					readIndex:  indexFromAuthMethodQuery,
					writeIndex: indexAuthMethodFromACLToken,
				},
			},
			indexLocality: {
				Name:         indexLocality,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[BoolQuery, *structs.ACLToken]{
					readIndex:  indexFromBoolQuery,
					writeIndex: indexLocalFromACLToken,
				},
			},
			indexExpiresGlobal: {
				Name:         indexExpiresGlobal,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[*TimeQuery, *structs.ACLToken]{
					readIndex:  indexFromTimeQuery,
					writeIndex: indexExpiresGlobalFromACLToken,
				},
			},
			indexExpiresLocal: {
				Name:         indexExpiresLocal,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[*TimeQuery, *structs.ACLToken]{
					readIndex:  indexFromTimeQuery,
					writeIndex: indexExpiresLocalFromACLToken,
				},
			},
			indexServiceName: {
				Name:         indexServiceName,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerMulti[Query, *structs.ACLToken]{
					readIndex:       indexFromQuery,
					writeIndexMulti: indexServiceNameFromACLToken,
				},
			},
		},
	}
}

func policiesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLPolicies,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			indexName: {
				Name:         indexName,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[Query, *structs.ACLPolicy, any]{
					readIndex:   indexFromQuery,
					writeIndex:  indexNameFromACLPolicy,
					prefixIndex: prefixIndexFromQuery,
				},
			},
		},
	}
}

func indexNameFromACLPolicy(p *structs.ACLPolicy) ([]byte, error) {
	if p.Name == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(p.Name))
	return b.Bytes(), nil
}

func rolesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLRoles,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			indexName: {
				Name:         indexName,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[Query, *structs.ACLRole, any]{
					readIndex:   indexFromQuery,
					writeIndex:  indexNameFromACLRole,
					prefixIndex: prefixIndexFromQuery,
				},
			},
			indexPolicies: {
				Name: indexPolicies,
				// Need to allow missing for the anonymous token
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerMulti[Query, *structs.ACLRole]{
					readIndex:       indexFromUUIDQuery,
					writeIndexMulti: multiIndexPolicyFromACLRole,
				},
			},
		},
	}
}

func indexNameFromACLRole(r *structs.ACLRole) ([]byte, error) {
	if r.Name == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(r.Name))
	return b.Bytes(), nil
}

func indexFromUUIDQuery(q Query) ([]byte, error) {
	return uuidStringToBytes(q.Value)
}

func prefixIndexFromUUIDWithPeerQuery(q Query) ([]byte, error) {
	var b indexBuilder
	peername := q.PeerOrEmpty()
	if peername == "" {
		b.String(structs.LocalPeerKeyword)
	} else {
		b.String(strings.ToLower(peername))
	}
	uuidBytes, err := variableLengthUUIDStringToBytes(q.Value)
	if err != nil {
		return nil, err
	}
	return append(b.Bytes(), uuidBytes...), nil
}

func multiIndexPolicyFromACLRole(r *structs.ACLRole) ([][]byte, error) {
	count := len(r.Policies)
	if count == 0 {
		return nil, errMissingValueForIndex
	}

	vals := make([][]byte, 0, count)
	for _, link := range r.Policies {
		v, err := uuidStringToBytes(link.ID)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}

	return vals, nil
}

func bindingRulesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLBindingRules,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[string, *structs.ACLBindingRule]{
					readIndex:  indexFromUUIDString,
					writeIndex: indexIDFromACLBindingRule,
				},
			},
			indexAuthMethod: {
				Name:         indexAuthMethod,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[Query, *structs.ACLBindingRule]{
					readIndex:  indexFromQuery,
					writeIndex: indexAuthMethodFromACLBindingRule,
				},
			},
		},
	}
}

func indexIDFromACLBindingRule(r *structs.ACLBindingRule) ([]byte, error) {
	vv, err := uuidStringToBytes(r.ID)
	if err != nil {
		return nil, err
	}

	return vv, err
}

func indexAuthMethodFromACLBindingRule(r *structs.ACLBindingRule) ([]byte, error) {
	if r.AuthMethod == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(r.AuthMethod))
	return b.Bytes(), nil
}

func indexFromUUIDString(raw string) ([]byte, error) {
	uuid, err := uuidStringToBytes(raw)
	if err != nil {
		return nil, err
	}
	var b indexBuilder
	b.Raw(uuid)
	return b.Bytes(), nil
}

func indexAccessorIDFromACLToken(t *structs.ACLToken) ([]byte, error) {
	if t.AccessorID == "" {
		return nil, errMissingValueForIndex
	}

	uuid, err := uuidStringToBytes(t.AccessorID)
	if err != nil {
		return nil, err
	}
	var b indexBuilder
	b.Raw(uuid)
	return b.Bytes(), nil
}

func indexSecretIDFromACLToken(t *structs.ACLToken) ([]byte, error) {
	if t.SecretID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(t.SecretID)
	return b.Bytes(), nil
}

func indexFromStringCaseSensitive(s string) ([]byte, error) {
	var b indexBuilder
	b.String(s)
	return b.Bytes(), nil
}

func indexPoliciesFromACLToken(token *structs.ACLToken) ([][]byte, error) {
	links := token.Policies

	numLinks := len(links)
	if numLinks == 0 {
		return nil, errMissingValueForIndex
	}

	vals := make([][]byte, numLinks)

	for i, link := range links {
		id, err := uuidStringToBytes(link.ID)
		if err != nil {
			return nil, err
		}
		vals[i] = id
	}

	return vals, nil
}

func indexRolesFromACLToken(token *structs.ACLToken) ([][]byte, error) {
	links := token.Roles

	numLinks := len(links)
	if numLinks == 0 {
		return nil, errMissingValueForIndex
	}

	vals := make([][]byte, numLinks)

	for i, link := range links {
		id, err := uuidStringToBytes(link.ID)
		if err != nil {
			return nil, err
		}
		vals[i] = id
	}

	return vals, nil
}

func indexFromBoolQuery(q BoolQuery) ([]byte, error) {
	var b indexBuilder
	b.Bool(q.Value)
	return b.Bytes(), nil
}

func indexLocalFromACLToken(token *structs.ACLToken) ([]byte, error) {
	var b indexBuilder
	b.Bool(token.Local)
	return b.Bytes(), nil
}

func indexFromTimeQuery(q *TimeQuery) ([]byte, error) {
	var b indexBuilder
	b.Time(q.Value)
	return b.Bytes(), nil
}

func indexExpiresLocalFromACLToken(token *structs.ACLToken) ([]byte, error) {
	return indexExpiresFromACLToken(token, true)
}

func indexExpiresGlobalFromACLToken(token *structs.ACLToken) ([]byte, error) {
	return indexExpiresFromACLToken(token, false)
}

func indexExpiresFromACLToken(t *structs.ACLToken, local bool) ([]byte, error) {
	if t.Local != local {
		return nil, errMissingValueForIndex
	}
	if !t.HasExpirationTime() {
		return nil, errMissingValueForIndex
	}
	if t.ExpirationTime.Unix() < 0 {
		return nil, fmt.Errorf("token expiration time cannot be before the unix epoch: %s", t.ExpirationTime)
	}

	var b indexBuilder
	b.Time(*t.ExpirationTime)
	return b.Bytes(), nil
}

func indexServiceNameFromACLToken(token *structs.ACLToken) ([][]byte, error) {
	vals := make([][]byte, 0, len(token.ServiceIdentities))
	for _, id := range token.ServiceIdentities {
		if id != nil && id.ServiceName != "" {
			var b indexBuilder
			b.String(strings.ToLower(id.ServiceName))
			vals = append(vals, b.Bytes())
		}
	}
	if len(vals) == 0 {
		return nil, errMissingValueForIndex
	}
	return vals, nil
}

func authMethodsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLAuthMethods,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[Query, *structs.ACLAuthMethod]{
					readIndex:  indexFromQuery,
					writeIndex: indexNameFromACLAuthMethod,
				},
			},
		},
	}
}

func indexNameFromACLAuthMethod(m *structs.ACLAuthMethod) ([]byte, error) {
	if m.Name == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(m.Name))
	return b.Bytes(), nil
}
