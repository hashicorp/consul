package structs

import (
	"testing"
)

func TestStructs_PreparedQuery_GetACLPrefix(t *testing.T) {
	ephemeral := &PreparedQuery{}
	if prefix, ok := ephemeral.GetACLPrefix(); ok {
		t.Fatalf("bad: %s", prefix)
	}

	named := &PreparedQuery{
		Name: "hello",
	}
	if prefix, ok := named.GetACLPrefix(); !ok || prefix != "hello" {
		t.Fatalf("bad: ok=%v, prefix=%#v", ok, prefix)
	}

	tmpl := &PreparedQuery{
		Name: "",
		Template: QueryTemplateOptions{
			Type: QueryTemplateTypeNamePrefixMatch,
		},
	}
	if prefix, ok := tmpl.GetACLPrefix(); !ok || prefix != "" {
		t.Fatalf("bad: ok=%v prefix=%#v", ok, prefix)
	}
}

func TestPreparedQueryExecuteRequest_CacheInfoKey(t *testing.T) {
	// TODO: should these fields be included in the key?
	ignored := []string{"Agent", "QueryOptions"}
	assertCacheInfoKeyIsComplete(t, &PreparedQueryExecuteRequest{}, ignored...)
}
