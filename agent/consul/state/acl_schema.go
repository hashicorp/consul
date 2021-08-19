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

	indexPolicies   = "policies"
	indexRoles      = "roles"
	indexAuthMethod = "authmethod"
	indexLocal      = "local"
	indexName       = "name"
)

func tokensTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLTokens,
		Indexes: map[string]*memdb.IndexSchema{
			"accessor": {
				Name: "accessor",
				// DEPRECATED (ACL-Legacy-Compat) - we should not AllowMissing here once legacy compat is removed
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "SecretID",
					Lowercase: false,
				},
			},
			indexPolicies: {
				Name: indexPolicies,
				// Need to allow missing for the anonymous token
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenPoliciesIndex{},
			},
			indexRoles: {
				Name:         indexRoles,
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenRolesIndex{},
			},
			indexAuthMethod: {
				Name:         indexAuthMethod,
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "AuthMethod",
					Lowercase: false,
				},
			},
			indexLocal: {
				Name:         indexLocal,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: func(obj interface{}) (bool, error) {
						if token, ok := obj.(*structs.ACLToken); ok {
							return token.Local, nil
						}
						return false, nil
					},
				},
			},
			"expires-global": {
				Name:         "expires-global",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenExpirationIndex{LocalFilter: false},
			},
			"expires-local": {
				Name:         "expires-local",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &TokenExpirationIndex{LocalFilter: true},
			},

			//DEPRECATED (ACL-Legacy-Compat) - This index is only needed while we support upgrading v1 to v2 acls
			// This table indexes all the ACL tokens that do not have an AccessorID
			"needs-upgrade": {
				Name:         "needs-upgrade",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: func(obj interface{}) (bool, error) {
						if token, ok := obj.(*structs.ACLToken); ok {
							return token.AccessorID == "", nil
						}
						return false, nil
					},
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
				Indexer: indexerSingleWithPrefix{
					readIndex:   indexFromQuery,
					writeIndex:  indexNameFromACLPolicy,
					prefixIndex: prefixIndexFromQuery,
				},
			},
		},
	}
}

func indexNameFromACLPolicy(raw interface{}) ([]byte, error) {
	p, ok := raw.(*structs.ACLPolicy)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLPolicy index", raw)
	}

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
				Indexer: indexerSingleWithPrefix{
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
				Indexer: indexerMulti{
					readIndex:       indexFromUUIDQuery,
					writeIndexMulti: multiIndexPolicyFromACLRole,
				},
			},
		},
	}
}

func indexNameFromACLRole(raw interface{}) ([]byte, error) {
	p, ok := raw.(*structs.ACLRole)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLRole index", raw)
	}

	if p.Name == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(p.Name))
	return b.Bytes(), nil
}

func indexFromUUIDQuery(raw interface{}) ([]byte, error) {
	q, ok := raw.(Query)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for UUIDQuery index", raw)
	}
	return uuidStringToBytes(q.Value)
}

func prefixIndexFromUUIDQuery(arg interface{}) ([]byte, error) {
	switch v := arg.(type) {
	case *structs.EnterpriseMeta:
		return nil, nil
	case structs.EnterpriseMeta:
		return nil, nil
	case Query:
		return variableLengthUUIDStringToBytes(v.Value)
	}

	return nil, fmt.Errorf("unexpected type %T for Query prefix index", arg)
}

func multiIndexPolicyFromACLRole(raw interface{}) ([][]byte, error) {
	role, ok := raw.(*structs.ACLRole)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLRole index", raw)
	}

	count := len(role.Policies)
	if count == 0 {
		return nil, errMissingValueForIndex
	}

	vals := make([][]byte, 0, count)
	for _, link := range role.Policies {
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
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			indexAuthMethod: {
				Name:         indexAuthMethod,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "AuthMethod",
					Lowercase: true,
				},
			},
		},
	}
}

func authMethodsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLAuthMethods,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Name",
					Lowercase: true,
				},
			},
		},
	}
}
