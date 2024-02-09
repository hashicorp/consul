package authv2beta1

// IsEmpty returns true if a destination rule has no fields defined.
func (d *DestinationRule) IsEmpty() bool {
	if d == nil {
		return true
	}
	return len(d.PathExact) == 0 &&
		len(d.PathPrefix) == 0 &&
		len(d.PathRegex) == 0 &&
		len(d.Methods) == 0 &&
		len(d.Headers) == 0 &&
		len(d.PortNames) == 0 &&
		len(d.Exclude) == 0
}

// IsEmpty returns true if an exclude permission has no fields defined.
func (e *ExcludePermissionRule) IsEmpty() bool {
	if e == nil {
		return true
	}
	return len(e.PathExact) == 0 &&
		len(e.PathPrefix) == 0 &&
		len(e.PathRegex) == 0 &&
		len(e.Methods) == 0 &&
		len(e.Headers) == 0 &&
		len(e.PortNames) == 0
}

// PortsOnly returns true if a destination rule only specifies port criteria
func (d *DestinationRule) PortsOnly() bool {
	if d.IsEmpty() {
		return false
	}
	excludePortsOnly := true
	for _, e := range d.Exclude {
		if !e.PortsOnly() {
			excludePortsOnly = false
		}
	}
	return len(d.PathExact) == 0 &&
		len(d.PathPrefix) == 0 &&
		len(d.PathRegex) == 0 &&
		len(d.Methods) == 0 &&
		len(d.Headers) == 0 &&
		(len(d.PortNames) != 0 || excludePortsOnly)
}

// PortsOnly returns true if an exclude rule only specifies port criteria
func (e *ExcludePermissionRule) PortsOnly() bool {
	if e == nil {
		return false
	}
	return len(e.PathExact) == 0 &&
		len(e.PathPrefix) == 0 &&
		len(e.PathRegex) == 0 &&
		len(e.Methods) == 0 &&
		len(e.Headers) == 0 &&
		len(e.PortNames) != 0
}
