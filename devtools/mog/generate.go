package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
)

func generateFiles(cfg config, targets map[string]targetPkg) error {
	byOutput := configsByOutput(cfg.Structs)

	for _, group := range byOutput {
		var decls []ast.Decl
		imports := newImports()

		for _, cfg := range group {
			t := targets[cfg.Target.Package].Structs[cfg.Target.Struct]
			if t.Name == "" {
				return fmt.Errorf("failed to locate target %v for %v", cfg.Target, cfg.Source)
			}

			gen, err := generateConversion(cfg, t, imports)
			if err != nil {
				return fmt.Errorf("failed to generate conversion for %v: %w", cfg.Source, err)
			}
			decls = append(decls, gen.To, gen.From)

			// TODO: generate round trip testcase
		}

		output := filepath.Join(cfg.SourcePkg.Path, group[0].Output)
		file := &ast.File{Name: &ast.Ident{Name: cfg.SourcePkg.Name}}

		// Add all imports as the first declaration
		// TODO: dedupe imports, handle conflicts
		file.Decls = append([]ast.Decl{imports.Decl()}, decls...)

		if err := writeFile(output, file); err != nil {
			return fmt.Errorf("failed to write generated code to %v: %w", output, err)
		}
	}
	return nil
}

// configsByOutput sorts and groups the configs by the Output filename. Each
// group is sorted by name of struct.
func configsByOutput(cfgs []structConfig) [][]structConfig {
	if len(cfgs) == 0 {
		return nil
	}

	var result [][]structConfig
	sort.Slice(cfgs, func(i, j int) bool {
		if cfgs[i].Output == cfgs[j].Output {
			return cfgs[i].Source < cfgs[j].Source
		}
		return cfgs[i].Output < cfgs[j].Output
	})

	var group []structConfig
	output := cfgs[0].Output
	for _, c := range cfgs {
		if c.Output != output {
			result = append(result, group)
			group = []structConfig{}
			output = c.Output
		}

		group = append(group, c)
	}
	return append(result, group)
}

var (
	varNameSource = "s"
	varNameTarget = "t"
)

func generateConversion(cfg structConfig, t targetStruct, imports *imports) (generated, error) {
	var g generated

	imports.Add("", cfg.Target.Package)
	to := generateToFunc(cfg, imports)
	from := generateFromFunc(cfg, imports)

	// TODO: would it make sense to store the fields as a map instead of building it here?
	sourceFields := sourceFieldMap(cfg.Fields)
	for _, field := range t.Fields {
		name := field.Name()

		// TODO: test case to include ignored field
		if _, contains := cfg.IgnoreFields[name]; contains {
			continue
		}

		sourceField := sourceFields[name]
		// TODO: store error for missing sourceField, and return error at the end

		// TODO: handle pointer

		srcExpr := &ast.SelectorExpr{
			X:   &ast.Ident{Name: varNameSource},
			Sel: &ast.Ident{Name: sourceField.SourceName},
		}
		targetExpr := &ast.SelectorExpr{
			X:   &ast.Ident{Name: varNameTarget},
			Sel: &ast.Ident{Name: name},
		}

		to.Body.List = append(to.Body.List,
			newAssignStmt(targetExpr, srcExpr, sourceField.FuncTo))

		from.Body.List = append(from.Body.List,
			newAssignStmt(srcExpr, targetExpr, sourceField.FuncFrom))
	}

	returnStmt := &ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: varNameTarget}}}
	to.Body.List = append(to.Body.List, returnStmt)

	returnStmt = &ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: varNameSource}}}
	from.Body.List = append(from.Body.List, returnStmt)

	g.To = to
	g.From = from

	return g, nil
}

// TODO: test case with funcFrom/FuncTo
func newAssignStmt(left ast.Expr, right ast.Expr, funcName string) *ast.AssignStmt {
	if funcName != "" {
		right = &ast.CallExpr{
			Fun:  &ast.Ident{Name: funcName},
			Args: []ast.Expr{right},
		}
	}

	return &ast.AssignStmt{
		Lhs: []ast.Expr{left},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{right},
	}
}

