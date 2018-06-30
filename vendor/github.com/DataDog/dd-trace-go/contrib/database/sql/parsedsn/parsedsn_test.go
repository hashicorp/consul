package parsedsn

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMySQL(t *testing.T) {
	assert := assert.New(t)

	expected := map[string]string{
		"user":   "bob",
		"host":   "1.2.3.4",
		"port":   "5432",
		"dbname": "mydb",
	}
	m, err := MySQL("bob:secret@tcp(1.2.3.4:5432)/mydb")
	assert.Equal(nil, err)
	assert.True(reflect.DeepEqual(expected, m))
}

func TestPostgres(t *testing.T) {
	assert := assert.New(t)

	expected := map[string]string{
		"user":    "bob",
		"host":    "1.2.3.4",
		"port":    "5432",
		"dbname":  "mydb",
		"sslmode": "verify-full",
	}
	m, err := Postgres("postgres://bob:secret@1.2.3.4:5432/mydb?sslmode=verify-full")
	assert.Equal(nil, err)
	assert.True(reflect.DeepEqual(expected, m))

	expected = map[string]string{
		"user":             "dog",
		"port":             "5433",
		"host":             "master-db-master-active.postgres.service.consul",
		"dbname":           "dogdatastaging",
		"application_name": "trace-api",
	}
	dsn := "password=zMWmQz26GORmgVVKEbEl dbname=dogdatastaging application_name=trace-api port=5433 host=master-db-master-active.postgres.service.consul user=dog"
	m, err = Postgres(dsn)
	assert.Equal(nil, err)
	assert.True(reflect.DeepEqual(expected, m))
}
