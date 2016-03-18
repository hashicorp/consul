package setup

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/reflect"
)

// Reflect sets up the reflect middleware.
func Reflect(c *Controller) (middleware.Middleware, error) {
	if err := reflectParse(c); err != nil {
		return nil, err
	}
	return func(next middleware.Handler) middleware.Handler {
		return reflect.Reflect{Next: next}
	}, nil

}

func reflectParse(c *Controller) error {
	for c.Next() {
		if c.Val() == "reflect" {
			if c.NextArg() {
				return c.ArgErr()
			}
		}
	}
	return nil
}
