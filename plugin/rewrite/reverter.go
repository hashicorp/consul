package rewrite

import (
	"github.com/miekg/dns"
	"regexp"
	"strconv"
	"strings"
)

// ResponseRule contains a rule to rewrite a response with.
type ResponseRule struct {
	Active      bool
	Pattern     *regexp.Regexp
	Replacement string
}

// ResponseReverter reverses the operations done on the question section of a packet.
// This is need because the client will otherwise disregards the response, i.e.
// dig will complain with ';; Question section mismatch: got miek.nl/HINFO/IN'
type ResponseReverter struct {
	dns.ResponseWriter
	originalQuestion dns.Question
	ResponseRewrite  bool
	ResponseRules    []ResponseRule
}

// NewResponseReverter returns a pointer to a new ResponseReverter.
func NewResponseReverter(w dns.ResponseWriter, r *dns.Msg) *ResponseReverter {
	return &ResponseReverter{
		ResponseWriter:   w,
		originalQuestion: r.Question[0],
	}
}

// WriteMsg records the status code and calls the
// underlying ResponseWriter's WriteMsg method.
func (r *ResponseReverter) WriteMsg(res *dns.Msg) error {
	res.Question[0] = r.originalQuestion
	if r.ResponseRewrite {
		for _, rr := range res.Answer {
			name := rr.(*dns.A).Hdr.Name
			for _, rule := range r.ResponseRules {
				regexGroups := rule.Pattern.FindStringSubmatch(name)
				if len(regexGroups) == 0 {
					continue
				}
				s := rule.Replacement
				for groupIndex, groupValue := range regexGroups {
					groupIndexStr := "{" + strconv.Itoa(groupIndex) + "}"
					if strings.Contains(s, groupIndexStr) {
						s = strings.Replace(s, groupIndexStr, groupValue, -1)
					}
				}
				name = s
			}
			rr.(*dns.A).Hdr.Name = name
		}
	}
	return r.ResponseWriter.WriteMsg(res)
}

// Write is a wrapper that records the size of the message that gets written.
func (r *ResponseReverter) Write(buf []byte) (int, error) {
	n, err := r.ResponseWriter.Write(buf)
	return n, err
}

// Hijack implements dns.Hijacker. It simply wraps the underlying
// ResponseWriter's Hijack method if there is one, or returns an error.
func (r *ResponseReverter) Hijack() {
	r.ResponseWriter.Hijack()
	return
}
