/*
 * Copyright (c) 2014 by Farsight Security, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package framestream

import (
	"bufio"
	"encoding/binary"
	"io"
)

type DecoderOptions struct {
	MaxPayloadSize uint32
	ContentType    []byte
	Bidirectional  bool
}

type Decoder struct {
	buf     []byte
	opt     DecoderOptions
	reader  *bufio.Reader
	writer  *bufio.Writer
	stopped bool
}

func NewDecoder(r io.Reader, opt *DecoderOptions) (dec *Decoder, err error) {
	if opt == nil {
		opt = &DecoderOptions{}
	}
	if opt.MaxPayloadSize == 0 {
		opt.MaxPayloadSize = DEFAULT_MAX_PAYLOAD_SIZE
	}
	dec = &Decoder{
		buf:    make([]byte, opt.MaxPayloadSize),
		opt:    *opt,
		reader: bufio.NewReader(r),
		writer: nil,
	}

	var cf ControlFrame
	if opt.Bidirectional {
		w, ok := r.(io.Writer)
		if !ok {
			return dec, ErrType
		}
		dec.writer = bufio.NewWriter(w)

		// Read the ready control frame.
		err = cf.DecodeTypeEscape(dec.reader, CONTROL_READY)
		if err != nil {
			return
		}

		// Check content type.
		if !cf.MatchContentType(dec.opt.ContentType) {
			return dec, ErrContentTypeMismatch
		}

		// Send the accept control frame.
		accept := ControlAccept
		accept.SetContentType(dec.opt.ContentType)
		err = accept.EncodeFlush(dec.writer)
		if err != nil {
			return
		}
	}

	// Read the start control frame.
	err = cf.DecodeTypeEscape(dec.reader, CONTROL_START)
	if err != nil {
		return
	}

	// Check content type.
	if !cf.MatchContentType(dec.opt.ContentType) {
		return dec, ErrContentTypeMismatch
	}

	return
}

func (dec *Decoder) readFrame(frameLen uint32) (err error) {
	// Enforce limits on frame size.
	if frameLen > dec.opt.MaxPayloadSize {
		err = ErrDataFrameTooLarge
		return
	}

	// Read the frame.
	n, err := io.ReadFull(dec.reader, dec.buf[0:frameLen])
	if err != nil || uint32(n) != frameLen {
		return
	}
	return
}

func (dec *Decoder) Decode() (frameData []byte, err error) {
	if dec.stopped {
		err = EOF
		return
	}

	// Read the frame length.
	var frameLen uint32
	err = binary.Read(dec.reader, binary.BigEndian, &frameLen)
	if err != nil {
		return
	}
	if frameLen == 0 {
		// This is a control frame.
		var cf ControlFrame
		err = cf.Decode(dec.reader)
		if err != nil {
			return
		}
		if cf.ControlType == CONTROL_STOP {
			dec.stopped = true
			if dec.opt.Bidirectional {
				ff := &ControlFrame{ControlType: CONTROL_FINISH}
				err = ff.EncodeFlush(dec.writer)
				if err != nil {
					return
				}
			}
			return nil, EOF
		}
		if err != nil {
			return nil, err
		}

	} else {
		// This is a data frame.
		err = dec.readFrame(frameLen)
		if err != nil {
			return
		}
		frameData = dec.buf[0:frameLen]
	}

	return
}
