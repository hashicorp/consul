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

package dnstap

import (
	"bufio"
	"io"
	"log"
	"os"

	"github.com/golang/protobuf/proto"
)

type TextFormatFunc func(*Dnstap) ([]byte, bool)

type TextOutput struct {
	format        TextFormatFunc
	outputChannel chan []byte
	wait          chan bool
	writer        *bufio.Writer
}

func NewTextOutput(writer io.Writer, format TextFormatFunc) (o *TextOutput) {
	o = new(TextOutput)
	o.format = format
	o.outputChannel = make(chan []byte, outputChannelSize)
	o.writer = bufio.NewWriter(writer)
	o.wait = make(chan bool)
	return
}

func NewTextOutputFromFilename(fname string, format TextFormatFunc) (o *TextOutput, err error) {
	if fname == "" || fname == "-" {
		return NewTextOutput(os.Stdout, format), nil
	}
	writer, err := os.Create(fname)
	if err != nil {
		return
	}
	return NewTextOutput(writer, format), nil
}

func (o *TextOutput) GetOutputChannel() chan []byte {
	return o.outputChannel
}

func (o *TextOutput) RunOutputLoop() {
	dt := &Dnstap{}
	for frame := range o.outputChannel {
		if err := proto.Unmarshal(frame, dt); err != nil {
			log.Fatalf("dnstap.TextOutput: proto.Unmarshal() failed: %s\n", err)
			break
		}
		buf, ok := o.format(dt)
		if !ok {
			log.Fatalf("dnstap.TextOutput: text format function failed\n")
			break
		}
		if _, err := o.writer.Write(buf); err != nil {
			log.Fatalf("dnstap.TextOutput: write failed: %s\n", err)
			break
		}
		o.writer.Flush()
	}
	close(o.wait)
}

func (o *TextOutput) Close() {
	close(o.outputChannel)
	<-o.wait
	o.writer.Flush()
}
