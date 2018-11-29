// +build !ent

package config

import (
	"fmt"
//	"github.com/hashicorp/consul/agent/structs"
)

func (b *Builder) validateSegments(rt RuntimeConfig) error {
	if len(rt.Segments) > rt.SegmentLimit {
		return fmt.Errorf("Cannot exceed network segment limit of %d", rt.SegmentLimit)
	}
// 	takenPorts := make(map[int]string, len(rt.Segments))
	for _, segment := range rt.Segments {
		if segment.Name == "" {
			return fmt.Errorf("Segment name cannot be blank")
		}
		if len(segment.Name) > rt.SegmentNameLimit {
			return fmt.Errorf("Segment name %q exceeds maximum length of %d", segment.Name, rt.SegmentNameLimit)
		}
/* 		previous, ok := takenPorts[segment.Port]
		if ok {
			return fmt.Errorf("Segment %q port %d overlaps with segment %q", segment.Name, segment.Port, previous)
		}
		takenPorts[segment.Port] = segment.Name
*/
	}

	return nil
}
