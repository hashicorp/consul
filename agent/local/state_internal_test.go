// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package local

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/stretchr/testify/require"
)

func TestRegistrationTokenFallback(t *testing.T) {
	svcId := structs.NewServiceID("redis", nil)
	addServiceFn := func(l *State, isLocal bool) error {
		return l.AddServiceWithChecks(&structs.NodeService{ID: svcId.ID}, nil, "", isLocal)
	}
	svcTokenFallback := func(l *State) func() string {
		return l.serviceRegistrationTokenFallback(svcId)
	}
	testRegistrationTokenFallback(t, "service", addServiceFn, svcTokenFallback)

	checkId := structs.NewCheckID("redis-check", nil)
	addCheckFn := func(l *State, isLocal bool) error {
		return l.AddCheck(&structs.HealthCheck{CheckID: checkId.ID}, "", isLocal)
	}
	checkTokenFallback := func(l *State) func() string {
		return l.checkRegistrationTokenFallback(checkId)
	}
	testRegistrationTokenFallback(t, "check", addCheckFn, checkTokenFallback)
}

func testRegistrationTokenFallback(
	t *testing.T,
	prefix string,
	addResourceFn func(*State, bool) error,
	tokenFallback func(*State) func() string,
) {
	cases := map[string]struct {
		registrationToken string
		isLocal           bool
		addResource       func(*State, bool) error
		expToken          string
	}{
		"defaults to empty token": {},
		"empty token when registration token not configured": {
			addResource: addResourceFn,
		},
		"empty token when resource not found": {
			registrationToken: "token123",
		},
		"registration token is used when resource is locally-defined": {
			registrationToken: "token123",
			addResource:       addResourceFn,
			isLocal:           true,
			expToken:          "token123",
		},
		"empty token when resource is not locally-defined": {
			registrationToken: "token123",
			addResource:       addResourceFn,
		},
	}
	for name, c := range cases {
		t.Run(prefix+" "+name, func(t *testing.T) {
			tokens := new(token.Store)
			tokens.Load(token.Config{
				ACLConfigFileRegistrationToken: c.registrationToken,
			}, nil)

			l := NewState(Config{}, nil, tokens)
			l.TriggerSyncChanges = func() {}

			if c.addResource != nil {
				require.NoError(t, c.addResource(l, c.isLocal))
			}

			fn := tokenFallback(l)
			require.Equal(t, c.expToken, fn())
		})
	}
}
