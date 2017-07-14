package command

import "testing"

func TestCatalogCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(CatalogCommand))
}
