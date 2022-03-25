package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	flagPath = flag.String("path", "", "path of file to load")
	verbose  = flag.Bool("v", false, "verbose output")
)

const (
	annotationPrefix = "@consul-rpc-glue:"
	outputFileSuffix = ".rpcglue.pb.go"
)

func main() {
	flag.Parse()

	log.SetFlags(0)

	if *flagPath == "" {
		log.Fatal("missing required -path argument")
	}

	if err := run(*flagPath); err != nil {
		log.Fatal(err)
	}
}

func run(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		return fmt.Errorf("argument must be a file: %s", path)
	}

	if !strings.HasSuffix(path, ".pb.go") {
		return fmt.Errorf("file must end with .pb.go: %s", path)
	}

	if err := processFile(path); err != nil {
		return fmt.Errorf("error processing file %q: %v", path, err)
	}

	return nil
}

func processFile(path string) error {
	if *verbose {
		log.Printf("visiting file %q", path)
	}

	fset := token.NewFileSet()
	tree, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	v := visitor{}
	ast.Walk(&v, tree)
	if err := v.Err(); err != nil {
		return err
	}

	if len(v.Types) == 0 {
		return nil
	}

	if *verbose {
		log.Printf("Package: %s", v.Package)
		log.Printf("BuildTags: %v", v.BuildTags)
		log.Println()
		for _, typ := range v.Types {
			log.Printf("Type: %s", typ.Name)
			ann := typ.Annotation
			if ann.ReadRequest != "" {
				log.Printf("    ReadRequest from %s", ann.ReadRequest)
			}
			if ann.WriteRequest != "" {
				log.Printf("    WriteRequest from %s", ann.WriteRequest)
			}
			if ann.TargetDatacenter != "" {
				log.Printf("    TargetDatacenter from %s", ann.TargetDatacenter)
			}
		}
	}

	// generate output

	var buf bytes.Buffer

	if len(v.BuildTags) > 0 {
		for _, line := range v.BuildTags {
			buf.WriteString(line + "\n")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("// Code generated by proto-gen-rpc-glue. DO NOT EDIT.\n\n")
	buf.WriteString("package " + v.Package + "\n")
	buf.WriteString(`
import (
	"time"
)

`)
	for _, typ := range v.Types {
		if typ.Annotation.WriteRequest != "" {
			buf.WriteString(fmt.Sprintf(`
func (msg *%[1]s) AllowStaleRead() bool {
	return false
}

func (msg *%[1]s) HasTimedOut(start time.Time, rpcHoldTimeout time.Duration, a time.Duration, b time.Duration) (bool, error) {
	if msg == nil || msg.%[2]s == nil {
		return false, nil
	}
	return msg.%[2]s.HasTimedOut(start, rpcHoldTimeout, a, b)
}

func (msg *%[1]s) IsRead() bool {
	return false
}

func (msg *%[1]s) SetTokenSecret(s string) {
	msg.%[2]s.SetTokenSecret(s)
}

func (msg *%[1]s) TokenSecret() string {
	if msg == nil || msg.%[2]s == nil {
		return ""
	}
	return msg.%[2]s.TokenSecret()
}

func (msg *%[1]s) Token() string {
	if msg.%[2]s == nil {
		return ""
	}
	return msg.%[2]s.Token
}
`, typ.Name, typ.Annotation.WriteRequest))
		}
		if typ.Annotation.ReadRequest != "" {
			buf.WriteString(fmt.Sprintf(`
func (msg *%[1]s) IsRead() bool {
	return true
}

func (msg *%[1]s) AllowStaleRead() bool {
	return msg.%[2]s.AllowStaleRead()
}

func (msg *%[1]s) HasTimedOut(start time.Time, rpcHoldTimeout time.Duration, a time.Duration, b time.Duration) (bool, error) {
	if msg == nil || msg.%[2]s == nil {
		return false, nil
	}
	return msg.%[2]s.HasTimedOut(start, rpcHoldTimeout, a, b)
}

func (msg *%[1]s) SetTokenSecret(s string) {
	msg.%[2]s.SetTokenSecret(s)
}

func (msg *%[1]s) TokenSecret() string {
	if msg == nil || msg.%[2]s == nil {
		return ""
	}
	return msg.%[2]s.TokenSecret()
}

func (msg *%[1]s) Token() string {
	if msg.%[2]s == nil {
		return ""
	}
	return msg.%[2]s.Token
}
`, typ.Name, typ.Annotation.ReadRequest))
		}
		if typ.Annotation.TargetDatacenter != "" {
			buf.WriteString(fmt.Sprintf(`
func (msg *%[1]s) RequestDatacenter() string {
	if msg == nil || msg.%[2]s == nil {
		return ""
	}
	return msg.%[2]s.GetDatacenter()
}
`, typ.Name, typ.Annotation.TargetDatacenter))
		}
	}

	// write to disk
	outFile := strings.TrimSuffix(path, ".pb.go") + outputFileSuffix
	if err := os.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
		return err
	}

	// clean up
	cmd := exec.Command("gofmt", "-s", "-w", outFile)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running 'gofmt -s -w %q': %v", outFile, err)
	}

	return nil
}

