// Package log implements basic but useful request (access) logging plugin.
package log

import (
	"context"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/replacer"
	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Logger is a basic request logging plugin.
type Logger struct {
	Next  plugin.Handler
	Rules []Rule

	repl replacer.Replacer
}

// ServeDNS implements the plugin.Handler interface.
func (l Logger) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	name := state.Name()
	for _, rule := range l.Rules {
		if !plugin.Name(rule.NameScope).Matches(name) {
			continue
		}

		rrw := dnstest.NewRecorder(w)
		rc, err := plugin.NextOrFailure(l.Name(), l.Next, ctx, rrw, r)

		// If we don't set up a class in config, the default "all" will be added
		// and we shouldn't have an empty rule.Class.
		_, ok := rule.Class[response.All]
		var ok1 bool
		if !ok {
			tpe, _ := response.Typify(rrw.Msg, time.Now().UTC())
			class := response.Classify(tpe)
			_, ok1 = rule.Class[class]
		}
		if ok || ok1 {
			logstr := l.repl.Replace(ctx, state, rrw, rule.Format)
			clog.Infof(logstr)
		}

		return rc, err

	}
	return plugin.NextOrFailure(l.Name(), l.Next, ctx, w, r)
}

// Name implements the Handler interface.
func (l Logger) Name() string { return "log" }

// Rule configures the logging plugin.
type Rule struct {
	NameScope string
	Class     map[response.Class]struct{}
	Format    string
}

const (
	// CommonLogFormat is the common log format.
	CommonLogFormat = `{remote}:{port} ` + replacer.EmptyValue + ` {>id} "{type} {class} {name} {proto} {size} {>do} {>bufsize}" {rcode} {>rflags} {rsize} {duration}`
	// CombinedLogFormat is the combined log format.
	CombinedLogFormat = CommonLogFormat + ` "{>opcode}"`
	// DefaultLogFormat is the default log format.
	DefaultLogFormat = CommonLogFormat
)
