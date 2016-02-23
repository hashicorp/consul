package structs

import (
	"testing"
)

func TestStructs_PreparedQuery_GetACLPrefix(t *testing.T) {
	query := &PreparedQuery{
		Service: ServiceQuery{
			Service: "foo",
		},
	}
	if prefix := query.GetACLPrefix(); prefix != "foo" {
		t.Fatalf("bad: %s", prefix)
	}
}
