package main

import (
	"flag"
	"fmt"
	"go/types"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
}

func run(args []string) error {
	flags, opts := setupFlags(args[0])
	err := flags.Parse(os.Args[1:])
	switch {
	case err == flag.ErrHelp:
		flags.Usage()
		return nil
	case err != nil:
		return err
	}
	return runMog(*opts)
}

type options struct {
	source string
}

func setupFlags(name string) (*flag.FlagSet, *options) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	opts := &options{}

	flags.StringVar(&opts.source, "source", ".", "package path for source structs")
	return flags, opts
}

func runMog(opts options) error {
	_, err := loadStructs(opts.source, sourceStructs)
	if err != nil {
		return fmt.Errorf("failed to load source from %s: %w", opts.source, err)
	}

	// TODO: compile the list of target packages from the annotations
	// TODO: load target structs
	// TODO: generate conversion functions and tests
	// TODO: write files

	return nil
}

type pkg struct {
	structNames []string
	structs     map[string]*types.Struct
	// TODO: buildTags string
}

func loadStructs(path string, filter func(p types.Object) bool) (pkg, error) {
	p := pkg{}
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedTypesInfo}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		return p, err
	}
	for _, pkg := range pkgs {
		if err := packageLoadErrors(pkg); err != nil {
			return p, err
		}

		for ident, obj := range pkg.TypesInfo.Defs {
			if !filter(obj) {
				continue
			}

			p.structNames = append(p.structNames, ident.Name)
			p.structs[ident.Name] = obj.Type().(*types.Struct) // FIXME
		}
	}

	return p, nil
}

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

func sourceStructs(o types.Object) bool {
	return false
}
