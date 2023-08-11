// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

func TestTokenWriter_Create_Validation(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	existingToken := &structs.ACLToken{
		AccessorID: generateID(t),
		SecretID:   generateID(t),
	}
	require.NoError(t, store.ACLTokenSet(0, existingToken))

	writer := buildTokenWriter(store, aclCache)

	testCases := map[string]struct {
		token         structs.ACLToken
		fromLogin     bool
		errorContains string
	}{
		"AccessorID not a UUID": {
			token:         structs.ACLToken{AccessorID: "not-a-uuid"},
			errorContains: "not a valid UUID",
		},
		"AccessorID is reserved": {
			token:         structs.ACLToken{AccessorID: structs.ACLReservedIDPrefix + generateID(t)},
			errorContains: "reserved",
		},
		"AccessorID already in use (as AccessorID)": {
			token:         structs.ACLToken{AccessorID: existingToken.AccessorID},
			errorContains: "already in use",
		},
		"AccessorID already in use (as SecretID)": {
			token:         structs.ACLToken{AccessorID: existingToken.SecretID},
			errorContains: "already in use",
		},
		"SecretID not a UUID": {
			token:         structs.ACLToken{SecretID: "not-a-uuid"},
			errorContains: "not a valid UUID",
		},
		"SecretID is reserved": {
			token:         structs.ACLToken{SecretID: structs.ACLReservedIDPrefix + generateID(t)},
			errorContains: "reserved",
		},
		"SecretID already in use (as AccessorID)": {
			token:         structs.ACLToken{SecretID: existingToken.AccessorID},
			errorContains: "already in use",
		},
		"SecretID already in use (as SecretID)": {
			token:         structs.ACLToken{SecretID: existingToken.SecretID},
			errorContains: "already in use",
		},
		"ExpirationTTL is negative": {
			token:         structs.ACLToken{ExpirationTTL: -1},
			errorContains: "should be > 0",
		},
		"ExpirationTTL and ExpirationTime both set": {
			token: structs.ACLToken{
				ExpirationTTL:  2 * time.Hour,
				ExpirationTime: timePointer(time.Now().Add(1 * time.Hour)),
			},
			errorContains: "cannot both be set",
		},
		"ExpirationTTL > MaxExpirationTTL": {
			token:         structs.ACLToken{ExpirationTTL: 48 * time.Hour},
			errorContains: "cannot be more than 24h0m0s in the future",
		},
		"ExpirationTTL < MinExpirationTTL": {
			token:         structs.ACLToken{ExpirationTTL: 30 * time.Second},
			errorContains: "cannot be less than 1m0s in the future",
		},
		"ExpirationTime before CreateTime": {
			token:         structs.ACLToken{ExpirationTime: timePointer(time.Now().Add(-5 * time.Minute))},
			errorContains: "ExpirationTime cannot be before CreateTime",
		},
		"AuthMethod not set for login": {
			token:         structs.ACLToken{},
			fromLogin:     true,
			errorContains: "AuthMethod field is required during login",
		},
		"AuthMethod set outside of login": {
			token:         structs.ACLToken{AuthMethod: "some-auth-method"},
			fromLogin:     false,
			errorContains: "AuthMethod field is disallowed outside of login",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			_, err := writer.Create(&tc.token, tc.fromLogin)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestTokenWriter_Create_IDGeneration(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	writer := buildTokenWriter(store, aclCache)

	t.Run("AccessorID", func(t *testing.T) {
		token := &structs.ACLToken{
			SecretID: generateID(t),
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{ServiceName: "some-service"},
			},
		}

		updated, err := writer.Create(token, false)
		require.NoError(t, err)
		require.NotEmpty(t, updated.AccessorID)
	})

	t.Run("SecretID", func(t *testing.T) {
		token := &structs.ACLToken{
			AccessorID: generateID(t),
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{ServiceName: "some-service"},
			},
		}

		updated, err := writer.Create(token, false)
		require.NoError(t, err)
		require.NotEmpty(t, updated.SecretID)
	})
}

func TestTokenWriter_Roles(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	role := &structs.ACLRole{
		ID:   generateID(t),
		Name: generateID(t),
	}
	require.NoError(t, store.ACLRoleSet(0, role))

	writer := buildTokenWriter(store, aclCache)

	testCases := map[string]struct {
		input         []structs.ACLTokenRoleLink
		output        []structs.ACLTokenRoleLink
		errorContains string
	}{
		"valid role ID": {
			input:  []structs.ACLTokenRoleLink{{ID: role.ID}},
			output: []structs.ACLTokenRoleLink{{ID: role.ID, Name: role.Name}},
		},
		"valid role name": {
			input:  []structs.ACLTokenRoleLink{{Name: role.Name}},
			output: []structs.ACLTokenRoleLink{{ID: role.ID, Name: role.Name}},
		},
		"invalid role ID": {
			input:         []structs.ACLTokenRoleLink{{ID: generateID(t)}},
			errorContains: "No such ACL role with ID",
		},
		"invalid role name": {
			input:         []structs.ACLTokenRoleLink{{Name: "invalid-role-name"}},
			errorContains: "No such ACL role with name",
		},
		"links are de-duplicated": {
			input:  []structs.ACLTokenRoleLink{{ID: role.ID}, {ID: role.ID}},
			output: []structs.ACLTokenRoleLink{{ID: role.ID, Name: role.Name}},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			updated, err := writer.Create(&structs.ACLToken{Roles: tc.input}, false)
			if tc.errorContains == "" {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.output, updated.Roles)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			}
		})
	}
}

