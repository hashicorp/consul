// muxado is an implementation of a general-purpose stream-multiplexing protocol.
//
// muxado allows clients applications to multiplex a single stream-oriented connection,
// like a TCP connection, and communicate over many streams on top of it. muxado accomplishes
// this by chunking data sent over each stream into frames and then reassembling the
// frames and buffering the data before being passed up to the application
// layer on the other side.
//
// muxado is very nearly an exact implementation of the HTTP2 framing layer while leaving out all
// the HTTP-specific parts. It is heavily inspired by HTTP2/SPDY/WebMUX.
//
// muxado's documentation uses the following terms consistently for easier communication:
// - "a transport" is an underlying stream (typically TCP) over which frames are sent between
// endpoints
// - "a stream" is any of the full-duplex byte-streams multiplexed over the transport
// - "a session" refers to an instance of the muxado protocol running over a transport between
// two endpoints
//
// Perhaps the best part of muxado is the interface exposed to client libraries. Since new
// streams may be initiated by both sides at any time, a muxado.Session implements the net.Listener
// interface (almost! Go unfortunately doesn't support covariant interface satisfaction so there's
// a shim). Each muxado stream implements the net.Conn interface. This allows you to integrate
// muxado into existing code which works with these interfaces (which is most Golang networking code)
// with very little difficulty. Consider the following toy example. Here we'll initiate a new secure
// connection to a server, and then ask it which application it wants via an HTTP request over a muxado stream
// and then serve an entire HTTP application *to the server*.
//
//
// 	sess, err := muxado.DialTLS("tcp", "example.com:1234", new(tls.Config))
// 	client := &http.Client{Transport: &http.Transport{Dial: sess.NetDial}}
// 	resp, err := client.Get("http://example.com/appchoice")
// 	switch getChoice(resp.Body) {
// 	case "foo":
// 		http.Serve(sess.NetListener(), fooHandler)
// 	case "bar":
//		http.Serve(sess.NetListener(), barHandler)
// 	}
//
//
// In addition to enabling multiple streams over a single connection, muxado enables other
// behaviors which can be useful to the application layer:
// - Both sides of a muxado session may initiate new streams
// - muxado can transparently run application-level heartbeats and timeout dead sessions
// - When connections fail, muxado indicates to the application which streams may be safely retried
// - muxado supports prioritizing streams to maximize useful throughput when bandwidth-constrained
//
// A few examples of what these capabilities might make muxado useful for:
// - eliminating custom async/pipeling code for your protocols
// - eliminating connection pools in your protocols
// - eliminating custom NAT traversal logic for enabling server-initiated streams
//
// muxado has been tuned to be very performant within the limits of what you can expect of pure-Go code.
// Some of muxado's code looks unidiomatic in the quest for better performance. (Locks over channels, never allocating
// from the heap, etc). muxado will typically outperform TCP connections when rapidly initiating many new
// streams with small payloads. When sending a large payload over a single stream, muxado's worst case, it can
// be 2-3x slower and does not parallelize well.
package muxado
