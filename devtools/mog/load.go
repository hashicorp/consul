package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

type pkg struct {
	// TODO: buildTags string
	structs map[string]structDecl
}

type structDecl struct {
	Doc    []*ast.Comment
	Fields []*ast.Field
}

func (p pkg) Names() []string {
	names := make([]string, 0, len(p.structs))
	for name := range p.structs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func loadStructs(path string, filter func(doc *ast.CommentGroup) bool) (pkg, error) {
	p := pkg{structs: map[string]structDecl{}}
	cfg := &packages.Config{Mode: modeLoadAll}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		return p, err
	}
	for _, pkg := range pkgs {
		if err := packageLoadErrors(pkg); err != nil {
			return p, err
		}

		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				genDecl := declAsTypeGenDecl(decl)
				if genDecl == nil {
					continue
				}

				for _, spec := range genDecl.Specs {
					spec := specAsExpectedTypeSpec(spec)
					if spec == nil {
						continue
					}

					// godoc may be on the GenDecl or the TypeSpec
					doc := spec.Doc
					if doc == nil {
						doc = genDecl.Doc
					}

					structType, ok := spec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					if !filter(doc) {
						continue
					}

					p.structs[spec.Name.Name] = structDecl{
						Doc:    doc.List,
						Fields: structType.Fields.List,
					}
				}
			}
		}
	}

	return p, nil
}

// TODO: trim this is All isn't needed
var modeLoadAll = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedTypesSizes

func packageLoadErrors(pkg *packages.Package) error {
	if len(pkg.Errors) == 0 {
		return nil
	}

	buf := new(strings.Builder)
	for _, err := range pkg.Errors {
		buf.WriteString("\n")
		buf.WriteString(err.Error())
	}
	return fmt.Errorf("package %s has errors: %s", pkg.PkgPath, buf.String())
}

func declAsTypeGenDecl(o ast.Decl) *ast.GenDecl {
	if o == nil {
		return nil
	}
	decl, ok := o.(*ast.GenDecl)
	if !ok {
		return nil
	}
	if decl.Tok != token.TYPE {
		return nil
	}
	return decl
}

func specAsExpectedTypeSpec(s ast.Spec) *ast.TypeSpec {
	spec, ok := s.(*ast.TypeSpec)
	if !ok {
		return nil
	}
	if !spec.Name.IsExported() {
		return nil
	}
	return spec
}

// sourceStructs scans the lines in the doc comment group and returns true if
// one of the lines contains the comment which identifies the struct as one
// that should be used for the source of type conversion.
func sourceStructs(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	return structAnnotationIndex(doc.List) != -1
}

func structAnnotationIndex(doc []*ast.Comment) int {
	for i, line := range doc {
		text := strings.TrimSpace(strings.TrimLeft(line.Text, "/"))
		if text == "mog annotation:" {
			return i
		}
	}
	return -1
}
