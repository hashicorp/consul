// Package log implements basic but useful request (access) logging middleware.
package log

import (
	"log"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/metrics"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Logger is a basic request logging middleware.
type Logger struct {
	Next      middleware.Handler
	Rules     []Rule
	ErrorFunc func(dns.ResponseWriter, *dns.Msg, int) // failover error handler
}

func (l Logger) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}
	for _, rule := range l.Rules {
		if middleware.Name(rule.NameScope).Matches(state.Name()) {
			responseRecorder := middleware.NewResponseRecorder(w)
			rcode, err := l.Next.ServeDNS(ctx, responseRecorder, r)

			if rcode > 0 {
				// There was an error up the chain, but no response has been written yet.
				// The error must be handled here so the log entry will record the response size.
				if l.ErrorFunc != nil {
					l.ErrorFunc(responseRecorder, r, rcode)
				} else {
					rc := middleware.RcodeToString(rcode)

					answer := new(dns.Msg)
					answer.SetRcode(r, rcode)
					state.SizeAndDo(answer)

					metrics.Report(metrics.Dropped, state.Proto(), rc, answer.Len(), time.Now())
					w.WriteMsg(answer)
				}
				rcode = 0
			}
			rep := middleware.NewReplacer(r, responseRecorder, CommonLogEmptyValue)
			rule.Log.Println(rep.Replace(rule.Format))
			return rcode, err

		}
	}
	return l.Next.ServeDNS(ctx, w, r)
}

// Rule configures the logging middleware.
type Rule struct {
	NameScope  string
	OutputFile string
	Format     string
	Log        *log.Logger
	Roller     *middleware.LogRoller
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
