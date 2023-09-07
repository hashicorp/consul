// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"flag"
	"os"
	"testing"

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
