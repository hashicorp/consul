// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pbproxystate

import (
	"testing"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/stretchr/testify/require"
)

// TestMirrorsCatalogProtocol ensures that there is no unintended drift between pbcatalog.Protocol and
// pbproxystate.Protocol.
func TestMirrorsCatalogProtocol(t *testing.T) {
	require.Equal(t, pbcatalog.Protocol_value, Protocol_value, "pbcatalog.Protocol and pbproxystate.Protocol have diverged")
	for i := range pbcatalog.Protocol_name {
		require.Equal(t, pbcatalog.Protocol_name[i], Protocol_name[i],
			"pbcatalog.Protocol and pbproxystate.Protocol ordinals do not match;"+
				" ordinals for equivalent values must match so that casting between them produces expected results")
	}
}
