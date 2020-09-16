package main

import (
	"bytes"
	"go/ast"
	"go/format"
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
		Fields: []fieldConfig{
			{SourceName: "Iden", TargetName: "ID"},
		},
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

	buf := new(bytes.Buffer)
	err = format.Node(buf, new(token.FileSet), file)
	assert.NilError(t, err)

	golden.Assert(t, buf.String(), t.Name()+"-expected")
	// TODO: check gen.RoundTripTest
}

func TestImports(t *testing.T) {
	imp := newImports()

	t.Run("add duplicate import", func(t *testing.T) {
		imp.Add("", "example.com/foo")
		imp.Add("", "example.com/foo")

		expected := &imports{
			byPkgPath: map[string]string{"example.com/foo": "foo"},
			byAlias:   map[string]string{"foo": "example.com/foo"},
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
		buf := new(bytes.Buffer)
		format.Node(buf, new(token.FileSet), imp.Decl())
		golden.Assert(t, buf.String(), "TestImports-Decls-expected")
	})
}
