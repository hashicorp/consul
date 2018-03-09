package test

import "fmt"

// Config stores the configuration of mysql and postgres databases for testing purposes
type Config struct {
	Template string
	User     string
	Password string
	Host     string
	Port     string
	DBName   string
}

// DSN returns the formatted DSN corresponding to each configuration
func (c Config) DSN() string {
	return fmt.Sprintf(c.Template, c.User, c.Password, c.Host, c.Port, c.DBName)
}

// MySQLConfig stores the configuration of our mysql test server
var MySQLConfig = Config{
	"%s:%s@tcp(%s:%s)/%s",
	"ubuntu",
	"",
	"127.0.0.1",
	"3306",
	"circle_test",
}

// PostgresConfig stores the configuration of our postgres test server
var PostgresConfig = Config{
	"postgres://%s:%s@%s:%s/%s?sslmode=disable",
	"ubuntu",
	"",
	"127.0.0.1",
	"5432",
	"circle_test",
}
