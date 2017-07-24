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

type EncoderOptions struct {
	ContentType   []byte
	Bidirectional bool
}

type Encoder struct {
	writer *bufio.Writer
	reader *bufio.Reader
	opt    EncoderOptions
	buf    []byte
}

func NewEncoder(w io.Writer, opt *EncoderOptions) (enc *Encoder, err error) {
	if opt == nil {
		opt = &EncoderOptions{}
	}
	enc = &Encoder{
		writer: bufio.NewWriter(w),
		opt:    *opt,
	}

	if opt.Bidirectional {
		r, ok := w.(io.Reader)
		if !ok {
			return nil, ErrType
		}
		enc.reader = bufio.NewReader(r)
		ready := ControlReady
		ready.SetContentType(opt.ContentType)
		if err = ready.EncodeFlush(enc.writer); err != nil {
			return
		}

		var accept ControlFrame
		if err = accept.DecodeTypeEscape(enc.reader, CONTROL_ACCEPT); err != nil {
			return
		}

		if !accept.MatchContentType(opt.ContentType) {
			return nil, ErrContentTypeMismatch
		}
	}

	// Write the start control frame.
	start := ControlStart
	start.SetContentType(opt.ContentType)
	err = start.Encode(enc.writer)
	if err != nil {
		return
	}

	return
}

func (enc *Encoder) Close() (err error) {
	err = ControlStop.EncodeFlush(enc.writer)
	if err != nil || !enc.opt.Bidirectional {
		return
	}

	var finish ControlFrame
	return finish.DecodeTypeEscape(enc.reader, CONTROL_FINISH)
}

func (enc *Encoder) Write(frame []byte) (n int, err error) {
	err = binary.Write(enc.writer, binary.BigEndian, uint32(len(frame)))
	if err != nil {
		return
	}
	return enc.writer.Write(frame)
}

func (enc *Encoder) Flush() error {
	return enc.writer.Flush()
}
