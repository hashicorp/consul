# muxado - Stream multiplexing for Go

## What is stream multiplexing?
Imagine you have a single stream (a bi-directional stream of bytes) like a TCP connection. Stream multiplexing
is a method for enabling the transmission of multiple simultaneous streams over the one underlying transport stream.

## What is muxado?
muxado is an implementation of a stream multiplexing library in Go that can be layered on top of a net.Conn to multiplex that stream.
muxado's protocol is not currently documented explicitly, but it is very nearly an implementation of the HTTP2
framing layer with all of the HTTP-specific bits removed. It is heavily inspired by HTTP2, SPDY, and WebMUX.

## How does it work?
Simplifying, muxado chunks data sent over each multiplexed stream and transmits each piece
as a "frame" over the transport stream. It then sends these frames,
often interleaving data for multiple streams, to the remote side.
The remote endpoint then reassembles the frames into distinct streams
of data which are presented to the application layer.

## What good is it anyways?
A stream multiplexing library is a powerful tool for an application developer's toolbox which solves a number of problems:

- It allows developers to implement asynchronous/pipelined protocols with ease. Instead of matching requests with responses in your protocols, just open a new stream for each request and communicate over that.
- muxado can do application-level keep-alives and dead-session detection so that you don't have to write heartbeat code ever again.
- You never need to build connection pools for services running your protocol. You can open as many independent, concurrent streams as you need without incurring any round-trip latency costs.
- muxado allows the server to initiate new streams to clients which is normally very difficult without NAT-busting trickery.

## Show me the code!
As much as possible, the muxado library strives to look and feel just like the standard library's net package. Here's how you initiate a new client session:

    sess, err := muxado.DialTLS("tcp", "example.com:1234", tlsConfig)
    
And a server:

    l, err := muxado.ListenTLS("tcp", ":1234", tlsConfig))
    for {
        sess, err := l.Accept()
        go handleSession(sess)
    }

Once you have a session, you can open new streams on it:

    stream, err := sess.Open()

And accept streams opened by the remote side:

    stream, err := sess.Accept()

Streams satisfy the net.Conn interface, so they're very familiar to work with:
    
    n, err := stream.Write(buf)
    n, err = stream.Read(buf)
    
muxado sessions and streams implement the net.Listener and net.Conn interfaces (with a small shim), so you can use them with existing golang libraries!

    sess, err := muxado.DialTLS("tcp", "example.com:1234", tlsConfig)
    http.Serve(sess.NetListener(), handler)

## A more extensive muxado client

    // open a new session to a remote endpoint
    sess, err := muxado.Dial("tcp", "example.com:1234")
    if err != nil {
	    panic(err)
    }

    // handle streams initiated by the server
    go func() {
	    for {
		    stream, err := sess.Accept()
		    if err != nil {
			    panic(err)
		    }

		    go handleStream(stream)
	    }
    }()

    // open new streams for application requests
    for req := range requests {
	    str, err := sess.Open()
	    if err != nil {
		    panic(err)
	    }

	    go func(stream muxado.Stream) {
		    defer stream.Close()

		    // send request
		    if _, err = stream.Write(req.serialize()); err != nil {
			    panic(err)
		    }

		    // read response
		    if buf, err := ioutil.ReadAll(stream); err != nil {
			    panic(err)
		    }

		    handleResponse(buf)
	    }(str)
    }

## How did you build it?
muxado is a modified implementation of the HTTP2 framing protocol with all of the HTTP-specific bits removed. It aims
for simplicity in the protocol by removing everything that is not core to multiplexing streams. The muxado code
is also built with the intention that its performance should be moderately good within the bounds of working in Go. As a result,
muxado does contain some unidiomatic code.

## API documentation
API documentation is available on godoc.org:

[muxado API documentation](https://godoc.org/github.com/inconshreveable/muxado)

## What are its biggest drawbacks?
Any stream-multiplexing library over TCP will suffer from head-of-line blocking if the next packet to service gets dropped.
muxado is also a poor choice when sending large payloads and speed is a priority.
It shines best when the application workload needs to quickly open a large number of small-payload streams.

## Status
Most of muxado's features are implemented (and tested!), but there are many that are still rough or could be improved. See the TODO file for suggestions on what needs to improve.

## License
Apache
