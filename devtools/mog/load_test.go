package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadStructs_WithFilterSourceStructs(t *testing.T) {
	p, err := loadStructs("./internal/sourcepkg", sourceStructs)
	require.NoError(t, err)
	require.Equal(t, []string{"GroupedSample", "Sample"}, p.Names())
	_, ok := p.structs["Sample"]
	require.True(t, ok)
	_, ok = p.structs["GroupedSample"]
	require.True(t, ok)

	// TODO: check the value in structs map
}
