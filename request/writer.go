package request

import "github.com/miekg/dns"

// ScrubWriter will, when writing the message, call scrub to make it fit the client's buffer.
type ScrubWriter struct {
	dns.ResponseWriter
	req *dns.Msg // original request
}

// NewScrubWriter returns a new and initialized ScrubWriter.
func NewScrubWriter(req *dns.Msg, w dns.ResponseWriter) *ScrubWriter { return &ScrubWriter{w, req} }

// WriteMsg overrides the default implementation of the underlaying dns.ResponseWriter and calls
// scrub on the message m and will then write it to the client.
func (s *ScrubWriter) WriteMsg(m *dns.Msg) error {
	state := Request{Req: s.req, W: s.ResponseWriter}
	n := state.Scrub(m)
	return s.ResponseWriter.WriteMsg(n)
}
