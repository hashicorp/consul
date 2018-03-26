package connect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Signing ID should never authorize
func TestSpiffeIDSigningAuthorize(t *testing.T) {
	var id SpiffeIDSigning
	auth, ok := id.Authorize(nil)
	assert.False(t, auth)
	assert.True(t, ok)
}
