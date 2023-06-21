// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

// ErrCannotWriteGlobalToken indicates that writing a token failed because
// the token is global and this is a non-primary datacenter.
var ErrCannotWriteGlobalToken = errors.New("Cannot upsert global tokens within this datacenter")

// NewTokenWriter creates a new token writer.
func NewTokenWriter(cfg TokenWriterConfig) *TokenWriter {
	return &TokenWriter{cfg}
}

// TokenWriter encapsulates the logic of writing ACL tokens to the state store
// including validation, cache purging, etc.
type TokenWriter struct {
	TokenWriterConfig
}

type TokenWriterConfig struct {
	RaftApply RaftApplyFn
	ACLCache  ACLCache
	Store     TokenWriterStore
	CheckUUID lib.UUIDCheckFunc

	MaxExpirationTTL time.Duration
	MinExpirationTTL time.Duration

	PrimaryDatacenter   string
	InPrimaryDatacenter bool
	LocalTokensEnabled  bool
}

type RaftApplyFn func(structs.MessageType, interface{}) (interface{}, error)

//go:generate mockery --name ACLCache --inpackage
type ACLCache interface {
	RemoveIdentityWithSecretToken(secretToken string)
}

type TokenWriterStore interface {
	ACLTokenGetByAccessor(ws memdb.WatchSet, accessorID string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLToken, error)
	ACLTokenGetBySecret(ws memdb.WatchSet, secretID string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLToken, error)
	ACLRoleGetByID(ws memdb.WatchSet, id string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLRole, error)
	ACLRoleGetByName(ws memdb.WatchSet, name string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLRole, error)
	ACLPolicyGetByID(ws memdb.WatchSet, id string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLPolicy, error)
	ACLPolicyGetByName(ws memdb.WatchSet, name string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLPolicy, error)
	ACLTokenUpsertValidateEnterprise(token *structs.ACLToken, existing *structs.ACLToken) error
}

// Create a new token. Setting fromLogin to true changes behavior slightly for
// tokens created by login (as opposed to set manually via the API).
func (w *TokenWriter) Create(token *structs.ACLToken, fromLogin bool) (*structs.ACLToken, error) {
	if err := w.checkCanWriteToken(token); err != nil {
		return nil, err
	}

	if token.AccessorID == "" {
		// Caller didn't provide an AccessorID, so generate one.
		id, err := lib.GenerateUUID(w.CheckUUID)
		if err != nil {
			return nil, fmt.Errorf("Failed to generate AccessorID: %w", err)
		}
		token.AccessorID = id
	} else {
		// Check the AccessorID is valid and not already in-use.
		if err := validateTokenID(token.AccessorID); err != nil {
			return nil, fmt.Errorf("Invalid Token: AccessorID - %w", err)
		}
		if inUse, err := w.tokenIDInUse(token.AccessorID); err != nil {
			return nil, fmt.Errorf("Failed to lookup ACL token: %w", err)
		} else if inUse {
			return nil, errors.New("Invalid Token: AccessorID is already in use")
		}
	}

	if token.SecretID == "" {
		// Caller didn't provide a SecretID, so generate one.
		id, err := lib.GenerateUUID(w.CheckUUID)
		if err != nil {
			return nil, fmt.Errorf("Failed to generate SecretID: %w", err)
		}
		token.SecretID = id
	} else {
		// Check the SecretID is valid and not already in-use.
		if err := validateTokenID(token.SecretID); err != nil {
			return nil, fmt.Errorf("Invalid Token: SecretID - %w", err)
		}
		if inUse, err := w.tokenIDInUse(token.SecretID); err != nil {
			return nil, fmt.Errorf("Failed to lookup ACL token: %w", err)
		} else if inUse {
			return nil, errors.New("Invalid Token: SecretID is already in use")
		}
	}

	token.CreateTime = time.Now()

	// Ensure ExpirationTTL is valid if provided.
	if token.ExpirationTTL < 0 {
		return nil, fmt.Errorf("Token Expiration TTL '%s' should be > 0", token.ExpirationTTL)
	} else if token.ExpirationTTL > 0 {
		if token.HasExpirationTime() {
			return nil, errors.New("Token Expiration TTL and Expiration Time cannot both be set")
		}

		expirationTime := token.CreateTime.Add(token.ExpirationTTL)
		token.ExpirationTime = &expirationTime
		token.ExpirationTTL = 0
	}

	if token.HasExpirationTime() {
		if token.ExpirationTime.Before(token.CreateTime) {
			return nil, errors.New("ExpirationTime cannot be before CreateTime")
		}

		expiresIn := token.ExpirationTime.Sub(token.CreateTime)

		if expiresIn > w.MaxExpirationTTL {
			return nil, fmt.Errorf("ExpirationTime cannot be more than %s in the future (was %s)",
				w.MaxExpirationTTL, expiresIn)
		}

		if expiresIn < w.MinExpirationTTL {
			return nil, fmt.Errorf("ExpirationTime cannot be less than %s in the future (was %s)",
				w.MinExpirationTTL, expiresIn)
		}
	}

	if fromLogin {
		if token.AuthMethod == "" {
			return nil, errors.New("AuthMethod field is required during login")
		}
	} else {
		if token.AuthMethod != "" {
			return nil, errors.New("AuthMethod field is disallowed outside of login")
		}
	}

	return w.write(token, nil, fromLogin)
}

