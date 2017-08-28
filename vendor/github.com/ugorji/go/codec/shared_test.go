// Copyright (c) 2012-2015 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

// This file sets up the variables used, including testInitFns.
// Each file should add initialization that should be performed
// after flags are parsed.
//
// init is a multi-step process:
//   - setup vars (handled by init functions in each file)
//   - parse flags
//   - setup derived vars (handled by pre-init registered functions - registered in init function)
//   - post init (handled by post-init registered functions - registered in init function)
// This way, no one has to manage carefully control the initialization
// using file names, etc.
//
// Tests which require external dependencies need the -tag=x parameter.
// They should be run as:
//    go test -tags=x -run=. <other parameters ...>
// Benchmarks should also take this parameter, to include the sereal, xdr, etc.
// To run against codecgen, etc, make sure you pass extra parameters.
// Example usage:
//    go test "-tags=x codecgen unsafe" -bench=. <other parameters ...>
//
// To fully test everything:
//    go test -tags=x -benchtime=100ms -tv -bg -bi  -brw -bu -v -run=. -bench=.

// Handling flags
// codec_test.go will define a set of global flags for testing, including:
//   - Use Reset
//   - Use IO reader/writer (vs direct bytes)
//   - Set Canonical
//   - Set InternStrings
//   - Use Symbols
//
// This way, we can test them all by running same set of tests with a different
// set of flags.
//
// Following this, all the benchmarks will utilize flags set by codec_test.go
// and will not redefine these "global" flags.

import (
	"bytes"
	"flag"
	"sync"
)

// DO NOT REMOVE - replacement line for go-codec-bench import declaration tag //

type testHED struct {
	H Handle
	E *Encoder
	D *Decoder
}

var (
	testNoopH    = NoopHandle(8)
	testMsgpackH = &MsgpackHandle{}
	testBincH    = &BincHandle{}
	testSimpleH  = &SimpleHandle{}
	testCborH    = &CborHandle{}
	testJsonH    = &JsonHandle{}

	testHandles     []Handle
	testPreInitFns  []func()
	testPostInitFns []func()

	testOnce sync.Once

	testHEDs []testHED
)

// flag variables used by tests (and bench)
var (
	testVerbose        bool
	testInitDebug      bool
	testUseIoEncDec    bool
	testStructToArray  bool
	testCanonical      bool
	testUseReset       bool
	testWriteNoSymbols bool
	testSkipIntf       bool
	testInternStr      bool
	testUseMust        bool
	testCheckCircRef   bool
	testJsonIndent     int
	testMaxInitLen     int

	testJsonHTMLCharsAsIs bool
)

// flag variables used by bench
var (
	benchDoInitBench      bool
	benchVerify           bool
	benchUnscientificRes  bool = false
	benchMapStringKeyOnly bool
	//depth of 0 maps to ~400bytes json-encoded string, 1 maps to ~1400 bytes, etc
	//For depth>1, we likely trigger stack growth for encoders, making benchmarking unreliable.
	benchDepth     int
	benchInitDebug bool
)

func init() {
	testHEDs = make([]testHED, 0, 32)
	testHandles = append(testHandles,
		testNoopH, testMsgpackH, testBincH, testSimpleH,
		testCborH, testJsonH)
	testInitFlags()
	benchInitFlags()
}

