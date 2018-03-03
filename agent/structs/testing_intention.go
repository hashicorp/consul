package structs

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestIntention returns a valid, uninserted (no ID set) intention.
func TestIntention(t testing.T) *Intention {
	return &Intention{
		SourceNS:        "eng",
		SourceName:      "api",
		DestinationNS:   "eng",
		DestinationName: "db",
		Action:          IntentionActionAllow,
		SourceType:      IntentionSourceConsul,
		Meta:            map[string]string{},
	}
}
