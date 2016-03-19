package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Replacer is a type which can replace placeholder
// substrings in a string with actual values from a
// http.Request and responseRecorder. Always use
// NewReplacer to get one of these.
type Replacer interface {
	Replace(string) string
	Set(key, value string)
}

type replacer struct {
	replacements map[string]string
	emptyValue   string
}

// NewReplacer makes a new replacer based on r and rr.
// Do not create a new replacer until r and rr have all
// the needed values, because this function copies those
// values into the replacer. rr may be nil if it is not
// available. emptyValue should be the string that is used
// in place of empty string (can still be empty string).
func NewReplacer(r *dns.Msg, rr *ResponseRecorder, emptyValue string) Replacer {
	state := State{W: rr, Req: r}
	rep := replacer{
		replacements: map[string]string{
			"{type}":  state.Type(),
			"{name}":  state.Name(),
			"{class}": state.Class(),
			"{proto}": state.Proto(),
			"{when}": func() string {
				return time.Now().Format(timeFormat)
			}(),
			"{remote}": state.IP(),
			"{port}": func() string {
				p, _ := state.Port()
				return p
			}(),
		},
		emptyValue: emptyValue,
	}
	if rr != nil {
		rcode := dns.RcodeToString[rr.rcode]
		if rcode == "" {
			rcode = strconv.Itoa(rr.rcode)
		}
		rep.replacements["{rcode}"] = rcode
		rep.replacements["{size}"] = strconv.Itoa(rr.size)
		rep.replacements["{latency}"] = time.Since(rr.start).String()
	}

	// Header placeholders (case-insensitive)
	// TODO(miek): syntax for flags and document it
	rep.replacements[headerReplacer+"id}"] = strconv.Itoa(int(r.Id))
	rep.replacements[headerReplacer+"opcode}"] = strconv.Itoa(int(r.Opcode))

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

const (
	timeFormat     = "02/Jan/2006:15:04:05 -0700"
	headerReplacer = "{>"
)
