package agent

import (
	"testing"
)

func TestUUIDToUint64(t *testing.T) {
	inp := "cb9a81ad-fff6-52ac-92a7-5f70687805ec"

	// Output value was computed using python
	if uuidToUint64(inp) != 6430540886266763072 {
		t.Fatalf("bad")
	}
}
