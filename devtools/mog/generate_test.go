package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"math/rand"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestConfigsByOutput(t *testing.T) {
	cfgs := []structConfig{
		{Output: "FileDelta.go", Source: "Cee"},
		{Output: "FileAlpha.go", Source: "Alpha"},
		{Output: "FileBeta.go", Source: "Beta"},
		{Output: "FileCee.go", Source: "Cee"},
		{Output: "FileDelta.go", Source: "Alpha"},
		{Output: "FileAlpha.go", Source: "Beta"},
		{Output: "FileBeta.go", Source: "Cee"},
		{Output: "FileBeta.go", Source: "Delta"},
		{Output: "FileBeta.go", Source: "Zeta"},
	}

	seed := time.Now().UnixNano()
	t.Logf("Seed %d", seed)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(cfgs), func(i, j int) {
		cfgs[i], cfgs[j] = cfgs[j], cfgs[i]
	})

	actual := configsByOutput(cfgs)
	expected := [][]structConfig{
		{
			{Output: "FileAlpha.go", Source: "Alpha"},
			{Output: "FileAlpha.go", Source: "Beta"},
		},
		{
			{Output: "FileBeta.go", Source: "Beta"},
			{Output: "FileBeta.go", Source: "Cee"},
			{Output: "FileBeta.go", Source: "Delta"},
			{Output: "FileBeta.go", Source: "Zeta"},
		},
		{
			{Output: "FileCee.go", Source: "Cee"},
		},
		{
			{Output: "FileDelta.go", Source: "Alpha"},
			{Output: "FileDelta.go", Source: "Cee"},
		},
	}
	require.Equal(t, expected, actual)
}

func TestGenerateConversion(t *testing.T) {
	c := structConfig{
		Source:           "Node",
		FuncNameFragment: "Core",
		Target: target{
			Package: "example.com/org/project/core",
			Struct:  "Node",
		},
		Fields: []fieldConfig{{
			SourceName: "Iden",
			SourceExpr: &ast.Ident{Name: "string"},
			TargetName: "ID",
			SourceType: types.Typ[types.String],
		}},
	}
	target := targetStruct{
		Fields: []*types.Var{
			newField("ID", types.Typ[types.String]),
		},
	}
	imports := newImports()
	gen, err := generateConversion(c, target, imports)
	assert.NilError(t, err)

	file := &ast.File{Name: &ast.Ident{Name: "src"}}
	file.Decls = append(file.Decls, imports.Decl())
	file.Decls = append(file.Decls, gen.To, gen.From)

	out, err := astToBytes(&token.FileSet{}, file)
	assert.NilError(t, err)

	if *shouldPrint {
		t.Logf("OUTPUT\n%s\n", PrependLineNumbers(string(out)))
	}
	golden.Assert(t, string(out), t.Name()+"-expected")
	// TODO: check gen.RoundTripTest
}

func TestGenerateConversion_WithMissingSourceField(t *testing.T) {
	c := structConfig{
		Source:           "Node",
		FuncNameFragment: "Core",
		Target: target{
			Package: "example.com/org/project/core",
			Struct:  "Node",
		},
		Fields: []fieldConfig{{
			SourceName: "Iden",
			SourceExpr: &ast.Ident{Name: "string"},
			TargetName: "ID",
			SourceType: types.Typ[types.String],
		}},
	}
	target := targetStruct{
		Fields: []*types.Var{
			newField("ID", types.Typ[types.String]),
			newField("Name", types.Typ[types.String]),
		},
	}
	imports := newImports()
	_, err := generateConversion(c, target, imports)
	expected := "struct Node is missing field Name. Add the missing field or exclude it"
	assert.ErrorContains(t, err, expected)
}

func TestImports(t *testing.T) {
	imp := newImports()

	t.Run("add duplicate import", func(t *testing.T) {
		imp.Add("", "example.com/foo")
		imp.Add("", "example.com/foo")

		expected := &imports{
			byPkgPath: map[string]string{"example.com/foo": "foo"},
			byAlias:   map[string]string{"foo": "example.com/foo"},
			hasAlias:  make(map[string]struct{}),
		}
		assert.DeepEqual(t, expected, imp, gocmp.AllowUnexported(imports{}))
	})

	t.Run("AliasFor", func(t *testing.T) {
		imp.Add("somefoo", "example.com/some/foo")
		imp.Add("", "example.com/stars")

		assert.Equal(t, "somefoo", imp.AliasFor("example.com/some/foo"))
		assert.Equal(t, "stars", imp.AliasFor("example.com/stars"))
	})

	if t.Failed() {
		t.Skip("Decls value depends on previous subtests")
	}
	t.Run("Decls", func(t *testing.T) {
		file := &ast.File{Name: &ast.Ident{Name: "src"}}
		file.Decls = append(file.Decls, imp.Decl())
		out, err := astToBytes(&token.FileSet{}, file)
		assert.NilError(t, err)
		if *shouldPrint {
			t.Logf("OUTPUT\n%s\n", PrependLineNumbers(string(out)))
		}
		golden.Assert(t, string(out), "TestImports-Decls-expected")
	})
}
