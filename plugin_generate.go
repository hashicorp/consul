//+build ignore

package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"

	"golang.org/x/tools/go/ast/astutil"
)

func GenerateFile(fset *token.FileSet, file *ast.File) ([]byte, error) {
	var output []byte
	buffer := bytes.NewBuffer(output)
	if err := printer.Fprint(buffer, fset, file); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func main() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, caddyrun, nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("failed to parse %s: %s", caddyrun, err)
	}
	astutil.AddNamedImport(fset, f, "_", coredns)
	astutil.DeleteNamedImport(fset, f, "_", caddy)

	out, err := GenerateFile(fset, f)
	if err := ioutil.WriteFile(caddyrun, out, 0644); err != nil {
		log.Fatalf("failed to write go file: %s", err)
	}
}

const (
	coredns = "github.com/miekg/coredns/core"
	caddy   = "github.com/mholt/caddy/caddyhttp"

	// If everything is OK and we are sitting in CoreDNS' dir, this is where run.go should be.
	caddyrun = "../../mholt/caddy/caddy/caddymain/run.go"
)