func TestTokenWriter_Policies(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	policy := &structs.ACLPolicy{
		ID:   generateID(t),
		Name: generateID(t),
	}
	require.NoError(t, store.ACLPolicySet(0, policy))

	writer := buildTokenWriter(store, aclCache)

	testCases := map[string]struct {
		input         []structs.ACLTokenPolicyLink
		output        []structs.ACLTokenPolicyLink
		errorContains string
	}{
		"valid policy ID": {
			input:  []structs.ACLTokenPolicyLink{{ID: policy.ID}},
			output: []structs.ACLTokenPolicyLink{{ID: policy.ID, Name: policy.Name}},
		},
		"valid policy name": {
			input:  []structs.ACLTokenPolicyLink{{Name: policy.Name}},
			output: []structs.ACLTokenPolicyLink{{ID: policy.ID, Name: policy.Name}},
		},
		"invalid policy ID": {
			input:         []structs.ACLTokenPolicyLink{{ID: generateID(t)}},
			errorContains: "No such ACL policy with ID",
		},
		"invalid policy name": {
			input:         []structs.ACLTokenPolicyLink{{Name: "invalid-policy-name"}},
			errorContains: "No such ACL policy with name",
		},
		"links are de-duplicated": {
			input:  []structs.ACLTokenPolicyLink{{ID: policy.ID}, {ID: policy.ID}},
			output: []structs.ACLTokenPolicyLink{{ID: policy.ID, Name: policy.Name}},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			updated, err := writer.Create(&structs.ACLToken{Policies: tc.input}, false)
			if tc.errorContains == "" {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.output, updated.Policies)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			}
		})
	}
}

func TestTokenWriter_ServiceIdentities(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	writer := buildTokenWriter(store, aclCache)

	testCases := map[string]struct {
		input         []*structs.ACLServiceIdentity
		tokenLocal    bool
		output        []*structs.ACLServiceIdentity
		errorContains string
	}{
		"empty service name": {
			input:         []*structs.ACLServiceIdentity{{ServiceName: ""}},
			errorContains: "missing the service name",
		},
		"datacenters given on local token": {
			input:         []*structs.ACLServiceIdentity{{ServiceName: "web", Datacenters: []string{"dc1", "dc2"}}},
			tokenLocal:    true,
			errorContains: "cannot specify a list of datacenters on a local token",
		},
		"invalid service name": {
			input:         []*structs.ACLServiceIdentity{{ServiceName: "INVALID!"}},
			errorContains: "has an invalid name",
		},
		"duplicate identities are merged": {
			input: []*structs.ACLServiceIdentity{
				{ServiceName: "web", Datacenters: []string{"dc1"}},
				{ServiceName: "web", Datacenters: []string{"dc2"}},
			},
			output: []*structs.ACLServiceIdentity{{ServiceName: "web", Datacenters: []string{"dc1", "dc2"}}},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			updated, err := writer.Create(&structs.ACLToken{
				ServiceIdentities: tc.input,
				Local:             tc.tokenLocal,
			}, false)
			if tc.errorContains == "" {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.output, updated.ServiceIdentities)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			}
		})
	}
}

