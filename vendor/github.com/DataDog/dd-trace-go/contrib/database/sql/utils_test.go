package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringInSlice(t *testing.T) {
	assert := assert.New(t)

	list := []string{"mysql", "postgres", "pq"}
	assert.True(stringInSlice(list, "pq"))
	assert.False(stringInSlice(list, "Postgres"))
}

func TestDSNAndService(t *testing.T) {
	assert := assert.New(t)

	dsn := "postgres://ubuntu@127.0.0.1:5432/circle_test?sslmode=disable"
	service := "master-db"

	dsnAndService := "postgres://ubuntu@127.0.0.1:5432/circle_test?sslmode=disable|master-db"
	assert.Equal(dsnAndService, newDSNAndService(dsn, service))

	actualDSN, actualService := parseDSNAndService(dsnAndService)
	assert.Equal(dsn, actualDSN)
	assert.Equal(service, actualService)
}
