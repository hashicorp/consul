package node

import (
	"fmt"
	"os"

	"github.com/testcontainers/testcontainers-go"
)

type NodeLogConsumer struct {
	Prefix string
}

var _ testcontainers.LogConsumer = (*NodeLogConsumer)(nil)

func (c *NodeLogConsumer) Accept(log testcontainers.Log) {
	switch log.LogType {
	case "STDOUT":
		fmt.Fprint(os.Stdout, c.Prefix+" ~~ "+string(log.Content))
	case "STDERR":
		fmt.Fprint(os.Stderr, c.Prefix+" ~~ "+string(log.Content))
	}
}
