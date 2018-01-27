package reload

import (
	"crypto/md5"
	"log"
	"time"

	"github.com/mholt/caddy"
)

// reload periodically checks if the Corefile has changed, and reloads if so

type reload struct {
	instance *caddy.Instance
	interval time.Duration
	sum      [md5.Size]byte
	stopped  bool
	quit     chan bool
}

func hook(event caddy.EventName, info interface{}) error {
	if event != caddy.InstanceStartupEvent {
		return nil
	}

	// if reload is removed from the Corefile, then the hook
	// is still registered but setup is never called again
	// so we need a flag to tell us not to reload
	if r.stopped {
		return nil
	}

	// this should be an instance. ok to panic if not
	r.instance = info.(*caddy.Instance)
	r.sum = md5.Sum(r.instance.Caddyfile().Body())

	go func() {
		tick := time.NewTicker(r.interval)

		for {
			select {
			case <-tick.C:
				corefile, err := caddy.LoadCaddyfile(r.instance.Caddyfile().ServerType())
				if err != nil {
					continue
				}
				s := md5.Sum(corefile.Body())
				if s != r.sum {
					_, err := r.instance.Restart(corefile)
					if err != nil {
						log.Printf("[ERROR] Corefile changed but reload failed: %s\n", err)
						continue
					}
					// we are done, this hook gets called again with new instance
					r.stopped = true
					return
				}
			case <-r.quit:
				return
			}
		}
	}()

	return nil
}
