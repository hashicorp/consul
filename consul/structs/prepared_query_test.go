package structs

import (
	"testing"
)

func TestStructs_PreparedQuery_GetACLPrefix(t *testing.T) {
	ephemeral := &PreparedQuery{}
	if prefix, ok := ephemeral.GetACLPrefix(); ok {
		t.Fatalf("bad: %s", prefix)
	}

	named := &PreparedQuery{Name: "hello"}
	if prefix, ok := named.GetACLPrefix(); !ok || prefix != "hello" {
		t.Fatalf("bad: %#v", prefix)
	}
}
