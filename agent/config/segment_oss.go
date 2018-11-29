// +build !ent

package config

import (
	"fmt"
)

func (b *Builder) validateSegments(rt RuntimeConfig) error {
	if len(rt.Segments) > rt.SegmentLimit {
		return fmt.Errorf("Cannot exceed network segment limit of %d", rt.SegmentLimit)
	}
	for _, segment := range rt.Segments {
		if segment.Name == "" {
			return fmt.Errorf("Segment name cannot be blank")
		}
		if len(segment.Name) > rt.SegmentNameLimit {
			return fmt.Errorf("Segment name %q exceeds maximum length of %d", segment.Name, rt.SegmentNameLimit)
		}
	}
	return nil
}
