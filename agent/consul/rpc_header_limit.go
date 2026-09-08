// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"bufio"
	"encoding/binary"
	"errors"
	"net"
	"sync"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	netRPC "github.com/hashicorp/consul-net-rpc/net/rpc"
)

// rpcMaxHeaderBytes is used when RPCMaxHeaderBytes is not configured. It bounds
// the encoded size of a single RPC request header (ServiceMethod + Seq) as well
// as any length prefix contained within it. A valid ServiceMethod is far below
// this; 512 bytes is already extremely generous.
const rpcMaxHeaderBytes = 512

// maxHeaderScanDepth bounds nesting while validating a request header. A well
// formed header is a flat map, so this is only a guard against pathological or
// malicious nesting.
const maxHeaderScanDepth = 32

// defaultBufioReaderSize matches bufio.NewReader's default buffer size. The
// header read buffer is never sized below this.
const defaultBufioReaderSize = 4096

// errHeaderTooLarge is returned when an RPC request header declares a length
// prefix (or an overall size) larger than the configured maximum. It is
// returned before the MessagePack decoder allocates memory for the value, which
// is what prevents the pre-authorization memory-exhaustion vector: a crafted
// str32/bin32/array32/map32 length no longer drives a large allocation.
var errHeaderTooLarge = errors.New("rpc: request header exceeds maximum allowed size")

// boundedHeaderCodec is a net/rpc ServerCodec that bounds the size of each
// request header before it is decoded, then decodes the body without any size
// restriction (legitimate request bodies may be large).
//
// The MessagePack decoder allocates a byte slice of the declared length before
// reading the value from the connection (see go-msgpack ioDecReader.readn:
// make([]byte, n) precedes the read). An io.LimitedReader cannot prevent this,
// because the allocation happens before any read is attempted. Instead, this
// codec validates every length prefix in the request header against
// maxHeaderBytes *before* handing the header bytes to the decoder. If any
// prefix (or the header as a whole) exceeds the cap, the header is rejected and
// the connection is closed before method lookup, rate limiting, or ACL
// evaluation runs.
//
// Header and body are read through a single bufio.Reader so no bytes are lost
// between the validation peek and the decode.
type boundedHeaderCodec struct {
	conn           net.Conn
	br             *bufio.Reader
	dec            *codec.Decoder
	enc            *codec.Encoder
	bufW           *bufio.Writer
	maxHeaderBytes int

	writeLock sync.Mutex

	closeOnce sync.Once
	closeErr  error
}

// newBoundedHeaderCodec builds a ServerCodec for conn that enforces a per
// request header byte cap. A non-positive maxHeaderBytes falls back to
// rpcMaxHeaderBytes.
func newBoundedHeaderCodec(conn net.Conn, h *codec.MsgpackHandle, maxHeaderBytes int) *boundedHeaderCodec {
	if maxHeaderBytes <= 0 {
		maxHeaderBytes = rpcMaxHeaderBytes
	}
	// The header validator may Peek up to maxHeaderBytes bytes, so the read
	// buffer must be at least that large or bufio would return ErrBufferFull
	// for a validly-sized header. Never shrink below bufio's default.
	readBufSize := maxHeaderBytes + 1
	if readBufSize < defaultBufioReaderSize {
		readBufSize = defaultBufioReaderSize
	}
	br := bufio.NewReaderSize(conn, readBufSize)
	bufW := bufio.NewWriter(conn)
	return &boundedHeaderCodec{
		conn:           conn,
		br:             br,
		dec:            codec.NewDecoder(br, h),
		enc:            codec.NewEncoder(bufW, h),
		bufW:           bufW,
		maxHeaderBytes: maxHeaderBytes,
	}
}

// ReadRequestHeader validates that the incoming request header fits within the
// configured byte cap before decoding it. Validation only inspects length
// prefixes; the trusted decoder then performs the actual decode, now guaranteed
// not to allocate more than maxHeaderBytes for the header.
func (c *boundedHeaderCodec) ReadRequestHeader(r *netRPC.Request) error {
	if err := c.validateHeaderSize(); err != nil {
		return err
	}
	return c.dec.Decode(r)
}

// validateHeaderSize peeks (without consuming) at the buffered header bytes and
// walks the MessagePack structure, rejecting any length prefix or overall size
// beyond maxHeaderBytes. It only ever requests up to maxHeaderBytes bytes, so a
// slow or oversized header cannot force it to buffer without bound.
func (c *boundedHeaderCodec) validateHeaderSize() error {
	need := 1
	for {
		if need > c.maxHeaderBytes {
			return errHeaderTooLarge
		}
		buf, err := c.br.Peek(need)
		if len(buf) < need {
			// bufio.Peek returns fewer bytes than requested only together
			// with an error (EOF, unexpected EOF, timeout, or a closed
			// connection). Surface it so the server closes the connection.
			if err != nil {
				return err
			}
			return errHeaderTooLarge
		}
		consumed, status := scanMsgpackValue(buf, c.maxHeaderBytes, 0)
		switch status {
		case scanDone:
			return nil
		case scanNeedMore:
			// The scanner must always ask for strictly more bytes than we
			// already have, otherwise the loop could stall. Treat any
			// non-progress as a malformed/oversized header.
			if consumed <= need {
				return errHeaderTooLarge
			}
			need = consumed
		default: // scanTooLarge / malformed
			return errHeaderTooLarge
		}
	}
}

