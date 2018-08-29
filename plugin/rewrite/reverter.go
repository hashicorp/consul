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
	Type        string
	Pattern     *regexp.Regexp
	Replacement string
	Ttl         uint32
}

// ResponseReverter reverses the operations done on the question section of a packet.
// This is need because the client will otherwise disregards the response, i.e.
// dig will complain with ';; Question section mismatch: got example.org/HINFO/IN'
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

// WriteMsg records the status code and calls the underlying ResponseWriter's WriteMsg method.
func (r *ResponseReverter) WriteMsg(res *dns.Msg) error {
	res.Question[0] = r.originalQuestion
	if r.ResponseRewrite {
		for _, rr := range res.Answer {
			var isNameRewritten bool = false
			var isTtlRewritten bool = false
			var name string = rr.Header().Name
			var ttl uint32 = rr.Header().Ttl
			for _, rule := range r.ResponseRules {
				if rule.Type == "" {
					rule.Type = "name"
				}
				switch rule.Type {
				case "name":
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
					isNameRewritten = true
				case "ttl":
					ttl = rule.Ttl
					isTtlRewritten = true
				}
			}
			if isNameRewritten == true {
				rr.Header().Name = name
			}
			if isTtlRewritten == true {
				rr.Header().Ttl = ttl
			}
		}
	}
	return r.ResponseWriter.WriteMsg(res)
}

// Write is a wrapper that records the size of the message that gets written.
func (r *ResponseReverter) Write(buf []byte) (int, error) {
	n, err := r.ResponseWriter.Write(buf)
	return n, err
}
