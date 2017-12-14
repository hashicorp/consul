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

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/dnstap/golang-dnstap"
)

var (
	flagReadTcp   = flag.String("l", "", "read dnstap payloads from tcp/ip")
	flagReadFile  = flag.String("r", "", "read dnstap payloads from file")
	flagReadSock  = flag.String("u", "", "read dnstap payloads from unix socket")
	flagWriteFile = flag.String("w", "-", "write output to file")
	flagQuietText = flag.Bool("q", false, "use quiet text output")
	flagYamlText  = flag.Bool("y", false, "use verbose YAML output")
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Quiet text output format mnemonics:
    AQ: AUTH_QUERY
    AR: AUTH_RESPONSE
    RQ: RESOLVER_QUERY
    RR: RESOLVER_RESPONSE
    CQ: CLIENT_QUERY
    CR: CLIENT_RESPONSE
    FQ: FORWARDER_QUERY
    FR: FORWARDER_RESPONSE
    SQ: STUB_QUERY
    SR: STUB_RESPONSE
    TQ: TOOL_QUERY
    TR: TOOL_RESPONSE
`)
}

func outputOpener(fname string, text, yaml bool) func() dnstap.Output {
	return func() dnstap.Output {
		var o dnstap.Output
		var err error
		if text {
			o, err = dnstap.NewTextOutputFromFilename(fname, dnstap.TextFormat)
		} else if yaml {
			o, err = dnstap.NewTextOutputFromFilename(fname, dnstap.YamlFormat)
		} else {
			o, err = dnstap.NewFrameStreamOutputFromFilename(fname)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "dnstap: Failed to open output file: %s\n", err)
			os.Exit(1)
		}

		go o.RunOutputLoop()
		return o
	}
}

func outputLoop(opener func() dnstap.Output, data <-chan []byte, done chan<- struct{}) {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt, syscall.SIGHUP)
	o := opener()
	defer func() {
		o.Close()
		close(done)
		os.Exit(0)
	}()
	for {
		select {
		case b, ok := <-data:
			if !ok {
				return
			}
			o.GetOutputChannel() <- b
		case sig := <-sigch:
			if sig == syscall.SIGHUP {
				o.Close()
				o = opener()
				continue
			}
			return
		}
	}
}

func main() {
	var err error
	var i dnstap.Input

	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(0)
	flag.Usage = usage

	// Handle command-line arguments.
	flag.Parse()

	if *flagReadFile == "" && *flagReadSock == "" && *flagReadTcp == "" {
		fmt.Fprintf(os.Stderr, "dnstap: Error: no inputs specified.\n")
		os.Exit(1)
	}

	if *flagWriteFile == "-" {
		if *flagQuietText == false && *flagYamlText == false {
			*flagQuietText = true
		}
	}

	if *flagReadFile != "" && *flagReadSock != "" && *flagReadTcp != "" {
		fmt.Fprintf(os.Stderr, "dnstap: Error: specify exactly one of -r, -u or -l.\n")
		os.Exit(1)
	}

	// Start the output loop.
	output := make(chan []byte, 1)
	opener := outputOpener(*flagWriteFile, *flagQuietText, *flagYamlText)
	outDone := make(chan struct{})
	go outputLoop(opener, output, outDone)

	// Open the input and start the input loop.
	if *flagReadFile != "" {
		i, err = dnstap.NewFrameStreamInputFromFilename(*flagReadFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dnstap: Failed to open input file: %s\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "dnstap: opened input file %s\n", *flagReadFile)
	} else if *flagReadSock != "" {
		i, err = dnstap.NewFrameStreamSockInputFromPath(*flagReadSock)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dnstap: Failed to open input socket: %s\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "dnstap: opened input socket %s\n", *flagReadSock)
	} else if *flagReadTcp != "" {
		l, err := net.Listen("tcp", *flagReadTcp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dnstap: Failed to listen: %s\n", err)
			os.Exit(1)
		}
		i = dnstap.NewFrameStreamSockInput(l)
	}
	i.ReadInto(output)

	// Wait for input loop to finish.
	i.Wait()
	close(output)

	<-outDone
}