// ReadRequestBody decodes the request body. Bodies are intentionally not size
// bounded here: they are decoded only after method lookup and the request rate
// limiter have run, and legitimate requests may carry large bodies.
func (c *boundedHeaderCodec) ReadRequestBody(body interface{}) error {
	if body == nil {
		// net/rpc asks us to discard the body (e.g. unknown method).
		var throwaway interface{}
		return c.dec.Decode(&throwaway)
	}
	return c.dec.Decode(body)
}

// WriteResponse encodes the response header and body back to the client.
func (c *boundedHeaderCodec) WriteResponse(r *netRPC.Response, body interface{}) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	if err := c.enc.Encode(r); err != nil {
		return err
	}
	if err := c.enc.Encode(body); err != nil {
		return err
	}
	return c.bufW.Flush()
}

// SourceAddr returns the client's remote address, used by the request rate
// limiter.
func (c *boundedHeaderCodec) SourceAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Close is idempotent, as required by the ServerCodec contract.
func (c *boundedHeaderCodec) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

// scanStatus reports the outcome of scanning a single MessagePack value.
type scanStatus int

const (
	// scanDone means a complete value was validated within the provided bytes.
	scanDone scanStatus = iota
	// scanNeedMore means more bytes are required; the returned count is the
	// total number of bytes that must be available to make progress and is
	// always <= max.
	scanNeedMore
	// scanTooLarge means a length prefix or the overall size exceeds max, or
	// the input is malformed. In every case the header must be rejected.
	scanTooLarge
)

// scanMsgpackValue validates a single MessagePack value at the start of buf
// without allocating memory for its contents. It enforces that no declared
// length (string, binary, array, map, or extension) and no cumulative offset
// exceeds max. This runs before the real decoder, so an oversized length prefix
// is rejected before make([]byte, n) can be reached.
//
// On success it returns the number of bytes the value occupies and scanDone. If
// buf is too short it returns the number of bytes needed (<= max) and
// scanNeedMore. If the value is oversized or malformed it returns scanTooLarge.
func scanMsgpackValue(buf []byte, max, depth int) (int, scanStatus) {
	if depth > maxHeaderScanDepth {
		return 0, scanTooLarge
	}
	if max < 1 {
		// No budget remaining: any value needs at least one byte.
		return 0, scanTooLarge
	}
	if len(buf) < 1 {
		return 1, scanNeedMore
	}
	b := buf[0]

	switch {
	case b <= 0x7f, b >= 0xe0:
		// positive or negative fixint
		return 1, scanDone
	case b >= 0x80 && b <= 0x8f:
		// fixmap
		return scanMap(buf, 1, int(b&0x0f), max, depth)
	case b >= 0x90 && b <= 0x9f:
		// fixarray
		return scanArray(buf, 1, int(b&0x0f), max, depth)
	case b >= 0xa0 && b <= 0xbf:
		// fixstr
		return scanRaw(buf, 1, int(b&0x1f), max)
	}

	switch b {
	case 0xc0, 0xc2, 0xc3:
		// nil, false, true
		return 1, scanDone
	case 0xcc, 0xd0:
		return scanFixed(buf, 1+1, max) // uint8 / int8
	case 0xcd, 0xd1:
		return scanFixed(buf, 1+2, max) // uint16 / int16
	case 0xce, 0xd2, 0xca:
		return scanFixed(buf, 1+4, max) // uint32 / int32 / float32
	case 0xcf, 0xd3, 0xcb:
		return scanFixed(buf, 1+8, max) // uint64 / int64 / float64
	case 0xd9, 0xc4:
		return scanLenPrefixed(buf, 1, 1, max) // str8 / bin8
	case 0xda, 0xc5:
		return scanLenPrefixed(buf, 1, 2, max) // str16 / bin16
	case 0xdb, 0xc6:
		return scanLenPrefixed(buf, 1, 4, max) // str32 / bin32
	case 0xdc:
		return scanArrayHeader(buf, 1, 2, max, depth) // array16
	case 0xdd:
		return scanArrayHeader(buf, 1, 4, max, depth) // array32
	case 0xde:
		return scanMapHeader(buf, 1, 2, max, depth) // map16
	case 0xdf:
		return scanMapHeader(buf, 1, 4, max, depth) // map32
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8:
		// fixext1/2/4/8/16: 1 type byte + 2^(b-0xd4) data bytes.
		return scanFixed(buf, 1+1+(1<<(b-0xd4)), max)
	case 0xc7:
		return scanExt(buf, 1, max) // ext8
	case 0xc8:
		return scanExt(buf, 2, max) // ext16
	case 0xc9:
		return scanExt(buf, 4, max) // ext32
	default:
		// 0xc1 is never used in MessagePack.
		return 0, scanTooLarge
	}
}

