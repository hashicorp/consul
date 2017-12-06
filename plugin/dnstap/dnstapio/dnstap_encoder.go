package dnstapio

import (
	"encoding/binary"
	"fmt"
	"io"

	tap "github.com/dnstap/golang-dnstap"
	fs "github.com/farsightsec/golang-framestream"
	"github.com/golang/protobuf/proto"
)

const (
	frameLenSize = 4
	protobufSize = 1024 * 1024
)

type dnstapEncoder struct {
	fse    *fs.Encoder
	opts   *fs.EncoderOptions
	writer io.Writer
	buffer *proto.Buffer
}

func newDnstapEncoder(o *fs.EncoderOptions) *dnstapEncoder {
	return &dnstapEncoder{
		opts:   o,
		buffer: proto.NewBuffer(make([]byte, 0, protobufSize)),
	}
}

func (enc *dnstapEncoder) resetWriter(w io.Writer) error {
	fse, err := fs.NewEncoder(w, enc.opts)
	if err != nil {
		return err
	}
	if err = fse.Flush(); err != nil {
		return err
	}
	enc.fse = fse
	enc.writer = w
	return nil
}

func (enc *dnstapEncoder) writeMsg(msg *tap.Dnstap) error {
	if len(enc.buffer.Bytes()) >= protobufSize {
		if err := enc.flushBuffer(); err != nil {
			return err
		}
	}
	bufLen := len(enc.buffer.Bytes())
	// add placeholder for frame length
	if err := enc.buffer.EncodeFixed32(0); err != nil {
		enc.buffer.SetBuf(enc.buffer.Bytes()[:bufLen])
		return err
	}
	if err := enc.buffer.Marshal(msg); err != nil {
		enc.buffer.SetBuf(enc.buffer.Bytes()[:bufLen])
		return err
	}
	enc.encodeFrameLen(enc.buffer.Bytes()[bufLen:])
	return nil
}

func (enc *dnstapEncoder) flushBuffer() error {
	if enc.fse == nil || enc.writer == nil {
		return fmt.Errorf("no writer")
	}

	buf := enc.buffer.Bytes()
	written := 0
	for written < len(buf) {
		n, err := enc.writer.Write(buf[written:])
		written += n
		if err != nil {
			return err
		}
	}
	enc.buffer.Reset()
	return nil
}

func (enc *dnstapEncoder) encodeFrameLen(buf []byte) {
	binary.BigEndian.PutUint32(buf, uint32(len(buf)-4))
}

func (enc *dnstapEncoder) close() error {
	if enc.fse != nil {
		return enc.fse.Close()
	}
	return nil
}
