package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strings"
)

type config struct {
	// TODO: maybe move Path, PkgPath, BuildTags, etc onto a separate struct
	// so that sourcePkg.Structs is not passed around.
	SourcePkg sourcePkg
	Structs   []structConfig
}

type structConfig struct {
	// Source struct name.
	Source           string
	Target           target
	Output           string
	FuncNameFragment string
	IgnoreFields     stringSet
	FuncFrom         string
	FuncTo           string
	Fields           []fieldConfig
}

type stringSet map[string]struct{}

func newStringSetFromSlice(s []string) stringSet {
	ss := make(stringSet, len(s))
	for _, i := range s {
		ss[i] = struct{}{}
	}
	return ss
}

type target struct {
	Package string
	Struct  string
}

func (t target) String() string {
	return t.Package + "." + t.Struct
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

func configsFromAnnotations(pkg sourcePkg) (config, error) {
	names := pkg.StructNames()
	c := config{Structs: make([]structConfig, 0, len(names))}
	c.SourcePkg = pkg

	for _, name := range names {
		strct := pkg.Structs[name]
		cfg, err := parseStructAnnotation(name, strct.Doc)
		if err != nil {
			return c, fmt.Errorf("from source struct %v: %w", name, err)
		}

		for _, field := range strct.Fields {
			f, err := parseFieldAnnotation(field)
			if err != nil {
				return c, fmt.Errorf("from source struct %v: %w", name, err)
			}
			cfg.Fields = append(cfg.Fields, f)
		}

		// TODO: test case
		if err := cfg.Validate(); err != nil {
			return c, fmt.Errorf("invalid config for %v: %w", name, err)
		}

		c.Structs = append(c.Structs, cfg)
	}

	return c, nil
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
			c.IgnoreFields = newStringSetFromSlice(strings.Split(value, ","))
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

func (c structConfig) Validate() error {
	var errs []error
	fmsg := "missing value for required annotation %q"
	if c.Target.Struct == "" {
		errs = append(errs, fmt.Errorf(fmsg, "target"))
	}
	if c.Output == "" {
		errs = append(errs, fmt.Errorf(fmsg, "output"))
	}
	if c.FuncNameFragment == "" {
		errs = append(errs, fmt.Errorf(fmsg, "name"))
	}
	return fmtErrors("invalid annotations", errs)
}

func parseFieldAnnotation(field *ast.Field) (fieldConfig, error) {
	var c fieldConfig

	name, err := fieldName(field)
	if err != nil {
		return c, err
	}

	c.SourceName = name
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

// TODO test cases for embedded types
func fieldName(field *ast.Field) (string, error) {
	if len(field.Names) > 0 {
		return field.Names[0].Name, nil
	}

	switch n := field.Type.(type) {
	case *ast.Ident:
		return n.Name, nil
	case *ast.SelectorExpr:
		return n.Sel.Name, nil
	}

	buf := new(bytes.Buffer)
	_ = format.Node(buf, new(token.FileSet), field.Type)
	return "", fmt.Errorf("failed to determine field name for type %v", buf.String())
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

func fmtErrors(msg string, errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		b := new(strings.Builder)

		for _, err := range errs {
			b.WriteString("\n   ")
			b.WriteString(err.Error())
		}
		return fmt.Errorf(msg+":%s\n", b.String())
	}
}

// TODO: test cases
func applyAutoConvertFunctions(cfgs []structConfig) []structConfig {
	byName := make(map[string]structConfig, len(cfgs))
	for _, s := range cfgs {
		byName[s.Source] = s
	}

	for structIdx, s := range cfgs {
		for fieldIdx, f := range s.Fields {
			if _, ignored := s.IgnoreFields[f.SourceName]; ignored {
				continue
			}

			if f.FuncTo != "" || f.FuncFrom != "" {
				continue
			}

			ident, ok := f.SourceType.(*ast.Ident)
			if !ok {
				continue
			}

			structCfg, ok := byName[ident.Name]
			if !ok {
				// TODO: log warning that auto convert did not work
				continue
			}

			f.FuncFrom = funcNameFrom(structCfg)
			f.FuncTo = funcNameTo(structCfg)
			s.Fields[fieldIdx] = f
		}
		cfgs[structIdx] = s
	}
	return cfgs
}
