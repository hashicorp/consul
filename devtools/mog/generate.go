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
)

func generateFiles(cfg config, targets map[string]targetPkg) error {
	byOutput := configsByOutput(cfg.Structs)
	for _, group := range byOutput {
		output := filepath.Join(cfg.SourcePkg.Path, group[0].Output)
		file := newASTFile(cfg.SourcePkg.Name)

		for _, cfg := range group {
			t := targets[cfg.Target.Package].Structs[cfg.Target.Struct]
			if t.Name == "" {
				return fmt.Errorf("unable to locate target %v for %v", cfg.Target, cfg.Source)
			}

			gen, err := generateConversion(cfg, t)
			if err != nil {
				return fmt.Errorf("failed to generate conversion for %v: %w", cfg.Source, err)
			}
			file.Decls = append(file.Decls, gen.To, gen.From)

			// TODO: generate tests
		}

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

func newASTFile(pkg string) *ast.File {
	return &ast.File{
		Name: &ast.Ident{Name: pkg},
		// TODO: Imports:    nil, // what imports are needed?
	}
}

var (
	varNameSource = "s"
	varNameTarget = "t"
)

func generateConversion(cfg structConfig, t targetStruct) (generated, error) {
	var g generated

	to := generateToFunc(cfg)
	from := generateFromFunc(cfg)

	// TODO: would it make sense to store the fields as a map instead of building it here?
	sourceFields := sourceFieldMap(cfg.Fields)
	for _, field := range t.Fields {
		sourceField := sourceFields[field.Name()]
		// TODO: skip missing source fields, and record the field name for later
		// and error if the field is not in ignore-fields

		// TODO: add support for func-from, func-to
		// TODO: handle pointer

		srcExpr := []ast.Expr{&ast.SelectorExpr{
			X:   &ast.Ident{Name: varNameSource},
			Sel: &ast.Ident{Name: sourceField.SourceName},
		}}
		targetExpr := []ast.Expr{&ast.SelectorExpr{
			X:   &ast.Ident{Name: varNameTarget},
			Sel: &ast.Ident{Name: field.Name()},
		}}

		stmt := &ast.AssignStmt{
			Lhs: targetExpr,
			Tok: token.ASSIGN,
			Rhs: srcExpr,
		}
		to.Body.List = append(to.Body.List, stmt)

		stmt = &ast.AssignStmt{
			Lhs: srcExpr,
			Tok: token.ASSIGN,
			Rhs: targetExpr,
		}
		from.Body.List = append(from.Body.List, stmt)
	}

	// TODO: add func-from, func-to calls

	returnStmt := &ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: varNameTarget}}}
	to.Body.List = append(to.Body.List, returnStmt)

	returnStmt = &ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: varNameSource}}}
	from.Body.List = append(from.Body.List, returnStmt)

	g.To = to
	g.From = from

	return g, nil
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
	To            *ast.FuncDecl
	From          *ast.FuncDecl
	RoundTripTest *ast.FuncDecl
}

func generateToFunc(cfg structConfig) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		// TODO: lookup import name instead of assuming basename
		X:   &ast.Ident{Name: path.Base(cfg.Target.Package)},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	return &ast.FuncDecl{
		Name: &ast.Ident{Name: cfg.Source + "To" + cfg.FuncNameFragment},
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

func generateFromFunc(cfg structConfig) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		// TODO: lookup import name instead of assuming basename
		X:   &ast.Ident{Name: path.Base(cfg.Target.Package)},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	return &ast.FuncDecl{
		Name: &ast.Ident{Name: "New" + cfg.Source + "From" + cfg.FuncNameFragment},
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
