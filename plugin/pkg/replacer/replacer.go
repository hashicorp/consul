package replacer

import (
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Replacer is a type which can replace placeholder
// substrings in a string with actual values from a
// dns.Msg and responseRecorder. Always use
// NewReplacer to get one of these.
type Replacer interface {
	Replace(string) string
	Set(key, value string)
}

type replacer struct {
	replacements map[string]string
	emptyValue   string
}

// New makes a new replacer based on r and rr.
// Do not create a new replacer until r and rr have all
// the needed values, because this function copies those
// values into the replacer. rr may be nil if it is not
// available. emptyValue should be the string that is used
// in place of empty string (can still be empty string).
func New(r *dns.Msg, rr *dnstest.Recorder, emptyValue string) Replacer {
	req := request.Request{W: rr, Req: r}
	rep := replacer{
		replacements: map[string]string{
			"{type}":  req.Type(),
			"{name}":  req.Name(),
			"{class}": req.Class(),
			"{proto}": req.Proto(),
			"{when}": func() string {
				return time.Now().Format(timeFormat)
			}(),
			"{size}":   strconv.Itoa(req.Len()),
			"{remote}": req.IP(),
			"{port}":   req.Port(),
		},
		emptyValue: emptyValue,
	}
	if rr != nil {
		rcode := dns.RcodeToString[rr.Rcode]
		if rcode == "" {
			rcode = strconv.Itoa(rr.Rcode)
		}
		rep.replacements["{rcode}"] = rcode
		rep.replacements["{rsize}"] = strconv.Itoa(rr.Len)
		rep.replacements["{duration}"] = time.Since(rr.Start).String()
		if rr.Msg != nil {
			rep.replacements[headerReplacer+"rflags}"] = flagsToString(rr.Msg.MsgHdr)
		}
	}

	// Header placeholders (case-insensitive)
	rep.replacements[headerReplacer+"id}"] = strconv.Itoa(int(r.Id))
	rep.replacements[headerReplacer+"opcode}"] = strconv.Itoa(r.Opcode)
	rep.replacements[headerReplacer+"do}"] = boolToString(req.Do())
	rep.replacements[headerReplacer+"bufsize}"] = strconv.Itoa(req.Size())

	return rep
}

// Replace performs a replacement of values on s and returns
// the string with the replaced values.
func (r replacer) Replace(s string) string {
	// Header replacements - these are case-insensitive, so we can't just use strings.Replace()
	for strings.Contains(s, headerReplacer) {
		idxStart := strings.Index(s, headerReplacer)
		endOffset := idxStart + len(headerReplacer)
		idxEnd := strings.Index(s[endOffset:], "}")
		if idxEnd > -1 {
			placeholder := strings.ToLower(s[idxStart : endOffset+idxEnd+1])
			replacement := r.replacements[placeholder]
			if replacement == "" {
				replacement = r.emptyValue
			}
			s = s[:idxStart] + replacement + s[endOffset+idxEnd+1:]
		} else {
			break
		}
	}

	// Regular replacements - these are easier because they're case-sensitive
	for placeholder, replacement := range r.replacements {
		if replacement == "" {
			replacement = r.emptyValue
		}
		s = strings.Replace(s, placeholder, replacement, -1)
	}

	return s
}

// Set sets key to value in the replacements map.
func (r replacer) Set(key, value string) {
	r.replacements["{"+key+"}"] = value
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

const (
	timeFormat     = "02/Jan/2006:15:04:05 -0700"
	headerReplacer = "{>"
)
