package sqlutils

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestGetDriverName(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("postgres", GetDriverName(&pq.Driver{}))
	assert.Equal("mysql", GetDriverName(&mysql.MySQLDriver{}))
	assert.Equal("", GetDriverName(nil))
}
