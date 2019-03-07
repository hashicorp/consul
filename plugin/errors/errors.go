// Package errors implements an error handling plugin.
package errors

import (
	"context"
	"regexp"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

var log = clog.NewWithPlugin("errors")

type pattern struct {
	ptimer  unsafe.Pointer
	count   uint32
	period  time.Duration
	pattern *regexp.Regexp
}

func (p *pattern) timer() *time.Timer {
	return (*time.Timer)(atomic.LoadPointer(&p.ptimer))
}

func (p *pattern) setTimer(t *time.Timer) {
	atomic.StorePointer(&p.ptimer, unsafe.Pointer(t))
}

// errorHandler handles DNS errors (and errors from other plugin).
type errorHandler struct {
	patterns []*pattern
	stopFlag uint32
	Next     plugin.Handler
}

func newErrorHandler() *errorHandler {
	return &errorHandler{}
}

func (h *errorHandler) logPattern(i int) {
	cnt := atomic.SwapUint32(&h.patterns[i].count, 0)
	if cnt > 0 {
		log.Errorf("%d errors like '%s' occurred in last %s",
			cnt, h.patterns[i].pattern.String(), h.patterns[i].period)
	}
}

func (h *errorHandler) inc(i int) bool {
	if atomic.LoadUint32(&h.stopFlag) > 0 {
		return false
	}
	if atomic.AddUint32(&h.patterns[i].count, 1) == 1 {
		ind := i
		t := time.AfterFunc(h.patterns[ind].period, func() {
			h.logPattern(ind)
		})
		h.patterns[ind].setTimer(t)
		if atomic.LoadUint32(&h.stopFlag) > 0 && t.Stop() {
			h.logPattern(ind)
		}
	}
	return true
}

func (h *errorHandler) stop() {
	atomic.StoreUint32(&h.stopFlag, 1)
	for i := range h.patterns {
		t := h.patterns[i].timer()
		if t != nil && t.Stop() {
			h.logPattern(i)
		}
	}
}

// ServeDNS implements the plugin.Handler interface.
func (h *errorHandler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	rcode, err := plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)

	if err != nil {
		strErr := err.Error()
		for i := range h.patterns {
			if h.patterns[i].pattern.MatchString(strErr) {
				if h.inc(i) {
					return rcode, err
				}
				break
			}
		}
		state := request.Request{W: w, Req: r}
		log.Errorf("%d %s %s: %s", rcode, state.Name(), state.Type(), strErr)
	}

	return rcode, err
}

// Name implements the plugin.Handler interface.
func (h *errorHandler) Name() string { return "errors" }
