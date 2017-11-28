// +build !ent

package sentinel

import (
	"log"
)

// New returns a new instance of the Sentinel code engine. This is only available
// in Consul Enterprise so this version always returns nil.
func New(logger *log.Logger) Evaluator {
	return nil
}
