package discover

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/go-discover/aws"
	"github.com/hashicorp/go-discover/azure"
	"github.com/hashicorp/go-discover/config"
	"github.com/hashicorp/go-discover/gce"
)

// Discoverer is the signature of the function to discover ip addresses of nodes
// for a given configuration. cfg is in "key=val key=val ..." format suitable
// for config.Parse() to understand.
type Discoverer func(cfg string, l *log.Logger) ([]string, error)

// Discoverers is the list of available discoverers.
var Discoverers = map[string]Discoverer{}

func init() {
	Discoverers = map[string]Discoverer{
		"aws":   aws.Discover,
		"gce":   gce.Discover,
		"azure": azure.Discover,
	}
}

// Discover executes the node discovery for a given provider. The
// configuration is expected to be in "key=val key=val ..." format and
// the provider name must be the first parameter.
//
// Example:
//
//  provider=aws region=eu-west-1 ...
//
func Discover(cfg string, l *log.Logger) ([]string, error) {
	args := strings.SplitN(cfg, " ", 2)
	m, err := config.Parse(args[0])
	if err != nil {
		return nil, err
	}
	p := m["provider"]
	if p == "" {
		return nil, fmt.Errorf("discover: missing provider")
	}
	d := Discoverers[p]
	if d == nil {
		return nil, fmt.Errorf("discover: unknown provider %q", p)
	}
	return d(args[1], l)
}
