package main

import (
	"bytes"
	"go/format"
	"go/token"
	"go/types"
	"math/rand"
	"testing"
	"time"

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
	gen, err := generateConversion(c, target)
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	err = format.Node(buf, new(token.FileSet), gen.To)
	assert.NilError(t, err)

	err = format.Node(buf, new(token.FileSet), gen.From)
	assert.NilError(t, err)

	golden.Assert(t, buf.String(), t.Name()+"-expected")
	// TODO: check gen.RoundTripTest
}
