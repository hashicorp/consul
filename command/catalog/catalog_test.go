package catalog

import (
	"strings"
	"testing"
)

func TestCatalogCommand_noTabs(t *testing.T) {
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("usage has tabs")
	}
}
