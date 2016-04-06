package middleware

import (
	"strconv"
	"time"

	"github.com/miekg/dns"
)

// ResponseRecorder is a type of ResponseWriter that captures
// the rcode code written to it and also the size of the message
// written in the response. A rcode code does not have
// to be written, however, in which case 0 must be assumed.
// It is best to have the constructor initialize this type
// with that default status code.
type ResponseRecorder struct {
	dns.ResponseWriter
	rcode int
	size  int
	msg   *dns.Msg
	start time.Time
}

// NewResponseRecorder makes and returns a new responseRecorder,
// which captures the DNS rcode from the ResponseWriter
// and also the length of the response message written through it.
func NewResponseRecorder(w dns.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		rcode:          0,
		msg:            nil,
		start:          time.Now(),
	}
}

// WriteMsg records the status code and calls the
// underlying ResponseWriter's WriteMsg method.
func (r *ResponseRecorder) WriteMsg(res *dns.Msg) error {
	r.rcode = res.Rcode
	// We may get called multiple times (axfr for instance).
	// Save the last message, but add the sizes.
	r.size += res.Len()
	r.msg = res
	return r.ResponseWriter.WriteMsg(res)
}

// Write is a wrapper that records the size of the message that gets written.
func (r *ResponseRecorder) Write(buf []byte) (int, error) {
	n, err := r.ResponseWriter.Write(buf)
	if err == nil {
		r.size += n
	}
	return n, err
}

// Size returns the size.
func (r *ResponseRecorder) Size() int {
	return r.size
}

// Rcode returns the rcode.
func (r *ResponseRecorder) Rcode() string {
	if rcode, ok := dns.RcodeToString[r.rcode]; ok {
		return rcode
	}
	return "RCODE" + strconv.Itoa(r.rcode)
}

// Start returns the start time of the ResponseRecorder.
func (r *ResponseRecorder) Start() time.Time {
	return r.start
}

// Msg returns the written message from the ResponseRecorder.
func (r *ResponseRecorder) Msg() *dns.Msg {
	return r.msg
}

// Hijack implements dns.Hijacker. It simply wraps the underlying
// ResponseWriter's Hijack method if there is one, or returns an error.
func (r *ResponseRecorder) Hijack() {
	r.ResponseWriter.Hijack()
	return
}
