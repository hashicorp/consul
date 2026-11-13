// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import "fmt"

// Validate checks that the spec is a well-formed state machine.
func (s ComponentSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: empty component name", ErrInvalidSpec)
	}
	if len(s.States) == 0 {
		return fmt.Errorf("%w: %s has no states", ErrInvalidSpec, s.Name)
	}
	if _, ok := s.States[s.Initial]; !ok {
		return fmt.Errorf("%w: %s initial state %q is not defined", ErrInvalidSpec, s.Name, s.Initial)
	}
	for name, st := range s.States {
		if name == "" {
			return fmt.Errorf("%w: %s has an empty state name", ErrInvalidSpec, s.Name)
		}
		if !st.Phase.valid() {
			return fmt.Errorf("%w: %s state %q has invalid phase %v", ErrInvalidSpec, s.Name, name, st.Phase)
		}
		if st.MaxDuration < 0 {
			return fmt.Errorf("%w: %s state %q has negative MaxDuration", ErrInvalidSpec, s.Name, name)
		}
		for _, next := range st.Next {
			if _, ok := s.States[next]; !ok {
				return fmt.Errorf("%w: %s state %q allows undefined next state %q", ErrInvalidSpec, s.Name, name, next)
			}
		}
	}
	return nil
}

func (s ComponentSpec) clone() ComponentSpec {
	states := make(map[StateName]StateSpec, len(s.States))
	for name, st := range s.States {
		st.Next = append([]StateName(nil), st.Next...)
		states[name] = st
	}
	s.States = states
	return s
}

func (s ComponentSpec) allows(from, to StateName) bool {
	for _, next := range s.States[from].Next {
		if next == to {
			return true
		}
	}
	return false
}