// Update an existing token.
func (w *TokenWriter) Update(token *structs.ACLToken) (*structs.ACLToken, error) {
	if err := w.checkCanWriteToken(token); err != nil {
		return nil, err
	}

	if _, err := uuid.ParseUUID(token.AccessorID); err != nil {
		return nil, errors.New("AccessorID is not a valid UUID")
	}

	// DEPRECATED (ACL-Legacy-Compat) - maybe get rid of this in the future
	//   and instead do a ParseUUID check. New tokens will not have
	//   secrets generated by users but rather they will always be UUIDs.
	//   However if users just continue the upgrade cycle they may still
	//   have tokens using secrets that are not UUIDS
	// The RootAuthorizer checks that the SecretID is not "allow", "deny"
	// or "manage" as a precaution against something accidentally using
	// one of these root policies by setting the secret to it.
	if acl.RootAuthorizer(token.SecretID) != nil {
		return nil, acl.PermissionDeniedError{Cause: "Cannot modify root ACL"}
	}

	_, match, err := w.Store.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	switch {
	case err != nil:
		return nil, fmt.Errorf("Failed acl token lookup by accessor: %w", err)
	case match == nil || match.IsExpired(time.Now()):
		return nil, fmt.Errorf("Cannot find token %q", token.AccessorID)
	}

	if token.SecretID == "" {
		token.SecretID = match.SecretID
	} else if match.SecretID != token.SecretID {
		return nil, errors.New("Changing a token's SecretID is not permitted")
	}

	if token.Local != match.Local {
		return nil, fmt.Errorf("Cannot toggle local mode of %s", token.AccessorID)
	}

	if token.AuthMethod == "" {
		token.AuthMethod = match.AuthMethod
	} else if match.AuthMethod != token.AuthMethod {
		return nil, fmt.Errorf("Cannot change AuthMethod of %s", token.AccessorID)
	}

	if token.ExpirationTTL != 0 {
		return nil, fmt.Errorf("Cannot change expiration time of %s", token.AccessorID)
	}

	if token.HasExpirationTime() {
		if !match.HasExpirationTime() || !match.ExpirationTime.Equal(*token.ExpirationTime) {
			return nil, fmt.Errorf("Cannot change expiration time of %s", token.AccessorID)
		}
	} else {
		token.ExpirationTime = match.ExpirationTime
	}

	token.CreateTime = match.CreateTime

	return w.write(token, match, false)
}

