// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

import (
	"fmt"
	"strconv"
)

func (s *Service) IsMeshEnabled() bool {
	for _, port := range s.GetPorts() {
		if port.Protocol == Protocol_PROTOCOL_MESH {
			return true
		}
	}
	return false
}

// FindTargetPort finds a ServicePort by its TargetPort value.
//
// Unlike FindPortByID, it will match a numeric TargetPort value. This is useful when
// looking up a service port by a workload port value, or when the data is known to
// be normalized to canonical target port values (e.g. computed routes).
func (s *Service) FindTargetPort(targetPort string) *ServicePort {
	if s == nil || targetPort == "" {
		return nil
	}

	for _, port := range s.GetPorts() {
		if port.TargetPort == targetPort {
			return port
		}
	}

	return nil
}

// FindPortByID finds a ServicePort by its VirtualPort or TargetPort value.
//
// Note that this will not match a target port if the given value is numeric.
// See Service.ServicePort doc for more information on how port IDs are matched.
func (s *Service) FindPortByID(id string) *ServicePort {
	if s == nil || id == "" {
		return nil
	}

	// If a port reference is numeric, it must be considered a virtual port.
	// See ServicePort doc for more information.
	if p, ok := toVirtualPort(id); ok {
		for _, port := range s.GetPorts() {
			if int(port.VirtualPort) == p {
				return port
			}
		}
	} else {
		for _, port := range s.GetPorts() {
			if port.TargetPort == id {
				return port
			}
		}
	}

	return nil
}

// MatchesPortId returns true if the given port ID is non-empty and matches the virtual
// or target port of the given ServicePort. See ServicePort doc for more information on
// how port IDs are matched.
//
// Note that this function does not validate the provided port ID. Configured service
// ports should be validated on write, prior to use of this function, which means any
// matching value is implicitly valid.
func (sp *ServicePort) MatchesPortId(id string) bool {
	if sp == nil || id == "" {
		return false
	}

	// If a port reference is numeric, it must be considered a virtual port.
	// See ServicePort doc for more information.
	if p, ok := toVirtualPort(id); ok {
		if int(sp.VirtualPort) == p {
			return true
		}
	} else {
		if sp.TargetPort == id {
			return true
		}
	}

	return false
}

// VirtualPortStr is a convenience helper for checking the virtual port against a port ID in config
// (e.g. keys in FailoverPolicy.PortConfigs). It returns the string representation of the virtual port.
func (sp *ServicePort) VirtualPortStr() string {
	if sp == nil {
		return ""
	}
	return fmt.Sprintf("%d", sp.VirtualPort)
}

func (sp *ServicePort) ToPrintableString() string {
	if sp == nil {
		return "<nil>"
	}
	if sp.VirtualPort > 0 {
		return fmt.Sprintf("%s (virtual %d)", sp.TargetPort, sp.VirtualPort)
	}
	return sp.TargetPort
}

// isVirtualPort returns the numeric virtual port value and true if the given port string is fully numeric.
// Otherwise, returns 0 and false. See ServicePort doc for more information.
func toVirtualPort(port string) (int, bool) {
	if p, err := strconv.Atoi(port); err == nil && p > 0 {
		return p, true
	}
	return 0, false
}