func sourceFieldMap(fields []fieldConfig) map[string]fieldConfig {
	result := make(map[string]fieldConfig, len(fields))
	for _, field := range fields {
		key := field.SourceName
		if field.TargetName != "" {
			key = field.TargetName
		}
		result[key] = field
	}
	return result
}

type generated struct {
	To   *ast.FuncDecl
	From *ast.FuncDecl

	// TODO: RoundTripTest *ast.FuncDecl
}

func generateToFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: path.Base(imports.AliasFor(cfg.Target.Package))},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	return &ast.FuncDecl{
		Name: &ast.Ident{Name: funcNameTo(cfg)},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameSource}},
					Type:  &ast.Ident{Name: cfg.Source},
				}},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: targetType}},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.DeclStmt{Decl: &ast.GenDecl{
					Tok: token.VAR,
					Specs: []ast.Spec{
						&ast.ValueSpec{
							Names: []*ast.Ident{{Name: varNameTarget}},
							Type:  targetType,
						},
					},
				}},
			},
		},
	}
}

func funcNameTo(cfg structConfig) string {
	return cfg.Source + "To" + cfg.FuncNameFragment
}

func funcNameFrom(cfg structConfig) string {
	return "New" + cfg.Source + "From" + cfg.FuncNameFragment
}

func generateFromFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: imports.AliasFor(cfg.Target.Package)},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	return &ast.FuncDecl{
		Name: &ast.Ident{Name: funcNameFrom(cfg)},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameTarget}},
					Type:  targetType,
				}},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: &ast.Ident{Name: cfg.Source}}},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.DeclStmt{Decl: &ast.GenDecl{
					Tok: token.VAR,
					Specs: []ast.Spec{
						&ast.ValueSpec{
							Names: []*ast.Ident{{Name: varNameSource}},
							Type:  &ast.Ident{Name: cfg.Source},
						},
					},
				}},
			},
		},
	}
}

// TODO: write build tags
// TODO: write file header
func writeFile(output string, file *ast.File) error {
	fh, err := os.Create(output)
	if err != nil {
		return err
	}
	return format.Node(fh, new(token.FileSet), file)
}

type imports struct {
	byPkgPath map[string]string
	byAlias   map[string]string
}

func newImports() *imports {
	return &imports{
		byPkgPath: make(map[string]string),
		byAlias:   make(map[string]string),
	}
}

// Add an import with an optional alias. If no alias is specified, the default
// alias will be path.Base(). The alias for a package should always be looked up
// from AliasFor.
//
// TODO: remove alias arg?
func (i *imports) Add(alias string, pkgPath string) {
	if _, exists := i.byPkgPath[pkgPath]; exists {
		return
	}

	if alias == "" {
		alias = path.Base(pkgPath)
	}

	_, exists := i.byAlias[alias]
	for n := 2; exists; n++ {
		alias = alias + strconv.Itoa(n)
		_, exists = i.byAlias[alias]
	}

	i.byPkgPath[pkgPath] = alias
	i.byAlias[alias] = pkgPath
}

func (i *imports) AliasFor(pkgPath string) string {
	return i.byPkgPath[pkgPath]
}

func (i *imports) Decl() *ast.GenDecl {
	decl := &ast.GenDecl{Tok: token.IMPORT}

	paths := make([]string, 0, len(i.byPkgPath))
	for pkgPath := range i.byPkgPath {
		paths = append(paths, pkgPath)
	}
	sort.Strings(paths)

	for _, pkgPath := range paths {
		imprt := &ast.ImportSpec{
			Name: &ast.Ident{Name: i.byPkgPath[pkgPath]},
			Path: &ast.BasicLit{Value: quote(pkgPath), Kind: token.STRING},
		}
		decl.Specs = append(decl.Specs, imprt)
	}
	return decl
}

func quote(v string) string {
	return `"` + v + `"`
}
