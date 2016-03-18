// Package log implements basic but useful request (access) logging middleware.
package log

import (
	"log"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

// Logger is a basic request logging middleware.
type Logger struct {
	Next      middleware.Handler
	Rules     []Rule
	ErrorFunc func(dns.ResponseWriter, *dns.Msg, int) // failover error handler
}

func (l Logger) ServeDNS(w dns.ResponseWriter, r *dns.Msg) (int, error) {
	for _, rule := range l.Rules {
		/*
			if middleware.Path(r.URL.Path).Matches(rule.PathScope) {
				responseRecorder := middleware.NewResponseRecorder(w)
				status, err := l.Next.ServeHTTP(responseRecorder, r)
				if status >= 400 {
					// There was an error up the chain, but no response has been written yet.
					// The error must be handled here so the log entry will record the response size.
					if l.ErrorFunc != nil {
						l.ErrorFunc(responseRecorder, r, status)
					} else {
						// Default failover error handler
						responseRecorder.WriteHeader(status)
						fmt.Fprintf(responseRecorder, "%d %s", status, http.StatusText(status))
					}
					status = 0
				}
				rep := middleware.NewReplacer(r, responseRecorder, CommonLogEmptyValue)
				rule.Log.Println(rep.Replace(rule.Format))
				return status, err
			}
		*/
		rule = rule
	}
	return l.Next.ServeDNS(w, r)
}

// Rule configures the logging middleware.
type Rule struct {
	PathScope  string
	OutputFile string
	Format     string
	Log        *log.Logger
	Roller     *middleware.LogRoller
}

const (
	// DefaultLogFilename is the default log filename.
	DefaultLogFilename = "access.log"
	// CommonLogFormat is the common log format.
	CommonLogFormat = `{remote} ` + CommonLogEmptyValue + ` [{when}] "{type} {name} {proto}" {rcode} {size}`
	// CommonLogEmptyValue is the common empty log value.
	CommonLogEmptyValue = "-"
	// CombinedLogFormat is the combined log format.
	CombinedLogFormat = CommonLogFormat + ` "{>Referer}" "{>User-Agent}"` // Something here as well
	// DefaultLogFormat is the default log format.
	DefaultLogFormat = CommonLogFormat
)
