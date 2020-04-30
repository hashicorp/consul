package pool

import (
	"bufio"
	"fmt"
	"net"
)

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
