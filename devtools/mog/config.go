package main

import (
	"fmt"
	"go/ast"
	"strings"
)

type structConfig struct {
	Source           string
	Target           target
	Output           string
	FuncNameFragment string
	IgnoreFields     []string
	FuncFrom         string
	FuncTo           string
	Fields           []fieldConfig
}

type target struct {
	Package string
	Struct  string
}

func newTarget(v string) target {
	i := strings.LastIndex(v, ".")
	if i == -1 {
		return target{Struct: v}
	}
	return target{Package: v[:i], Struct: v[i+1:]}
}

type fieldConfig struct {
	SourceName string
	SourceType ast.Expr
	TargetName string
	FuncFrom   string
	FuncTo     string
	// TODO: Pointer pointerSettings
}

func configsFromAnnotations(sources sourcePkg) ([]structConfig, error) {
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
			c.Target = newTarget(value)
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

	if len(field.Names) == 0 {
		return c, fmt.Errorf("no field name for type %v", field.Type)
	}
	c.SourceName = field.Names[0].Name
	c.SourceType = field.Type

	text := getFieldAnnotationLine(field.Doc)
	if text == "" {
		return c, nil
	}

	for _, part := range strings.Fields(text) {
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			return c, fmt.Errorf("invalid term '%v' in annotation, expected only one =", part)
		}
		value := kv[1]
		switch kv[0] {
		case "target":
			c.TargetName = value
		case "pointer":
			// TODO:
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

func getFieldAnnotationLine(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}

	prefix := "mog: "
	for _, line := range doc.List {
		text := strings.TrimSpace(strings.TrimLeft(line.Text, "/"))
		if strings.HasPrefix(text, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}
	return ""
}
