package prepared_query

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

var (
	bench = &structs.PreparedQuery{
		Name: "hello",
		Template: structs.QueryTemplateOptions{
			Type:   structs.QueryTemplateTypeNamePrefixMatch,
			Regexp: "^hello-(.*)-(.*)$",
		},
		Service: structs.ServiceQuery{
			Service: "${name.full}",
			Tags:    []string{"${name.prefix}", "${name.suffix}", "${match(0)}", "${match(1)}", "${match(2)}"},
		},
	}
)

func BenchmarkTemplate_Compile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Compile(bench)
		if err != nil {
			b.Fatalf("err: %v", err)
		}
	}
}

func BenchmarkTemplate_Render(b *testing.B) {
	compiled, err := Compile(bench)
	if err != nil {
		b.Fatalf("err: %v", err)
	}

	for i := 0; i < b.N; i++ {
		_, err := compiled.Render("hello-bench-mark")
		if err != nil {
			b.Fatalf("err: %v", err)
		}
	}
}

func TestTemplate_Compile(t *testing.T) {
	query := &structs.PreparedQuery{
		Name: "hello",
		Template: structs.QueryTemplateOptions{
			Type:   structs.QueryTemplateTypeNamePrefixMatch,
			Regexp: "^hello-(.*)$",
		},
		Service: structs.ServiceQuery{
			Service: "${name.full}",
			Tags:    []string{"${name.prefix}", "${name.suffix}", "${match(0)}", "${match(1)}", "${match(2)}"},
		},
	}

	compiled, err := Compile(query)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	rendered, err := compiled.Render("hello-everyone")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	fmt.Printf("%#v\n", *query)
	fmt.Printf("%#v\n", *rendered)
}
