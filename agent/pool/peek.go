package pool

import (
	"bufio"
	"fmt"
	"net"
)

// PeekForTLS will read the first byte on the conn to determine if the client
// request is a TLS connection request or a consul-specific framed rpc request.
//
// This function does not close the conn on an error.
//
// The returned conn has the initial read buffered internally for the purposes
// of not consuming the first byte. After that buffer is drained the conn is a
// pass through to the original conn.
//
// The TLS record layer governs the very first byte. The available options start
// at 20 as per:
//
//   - v1.2: https://tools.ietf.org/html/rfc5246#appendix-A.1
//   - v1.3: https://tools.ietf.org/html/rfc8446#appendix-B.1
//
// Note: this indicates that '0' is 'invalid'. Given that we only care about
// the first byte of a long-lived connection this is irrelevant, since you must
// always start out with a client hello handshake which is '22'.
func PeekForTLS(conn net.Conn) (net.Conn, bool, error) {
	br := bufio.NewReader(conn)

	// Grab enough to read the first byte. Then drain the buffer so future
	// reads can be direct.
	peeked, err := br.Peek(1)
	if err != nil {
		return nil, false, err
	} else if len(peeked) == 0 {
		return conn, false, nil
	}

	peeked, err = br.Peek(br.Buffered())
	if err != nil {
		return nil, false, err
	}

	isTLS := (peeked[0] > RPCMaxTypeValue)

	return &peekedConn{
		Peeked: peeked,
		Conn:   conn,
	}, isTLS, nil
}

// PeekFirstByte will read the first byte on the conn.
//
// This function does not close the conn on an error.
//
// The returned conn has the initial read buffered internally for the purposes
// of not consuming the first byte. After that buffer is drained the conn is a
// pass through to the original conn.
func PeekFirstByte(conn net.Conn) (net.Conn, byte, error) {
	br := bufio.NewReader(conn)

	// Grab enough to read the first byte. Then drain the buffer so future
	// reads can be direct.
	peeked, err := br.Peek(1)
	if err != nil {
		return nil, 0, err
	} else if len(peeked) == 0 {
		return conn, 0, fmt.Errorf("nothing to read")
	}
	peeked, err = br.Peek(br.Buffered())
	if err != nil {
		return nil, 0, err
	}

	return &peekedConn{
		Peeked: peeked,
		Conn:   conn,
	}, peeked[0], nil
}