func testInitFlags() {
	// delete(testDecOpts.ExtFuncs, timeTyp)
	flag.BoolVar(&testVerbose, "tv", false, "Test Verbose")
	flag.BoolVar(&testInitDebug, "tg", false, "Test Init Debug")
	flag.BoolVar(&testUseIoEncDec, "ti", false, "Use IO Reader/Writer for Marshal/Unmarshal")
	flag.BoolVar(&testStructToArray, "ts", false, "Set StructToArray option")
	flag.BoolVar(&testWriteNoSymbols, "tn", false, "Set NoSymbols option")
	flag.BoolVar(&testCanonical, "tc", false, "Set Canonical option")
	flag.BoolVar(&testInternStr, "te", false, "Set InternStr option")
	flag.BoolVar(&testSkipIntf, "tf", false, "Skip Interfaces")
	flag.BoolVar(&testUseReset, "tr", false, "Use Reset")
	flag.IntVar(&testJsonIndent, "td", 0, "Use JSON Indent")
	flag.IntVar(&testMaxInitLen, "tx", 0, "Max Init Len")
	flag.BoolVar(&testUseMust, "tm", true, "Use Must(En|De)code")
	flag.BoolVar(&testCheckCircRef, "tl", false, "Use Check Circular Ref")
	flag.BoolVar(&testJsonHTMLCharsAsIs, "tas", false, "Set JSON HTMLCharsAsIs")
}

func benchInitFlags() {
	flag.BoolVar(&benchMapStringKeyOnly, "bs", false, "Bench use maps with string keys only")
	flag.BoolVar(&benchInitDebug, "bg", false, "Bench Debug")
	flag.IntVar(&benchDepth, "bd", 1, "Bench Depth")
	flag.BoolVar(&benchDoInitBench, "bi", false, "Run Bench Init")
	flag.BoolVar(&benchVerify, "bv", false, "Verify Decoded Value during Benchmark")
	flag.BoolVar(&benchUnscientificRes, "bu", false, "Show Unscientific Results during Benchmark")
}

func testHEDGet(h Handle) *testHED {
	for i := range testHEDs {
		v := &testHEDs[i]
		if v.H == h {
			return v
		}
	}
	testHEDs = append(testHEDs, testHED{h, NewEncoder(nil, h), NewDecoder(nil, h)})
	return &testHEDs[len(testHEDs)-1]
}

func testInitAll() {
	flag.Parse()
	for _, f := range testPreInitFns {
		f()
	}
	for _, f := range testPostInitFns {
		f()
	}
}

func testCodecEncode(ts interface{}, bsIn []byte,
	fn func([]byte) *bytes.Buffer, h Handle) (bs []byte, err error) {
	// bs = make([]byte, 0, approxSize)
	var e *Encoder
	var buf *bytes.Buffer
	if testUseReset {
		e = testHEDGet(h).E
	} else {
		e = NewEncoder(nil, h)
	}
	if testUseIoEncDec {
		buf = fn(bsIn)
		e.Reset(buf)
	} else {
		bs = bsIn
		e.ResetBytes(&bs)
	}
	if testUseMust {
		e.MustEncode(ts)
	} else {
		err = e.Encode(ts)
	}
	if testUseIoEncDec {
		bs = buf.Bytes()
	}
	return
}

func testCodecDecode(bs []byte, ts interface{}, h Handle) (err error) {
	var d *Decoder
	var buf *bytes.Reader
	if testUseReset {
		d = testHEDGet(h).D
	} else {
		d = NewDecoder(nil, h)
	}
	if testUseIoEncDec {
		buf = bytes.NewReader(bs)
		d.Reset(buf)
	} else {
		d.ResetBytes(bs)
	}
	if testUseMust {
		d.MustDecode(ts)
	} else {
		err = d.Decode(ts)
	}
	return
}

// ----- functions below are used only by benchmarks alone

func fnBenchmarkByteBuf(bsIn []byte) (buf *bytes.Buffer) {
	// var buf bytes.Buffer
	// buf.Grow(approxSize)
	buf = bytes.NewBuffer(bsIn)
	buf.Truncate(0)
	return
}

func benchFnCodecEncode(ts interface{}, bsIn []byte, h Handle) (bs []byte, err error) {
	return testCodecEncode(ts, bsIn, fnBenchmarkByteBuf, h)
}

func benchFnCodecDecode(bs []byte, ts interface{}, h Handle) (err error) {
	return testCodecDecode(bs, ts, h)
}