func TestTokenWriter_NodeIdentities(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	writer := buildTokenWriter(store, aclCache)

	testCases := map[string]struct {
		input         []*structs.ACLNodeIdentity
		output        []*structs.ACLNodeIdentity
		errorContains string
	}{
		"empty service name": {
			input:         []*structs.ACLNodeIdentity{{NodeName: "", Datacenter: "dc1"}},
			errorContains: "missing the node name",
		},
		"empty datacenter": {
			input:         []*structs.ACLNodeIdentity{{NodeName: "web"}},
			errorContains: "missing the datacenter field",
		},
		"invalid node name": {
			input:         []*structs.ACLNodeIdentity{{NodeName: "INVALID!", Datacenter: "dc1"}},
			errorContains: "has an invalid name",
		},
		"duplicate identities are removed": {
			input: []*structs.ACLNodeIdentity{
				{NodeName: "web", Datacenter: "dc1"},
				{NodeName: "web", Datacenter: "dc2"},
				{NodeName: "web", Datacenter: "dc1"},
			},
			output: []*structs.ACLNodeIdentity{
				{NodeName: "web", Datacenter: "dc1"},
				{NodeName: "web", Datacenter: "dc2"},
			},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			updated, err := writer.Create(&structs.ACLToken{NodeIdentities: tc.input}, false)
			if tc.errorContains == "" {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.output, updated.NodeIdentities)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			}
		})
	}
}

func TestTokenWriter_Create_Expiration(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	role := &structs.ACLRole{
		ID:   generateID(t),
		Name: generateID(t),
	}
	require.NoError(t, store.ACLRoleSet(0, role))

	writer := buildTokenWriter(store, aclCache)

	t.Run("ExpirationTTL", func(t *testing.T) {
		token := &structs.ACLToken{
			AccessorID: generateID(t),
			SecretID:   generateID(t),
			Roles: []structs.ACLTokenRoleLink{
				{ID: role.ID},
			},
			ExpirationTTL: 10 * time.Minute,
		}

		updated, err := writer.Create(token, false)
		require.NoError(t, err)
		require.InEpsilon(t, 10*time.Minute, updated.ExpirationTime.Sub(time.Now()), 0.1)
		require.Zero(t, updated.ExpirationTTL)
	})

	t.Run("ExpirationTime", func(t *testing.T) {
		expirationTime := time.Now().Add(10 * time.Minute)

		token := &structs.ACLToken{
			AccessorID: generateID(t),
			SecretID:   generateID(t),
			Roles: []structs.ACLTokenRoleLink{
				{ID: role.ID},
			},
			ExpirationTime: &expirationTime,
		}

		updated, err := writer.Create(token, false)
		require.NoError(t, err)
		require.Equal(t, expirationTime, *updated.ExpirationTime)
	})
}

func TestTokenWriter_Create_Success(t *testing.T) {
	store := testStateStore(t)

	role := &structs.ACLRole{
		ID:   generateID(t),
		Name: "cluster-operators",
	}
	require.NoError(t, store.ACLRoleSet(0, role))

	token := &structs.ACLToken{
		AccessorID: generateID(t),
		SecretID:   generateID(t),
		Roles: []structs.ACLTokenRoleLink{
			{ID: role.ID},
		},
	}

	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", token.SecretID)
	defer aclCache.AssertExpectations(t)

	writer := buildTokenWriter(store, aclCache)

	updated, err := writer.Create(token, false)
	require.NoError(t, err)
	require.NotNil(t, updated)
}

