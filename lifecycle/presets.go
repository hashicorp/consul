// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import "time"

// State names used by the presets. Components may use their own names.
const (
	StateStarting      StateName = "Starting"
	StateReady         StateName = "Ready"
	StateDegraded      StateName = "Degraded"
	StateStopping      StateName = "Stopping"
	StateStopped       StateName = "Stopped"
	StateFailed        StateName = "Failed"
	StateBooting       StateName = "Booting"
	StateServing       StateName = "Serving"
	StateRestoring     StateName = "Restoring"
	StateRestoreFailed StateName = "RestoreFailed"
	StateReloading     StateName = "Reloading"
)

// Simple is a general purpose lifecycle:
//
//	Starting -> Ready <-> Degraded -> Stopping -> Stopped, and any -> Failed.
func Simple(name ComponentName) ComponentSpec {
	return ComponentSpec{
		Name:    name,
		Initial: StateStarting,
		States: map[StateName]StateSpec{
			StateStarting: {Phase: PhaseStarting, Next: []StateName{StateReady, StateDegraded, StateFailed, StateStopping}},
			StateReady:    {Phase: PhaseReady, Next: []StateName{StateDegraded, StateFailed, StateStopping}},
			StateDegraded: {Phase: PhaseDegraded, Next: []StateName{StateReady, StateFailed, StateStopping}},
			StateStopping: {Phase: PhaseTransitioning, Next: []StateName{StateStopped, StateFailed}},
			StateStopped:  {Phase: PhaseStopped, Next: []StateName{StateStarting}},
			StateFailed:   {Phase: PhaseFailed, Next: []StateName{StateStarting, StateStopping}},
		},
	}
}

// Restorable is for components whose whole state can be replaced while they
// serve, for example a state store restored from a snapshot:
//
//	Booting -> Serving <-> Restoring, Restoring -> RestoreFailed -> Restoring|Serving.
func Restorable(name ComponentName) ComponentSpec {
	return ComponentSpec{
		Name:    name,
		Initial: StateBooting,
		States: map[StateName]StateSpec{
			StateBooting:       {Phase: PhaseStarting, Next: []StateName{StateServing, StateRestoring}},
			StateServing:       {Phase: PhaseReady, Next: []StateName{StateRestoring}},
			StateRestoring:     {Phase: PhaseTransitioning, Next: []StateName{StateServing, StateRestoreFailed}},
			StateRestoreFailed: {Phase: PhaseFailed, Next: []StateName{StateRestoring, StateServing}},
		},
	}
}

// Reloadable is for components that reload configuration in place. Each reload
// bumps the generation, so listeners can reset anything tied to the old one:
//
//	Starting -> Ready <-> Reloading, Reloading -> Failed -> Reloading|Ready.
func Reloadable(name ComponentName) ComponentSpec {
	return ComponentSpec{
		Name:    name,
		Initial: StateStarting,
		States: map[StateName]StateSpec{
			StateStarting:  {Phase: PhaseStarting, Next: []StateName{StateReady, StateFailed}},
			StateReady:     {Phase: PhaseReady, Next: []StateName{StateReloading}},
			StateReloading: {Phase: PhaseTransitioning, Next: []StateName{StateReady, StateFailed}},
			StateFailed:    {Phase: PhaseFailed, Next: []StateName{StateReloading, StateReady}},
		},
	}
}

// WithMaxDuration returns a copy of the spec with a stuck threshold on one state.
func (s ComponentSpec) WithMaxDuration(state StateName, d time.Duration) ComponentSpec {
	s = s.clone()
	if st, ok := s.States[state]; ok {
		st.MaxDuration = d
		s.States[state] = st
	}
	return s
}
