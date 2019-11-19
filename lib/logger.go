package lib

import (
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
)

func DefaultLogger() *log.Logger {
	consulLogger := hclog.New(&hclog.LoggerOptions{
		Level:  log.LstdFlags,
		Output: os.Stderr,
	})
	return consulLogger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})

}
