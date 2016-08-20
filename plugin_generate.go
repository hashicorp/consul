//+build ignore

package main

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"strconv"
)

func AddImportToFile(file, imprt string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	for _, s := range f.Imports {
		iSpec := &ast.ImportSpec{Path: &ast.BasicLit{Value: s.Path.Value}}
		if iSpec.Path.Value == strconv.Quote(imprt) {
			return nil, errors.New("coredns import already found")
		}
	}

	for i := 0; i < len(f.Decls); i++ {
		d := f.Decls[i]

		switch d.(type) {
		case *ast.FuncDecl:
			// No action
		case *ast.GenDecl:
			dd := d.(*ast.GenDecl)

			// IMPORT Declarations
			if dd.Tok == token.IMPORT {
				// Add the new import
				iSpec := &ast.ImportSpec{Name: &ast.Ident{Name: "_"}, Path: &ast.BasicLit{Value: strconv.Quote(imprt)}}
				dd.Specs = append(dd.Specs, iSpec)
				break
			}
		}
	}

	ast.SortImports(fset, f)

	out, err := GenerateFile(fset, f)
	return out, err
}

func GenerateFile(fset *token.FileSet, file *ast.File) ([]byte, error) {
	var output []byte
	buffer := bytes.NewBuffer(output)
	if err := printer.Fprint(buffer, fset, file); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

const (
	coredns = "github.com/miekg/coredns/core"
	// If everything is OK and we are sitting in CoreDNS' dir, this is where run.go should be.
	caddyrun = "../../mholt/caddy/caddy/caddymain/run.go"
)

func main() {
	out, err := AddImportToFile(caddyrun, coredns)
	if err != nil {
		log.Printf("failed to add import: %s", err)
		return
	}
	if err := ioutil.WriteFile(caddyrun, out, 0644); err != nil {
		log.Fatalf("failed to write go file: %s", err)
	}
}
