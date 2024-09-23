// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

type V2WriteRequest struct {
	Data map[string]any `json:"data"`
}

type V2WriteResponse struct {
	ID struct {
		Name string `json:"name"`
	} `json:"id"`
	Data map[string]any `json:"data"`
}

// We are testing a v2 endpoint here in the v1 api module as a temporary measure to
// support v2 CRUD operations, until we have a final design for v2 api clients.
func TestAPI_RawV2ExportedServices(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.EnableDebug = true
	})

	defer s.Stop()

	endpoint := strings.ToLower(fmt.Sprintf("/api/multicluster/v2/exportedservices/e1"))
	wResp := &V2WriteResponse{}

	wReq := &V2WriteRequest{
		Data: map[string]any{
			"consumers": []map[string]any{
				{"peer": "p1"},
			},
			"services": []string{"s1"},
		},
	}

	_, err := c.Raw().Write(endpoint, wReq, wResp, &WriteOptions{Datacenter: "dc1"})
	require.NoError(t, err)
	require.NotEmpty(t, wResp.ID.Name)

	qOpts := &QueryOptions{Datacenter: "dc1"}

	var out map[string]interface{}
	_, err = c.Raw().Query(endpoint, &out, qOpts)
	require.NoError(t, err)

	require.Equal(t, map[string]any{
		"consumers": []any{
			map[string]any{"peer": "p1"},
		},
		"services": []any{"s1"},
	}, out["data"])

	_, err = c.Raw().Delete(endpoint, qOpts)
	require.NoError(t, err)

	out = make(map[string]interface{})
	_, err = c.Raw().Query(endpoint, &out, qOpts)
	require.ErrorContains(t, err, "404")
}
