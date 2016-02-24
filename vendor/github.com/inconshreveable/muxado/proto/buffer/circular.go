package buffer

import (
	"errors"
	"io"
)

var (
	FullError = errors.New("Buffer is full")
)

// Reads as much data
func readInto(rd io.Reader, p []byte) (n int, err error) {
	var nr int
	for n < len(p) {
		nr, err = rd.Read(p[n:])
		n += nr
		if err != nil {
			return
		}
	}
	return
}

// A circular buffer on top of a byte-array
// NOTE: It does not implement the Write() method, it implements ReadFrom()
// to avoid copies
type Circular struct {
	buf  []byte // the bytes
	size int    // == len(buf)
	head int    // index of the next byte to read
	tail int    // index of the last byte available to read
}

// Returns a new circular buffer of the given size
func NewCircular(size int) *Circular {
	return &Circular{
		buf:  make([]byte, size+1),
		size: size + 1,
	}
}

// Copy data from the given reader into the buffer
// Any errors encountered while reading are returned EXCEPT io.EOF.
// If the reader fills the buffer, it returns buffer.FullError
func (c *Circular) ReadFrom(rd io.Reader) (n int, err error) {
	// IF:
	// [---H+++T--]
	if c.tail >= c.head {
		n, err = readInto(rd, c.buf[c.tail:])
		c.tail = (c.tail + n) % c.size
		if err == io.EOF {
			return n, nil
		} else if err != nil {
			return
		}
	}

	// NOW:
	// [T---H++++] or [++T--H+++]
	n2, err := readInto(rd, c.buf[c.tail:c.head])
	n += n2
	c.tail += n2
	if err == nil {
		err = FullError
	} else if err == io.EOF {
		err = nil
	}
	return
}

// Read data out of the buffer. This never fails but may
// return n==0 if there is no data to be read
func (c *Circular) Read(p []byte) (n int, err error) {
	if c.head > c.tail {
		n = copy(p, c.buf[c.head:])
		c.head = (c.head + n) % c.size
		if c.head != 0 {
			return
		}
	}

	n2 := copy(p[n:], c.buf[c.head:c.tail])
	n += n2
	c.head += n2
	return
}
