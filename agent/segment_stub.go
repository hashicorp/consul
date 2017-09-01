// +build !ent

package agent

import (
	"github.com/hashicorp/consul/agent/structs"
)

func ValidateSegments(conf *Config) error {
	if conf.Segment != "" {
		return structs.ErrSegmentsNotSupported
	}

	if len(conf.Segments) > 0 {
		return structs.ErrSegmentsNotSupported
	}

	return nil
}
