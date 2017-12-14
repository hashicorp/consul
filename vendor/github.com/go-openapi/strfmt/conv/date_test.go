package conv

import (
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
)

func TestDateValue(t *testing.T) {
	assert.Equal(t, strfmt.Date{}, DateValue(nil))
	date := strfmt.Date(time.Now())
	assert.Equal(t, date, DateValue(&date))
}
