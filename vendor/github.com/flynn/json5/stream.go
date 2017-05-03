// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json5

import (
	"bytes"
	"errors"
	"io"
)

// A Decoder reads and decodes JSON values from an input stream.
type Decoder struct {
	r     io.Reader
	buf   []byte
	d     decodeState
	scanp int // start of unread data in buf
	scan  scanner
	err   error

	tokenState int
	tokenStack []int
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the JSON values requested.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// UseNumber causes the Decoder to unmarshal a number into an interface{} as a
// Number instead of as a float64.
func (dec *Decoder) UseNumber() { dec.d.useNumber = true }

// Decode reads the next JSON-encoded value from its
// input and stores it in the value pointed to by v.
//
// See the documentation for Unmarshal for details about
// the conversion of JSON into a Go value.
func (dec *Decoder) Decode(v interface{}) error {
	if dec.err != nil {
		return dec.err
	}

	// Read whole value into buffer.
	n, err := dec.readValue()
	if err != nil {
		return err
	}
	dec.d.init(dec.buf[dec.scanp : dec.scanp+n])
	dec.scanp += n

	// Don't save err from unmarshal into dec.err:
	// the connection is still usable since we read a complete JSON
	// object from it before the error happened.
	err = dec.d.unmarshal(v)

	return err
}

// Buffered returns a reader of the data remaining in the Decoder's
// buffer. The reader is valid until the next call to Decode.
func (dec *Decoder) Buffered() io.Reader {
	return bytes.NewReader(dec.buf[dec.scanp:])
}

// readValue reads a JSON value into dec.buf.
// It returns the length of the encoding.
func (dec *Decoder) readValue() (int, error) {
	dec.scan.reset()

	scanp := dec.scanp
	var err error
Input:
	for {
		// Look in the buffer for a new value.
		for i, c := range dec.buf[scanp:] {
			dec.scan.bytes++
			v := dec.scan.step(&dec.scan, c)
			if v == scanEnd {
				scanp += i
				break Input
			}
			// scanEnd is delayed one byte.
			// We might block trying to get that byte from src,
			// so instead invent a space byte.
			if (v == scanEndObject || v == scanEndArray) && dec.scan.step(&dec.scan, ' ') == scanEnd {
				scanp += i + 1
				break Input
			}
			if v == scanError {
				dec.err = dec.scan.err
				return 0, dec.scan.err
			}
		}
		scanp = len(dec.buf)

		// Did the last read have an error?
		// Delayed until now to allow buffer scan.
		if err != nil {
			if err == io.EOF {
				if dec.scan.step(&dec.scan, ' ') == scanEnd {
					break Input
				}
				if nonSpace(dec.buf) {
					err = io.ErrUnexpectedEOF
				}
			}
			dec.err = err
			return 0, err
		}

		n := scanp - dec.scanp
		err = dec.refill()
		scanp = dec.scanp + n
	}
	return scanp - dec.scanp, nil
}

func (dec *Decoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if dec.scanp > 0 {
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}

	// Grow buffer if not large enough.
	const minRead = 512
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		dec.buf = newBuf
	}

	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[0 : len(dec.buf)+n]

	return err
}

func nonSpace(b []byte) bool {
	for _, c := range b {
		if !isSpace(c) {
			return true
		}
	}
	return false
}

// RawMessage is a raw encoded JSON value.
// It implements Marshaler and Unmarshaler and can
// be used to delay JSON decoding or precompute a JSON encoding.
type RawMessage []byte

// UnmarshalJSON sets *m to a copy of data.
func (m *RawMessage) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalJSON on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}

var _ Unmarshaler = (*RawMessage)(nil)