type TypeInfo struct {
	Name       string
	Annotation Annotation
}

type visitor struct {
	Package   string
	BuildTags []string
	Types     []TypeInfo
	Errs      []error
}

func (v *visitor) Err() error {
	switch len(v.Errs) {
	case 0:
		return nil
	case 1:
		return v.Errs[0]
	default:
		//
		var s []string
		for _, e := range v.Errs {
			s = append(s, e.Error())
		}
		return errors.New(strings.Join(s, "; "))
	}
}

var _ ast.Visitor = (*visitor)(nil)

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return v
	}

	switch x := node.(type) {
	case *ast.File:
		v.Package = x.Name.Name
		v.BuildTags = getRawBuildTags(x)
		for _, d := range x.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}

			if gd.Doc == nil {
				continue
			} else if len(gd.Specs) != 1 {
				continue
			}
			spec := gd.Specs[0]

			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			ann, err := getAnnotation(gd.Doc.List)
			if err != nil {
				v.Errs = append(v.Errs, err)
				continue
			} else if ann.IsZero() {
				continue
			}

			v.Types = append(v.Types, TypeInfo{
				Name:       typeSpec.Name.Name,
				Annotation: ann,
			})

		}
	}
	return v
}

type Annotation struct {
	ReadRequest      string
	WriteRequest     string
	TargetDatacenter string
}

func (a Annotation) IsZero() bool {
	return a == Annotation{}
}

func getAnnotation(doc []*ast.Comment) (Annotation, error) {
	raw, ok := getRawStructAnnotation(doc)
	if !ok {
		return Annotation{}, nil
	}

	var ann Annotation

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "ReadRequest":
			ann.ReadRequest = "ReadRequest"
		case strings.HasPrefix(part, "ReadRequest"):
			ann.ReadRequest = strings.TrimPrefix(part, "ReadRequest")

		case part == "WriteRequest":
			ann.WriteRequest = "WriteRequest"
		case strings.HasPrefix(part, "WriteRequest"):
			ann.WriteRequest = strings.TrimPrefix(part, "WriteRequest")

		case part == "TargetDatacenter":
			ann.TargetDatacenter = "TargetDatacenter"
		case strings.HasPrefix(part, "TargetDatacenter"):
			ann.TargetDatacenter = strings.TrimPrefix(part, "TargetDatacenter")

		default:
			return Annotation{}, fmt.Errorf("unexpected annotation part: %s", part)
		}
	}

	return ann, nil
}

func getRawStructAnnotation(doc []*ast.Comment) (string, bool) {
	for _, line := range doc {
		text := strings.TrimSpace(strings.TrimLeft(line.Text, "/"))

		ann := strings.TrimSpace(strings.TrimPrefix(text, annotationPrefix))

		if text != ann {
			return ann, true
		}
	}
	return "", false
}

func getRawBuildTags(file *ast.File) []string {
	// build tags are always the first group, at the very top
	if len(file.Comments) == 0 {
		return nil
	}
	cg := file.Comments[0]

	var out []string
	for _, line := range cg.List {
		text := strings.TrimSpace(strings.TrimLeft(line.Text, "/"))

		if !strings.HasPrefix(text, "go:build ") && !strings.HasPrefix(text, "+build") {
			break // stop at first non-build-tag
		}

		out = append(out, line.Text)
	}

	return out
}
