// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package pbpeering

func (r *GenerateTokenRequest) PartitionOrDefault() string {
	return ""
}

func (p *Peering) PartitionOrDefault() string {
	return ""
}

func (ptb *PeeringTrustBundle) PartitionOrDefault() string {
	return ""
}