func TestTokenWriter_Update_Validation(t *testing.T) {
	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", mock.Anything)

	store := testStateStore(t)

	token := &structs.ACLToken{
		AccessorID:     generateID(t),
		SecretID:       generateID(t),
		ExpirationTime: timePointer(time.Now().Add(1 * time.Hour)),
	}
	expiredToken := &structs.ACLToken{
		AccessorID:     generateID(t),
		SecretID:       generateID(t),
		ExpirationTime: timePointer(time.Now().Add(-1 * time.Hour)),
	}
	require.NoError(t, store.ACLTokenBatchSet(0, []*structs.ACLToken{token, expiredToken}, state.ACLTokenSetOptions{}))

	writer := buildTokenWriter(store, aclCache)

	testCases := map[string]struct {
		token         structs.ACLToken
		errorContains string
	}{
		"AccessorID not a UUID": {
			token:         structs.ACLToken{AccessorID: "not-a-uuid"},
			errorContains: "not a valid UUID",
		},
		"SecretID is a legacy root policy name": {
			token:         structs.ACLToken{AccessorID: token.AccessorID, SecretID: "allow"},
			errorContains: "Cannot modify root ACL",
		},
		"AccessorID does not match any token": {
			token:         structs.ACLToken{AccessorID: generateID(t)},
			errorContains: "Cannot find token",
		},
		"AccessorID matches expired token": {
			token:         structs.ACLToken{AccessorID: expiredToken.AccessorID},
			errorContains: "Cannot find token",
		},
		"SecretID changed": {
			token:         structs.ACLToken{AccessorID: token.AccessorID, SecretID: generateID(t)},
			errorContains: "Changing a token's SecretID is not permitted",
		},
		"Local changed": {
			token:         structs.ACLToken{AccessorID: token.AccessorID, Local: !token.Local},
			errorContains: "Cannot toggle local mode",
		},
		"AuthMethod changed": {
			token:         structs.ACLToken{AccessorID: token.AccessorID, AuthMethod: "some-other-auth-method"},
			errorContains: "Cannot change AuthMethod",
		},
		"ExpirationTTL is set": {
			token:         structs.ACLToken{AccessorID: token.AccessorID, ExpirationTTL: 5 * time.Minute},
			errorContains: "Cannot change expiration time",
		},
		"ExpirationTime changed": {
			token:         structs.ACLToken{AccessorID: token.AccessorID, ExpirationTime: timePointer(token.ExpirationTime.Add(1 * time.Minute))},
			errorContains: "Cannot change expiration time",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			_, err := writer.Update(&tc.token)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func TestTokenWriter_Update_Success(t *testing.T) {
	store := testStateStore(t)

	authMethod := &structs.ACLAuthMethod{
		Name: generateID(t),
		Type: "jwt",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	token := &structs.ACLToken{
		AccessorID:     generateID(t),
		SecretID:       generateID(t),
		ExpirationTime: timePointer(time.Now().Add(1 * time.Hour)),
		AuthMethod:     authMethod.Name,
	}
	token.SetHash(true)
	require.NoError(t, store.ACLTokenSet(0, token))

	aclCache := &MockACLCache{}
	aclCache.On("RemoveIdentityWithSecretToken", token.SecretID)
	defer aclCache.AssertExpectations(t)

	writer := buildTokenWriter(store, aclCache)
	updated, err := writer.Update(&structs.ACLToken{
		AccessorID:  token.AccessorID,
		Description: "New Description",
	})
	require.NoError(t, err)
	require.Equal(t, "New Description", updated.Description)

	// These should've been left as-is.
	require.Equal(t, token.SecretID, updated.SecretID)
	require.Equal(t, token.Local, updated.Local)
	require.Equal(t, token.AuthMethod, updated.AuthMethod)
	require.Equal(t, token.ExpirationTime, updated.ExpirationTime)
	require.Equal(t, token.CreateTime, updated.CreateTime)
	require.NotEqual(t, token.Hash, updated.Hash)
}

func TestTokenWriter_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := testStateStore(t)

		token := &structs.ACLToken{
			AccessorID: generateID(t),
			SecretID:   generateID(t),
			Local:      true,
		}
		require.NoError(t, store.ACLTokenSet(0, token))

		aclCache := NewMockACLCache(t)
		aclCache.On("RemoveIdentityWithSecretToken", token.SecretID).Return()

		var deletedIDs []string
		writer := NewTokenWriter(TokenWriterConfig{
			LocalTokensEnabled: true,
			ACLCache:           aclCache,
			Store:              store,
			RaftApply: func(msgType structs.MessageType, msg interface{}) (interface{}, error) {
				if msgType != structs.ACLTokenDeleteRequestType {
					return nil, fmt.Errorf("unexpected message type: %v", msgType)
				}

				req, ok := msg.(*structs.ACLTokenBatchDeleteRequest)
				if !ok {
					return nil, fmt.Errorf("unexpected message: %T", msg)
				}
				deletedIDs = req.TokenIDs

				return nil, nil
			},
		})
		err := writer.Delete(token.SecretID, false)
		require.NoError(t, err)
		require.Equal(t, []string{token.AccessorID}, deletedIDs)
	})

	t.Run("local tokens disabled", func(t *testing.T) {
		store := testStateStore(t)

		token := &structs.ACLToken{
			AccessorID: generateID(t),
			SecretID:   generateID(t),
			Local:      true,
		}

		require.NoError(t, store.ACLTokenSet(0, token))
		writer := NewTokenWriter(TokenWriterConfig{
			LocalTokensEnabled: false,
			Store:              store,
		})

		err := writer.Delete(token.SecretID, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Cannot upsert tokens within this datacenter")
	})

	t.Run("global token in non-primary datacenter", func(t *testing.T) {
		store := testStateStore(t)

		token := &structs.ACLToken{
			AccessorID: generateID(t),
			SecretID:   generateID(t),
			Local:      false,
		}
		require.NoError(t, store.ACLTokenSet(0, token))

		writer := NewTokenWriter(TokenWriterConfig{
			LocalTokensEnabled:  true,
			InPrimaryDatacenter: false,
			Store:               store,
		})

		err := writer.Delete(token.SecretID, false)
		require.Error(t, err)
		require.Equal(t, ErrCannotWriteGlobalToken, err)
	})

	t.Run("token not found", func(t *testing.T) {
		store := testStateStore(t)

		writer := NewTokenWriter(TokenWriterConfig{
			LocalTokensEnabled: true,
			Store:              store,
		})
		err := writer.Delete(generateID(t), false)
		require.Error(t, err)
		require.True(t, errors.Is(err, acl.ErrNotFound))
	})

	t.Run("logout requires token to be created by login", func(t *testing.T) {
		store := testStateStore(t)

		token := &structs.ACLToken{
			AccessorID: generateID(t),
			SecretID:   generateID(t),
			Local:      true,
		}
		require.NoError(t, store.ACLTokenSet(0, token))

		writer := NewTokenWriter(TokenWriterConfig{
			LocalTokensEnabled: true,
			Store:              store,
		})
		err := writer.Delete(token.SecretID, true)
		require.Error(t, err)
		require.True(t, errors.Is(err, acl.ErrPermissionDenied))
		require.Contains(t, err.Error(), "wasn't created via login")
	})
}

func raftApplyACLTokenSet(store *state.Store) RaftApplyFn {
	return func(msgType structs.MessageType, msg interface{}) (interface{}, error) {
		if msgType != structs.ACLTokenSetRequestType {
			return nil, fmt.Errorf("unexpected message type: %v", msgType)
		}

		req, ok := msg.(*structs.ACLTokenBatchSetRequest)
		if !ok {
			return nil, fmt.Errorf("unexpected message: %T", msg)
		}

		err := store.ACLTokenBatchSet(0, req.Tokens, state.ACLTokenSetOptions{
			CAS:                          req.CAS,
			AllowMissingPolicyAndRoleIDs: req.AllowMissingLinks,
			ProhibitUnprivileged:         req.ProhibitUnprivileged,
		})
		return nil, err
	}
}

func timePointer(t time.Time) *time.Time { return &t }

func buildTokenWriter(store *state.Store, aclCache ACLCache) *TokenWriter {
	return NewTokenWriter(TokenWriterConfig{
		RaftApply:           raftApplyACLTokenSet(store),
		ACLCache:            aclCache,
		Store:               store,
		MinExpirationTTL:    1 * time.Minute,
		MaxExpirationTTL:    24 * time.Hour,
		PrimaryDatacenter:   "dc1",
		InPrimaryDatacenter: true,
		LocalTokensEnabled:  true,
	})
}
