// Package log implements basic but useful request (access) logging plugin.
package log

import (
	"log"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics/vars"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/rcode"
	"github.com/coredns/coredns/plugin/pkg/replacer"
	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Logger is a basic request logging plugin.
type Logger struct {
	Next      plugin.Handler
	Rules     []Rule
	ErrorFunc func(dns.ResponseWriter, *dns.Msg, int) // failover error handler
}

// ServeDNS implements the plugin.Handler interface.
func (l Logger) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	for _, rule := range l.Rules {
		if !plugin.Name(rule.NameScope).Matches(state.Name()) {
			continue
		}

		rrw := dnstest.NewRecorder(w)
		rc, err := plugin.NextOrFailure(l.Name(), l.Next, ctx, rrw, r)

		if rc > 0 {
			// There was an error up the chain, but no response has been written yet.
			// The error must be handled here so the log entry will record the response size.
			if l.ErrorFunc != nil {
				l.ErrorFunc(rrw, r, rc)
			} else {
				answer := new(dns.Msg)
				answer.SetRcode(r, rc)
				state.SizeAndDo(answer)

				vars.Report(state, vars.Dropped, rcode.ToString(rc), answer.Len(), time.Now())

				w.WriteMsg(answer)
			}
			rc = 0
		}

		tpe, _ := response.Typify(rrw.Msg, time.Now().UTC())
		class := response.Classify(tpe)
		if rule.Class == response.All || rule.Class == class {
			rep := replacer.New(r, rrw, CommonLogEmptyValue)
			rule.Log.Println(rep.Replace(rule.Format))
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
	Class     response.Class
	Format    string
	Log       *log.Logger
}

const (
	// CommonLogFormat is the common log format.
	CommonLogFormat = `{remote} ` + CommonLogEmptyValue + ` [{when}] {>id} "{type} {class} {name} {proto} {size} {>do} {>bufsize}" {rcode} {>rflags} {rsize} {duration}`
	// CommonLogEmptyValue is the common empty log value.
	CommonLogEmptyValue = "-"
	// CombinedLogFormat is the combined log format.
	CombinedLogFormat = CommonLogFormat + ` "{>opcode}"`
	// DefaultLogFormat is the default log format.
	DefaultLogFormat = CommonLogFormat
)
