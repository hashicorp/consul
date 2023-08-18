// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestCE_IntentionsCreate_failure(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	doCreate := func(t *testing.T, srcNS, dstNS string) {
		t.Helper()
		args := structs.TestIntention(t)
		args.SourceNS = srcNS
		args.SourceName = "*"
		args.DestinationNS = dstNS
		args.DestinationName = "*"
		req, _ := http.NewRequest("POST", "/v1/connect/intentions", jsonReader(args))
		resp := httptest.NewRecorder()
		_, err := a.srv.IntentionCreate(resp, req)
		require.Error(t, err)
	}

	t.Run("wildcard source namespace", func(t *testing.T) {
		doCreate(t, "*", "default")
	})
	t.Run("wildcard destination namespace", func(t *testing.T) {
		doCreate(t, "default", "*")
	})
	t.Run("wildcard source and destination namespaces", func(t *testing.T) {
		doCreate(t, "*", "*")
	})
	t.Run("non-default source namespace", func(t *testing.T) {
		doCreate(t, "foo", "default")
	})
	t.Run("non-default destination namespace", func(t *testing.T) {
		doCreate(t, "default", "foo")
	})
}
