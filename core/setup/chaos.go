package setup

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/chaos"
)

// Chaos configures a new Chaos middleware instance.
func Chaos(c *Controller) (middleware.Middleware, error) {
	version, authors, err := chaosParse(c)
	if err != nil {
		return nil, err
	}

	return func(next middleware.Handler) middleware.Handler {
		return chaos.Chaos{
			Next:    next,
			Version: version,
			Authors: authors,
		}
	}, nil
}

func chaosParse(c *Controller) (string, map[string]bool, error) {
	version := ""
	authors := make(map[string]bool)

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return defaultVersion, nil, nil
		}
		if len(args) == 1 {
			return args[0], nil, nil
		}
		version = args[0]
		for _, a := range args[1:] {
			authors[a] = true
		}
		return version, authors, nil
	}
	return version, authors, nil
}

const defaultVersion = "CoreDNS"
