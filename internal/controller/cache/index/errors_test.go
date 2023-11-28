package index

import (
	"testing"

	"github.com/hashicorp/consul/internal/testing/errors"
)

func TestErrorStrings(t *testing.T) {
	errors.TestErrorStrings(t, map[string]error{
		"MissingRequiredIndex": MissingRequiredIndexError{Name: "fake-index-name"},
	})
}
