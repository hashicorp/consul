package service

import (
	"fmt"
	"os"

	"github.com/testcontainers/testcontainers-go"
)

type ServiceLogConsumer struct {
	Prefix string
}

var _ testcontainers.LogConsumer = (*ServiceLogConsumer)(nil)

func (c *ServiceLogConsumer) Accept(log testcontainers.Log) {
	switch log.LogType {
	case "STDOUT":
		fmt.Fprint(os.Stdout, c.Prefix+" ~~ "+string(log.Content))
	case "STDERR":
		fmt.Fprint(os.Stderr, c.Prefix+" ~~ "+string(log.Content))
	}
}
