// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	t.Helper()
	m := protojson.MarshalOptions{
		Indent: "  ",
	}
	gotJSON, err := m.Marshal(pb)
	require.NoError(t, err)

	// protojson format is non-determinstic, so scrub it through the determinstic json.Marshal so
	// 'git diff' only shows real changes
	//
	// https://github.com/golang/protobuf/issues/1269
	var tmp map[string]any
	require.NoError(t, json.Unmarshal(gotJSON, &tmp))

	gotJSON, err = json.MarshalIndent(&tmp, "", "  ")
	require.NoError(t, err)

	return string(gotJSON)
}

func JSONToProxyTemplate(t *testing.T, json []byte) *pbmesh.ProxyStateTemplate {
	t.Helper()
	proxyTemplate := &pbmesh.ProxyStateTemplate{}
	m := protojson.UnmarshalOptions{}
	err := m.Unmarshal(json, proxyTemplate)
	require.NoError(t, err)
	return proxyTemplate
}

func goldenValue(t *testing.T, goldenFile string, actual string, update bool) string {
	t.Helper()
	return string(goldenValueBytes(t, goldenFile, actual, update))
}

func goldenValueBytes(t *testing.T, goldenFile string, actual string, update bool) []byte {
	t.Helper()
	goldenPath := filepath.Join("testdata", goldenFile) + ".golden"

	if update {
		bytes := []byte(actual)
		err := os.WriteFile(goldenPath, bytes, 0644)
		require.NoError(t, err)

		return bytes
	}

	content, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	return content
}
