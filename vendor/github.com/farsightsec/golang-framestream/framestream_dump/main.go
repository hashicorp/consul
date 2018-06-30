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

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/farsightsec/golang-framestream"
)

func main() {
	// Arguments.
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <INPUT FILE>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Dumps a FrameStreams formatted input file.\n\n")
		os.Exit(1)
	}
	fname := os.Args[1]

	// Open the input file.
	file, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}

	// Create the decoder.
	fs, err := framestream.NewDecoder(file, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Print the frames.
	fmt.Printf("Control frame [START] (%v bytes): %x\n", len(fs.ControlStart), fs.ControlStart)
	for {
		frame, err := fs.Decode()
		if err == framestream.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Data frame (%v bytes): %x\n", len(frame), frame)
	}
	if fs.ControlStop != nil {
		fmt.Printf("Control frame [STOP] (%v bytes): %x\n", len(fs.ControlStop), fs.ControlStop)
	}
}
