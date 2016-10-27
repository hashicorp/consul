// Package log implements basic but useful request (access) logging middleware.
package log

import (
	"log"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/metrics/vars"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/pkg/rcode"
	"github.com/miekg/coredns/middleware/pkg/replacer"
	"github.com/miekg/coredns/middleware/pkg/response"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Logger is a basic request logging middleware.
type Logger struct {
	Next      middleware.Handler
	Rules     []Rule
	ErrorFunc func(dns.ResponseWriter, *dns.Msg, int) // failover error handler
}

// ServeDNS implements the middleware.Handler interface.
func (l Logger) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	for _, rule := range l.Rules {
		if !middleware.Name(rule.NameScope).Matches(state.Name()) {
			continue
		}

		responseRecorder := dnsrecorder.New(w)
		rc, err := l.Next.ServeDNS(ctx, responseRecorder, r)

		if rc > 0 {
			// There was an error up the chain, but no response has been written yet.
			// The error must be handled here so the log entry will record the response size.
			if l.ErrorFunc != nil {
				l.ErrorFunc(responseRecorder, r, rc)
			} else {
				answer := new(dns.Msg)
				answer.SetRcode(r, rc)
				state.SizeAndDo(answer)

				vars.Report(state, vars.Dropped, rcode.ToString(rc), answer.Len(), time.Now())

				w.WriteMsg(answer)
			}
			rc = 0
		}

		class, _ := response.Classify(responseRecorder.Msg)
		if rule.Class == response.All || rule.Class == class {
			rep := replacer.New(r, responseRecorder, CommonLogEmptyValue)
			rule.Log.Println(rep.Replace(rule.Format))
		}

		return rc, err

	}
	return l.Next.ServeDNS(ctx, w, r)
}

// Name implements the Handler interface.
func (l Logger) Name() string { return "log" }

// Rule configures the logging middleware.
type Rule struct {
	NameScope  string
	Class      response.Class
	OutputFile string
	Format     string
	Log        *log.Logger
}

const (
	// DefaultLogFilename is the default log filename.
	DefaultLogFilename = "query.log"
	// CommonLogFormat is the common log format.
	CommonLogFormat = `{remote} ` + CommonLogEmptyValue + ` [{when}] "{type} {class} {name} {proto} {>do} {>bufsize}" {rcode} {size} {duration}`
	// CommonLogEmptyValue is the common empty log value.
	CommonLogEmptyValue = "-"
	// CombinedLogFormat is the combined log format.
	CombinedLogFormat = CommonLogFormat + ` "{>opcode}"`
	// DefaultLogFormat is the default log format.
	DefaultLogFormat = CommonLogFormat
)
