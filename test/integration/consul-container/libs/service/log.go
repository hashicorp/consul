// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package service

import (
	"fmt"
	"os"

	"github.com/testcontainers/testcontainers-go"
)

type LogConsumer struct {
	Prefix string
}

var _ testcontainers.LogConsumer = (*LogConsumer)(nil)

func (c *LogConsumer) Accept(log testcontainers.Log) {
	switch log.LogType {
	case "STDOUT":
		fmt.Fprint(os.Stdout, c.Prefix+" ~~ "+string(log.Content))
	case "STDERR":
		fmt.Fprint(os.Stderr, c.Prefix+" ~~ "+string(log.Content))
	}
}
