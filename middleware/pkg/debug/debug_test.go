package debug

import (
	"strings"
	"testing"
)

func TestIsDebug(t *testing.T) {
	if ok := IsDebug("o-o.debug.miek.nl."); ok != "miek.nl." {
		t.Errorf("expected o-o.debug.miek.nl. to be debug")
	}
	if ok := IsDebug(strings.ToLower("o-o.Debug.miek.nl.")); ok != "miek.nl." {
		t.Errorf("expected o-o.Debug.miek.nl. to be debug")
	}
	if ok := IsDebug("i-o.debug.miek.nl."); ok != "" {
		t.Errorf("expected i-o.Debug.miek.nl. to be non-debug")
	}
	if ok := IsDebug(strings.ToLower("i-o.Debug.")); ok != "" {
		t.Errorf("expected o-o.Debug. to be non-debug")
	}
}
