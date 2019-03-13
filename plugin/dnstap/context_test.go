package dnstap

import (
	"context"
	"testing"
)

func TestDnstapContext(t *testing.T) {
	ctx := ContextWithTapper(context.TODO(), Dnstap{})
	tapper := TapperFromContext(ctx)

	if tapper == nil {
		t.Fatal("Can't get tapper")
	}
}
