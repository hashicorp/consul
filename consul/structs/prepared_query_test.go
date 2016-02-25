package structs

import (
	"testing"
)

func TestStructs_PreparedQuery_GetACLPrefix(t *testing.T) {
	ephemeral := &PreparedQuery{}
	if prefix := ephemeral.GetACLPrefix(); prefix != nil {
		t.Fatalf("bad: %#v", prefix)
	}

	named := &PreparedQuery{Name: "hello"}
	if prefix := named.GetACLPrefix(); prefix == nil || *prefix != "hello" {
		t.Fatalf("bad: %#v", prefix)
	}
}