// Delete the ACL token with the given SecretID from the state store.
func (w *TokenWriter) Delete(secretID string, fromLogout bool) error {
	_, token, err := w.Store.ACLTokenGetBySecret(nil, secretID, nil)
	switch {
	case err != nil:
		return err
	case token == nil:
		return acl.ErrNotFound
	case token.AuthMethod == "" && fromLogout:
		return fmt.Errorf("%w: token wasn't created via login", acl.ErrPermissionDenied)
	}

	if err := w.checkCanWriteToken(token); err != nil {
		return err
	}

	if _, err := w.RaftApply(structs.ACLTokenDeleteRequestType, &structs.ACLTokenBatchDeleteRequest{
		TokenIDs: []string{token.AccessorID},
	}); err != nil {
		return fmt.Errorf("Failed to apply token delete request: %w", err)
	}

	w.ACLCache.RemoveIdentityWithSecretToken(token.SecretID)
	return nil
}

func validateTokenID(id string) error {
	if structs.ACLIDReserved(id) {
		return fmt.Errorf("UUIDs with the prefix %q are reserved", structs.ACLReservedPrefix)
	}
	if _, err := uuid.ParseUUID(id); err != nil {
		return errors.New("not a valid UUID")
	}
	return nil
}

func (w *TokenWriter) checkCanWriteToken(token *structs.ACLToken) error {
	if !w.LocalTokensEnabled {
		return fmt.Errorf("Cannot upsert tokens within this datacenter")
	}

	if !w.InPrimaryDatacenter && !token.Local {
		return ErrCannotWriteGlobalToken
	}

	return nil
}

func (w *TokenWriter) tokenIDInUse(id string) (bool, error) {
	_, accessorMatch, err := w.Store.ACLTokenGetByAccessor(nil, id, nil)
	switch {
	case err != nil:
		return false, err
	case accessorMatch != nil:
		return true, nil
	}

	_, secretMatch, err := w.Store.ACLTokenGetBySecret(nil, id, nil)
	switch {
	case err != nil:
		return false, err
	case secretMatch != nil:
		return true, nil
	}

	return false, nil
}

