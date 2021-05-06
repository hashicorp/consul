package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/rboyer/safeio"
)

func generateFiles(cfg config, targets map[string]targetPkg) error {
	byOutput := configsByOutput(cfg.Structs)

	for _, group := range byOutput {
		var decls []ast.Decl
		imports := newImports()

		for _, sourceStruct := range group {
			t := targets[sourceStruct.Target.Package].Structs[sourceStruct.Target.Struct]
			if t.Name == "" {
				return fmt.Errorf("failed to locate target %v for %v", sourceStruct.Target, sourceStruct.Source)
			}

			gen, err := generateConversion(sourceStruct, t, imports)
			if err != nil {
				return fmt.Errorf("failed to generate conversion for %v: %w", sourceStruct.Source, err)
			}
			decls = append(decls, gen.Decls()...)

			// TODO: generate round trip testcase
		}

		fset := &token.FileSet{}
		file := &ast.File{Name: &ast.Ident{Name: cfg.SourcePkg.Name}}
		output := filepath.Join(cfg.SourcePkg.Path, group[0].Output)

		// Add all imports as the first declaration
		// TODO: dedupe imports, handle conflicts
		file.Decls = append([]ast.Decl{imports.Decl()}, decls...)

		if err := astWriteToFile(output, fset, file); err != nil {
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

	var errs []error

	// TODO: would it make sense to store the fields as a map instead of building it here?
	sourceFields := sourceFieldMap(cfg.Fields)
	for _, field := range t.Fields {
		name := field.Name()

		// TODO: test case to include ignored field
		if _, contains := cfg.IgnoreFields[name]; contains {
			continue
		}

		sourceField, ok := sourceFields[name]
		if !ok {
			msg := "struct %v is missing field %v. Add the missing field or exclude it using ignore-fields."
			errs = append(errs, fmt.Errorf(msg, cfg.Source, name))
			continue
		}

		sourcePtr := sourceField.SourcePtr
		_, targetPtr := field.Type().(*types.Pointer)

		srcExpr := &ast.SelectorExpr{
			X:   &ast.Ident{Name: varNameSource},
			Sel: &ast.Ident{Name: sourceField.SourceName},
		}
		targetExpr := &ast.SelectorExpr{
			X:   &ast.Ident{Name: varNameTarget},
			Sel: &ast.Ident{Name: name},
		}

		to.Body.List = append(to.Body.List,
			newAssignStmt(targetExpr, srcExpr,
				sourceField.DynFuncTo(sourcePtr, targetPtr)))

		from.Body.List = append(from.Body.List,
			newAssignStmt(srcExpr, targetExpr,
				sourceField.DynFuncFrom(sourcePtr, targetPtr)))
	}

	returnStmt := &ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: varNameTarget}}}
	to.Body.List = append(to.Body.List, returnStmt)

	returnStmt = &ast.ReturnStmt{Results: []ast.Expr{&ast.Ident{Name: varNameSource}}}
	from.Body.List = append(from.Body.List, returnStmt)

	g.To = to
	g.PtrToPtr = generatePtrToPtrFunc(cfg, imports)
	g.ToPtr = generateStructToPtrFunc(cfg, imports)
	g.PtrTo = generatePtrToStructFunc(cfg, imports)

	g.From = from
	g.PtrFromPtr = generatePtrFromPtrFunc(cfg, imports)
	g.PtrFrom = generatePtrFromStructFunc(cfg, imports)
	g.FromPtr = generateStructFromPtrFunc(cfg, imports)

	return g, fmtErrors("failed to generate", errs)
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
	To                     *ast.FuncDecl
	PtrTo, ToPtr, PtrToPtr *ast.FuncDecl

	From                         *ast.FuncDecl
	PtrFrom, FromPtr, PtrFromPtr *ast.FuncDecl

	// TODO: RoundTripTest *ast.FuncDecl
}

func (g *generated) Decls() []ast.Decl {
	return []ast.Decl{
		g.To,
		g.PtrTo,
		g.ToPtr,
		g.PtrToPtr,
		g.From,
		g.PtrFrom,
		g.FromPtr,
		g.PtrFromPtr,
	}
}

func generateToFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: path.Base(imports.AliasFor(cfg.Target.Package))},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameTo(cfg, false, false)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a struct-to-struct conversion that translates FROM a protobuf"),
		Name: &ast.Ident{Name: funcName},
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

func generatePtrToPtrFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: path.Base(imports.AliasFor(cfg.Target.Package))},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameTo(cfg, true, true)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a pointer-to-pointer conversion that translates FROM a protobuf"),
		Name: &ast.Ident{Name: funcName},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameSource}},
					Type:  newPointerTo(cfg.Source),
				}},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: &ast.StarExpr{
					X: targetType,
				}}},
			},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			newIfNilReturnIdent(varNameSource, "nil"),
			&ast.AssignStmt{
				Lhs: []ast.Expr{&ast.Ident{Name: varNameTarget}},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{
					&ast.CallExpr{
						Fun:  &ast.Ident{Name: funcNameTo(cfg, false, false)},
						Args: []ast.Expr{newPointerTo(varNameSource)},
					},
				},
			},
			&ast.ReturnStmt{
				Results: []ast.Expr{newAddressOf(varNameTarget)},
			},
		}},
	}
}

func generateStructToPtrFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: path.Base(imports.AliasFor(cfg.Target.Package))},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameTo(cfg, false, true)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a struct-to-pointer conversion that translates FROM a protobuf"),
		Name: &ast.Ident{Name: funcName},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameSource}},
					Type:  &ast.Ident{Name: cfg.Source},
				}},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: &ast.StarExpr{
					X: targetType,
				}}},
			},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.AssignStmt{
				Lhs: []ast.Expr{&ast.Ident{Name: varNameTarget}},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{
					&ast.CallExpr{
						Fun:  &ast.Ident{Name: funcNameTo(cfg, false, false)},
						Args: []ast.Expr{&ast.Ident{Name: varNameSource}},
					},
				},
			},
			&ast.ReturnStmt{
				Results: []ast.Expr{newAddressOf(varNameTarget)},
			},
		}},
	}
}

func generatePtrToStructFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: path.Base(imports.AliasFor(cfg.Target.Package))},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameTo(cfg, true, false)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a pointer-to-struct conversion that translates FROM a protobuf"),
		Name: &ast.Ident{Name: funcName},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameSource}},
					Type:  newPointerTo(cfg.Source),
				}},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: targetType}},
			},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.DeclStmt{Decl: &ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{{Name: varNameTarget}},
						Type:  targetType,
					},
				},
			}},
			newIfNilReturnIdent(varNameSource, varNameTarget),
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.CallExpr{
						Fun:  &ast.Ident{Name: funcNameTo(cfg, false, false)},
						Args: []ast.Expr{newPointerTo(varNameSource)},
					},
				},
			},
		}},
	}
}

func funcNameTo(cfg structConfig, sourcePtr, targetPtr bool) string {
	if cfg.FuncNameFragment == "" {
		panic("FuncNameFragment is required")
	}
	return cfg.Source + maybePtr(sourcePtr) + "To" + cfg.FuncNameFragment + maybePtr(targetPtr)
}

func funcNameFrom(cfg structConfig, sourcePtr, targetPtr bool) string {
	if cfg.FuncNameFragment == "" {
		panic("FuncNameFragment is required")
	}
	return "New" + cfg.Source + maybePtr(sourcePtr) + "From" + cfg.FuncNameFragment + maybePtr(targetPtr)
}

func maybePtr(v bool) string {
	if v {
		return "Ptr"
	}
	return ""
}

func generateFromFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: imports.AliasFor(cfg.Target.Package)},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameFrom(cfg, false, false)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a struct-to-struct conversion that translates TO a protobuf"),
		Name: &ast.Ident{Name: funcName},
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

func generatePtrFromPtrFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: imports.AliasFor(cfg.Target.Package)},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameFrom(cfg, true, true)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a pointer-to-pointer conversion that translates TO a protobuf"),
		Name: &ast.Ident{Name: funcName},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameTarget}},
					Type: &ast.StarExpr{
						X: targetType,
					},
				}},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: newPointerTo(cfg.Source)}},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				newIfNilReturnIdent(varNameTarget, "nil"),
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: varNameSource}},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun:  &ast.Ident{Name: funcNameFrom(cfg, false, false)},
							Args: []ast.Expr{newPointerTo(varNameTarget)},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{newAddressOf(varNameSource)},
				},
			},
		},
	}
}

func generatePtrFromStructFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: imports.AliasFor(cfg.Target.Package)},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameFrom(cfg, false, true)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a pointer-to-struct conversion that translates TO a protobuf"),
		Name: &ast.Ident{Name: funcName},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameTarget}},
					Type: &ast.StarExpr{
						X: targetType,
					},
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
				newIfNilReturnIdent(varNameTarget, varNameSource),
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CallExpr{
							Fun:  &ast.Ident{Name: funcNameFrom(cfg, false, false)},
							Args: []ast.Expr{newPointerTo(varNameTarget)},
						},
					},
				},
			},
		},
	}
}

func generateStructFromPtrFunc(cfg structConfig, imports *imports) *ast.FuncDecl {
	targetType := &ast.SelectorExpr{
		X:   &ast.Ident{Name: imports.AliasFor(cfg.Target.Package)},
		Sel: &ast.Ident{Name: cfg.Target.Struct},
	}

	funcName := funcNameFrom(cfg, true, false)

	return &ast.FuncDecl{
		Doc:  newComments(funcName + " is a struct-pointer conversion that translates TO a protobuf"),
		Name: &ast.Ident{Name: funcName},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: varNameTarget}},
					Type:  targetType,
				}},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: newPointerTo(cfg.Source)}},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: varNameSource}},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun:  &ast.Ident{Name: funcNameFrom(cfg, false, false)},
							Args: []ast.Expr{&ast.Ident{Name: varNameTarget}},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{newAddressOf(varNameSource)},
				},
			},
		},
	}
}

func astWriteToFile(path string, fset *token.FileSet, file *ast.File) error {
	out, err := astToBytes(fset, file)
	if err != nil {
		return err
	}

	return writeFile(path, out)
}

func astToBytes(fset *token.FileSet, file *ast.File) ([]byte, error) {
	// Pretty print the AST node first.
	printConfig := &printer.Config{Mode: printer.TabIndent}
	var buf bytes.Buffer
	if err := printConfig.Fprint(&buf, fset, file); err != nil {
		return nil, err
	}
	out := buf.Bytes()

	// Now take a trip through "gofmt"
	return format.Source(out)
}

// TODO: write build tags
func writeFile(output string, contents []byte) error {
	fh, err := safeio.OpenFile(output, 0666)
	if err != nil {
		return err
	}
	defer fh.Close()

	if _, err := fmt.Fprint(fh, "// Code generated by mog. DO NOT EDIT.\n\n"); err != nil {
		return err
	}
	if _, err := fh.Write(contents); err != nil {
		return err
	}

	return fh.Commit()
}

type imports struct {
	byPkgPath map[string]string   // package => alias(or default)
	byAlias   map[string]string   // alias(or default) => package
	hasAlias  map[string]struct{} // package is using a non-default name
}

func newImports() *imports {
	return &imports{
		byPkgPath: make(map[string]string),
		byAlias:   make(map[string]string),
		hasAlias:  make(map[string]struct{}),
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

	hasAlias := false
	if alias == "" {
		alias = path.Base(pkgPath)
	} else {
		hasAlias = true
	}

	_, exists := i.byAlias[alias]
	for n := 2; exists; n++ {
		alias = alias + strconv.Itoa(n)
		_, exists = i.byAlias[alias]
	}

	i.byPkgPath[pkgPath] = alias
	i.byAlias[alias] = pkgPath
	if hasAlias {
		i.hasAlias[pkgPath] = struct{}{}
	} else {
		delete(i.hasAlias, pkgPath)
	}
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
			Path: &ast.BasicLit{Value: strconv.Quote(pkgPath), Kind: token.STRING},
		}

		if _, ok := i.hasAlias[pkgPath]; ok {
			imprt.Name = &ast.Ident{Name: i.byPkgPath[pkgPath]}
		}

		decl.Specs = append(decl.Specs, imprt)
	}
	return decl
}
