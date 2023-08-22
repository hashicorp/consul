//go:build !consulent
// +build !consulent

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
