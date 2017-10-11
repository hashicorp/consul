package cat

import (
	"strings"
	"testing"
)

func TestCatalogCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("usage has tabs")
	}
}
