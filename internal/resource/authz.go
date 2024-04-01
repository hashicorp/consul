// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

func peerNameV2ToV1(peer string) string {
	// The name of the local/default peer is different between v1 and v2.
	if peer == DefaultPeerName {
		return ""
	}
	return peer
}

func peerNameV1ToV2(peer string) string {
	// The name of the local/default peer is different between v1 and v2.
	if peer == "" {
		return DefaultPeerName
	}
	return peer
}
