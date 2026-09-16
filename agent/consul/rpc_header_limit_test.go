// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	netRPC "github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

// encodeHeader builds the msgpack wire bytes for a net/rpc request header.
func encodeHeader(t *testing.T, serviceMethod string, seq uint64) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, structs.MsgpackHandle)
	type rpcRequest struct {
		ServiceMethod string
		Seq           uint64
	}
	require.NoError(t, enc.Encode(rpcRequest{ServiceMethod: serviceMethod, Seq: seq}))
	return buf.Bytes()
}

func TestScanMsgpackValue(t *testing.T) {
	const max = 512

	// oversizedStr32 is a str32 header declaring ~4 GiB with no payload bytes.
	oversizedStr32 := func() []byte {
		b := []byte{0xdb, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(b[1:], 4*1024*1024*1024-1)
		return b
	}()

	// oversizedBin32 declares a 1 GiB binary value.
	oversizedBin32 := func() []byte {
		b := []byte{0xc6, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(b[1:], 1*1024*1024*1024)
		return b
	}()

	// oversizedMap32 declares a huge element count.
	oversizedMap32 := func() []byte {
		b := []byte{0xdf, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(b[1:], 500*1000*1000)
		return b
	}()

	// oversizedArray32 declares a huge element count.
	oversizedArray32 := func() []byte {
		b := []byte{0xdd, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(b[1:], 500*1000*1000)
		return b
	}()

	cases := []struct {
		name   string
		buf    []byte
		want   scanStatus
		wantOK bool // when scanDone, whether a positive consumed count is expected
	}{
		{name: "positive fixint", buf: []byte{0x01}, want: scanDone, wantOK: true},
		{name: "negative fixint", buf: []byte{0xe0}, want: scanDone, wantOK: true},
		{name: "nil", buf: []byte{0xc0}, want: scanDone, wantOK: true},
		{name: "fixstr within limit", buf: append([]byte{0xa3}, []byte("abc")...), want: scanDone, wantOK: true},
		{name: "valid header", buf: encodeHeader(t, "Catalog.Nodes", 1), want: scanDone, wantOK: true},
		{name: "empty needs more", buf: []byte{}, want: scanNeedMore},
		{name: "str8 prefix truncated needs more", buf: []byte{0xd9}, want: scanNeedMore},
		{name: "fixstr body truncated needs more", buf: []byte{0xa3, 'a'}, want: scanNeedMore},
		{name: "oversized str32", buf: oversizedStr32, want: scanTooLarge},
		{name: "oversized bin32", buf: oversizedBin32, want: scanTooLarge},
		{name: "oversized map32", buf: oversizedMap32, want: scanTooLarge},
		{name: "oversized array32", buf: oversizedArray32, want: scanTooLarge},
		{name: "oversized header via big method", buf: encodeHeader(t, strings.Repeat("X", 1024), 1), want: scanTooLarge},
		{name: "reserved 0xc1 is invalid", buf: []byte{0xc1}, want: scanTooLarge},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, status := scanMsgpackValue(tc.buf, max, 0)
			require.Equal(t, tc.want, status, "unexpected status (consumed=%d)", n)
			if status == scanDone && tc.wantOK {
				require.Greater(t, n, 0)
				require.LessOrEqual(t, n, max)
			}
			if status == scanNeedMore {
				require.Greater(t, n, 0)
				require.LessOrEqual(t, n, max, "needMore must never request more than the cap")
			}
		})
	}
}

// pipeCodec wires a boundedHeaderCodec to an in-memory connection and returns
// the client side for writing raw bytes.
func newPipeCodec(t *testing.T, maxHeaderBytes int) (*boundedHeaderCodec, net.Conn) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() {
		server.Close()
		client.Close()
	})
	c := newBoundedHeaderCodec(server, structs.MsgpackHandle, maxHeaderBytes)
	return c, client
}

func TestBoundedHeaderCodec_ReadRequestHeader(t *testing.T) {
	t.Run("normal header decodes", func(t *testing.T) {
		c, client := newPipeCodec(t, 512)
		go func() {
			client.Write(encodeHeader(t, "Catalog.Nodes", 7))
		}()

		var req netRPC.Request
		require.NoError(t, c.ReadRequestHeader(&req))
		require.Equal(t, "Catalog.Nodes", req.ServiceMethod)
		require.Equal(t, uint64(7), req.Seq)
	})

	t.Run("oversized header is rejected before allocation", func(t *testing.T) {
		c, client := newPipeCodec(t, 512)
		// Send only the str32 length prefix — no payload. A vulnerable decoder
		// would allocate ~4 GiB here; the guard must reject from the prefix
		// alone without reading (or allocating) the body.
		go func() {
			hdr := []byte{0x81, 0xad}
			hdr = append(hdr, []byte("ServiceMethod")...)
			prefix := []byte{0xdb, 0, 0, 0, 0}
			binary.BigEndian.PutUint32(prefix[1:], 4*1024*1024*1024-1)
			client.Write(append(hdr, prefix...))
		}()

		var req netRPC.Request
		err := c.ReadRequestHeader(&req)
		require.ErrorIs(t, err, errHeaderTooLarge)
	})

	t.Run("body may exceed the header cap", func(t *testing.T) {
		c, client := newPipeCodec(t, 512)
		// A large body (> cap) must be allowed once the header is accepted.
		body := strings.Repeat("z", 4096)
		go func() {
			client.Write(encodeHeader(t, "A.B", 1))
			enc := codec.NewEncoder(client, structs.MsgpackHandle)
			enc.Encode(body)
		}()

		var req netRPC.Request
		require.NoError(t, c.ReadRequestHeader(&req))
		require.Equal(t, "A.B", req.ServiceMethod)

		var got string
		require.NoError(t, c.ReadRequestBody(&got))
		require.Equal(t, body, got)
	})

	t.Run("clean close returns EOF", func(t *testing.T) {
		c, client := newPipeCodec(t, 512)
		go func() {
			time.Sleep(10 * time.Millisecond)
			client.Close()
		}()

		var req netRPC.Request
		err := c.ReadRequestHeader(&req)
		require.Error(t, err)
	})

	t.Run("large configured cap accepts a large header", func(t *testing.T) {
		// A cap above bufio's default buffer size must not spuriously reject a
		// validly-sized header via ErrBufferFull.
		const cap = 8192
		c, client := newPipeCodec(t, cap)
		method := "Svc." + strings.Repeat("m", 6000)
		go func() {
			client.Write(encodeHeader(t, method, 3))
		}()

		var req netRPC.Request
		require.NoError(t, c.ReadRequestHeader(&req))
		require.Equal(t, method, req.ServiceMethod)
	})
}
