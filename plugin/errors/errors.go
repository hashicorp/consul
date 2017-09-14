// Package errors implements an HTTP error handling plugin.
package errors

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// errorHandler handles DNS errors (and errors from other plugin).
type errorHandler struct {
	Next    plugin.Handler
	LogFile string
	Log     *log.Logger
}

// ServeDNS implements the plugin.Handler interface.
func (h errorHandler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	defer h.recovery(ctx, w, r)

	rcode, err := plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)

	if err != nil {
		state := request.Request{W: w, Req: r}
		errMsg := fmt.Sprintf("%s [ERROR %d %s %s] %v", time.Now().Format(timeFormat), rcode, state.Name(), state.Type(), err)

		h.Log.Println(errMsg)
	}

	return rcode, err
}

func (h errorHandler) Name() string { return "errors" }

func (h errorHandler) recovery(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
	rec := recover()
	if rec == nil {
		return
	}

	// Obtain source of panic
	// From: https://gist.github.com/swdunlop/9629168
	var name, file string // function name, file name
	var line int
	var pc [16]uintptr
	n := runtime.Callers(3, pc[:])
	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	// Trim file path
	delim := "/coredns/"
	pkgPathPos := strings.Index(file, delim)
	if pkgPathPos > -1 && len(file) > pkgPathPos+len(delim) {
		file = file[pkgPathPos+len(delim):]
	}

	panicMsg := fmt.Sprintf("%s [PANIC %s %s] %s:%d - %v", time.Now().Format(timeFormat), r.Question[0].Name, dns.Type(r.Question[0].Qtype), file, line, rec)
	// Currently we don't use the function name, since file:line is more conventional
	h.Log.Printf(panicMsg)
}

const timeFormat = "02/Jan/2006:15:04:05 -0700"