func (w *TokenWriter) write(token, existing *structs.ACLToken, fromLogin bool) (*structs.ACLToken, error) {
	roles, err := w.normalizeRoleLinks(token.Roles, &token.EnterpriseMeta)
	if err != nil {
		return nil, err
	}
	token.Roles = roles

	policies, err := w.normalizePolicyLinks(token.Policies, &token.EnterpriseMeta)
	if err != nil {
		return nil, err
	}
	token.Policies = policies

	serviceIdentities, err := w.normalizeServiceIdentities(token.ServiceIdentities, token.Local)
	if err != nil {
		return nil, err
	}
	token.ServiceIdentities = serviceIdentities

	nodeIdentities, err := w.normalizeNodeIdentities(token.NodeIdentities)
	if err != nil {
		return nil, err
	}
	token.NodeIdentities = nodeIdentities

	if err := w.enterpriseValidation(token, existing); err != nil {
		return nil, err
	}

	token.SetHash(true)

	// Persist the token by writing to Raft.
	_, err = w.RaftApply(structs.ACLTokenSetRequestType, &structs.ACLTokenBatchSetRequest{
		Tokens: structs.ACLTokens{token},
		// Logins may attempt to link to roles that do not exist. These may be
		// persisted, but don't allow tokens to be created that have no privileges
		// (i.e. role links that point nowhere).
		AllowMissingLinks:    fromLogin,
		ProhibitUnprivileged: fromLogin,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to apply token write request: %w", err)
	}

	// Purge the token from the ACL cache.
	w.ACLCache.RemoveIdentityWithSecretToken(token.SecretID)

	// Refresh the token from the state store.
	_, updatedToken, err := w.Store.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	if err != nil || updatedToken == nil {
		return nil, errors.New("Failed to retrieve token after insertion")
	}
	return updatedToken, nil
}

func (w *TokenWriter) normalizeRoleLinks(links []structs.ACLTokenRoleLink, entMeta *acl.EnterpriseMeta) ([]structs.ACLTokenRoleLink, error) {
	var normalized []structs.ACLTokenRoleLink
	uniqueIDs := make(map[string]struct{})

	for _, link := range links {
		if link.ID == "" {
			_, role, err := w.Store.ACLRoleGetByName(nil, link.Name, entMeta)
			switch {
			case err != nil:
				return nil, fmt.Errorf("Error looking up role for name: %q: %w", link.Name, err)
			case role == nil:
				return nil, fmt.Errorf("No such ACL role with name %q", link.Name)
			}
			link.ID = role.ID
		} else {
			_, role, err := w.Store.ACLRoleGetByID(nil, link.ID, entMeta)
			switch {
			case err != nil:
				return nil, fmt.Errorf("Error looking up role for ID: %q: %w", link.ID, err)
			case role == nil:
				return nil, fmt.Errorf("No such ACL role with ID %q", link.ID)
			}
		}

		// Do not persist the role name as the role could be renamed in the future.
		link.Name = ""

		// De-duplicate role links by ID.
		if _, ok := uniqueIDs[link.ID]; !ok {
			normalized = append(normalized, link)
			uniqueIDs[link.ID] = struct{}{}
		}
	}

	return normalized, nil
}

func (w *TokenWriter) normalizePolicyLinks(links []structs.ACLTokenPolicyLink, entMeta *acl.EnterpriseMeta) ([]structs.ACLTokenPolicyLink, error) {
	var normalized []structs.ACLTokenPolicyLink
	uniqueIDs := make(map[string]struct{})

	for _, link := range links {
		if link.ID == "" {
			_, role, err := w.Store.ACLPolicyGetByName(nil, link.Name, entMeta)
			switch {
			case err != nil:
				return nil, fmt.Errorf("Error looking up policy for name: %q: %w", link.Name, err)
			case role == nil:
				return nil, fmt.Errorf("No such ACL policy with name %q", link.Name)
			}
			link.ID = role.ID
		} else {
			_, role, err := w.Store.ACLPolicyGetByID(nil, link.ID, entMeta)
			switch {
			case err != nil:
				return nil, fmt.Errorf("Error looking up policy for ID: %q: %w", link.ID, err)
			case role == nil:
				return nil, fmt.Errorf("No such ACL policy with ID %q", link.ID)
			}
		}

		// Do not persist the role name as the role could be renamed in the future.
		link.Name = ""

		// De-duplicate role links by ID.
		if _, ok := uniqueIDs[link.ID]; !ok {
			normalized = append(normalized, link)
			uniqueIDs[link.ID] = struct{}{}
		}
	}

	return normalized, nil
}

func (w *TokenWriter) normalizeServiceIdentities(svcIDs structs.ACLServiceIdentities, tokenLocal bool) (structs.ACLServiceIdentities, error) {
	for _, id := range svcIDs {
		if id.ServiceName == "" {
			return nil, errors.New("Service identity is missing the service name field on this token")
		}
		if tokenLocal && len(id.Datacenters) > 0 {
			return nil, fmt.Errorf("Service identity %q cannot specify a list of datacenters on a local token", id.ServiceName)
		}
		if !acl.IsValidServiceIdentityName(id.ServiceName) {
			return nil, fmt.Errorf("Service identity %q has an invalid name. Only lowercase alphanumeric characters, '-' and '_' are allowed", id.ServiceName)
		}
	}
	return svcIDs.Deduplicate(), nil
}

func (w *TokenWriter) normalizeNodeIdentities(nodeIDs structs.ACLNodeIdentities) (structs.ACLNodeIdentities, error) {
	for _, id := range nodeIDs {
		if id.NodeName == "" {
			return nil, errors.New("Node identity is missing the node name field on this token")
		}
		if id.Datacenter == "" {
			return nil, errors.New("Node identity is missing the datacenter field on this token")
		}
		if !acl.IsValidNodeIdentityName(id.NodeName) {
			return nil, fmt.Errorf("Node identity has an invalid name. Only lowercase alphanumeric characters, '-' and '_' are allowed")
		}
	}
	return nodeIDs.Deduplicate(), nil
}
