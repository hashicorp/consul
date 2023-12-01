// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"flag"
	"os"
	"testing"

	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	return prototest.ProtoToJSON(t, pb)
}

func JSONToProxyTemplate(t *testing.T, json []byte) *pbmesh.ProxyStateTemplate {
	t.Helper()
	proxyTemplate := &pbmesh.ProxyStateTemplate{}
	m := protojson.UnmarshalOptions{}
	err := m.Unmarshal(json, proxyTemplate)
	require.NoError(t, err)
	return proxyTemplate
}
