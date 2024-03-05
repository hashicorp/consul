// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"

	"github.com/hashicorp/consul/sdk/testutil"
)

type V2WriteRequest struct {
	Metadata map[string]string `json:"metadata"`
	Data     map[string]any    `json:"data"`
	Owner    *pbresource.ID    `json:"owner"`
}

type V2WriteResponse struct {
	Metadata   map[string]string `json:"metadata"`
	Data       map[string]any    `json:"data"`
	Owner      *pbresource.ID    `json:"owner,omitempty"`
	ID         *pbresource.ID    `json:"id"`
	Version    string            `json:"version"`
	Generation string            `json:"generation"`
	Status     map[string]any    `json:"status"`
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

	var consumers []map[string]any
	consumers = append(consumers, map[string]any{"peer": "p1"})
	data := map[string]any{"consumers": consumers}
	data["services"] = []string{"s1"}
	wReq := &V2WriteRequest{
		Metadata: nil,
		Data:     data,
		Owner:    nil,
	}

	_, err := c.Raw().Write(endpoint, wReq, wResp, &WriteOptions{Datacenter: "dc1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if wResp.ID.Name == "" {
		t.Fatalf("no write response")
	}

	qOpts := &QueryOptions{Datacenter: "dc1"}
	var out map[string]interface{}
	_, err = c.Raw().Query(endpoint, &out, qOpts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	respData, _ := json.Marshal(out["data"])
	readData := &pbmulticluster.ExportedServices{}
	if err = protojson.Unmarshal(respData, readData); err != nil {
		t.Fatalf("invalid read response")
	}
	if len(readData.Services) != 1 {
		t.Fatalf("incorrect resource data")
	}

	_, err = c.Raw().Delete(endpoint, qOpts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out = make(map[string]interface{})
	_, err = c.Raw().Query(endpoint, &out, qOpts)
	require.ErrorContains(t, err, "404")
}
