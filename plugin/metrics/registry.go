package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type reg struct {
	sync.RWMutex
	r map[string]*prometheus.Registry
}

func newReg() *reg { return &reg{r: make(map[string]*prometheus.Registry)} }

// update sets the registry if not already there and returns the input. Or it returns
// a previous set value.
func (r *reg) getOrSet(addr string, pr *prometheus.Registry) *prometheus.Registry {
	r.Lock()
	defer r.Unlock()

	if v, ok := r.r[addr]; ok {
		return v
	}

	r.r[addr] = pr
	return pr
}