// readUintPrefix reads a big-endian unsigned integer of lenBytes at offset off.
func readUintPrefix(buf []byte, off, lenBytes int) (uint64, bool) {
	if len(buf) < off+lenBytes {
		return 0, false
	}
	switch lenBytes {
	case 1:
		return uint64(buf[off]), true
	case 2:
		return uint64(binary.BigEndian.Uint16(buf[off:])), true
	case 4:
		return uint64(binary.BigEndian.Uint32(buf[off:])), true
	default:
		return 0, false
	}
}

// scanFixed validates a value of exactly size bytes.
func scanFixed(buf []byte, size, max int) (int, scanStatus) {
	if size > max {
		return 0, scanTooLarge
	}
	if len(buf) < size {
		return size, scanNeedMore
	}
	return size, scanDone
}

// scanRaw validates a string/binary body of clen bytes that starts at
// dataOffset (i.e. the length prefix has already been consumed).
func scanRaw(buf []byte, dataOffset, clen, max int) (int, scanStatus) {
	total := dataOffset + clen
	if clen < 0 || total > max {
		return 0, scanTooLarge
	}
	if len(buf) < total {
		return total, scanNeedMore
	}
	return total, scanDone
}

// scanLenPrefixed validates a string/binary value whose length is encoded in
// prefixLen bytes immediately after the type byte at typeLen.
func scanLenPrefixed(buf []byte, typeLen, prefixLen, max int) (int, scanStatus) {
	headerLen := typeLen + prefixLen
	if headerLen > max {
		return 0, scanTooLarge
	}
	clen, ok := readUintPrefix(buf, typeLen, prefixLen)
	if !ok {
		return headerLen, scanNeedMore
	}
	if clen > uint64(max) {
		return 0, scanTooLarge
	}
	return scanRaw(buf, headerLen, int(clen), max)
}

// scanExt validates an ext8/16/32 value: prefixLen length bytes, 1 type byte,
// then the payload.
func scanExt(buf []byte, prefixLen, max int) (int, scanStatus) {
	headerLen := 1 + prefixLen + 1 // type byte + length + ext type byte
	if headerLen > max {
		return 0, scanTooLarge
	}
	clen, ok := readUintPrefix(buf, 1, prefixLen)
	if !ok {
		return headerLen, scanNeedMore
	}
	if clen > uint64(max) {
		return 0, scanTooLarge
	}
	return scanRaw(buf, headerLen, int(clen), max)
}

// scanArray validates count elements of an array whose header has already been
// consumed up to headerLen bytes.
func scanArray(buf []byte, headerLen, count, max, depth int) (int, scanStatus) {
	return scanElements(buf, headerLen, count, max, depth)
}

// scanMap validates count key/value pairs of a map whose header has already
// been consumed up to headerLen bytes.
func scanMap(buf []byte, headerLen, count, max, depth int) (int, scanStatus) {
	return scanElements(buf, headerLen, count*2, max, depth)
}

// scanArrayHeader reads an array16/array32 element count then validates the
// elements.
func scanArrayHeader(buf []byte, typeLen, prefixLen, max, depth int) (int, scanStatus) {
	headerLen := typeLen + prefixLen
	if headerLen > max {
		return 0, scanTooLarge
	}
	count, ok := readUintPrefix(buf, typeLen, prefixLen)
	if !ok {
		return headerLen, scanNeedMore
	}
	if count > uint64(max) {
		// Each element needs at least one byte, so it cannot fit in max.
		return 0, scanTooLarge
	}
	return scanElements(buf, headerLen, int(count), max, depth)
}

// scanMapHeader reads a map16/map32 pair count then validates the pairs.
func scanMapHeader(buf []byte, typeLen, prefixLen, max, depth int) (int, scanStatus) {
	headerLen := typeLen + prefixLen
	if headerLen > max {
		return 0, scanTooLarge
	}
	count, ok := readUintPrefix(buf, typeLen, prefixLen)
	if !ok {
		return headerLen, scanNeedMore
	}
	if count > uint64(max/2) {
		return 0, scanTooLarge
	}
	return scanElements(buf, headerLen, int(count)*2, max, depth)
}

// scanElements validates n consecutive values starting at offset off.
func scanElements(buf []byte, off, n, max, depth int) (int, scanStatus) {
	for i := 0; i < n; i++ {
		if off > max {
			return 0, scanTooLarge
		}
		if len(buf) <= off {
			return off + 1, scanNeedMore
		}
		consumed, status := scanMsgpackValue(buf[off:], max-off, depth+1)
		switch status {
		case scanDone:
			off += consumed
		case scanNeedMore:
			return off + consumed, scanNeedMore
		default:
			return 0, scanTooLarge
		}
	}
	if off > max {
		return 0, scanTooLarge
	}
	return off, scanDone
}
