package reload

import (
	"math/rand"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("reload", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

var r *reload
var once sync.Once

func setup(c *caddy.Controller) error {
	c.Next() // 'reload'
	args := c.RemainingArgs()

	if len(args) > 2 {
		return plugin.Error("reload", c.ArgErr())
	}

	i := defaultInterval
	if len(args) > 0 {
		d, err := time.ParseDuration(args[0])
		if err != nil {
			return err
		}
		i = d
	}

	j := defaultJitter
	if len(args) > 1 {
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return err
		}
		j = d
	}

	if j > i/2 {
		j = i / 2
	}

	jitter := time.Duration(rand.Int63n(j.Nanoseconds()) - (j.Nanoseconds() / 2))
	i = i + jitter

	r = &reload{interval: i, quit: make(chan bool)}
	once.Do(func() {
		caddy.RegisterEventHook("reload", hook)
	})

	c.OnFinalShutdown(func() error {
		r.quit <- true
		return nil
	})

	return nil
}

const (
	defaultInterval = 30 * time.Second
	defaultJitter   = 15 * time.Second
)
