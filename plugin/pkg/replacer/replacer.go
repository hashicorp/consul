package replacer

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin/metadata"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Replacer replaces labels for values in strings.
type Replacer struct {
	valueFunc func(request.Request, *dnstest.Recorder, string) string
	labels    []string
}

// labels are all supported labels that can be used in the default Replacer.
var labels = []string{
	"{type}",
	"{name}",
	"{class}",
	"{proto}",
	"{size}",
	"{remote}",
	"{port}",
	"{local}",
	// Header values.
	headerReplacer + "id}",
	headerReplacer + "opcode}",
	headerReplacer + "do}",
	headerReplacer + "bufsize}",
	// Recorded replacements.
	"{rcode}",
	"{rsize}",
	"{duration}",
	headerReplacer + "rrflags}",
}

// value returns the current value of label.
func value(state request.Request, rr *dnstest.Recorder, label string) string {
	switch label {
	case "{type}":
		return state.Type()
	case "{name}":
		return state.Name()
	case "{class}":
		return state.Class()
	case "{proto}":
		return state.Proto()
	case "{size}":
		return strconv.Itoa(state.Req.Len())
	case "{remote}":
		return addrToRFC3986(state.IP())
	case "{port}":
		return state.Port()
	case "{local}":
		return addrToRFC3986(state.LocalIP())
	// Header placeholders (case-insensitive).
	case headerReplacer + "id}":
		return strconv.Itoa(int(state.Req.Id))
	case headerReplacer + "opcode}":
		return strconv.Itoa(state.Req.Opcode)
	case headerReplacer + "do}":
		return boolToString(state.Do())
	case headerReplacer + "bufsize}":
		return strconv.Itoa(state.Size())
	// Recorded replacements.
	case "{rcode}":
		if rr == nil {
			return EmptyValue
		}
		rcode := dns.RcodeToString[rr.Rcode]
		if rcode == "" {
			rcode = strconv.Itoa(rr.Rcode)
		}
		return rcode
	case "{rsize}":
		if rr == nil {
			return EmptyValue
		}
		return strconv.Itoa(rr.Len)
	case "{duration}":
		if rr == nil {
			return EmptyValue
		}
		return strconv.FormatFloat(time.Since(rr.Start).Seconds(), 'f', -1, 64) + "s"
	case headerReplacer + "rrflags}":
		if rr != nil && rr.Msg != nil {
			return flagsToString(rr.Msg.MsgHdr)
		}
		return EmptyValue
	}
	return EmptyValue
}

// New makes a new replacer. This only needs to be called once in the setup and then call Replace for each incoming message.
// A replacer is safe for concurrent use.
func New() Replacer {
	return Replacer{
		valueFunc: value,
		labels:    labels,
	}
}

// Replace performs a replacement of values on s and returns the string with the replaced values.
func (r Replacer) Replace(ctx context.Context, state request.Request, rr *dnstest.Recorder, s string) string {
	for _, placeholder := range r.labels {
		if strings.Contains(s, placeholder) {
			s = strings.Replace(s, placeholder, r.valueFunc(state, rr, placeholder), -1)
		}
	}

	// Metadata label replacements. Scan for {/ and search for next }, replace that metadata label with
	// any meta data that is available.
	b := strings.Builder{}
	for strings.Contains(s, labelReplacer) {
		idxStart := strings.Index(s, labelReplacer)
		endOffset := idxStart + len(labelReplacer)
		idxEnd := strings.Index(s[endOffset:], "}")
		if idxEnd > -1 {
			label := s[idxStart+2 : endOffset+idxEnd]

			fm := metadata.ValueFunc(ctx, label)
			replacement := EmptyValue
			if fm != nil {
				replacement = fm()
			}

			b.WriteString(s[:idxStart])
			b.WriteString(replacement)
			s = s[endOffset+idxEnd+1:]
		} else {
			break
		}
	}

	b.WriteString(s)
	return b.String()
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// flagsToString checks all header flags and returns those
// that are set as a string separated with commas
func flagsToString(h dns.MsgHdr) string {
	flags := make([]string, 7)
	i := 0

	if h.Response {
		flags[i] = "qr"
		i++
	}

	if h.Authoritative {
		flags[i] = "aa"
		i++
	}
	if h.Truncated {
		flags[i] = "tc"
		i++
	}
	if h.RecursionDesired {
		flags[i] = "rd"
		i++
	}
	if h.RecursionAvailable {
		flags[i] = "ra"
		i++
	}
	if h.Zero {
		flags[i] = "z"
		i++
	}
	if h.AuthenticatedData {
		flags[i] = "ad"
		i++
	}
	if h.CheckingDisabled {
		flags[i] = "cd"
		i++
	}
	return strings.Join(flags[:i], ",")
}

// addrToRFC3986 will add brackets to the address if it is an IPv6 address.
func addrToRFC3986(addr string) string {
	if strings.Contains(addr, ":") {
		return "[" + addr + "]"
	}
	return addr
}

const (
	headerReplacer = "{>"
	labelReplacer  = "{/"
	// EmptyValue is the default empty value.
	EmptyValue = "-"
)
