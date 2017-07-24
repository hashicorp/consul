/*
 * Copyright (c) 2013-2014 by Farsight Security, Inc.
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
	"log"
	"net"
	"os"
)

type FrameStreamSockInput struct {
	wait     chan bool
	listener net.Listener
}

func NewFrameStreamSockInput(listener net.Listener) (input *FrameStreamSockInput) {
	input = new(FrameStreamSockInput)
	input.listener = listener
	return
}

func NewFrameStreamSockInputFromPath(socketPath string) (input *FrameStreamSockInput, err error) {
	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return
	}
	return NewFrameStreamSockInput(listener), nil
}

func (input *FrameStreamSockInput) ReadInto(output chan []byte) {
	for {
		conn, err := input.listener.Accept()
		if err != nil {
			log.Printf("net.Listener.Accept() failed: %s\n", err)
			continue
		}
		i, err := NewFrameStreamInput(conn, true)
		if err != nil {
			log.Printf("dnstap.NewFrameStreamInput() failed: %s\n", err)
			continue
		}
		log.Printf("dnstap.FrameStreamSockInput: accepted a socket connection\n")
		go i.ReadInto(output)
	}
}

func (input *FrameStreamSockInput) Wait() {
	select {}
}
