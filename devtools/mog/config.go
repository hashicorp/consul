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
	SourcePkg sourcePkg
	Structs   []structConfig
}

type structConfig struct {
	// Source struct name.
	Source           string
	Target           target
	Output           string
	FuncNameFragment string // general namespace for conversion functions
	IgnoreFields     stringSet
	FuncFrom         string
	FuncTo           string
	Fields           []fieldConfig
}

func (c *structConfig) ConvertFuncName(direction Direction) string {
	if c.FuncNameFragment == "" {
		panic("FuncNameFragment is required")
	}
	if direction == DirTo {
		return c.Source + "To" + c.FuncNameFragment
	}
	return c.Source + "From" + c.FuncNameFragment
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
	SourceExpr ast.Expr
	SourcePtr  bool // SourcePtr indicates if the source is a pointer type
	SourceType ast.Expr
	TargetName string
	FuncFrom   string
	FuncTo     string

	ConvertFuncFrom string
	ConvertFuncTo   string
}

type Direction string

func (d Direction) String() string { return string(d) }

const (
	DirFrom Direction = "From"
	DirTo             = "To"
)

func (c fieldConfig) UserFuncName(direction Direction) string {
	if direction == DirFrom {
		return c.FuncFrom
	}
	return c.FuncTo
}

// ConvertFuncName returns the name of a function that takes 2 pointers and
// returns nothing.
func (c fieldConfig) ConvertFuncName(direction Direction) string {
	if c.UserFuncName(direction) != "" {
		return ""
	}
	if direction == DirTo {
		return c.ConvertFuncTo
	}
	return c.ConvertFuncFrom
}

// configsFromAnnotations will examine the loaded structs from the given
// package and interpret both the mog annotations and the types of the fields.
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

		// Extract some pointer-ness info about the source.
		typesInfo := pkg.pkg.TypesInfo
		for i, sourceField := range cfg.Fields {
			o := typesInfo.Types[sourceField.SourceExpr].Type
			sourceField.SourceType, sourceField.SourcePtr = astTypeFromTypesType(nil, o, true)
			// TODO (fails on stuff like maps)
			// if sourceField.SourceType == nil {
			// 	return c, fmt.Errorf("source struct %v field %v is not a basic/named type nor a pointer to a basic/named type: %T",
			// 		name, sourceField.SourceName, o)
			// }

			// TODO: this could use some improvement
			sourceField.SourceType = stripCurrentPackagePrefix(sourceField.SourceType, c.SourcePkg.Name)

			cfg.Fields[i] = sourceField
		}

		c.Structs = append(c.Structs, cfg)
	}

	return c, nil
}

// TODO: syntax of mog annotations should be in readme
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

// TODO: syntax of mog annotations should be in readme
func parseFieldAnnotation(field *ast.Field) (fieldConfig, error) {
	var c fieldConfig

	name, err := fieldName(field)
	if err != nil {
		return c, err
	}

	c.SourceName = name
	c.SourceExpr = field.Type

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
			// TODO(rb): remove as unnecessary?
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
		return fmt.Errorf(msg+": %w", errs[0])
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
	// Index the structs by name so any struct can refer to conversion
	// functions for any other struct.
	byName := make(map[string]structConfig, len(cfgs))
	for _, s := range cfgs {
		byName[s.Source] = s
	}

	for structIdx, s := range cfgs {
		for fieldIdx, f := range s.Fields {
			if _, ignored := s.IgnoreFields[f.SourceName]; ignored {
				continue
			}

			// User supplied override function.
			if f.FuncTo != "" || f.FuncFrom != "" {
				continue
			}

			var (
				ident *ast.Ident
			)
			switch x := f.SourceExpr.(type) {
			case *ast.Ident:
				ident = x
			case *ast.StarExpr:
				var ok bool
				ident, ok = x.X.(*ast.Ident)
				if !ok {
					continue
				}
			default:
				continue
			}

			// Pull up type information for type of this field and attempt
			// auto-convert.
			//
			// Maybe explicitly skip primitives or stuff like strings?
			structCfg, ok := byName[ident.Name]
			if !ok {
				// TODO: log warning that auto convert did not work
				continue
			}

			// Capture this information so we can use it to know how to call
			// the conversion functions later.
			f.ConvertFuncFrom = structCfg.ConvertFuncName(DirFrom)
			f.ConvertFuncTo = structCfg.ConvertFuncName(DirTo)

			s.Fields[fieldIdx] = f
		}
		cfgs[structIdx] = s
	}
	return cfgs
}
