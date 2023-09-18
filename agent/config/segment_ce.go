//go:build !consulent
// +build !consulent

package config

import (
	"github.com/hashicorp/consul/agent/structs"
)

func (b *builder) validateSegments(rt RuntimeConfig) error {
	if rt.SegmentName != "" {
		return structs.ErrSegmentsNotSupported
	}
	if len(rt.Segments) > 0 {
		return structs.ErrSegmentsNotSupported
	}
	return nil
}
