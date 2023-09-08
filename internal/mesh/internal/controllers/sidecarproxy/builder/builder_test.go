// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

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
