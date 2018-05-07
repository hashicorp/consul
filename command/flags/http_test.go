package flags

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPFlagsSetToken(t *testing.T) {
	var f HTTPFlags
	require := require.New(t)
	require.Empty(f.Token())
	require.NoError(f.SetToken("foo"))
	require.Equal("foo", f.Token())
}
