package replacer

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin/metadata"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Replacer replaces labels for values in strings.
type Replacer struct{}

// New makes a new replacer. This only needs to be called once in the setup and
// then call Replace for each incoming message. A replacer is safe for concurrent use.
func New() Replacer {
	return Replacer{}
}

// Replace performs a replacement of values on s and returns the string with the replaced values.
func (r Replacer) Replace(ctx context.Context, state request.Request, rr *dnstest.Recorder, s string) string {
	return loadFormat(s).Replace(ctx, state, rr)
}

const (
	headerReplacer = "{>"
	// EmptyValue is the default empty value.
	EmptyValue = "-"
)

// labels are all supported labels that can be used in the default Replacer.
var labels = map[string]struct{}{
	"{type}":   {},
	"{name}":   {},
	"{class}":  {},
	"{proto}":  {},
	"{size}":   {},
	"{remote}": {},
	"{port}":   {},
	"{local}":  {},
	// Header values.
	headerReplacer + "id}":      {},
	headerReplacer + "opcode}":  {},
	headerReplacer + "do}":      {},
	headerReplacer + "bufsize}": {},
	// Recorded replacements.
	"{rcode}":                  {},
	"{rsize}":                  {},
	"{duration}":               {},
	headerReplacer + "rflags}": {},
}

// appendValue appends the current value of label.
func appendValue(b []byte, state request.Request, rr *dnstest.Recorder, label string) []byte {
	switch label {
	case "{type}":
		return append(b, state.Type()...)
	case "{name}":
		return append(b, state.Name()...)
	case "{class}":
		return append(b, state.Class()...)
	case "{proto}":
		return append(b, state.Proto()...)
	case "{size}":
		return strconv.AppendInt(b, int64(state.Req.Len()), 10)
	case "{remote}":
		return appendAddrToRFC3986(b, state.IP())
	case "{port}":
		return append(b, state.Port()...)
	case "{local}":
		return appendAddrToRFC3986(b, state.LocalIP())
	// Header placeholders (case-insensitive).
	case headerReplacer + "id}":
		return strconv.AppendInt(b, int64(state.Req.Id), 10)
	case headerReplacer + "opcode}":
		return strconv.AppendInt(b, int64(state.Req.Opcode), 10)
	case headerReplacer + "do}":
		return strconv.AppendBool(b, state.Do())
	case headerReplacer + "bufsize}":
		return strconv.AppendInt(b, int64(state.Size()), 10)
	// Recorded replacements.
	case "{rcode}":
		if rr == nil {
			return append(b, EmptyValue...)
		}
		if rcode := dns.RcodeToString[rr.Rcode]; rcode != "" {
			return append(b, rcode...)
		}
		return strconv.AppendInt(b, int64(rr.Rcode), 10)
	case "{rsize}":
		if rr == nil {
			return append(b, EmptyValue...)
		}
		return strconv.AppendInt(b, int64(rr.Len), 10)
	case "{duration}":
		if rr == nil {
			return append(b, EmptyValue...)
		}
		secs := time.Since(rr.Start).Seconds()
		return append(strconv.AppendFloat(b, secs, 'f', -1, 64), 's')
	case headerReplacer + "rflags}":
		if rr != nil && rr.Msg != nil {
			return appendFlags(b, rr.Msg.MsgHdr)
		}
		return append(b, EmptyValue...)
	default:
		return append(b, EmptyValue...)
	}
}

// appendFlags checks all header flags and appends those
// that are set as a string separated with commas
func appendFlags(b []byte, h dns.MsgHdr) []byte {
	origLen := len(b)
	if h.Response {
		b = append(b, "qr,"...)
	}
	if h.Authoritative {
		b = append(b, "aa,"...)
	}
	if h.Truncated {
		b = append(b, "tc,"...)
	}
	if h.RecursionDesired {
		b = append(b, "rd,"...)
	}
	if h.RecursionAvailable {
		b = append(b, "ra,"...)
	}
	if h.Zero {
		b = append(b, "z,"...)
	}
	if h.AuthenticatedData {
		b = append(b, "ad,"...)
	}
	if h.CheckingDisabled {
		b = append(b, "cd,"...)
	}
	if n := len(b); n > origLen {
		return b[:n-1] // trim trailing ','
	}
	return b
}

// appendAddrToRFC3986 will add brackets to the address if it is an IPv6 address.
func appendAddrToRFC3986(b []byte, addr string) []byte {
	if strings.IndexByte(addr, ':') != -1 {
		b = append(b, '[')
		b = append(b, addr...)
		b = append(b, ']')
	} else {
		b = append(b, addr...)
	}
	return b
}

type nodeType int

const (
	typeLabel    nodeType = iota // "{type}"
	typeLiteral                  // "foo"
	typeMetadata                 // "{/metadata}"
)

// A node represents a segment of a parsed format.  For example: "A {type}"
// contains two nodes: "A " (literal); and "{type}" (label).
type node struct {
	value string // Literal value, label or metadata label
	typ   nodeType
}

// A replacer is an ordered list of all the nodes in a format.
type replacer []node

func parseFormat(s string) replacer {
	// Assume there is a literal between each label - its cheaper to over
	// allocate once than allocate twice.
	rep := make(replacer, 0, strings.Count(s, "{")*2)
	for {
		// We find the right bracket then backtrack to find the left bracket.
		// This allows us to handle formats like: "{ {foo} }".
		j := strings.IndexByte(s, '}')
		if j < 0 {
			break
		}
		i := strings.LastIndexByte(s[:j], '{')
		if i < 0 {
			// Handle: "A } {foo}" by treating "A }" as a literal
			rep = append(rep, node{
				value: s[:j+1],
				typ:   typeLiteral,
			})
			s = s[j+1:]
			continue
		}

		val := s[i : j+1]
		var typ nodeType
		switch _, ok := labels[val]; {
		case ok:
			typ = typeLabel
		case strings.HasPrefix(val, "{/"):
			// Strip "{/}" from metadata labels
			val = val[2 : len(val)-1]
			typ = typeMetadata
		default:
			// Given: "A {X}" val is "{X}" expand it to the whole literal.
			val = s[:j+1]
			typ = typeLiteral
		}

		// Append any leading literal.  Given "A {type}" the literal is "A "
		if i != 0 && typ != typeLiteral {
			rep = append(rep, node{
				value: s[:i],
				typ:   typeLiteral,
			})
		}
		rep = append(rep, node{
			value: val,
			typ:   typ,
		})
		s = s[j+1:]
	}
	if len(s) != 0 {
		rep = append(rep, node{
			value: s,
			typ:   typeLiteral,
		})
	}
	return rep
}

var replacerCache sync.Map // map[string]replacer

func loadFormat(s string) replacer {
	if v, ok := replacerCache.Load(s); ok {
		return v.(replacer)
	}
	v, _ := replacerCache.LoadOrStore(s, parseFormat(s))
	return v.(replacer)
}

// bufPool stores pointers to scratch buffers.
var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 256)
	},
}

func (r replacer) Replace(ctx context.Context, state request.Request, rr *dnstest.Recorder) string {
	b := bufPool.Get().([]byte)
	for _, s := range r {
		switch s.typ {
		case typeLabel:
			b = appendValue(b, state, rr, s.value)
		case typeLiteral:
			b = append(b, s.value...)
		case typeMetadata:
			if fm := metadata.ValueFunc(ctx, s.value); fm != nil {
				b = append(b, fm()...)
			} else {
				b = append(b, EmptyValue...)
			}
		}
	}
	s := string(b)
	bufPool.Put(b[:0])
	return s
}
