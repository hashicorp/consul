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

	indexAccessor   = "accessor"
	indexPolicies   = "policies"
	indexRoles      = "roles"
	indexAuthMethod = "authmethod"
	indexLocality   = "locality"
	indexName       = "name"
)

func tokensTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableACLTokens,
		Indexes: map[string]*memdb.IndexSchema{
			indexAccessor: {
				Name: indexAccessor,
				// DEPRECATED (ACL-Legacy-Compat) - we should not AllowMissing here once legacy compat is removed
				AllowMissing: true,
				Unique:       true,
				Indexer: indexerSingle{
					readIndex:  readIndex(indexFromUUIDString),
					writeIndex: writeIndex(indexAccessorIDFromACLToken),
				},
			},
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle{
					readIndex:  readIndex(indexFromStringCaseSensitive),
					writeIndex: writeIndex(indexSecretIDFromACLToken),
				},
			},
			indexPolicies: {
				Name: indexPolicies,
				// Need to allow missing for the anonymous token
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerMulti{
					readIndex:       readIndex(indexFromUUIDQuery),
					writeIndexMulti: writeIndexMulti(indexPoliciesFromACLToken),
				},
			},
			indexRoles: {
				Name:         indexRoles,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerMulti{
					readIndex:       readIndex(indexFromUUIDQuery),
					writeIndexMulti: writeIndexMulti(indexRolesFromACLToken),
				},
			},
			indexAuthMethod: {
				Name:         indexAuthMethod,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle{
					readIndex:  readIndex(indexFromAuthMethodQuery),
					writeIndex: writeIndex(indexAuthMethodFromACLToken),
				},
			},
			indexLocality: {
				Name:         indexLocality,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle{
					readIndex:  readIndex(indexFromBoolQuery),
					writeIndex: writeIndex(indexLocalFromACLToken),
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

func indexFromUUIDString(raw interface{}) ([]byte, error) {
	index, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for UUID string index", raw)
	}
	uuid, err := uuidStringToBytes(index)
	if err != nil {
		return nil, err
	}
	var b indexBuilder
	b.Raw(uuid)
	return b.Bytes(), nil
}

func indexAccessorIDFromACLToken(raw interface{}) ([]byte, error) {
	p, ok := raw.(*structs.ACLToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLToken index", raw)
	}

	if p.AccessorID == "" {
		return nil, errMissingValueForIndex
	}

	uuid, err := uuidStringToBytes(p.AccessorID)
	if err != nil {
		return nil, err
	}
	var b indexBuilder
	b.Raw(uuid)
	return b.Bytes(), nil
}

func indexSecretIDFromACLToken(raw interface{}) ([]byte, error) {
	p, ok := raw.(*structs.ACLToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLToken index", raw)
	}

	if p.SecretID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(p.SecretID)
	return b.Bytes(), nil
}

func indexFromStringCaseSensitive(raw interface{}) ([]byte, error) {
	q, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for string prefix query", raw)
	}

	var b indexBuilder
	b.String(q)
	return b.Bytes(), nil
}

func indexPoliciesFromACLToken(raw interface{}) ([][]byte, error) {
	token, ok := raw.(*structs.ACLToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLToken index", raw)
	}
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

func indexRolesFromACLToken(raw interface{}) ([][]byte, error) {
	token, ok := raw.(*structs.ACLToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLToken index", raw)
	}
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

func indexFromBoolQuery(raw interface{}) ([]byte, error) {
	q, ok := raw.(BoolQuery)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for BoolQuery index", raw)
	}
	var b indexBuilder
	b.Bool(q.Value)
	return b.Bytes(), nil
}

func indexLocalFromACLToken(raw interface{}) ([]byte, error) {
	p, ok := raw.(*structs.ACLToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLPolicy index", raw)
	}

	var b indexBuilder
	b.Bool(p.Local)
	return b.Bytes(), nil
}
