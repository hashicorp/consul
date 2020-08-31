package main

import (
	"fmt"
	"go/ast"
	"strings"
)

type structConfig struct {
	Source           string
	Target           string // TODO: split package/struct name
	Output           string
	FuncNameFragment string
	IgnoreFields     []string
	FuncFrom         string
	FuncTo           string
	Fields           []fieldConfig
}

type fieldConfig struct {
	Source   *ast.Field
	Name     string
	FuncFrom string
	FuncTo   string
	// TODO: Pointer pointerSettings
}

func configsFromAnnotations(sources pkg) ([]structConfig, error) {
	names := sources.Names()
	cfgs := make([]structConfig, 0, len(names))
	for _, name := range names {
		strct := sources.structs[name]
		cfg, err := parseStructAnnotation(name, strct.Doc)
		if err != nil {
			return nil, fmt.Errorf("from source %v: %w", name, err)
		}

		for _, field := range strct.Fields {
			f, err := parseFieldAnnotation(field)
			if err != nil {
				return nil, fmt.Errorf("from source %v.%v: %w", name, fieldNameFromAST(field.Names), err)
			}
			cfg.Fields = append(cfg.Fields, f)
		}

		cfgs = append(cfgs, cfg)
	}
	// TODO: validate config - required values
	return cfgs, nil
}

func fieldNameFromAST(names []*ast.Ident) string {
	if len(names) == 0 {
		return "unknown"
	}
	return names[0].Name
}

func parseStructAnnotation(name string, doc []*ast.Comment) (structConfig, error) {
	c := structConfig{Source: name}

	i := structAnnotationIndex(doc)
	if i < 0 {
		return c, fmt.Errorf("missing struct annotation")
	}

	buf := new(strings.Builder)
	for _, line := range doc[i+1:] {
		buf.WriteString(strings.TrimLeft(line.Text, "/"))
	}
	for _, part := range strings.Fields(buf.String()) {
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			return c, fmt.Errorf("invalid term '%v' in annotation, expected only one =", part)
		}
		value := kv[1]
		switch kv[0] {
		case "target":
			c.Target = value
		case "output":
			c.Output = value
		case "name":
			c.FuncNameFragment = value
		case "ignore-fields":
			c.IgnoreFields = strings.Split(value, ",")
		case "func-from":
			c.FuncFrom = value
		case "func-to":
			c.FuncTo = value
		default:
			return c, fmt.Errorf("invalid annotation key %v in term '%v'", kv[0], part)
		}
	}

	return c, nil
}

func parseFieldAnnotation(field *ast.Field) (fieldConfig, error) {
	var c fieldConfig
	return c, nil
}
