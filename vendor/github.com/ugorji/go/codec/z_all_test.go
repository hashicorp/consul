// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

// +build alltests
// +build go1.7

package codec

// Run this using:
//   go test -tags=alltests -run=Suite -coverprofile=cov.out
//   go tool cover -html=cov.out
//
// Because build tags are a build time parameter, we will have to test out the
// different tags separately.
// Tags: x codecgen safe appengine notfastpath
//
// These tags should be added to alltests, e.g.
//   go test '-tags=alltests x codecgen' -run=Suite -coverprofile=cov.out
//
// To run all tests before submitting code, run:
//    a=( "" "safe" "codecgen" "notfastpath" "codecgen notfastpath" "codecgen safe" "safe notfastpath" )
//    for i in "${a[@]}"; do echo ">>>> TAGS: $i"; go test "-tags=alltests $i" -run=Suite; done
//
// This only works on go1.7 and above. This is when subtests and suites were supported.

import "testing"

// func TestMain(m *testing.M) {
// 	println("calling TestMain")
// 	// set some parameters
// 	exitcode := m.Run()
// 	os.Exit(exitcode)
// }

func testGroupResetFlags() {
	testUseMust = false
	testCanonical = false
	testUseMust = false
	testInternStr = false
	testUseIoEncDec = -1
	testStructToArray = false
	testCheckCircRef = false
	testUseReset = false
	testMaxInitLen = 0
	testUseIoWrapper = false
	testNumRepeatString = 8
	testEncodeOptions.RecursiveEmptyCheck = false
	testDecodeOptions.MapValueReset = false
	testUseIoEncDec = -1
	testDepth = 0
}

func testSuite(t *testing.T, f func(t *testing.T)) {
	// find . -name "*_test.go" | xargs grep -e 'flag.' | cut -d '&' -f 2 | cut -d ',' -f 1 | grep -e '^test'
	// Disregard the following: testInitDebug, testSkipIntf, testJsonIndent (Need a test for it)

	testReinit() // so flag.Parse() is called first, and never called again

	testDecodeOptions = DecodeOptions{}
	testEncodeOptions = EncodeOptions{}

	testGroupResetFlags()

	testReinit()
	t.Run("optionsFalse", f)

	testCanonical = true
	testUseMust = true
	testInternStr = true
	testUseIoEncDec = 0
	testStructToArray = true
	testCheckCircRef = true
	testUseReset = true
	testDecodeOptions.MapValueReset = true
	testEncodeOptions.RecursiveEmptyCheck = true
	testReinit()
	t.Run("optionsTrue", f)

	testDepth = 6
	testReinit()
	t.Run("optionsTrue-deepstruct", f)
	testDepth = 0

	// testEncodeOptions.AsSymbols = AsSymbolAll
	testUseIoWrapper = true
	testReinit()
	t.Run("optionsTrue-ioWrapper", f)

	testUseIoEncDec = -1

	// make buffer small enough so that we have to re-fill multiple times.
	testSkipRPCTests = true
	testUseIoEncDec = 128
	// testDecodeOptions.ReaderBufferSize = 128
	// testEncodeOptions.WriterBufferSize = 128
	testReinit()
	t.Run("optionsTrue-bufio", f)
	// testDecodeOptions.ReaderBufferSize = 0
	// testEncodeOptions.WriterBufferSize = 0
	testUseIoEncDec = -1
	testSkipRPCTests = false

	testNumRepeatString = 32
	testReinit()
	t.Run("optionsTrue-largestrings", f)

	// The following here MUST be tested individually, as they create
	// side effects i.e. the decoded value is different.
	// testDecodeOptions.MapValueReset = true // ok - no side effects
	// testDecodeOptions.InterfaceReset = true // error??? because we do deepEquals to verify
	// testDecodeOptions.ErrorIfNoField = true // error, as expected, as fields not there
	// testDecodeOptions.ErrorIfNoArrayExpand = true // no error, but no error case either
	// testDecodeOptions.PreferArrayOverSlice = true // error??? because slice != array.
	// .... however, update deepEqual to take this option
	// testReinit()
	// t.Run("optionsTrue-resetOptions", f)

	testGroupResetFlags()
}

/*
find . -name "codec_test.go" | xargs grep -e '^func Test' | \
    cut -d '(' -f 1 | cut -d ' ' -f 2 | \
    while read f; do echo "t.Run(\"$f\", $f)"; done
*/

func testCodecGroup(t *testing.T) {
	// println("running testcodecsuite")
	// <setup code>

	t.Run("TestBincCodecsTable", TestBincCodecsTable)
	t.Run("TestBincCodecsMisc", TestBincCodecsMisc)
	t.Run("TestBincCodecsEmbeddedPointer", TestBincCodecsEmbeddedPointer)
	t.Run("TestBincStdEncIntf", TestBincStdEncIntf)
	t.Run("TestBincMammoth", TestBincMammoth)
	t.Run("TestSimpleCodecsTable", TestSimpleCodecsTable)
	t.Run("TestSimpleCodecsMisc", TestSimpleCodecsMisc)
	t.Run("TestSimpleCodecsEmbeddedPointer", TestSimpleCodecsEmbeddedPointer)
	t.Run("TestSimpleStdEncIntf", TestSimpleStdEncIntf)
	t.Run("TestSimpleMammoth", TestSimpleMammoth)
	t.Run("TestMsgpackCodecsTable", TestMsgpackCodecsTable)
	t.Run("TestMsgpackCodecsMisc", TestMsgpackCodecsMisc)
	t.Run("TestMsgpackCodecsEmbeddedPointer", TestMsgpackCodecsEmbeddedPointer)
	t.Run("TestMsgpackStdEncIntf", TestMsgpackStdEncIntf)
	t.Run("TestMsgpackMammoth", TestMsgpackMammoth)
	t.Run("TestCborCodecsTable", TestCborCodecsTable)
	t.Run("TestCborCodecsMisc", TestCborCodecsMisc)
	t.Run("TestCborCodecsEmbeddedPointer", TestCborCodecsEmbeddedPointer)
	t.Run("TestCborMapEncodeForCanonical", TestCborMapEncodeForCanonical)
	t.Run("TestCborCodecChan", TestCborCodecChan)
	t.Run("TestCborStdEncIntf", TestCborStdEncIntf)
	t.Run("TestCborMammoth", TestCborMammoth)
	t.Run("TestJsonCodecsTable", TestJsonCodecsTable)
	t.Run("TestJsonCodecsMisc", TestJsonCodecsMisc)
	t.Run("TestJsonCodecsEmbeddedPointer", TestJsonCodecsEmbeddedPointer)
	t.Run("TestJsonCodecChan", TestJsonCodecChan)
	t.Run("TestJsonStdEncIntf", TestJsonStdEncIntf)
	t.Run("TestJsonMammoth", TestJsonMammoth)
	t.Run("TestJsonRaw", TestJsonRaw)
	t.Run("TestBincRaw", TestBincRaw)
	t.Run("TestMsgpackRaw", TestMsgpackRaw)
	t.Run("TestSimpleRaw", TestSimpleRaw)
	t.Run("TestCborRaw", TestCborRaw)
	t.Run("TestAllEncCircularRef", TestAllEncCircularRef)
	t.Run("TestAllAnonCycle", TestAllAnonCycle)
	t.Run("TestBincRpcGo", TestBincRpcGo)
	t.Run("TestSimpleRpcGo", TestSimpleRpcGo)
	t.Run("TestMsgpackRpcGo", TestMsgpackRpcGo)
	t.Run("TestCborRpcGo", TestCborRpcGo)
	t.Run("TestJsonRpcGo", TestJsonRpcGo)
	t.Run("TestMsgpackRpcSpec", TestMsgpackRpcSpec)
	t.Run("TestBincUnderlyingType", TestBincUnderlyingType)
	t.Run("TestJsonLargeInteger", TestJsonLargeInteger)
	t.Run("TestJsonDecodeNonStringScalarInStringContext", TestJsonDecodeNonStringScalarInStringContext)
	t.Run("TestJsonEncodeIndent", TestJsonEncodeIndent)

	t.Run("TestJsonSwallowAndZero", TestJsonSwallowAndZero)
	t.Run("TestCborSwallowAndZero", TestCborSwallowAndZero)
	t.Run("TestMsgpackSwallowAndZero", TestMsgpackSwallowAndZero)
	t.Run("TestBincSwallowAndZero", TestBincSwallowAndZero)
	t.Run("TestSimpleSwallowAndZero", TestSimpleSwallowAndZero)
	t.Run("TestJsonRawExt", TestJsonRawExt)
	t.Run("TestCborRawExt", TestCborRawExt)
	t.Run("TestMsgpackRawExt", TestMsgpackRawExt)
	t.Run("TestBincRawExt", TestBincRawExt)
	t.Run("TestSimpleRawExt", TestSimpleRawExt)
	t.Run("TestJsonMapStructKey", TestJsonMapStructKey)
	t.Run("TestCborMapStructKey", TestCborMapStructKey)
	t.Run("TestMsgpackMapStructKey", TestMsgpackMapStructKey)
	t.Run("TestBincMapStructKey", TestBincMapStructKey)
	t.Run("TestSimpleMapStructKey", TestSimpleMapStructKey)
	t.Run("TestJsonDecodeNilMapValue", TestJsonDecodeNilMapValue)
	t.Run("TestCborDecodeNilMapValue", TestCborDecodeNilMapValue)
	t.Run("TestMsgpackDecodeNilMapValue", TestMsgpackDecodeNilMapValue)
	t.Run("TestBincDecodeNilMapValue", TestBincDecodeNilMapValue)
	t.Run("TestSimpleDecodeNilMapValue", TestSimpleDecodeNilMapValue)
	t.Run("TestJsonEmbeddedFieldPrecedence", TestJsonEmbeddedFieldPrecedence)
	t.Run("TestCborEmbeddedFieldPrecedence", TestCborEmbeddedFieldPrecedence)
	t.Run("TestMsgpackEmbeddedFieldPrecedence", TestMsgpackEmbeddedFieldPrecedence)
	t.Run("TestBincEmbeddedFieldPrecedence", TestBincEmbeddedFieldPrecedence)
	t.Run("TestSimpleEmbeddedFieldPrecedence", TestSimpleEmbeddedFieldPrecedence)
	t.Run("TestJsonLargeContainerLen", TestJsonLargeContainerLen)
	t.Run("TestCborLargeContainerLen", TestCborLargeContainerLen)
	t.Run("TestMsgpackLargeContainerLen", TestMsgpackLargeContainerLen)
	t.Run("TestBincLargeContainerLen", TestBincLargeContainerLen)
	t.Run("TestSimpleLargeContainerLen", TestSimpleLargeContainerLen)
	t.Run("TestJsonMammothMapsAndSlices", TestJsonMammothMapsAndSlices)
	t.Run("TestCborMammothMapsAndSlices", TestCborMammothMapsAndSlices)
	t.Run("TestMsgpackMammothMapsAndSlices", TestMsgpackMammothMapsAndSlices)
	t.Run("TestBincMammothMapsAndSlices", TestBincMammothMapsAndSlices)
	t.Run("TestSimpleMammothMapsAndSlices", TestSimpleMammothMapsAndSlices)
	t.Run("TestJsonTime", TestJsonTime)
	t.Run("TestCborTime", TestCborTime)
	t.Run("TestMsgpackTime", TestMsgpackTime)
	t.Run("TestBincTime", TestBincTime)
	t.Run("TestSimpleTime", TestSimpleTime)
	t.Run("TestJsonUintToInt", TestJsonUintToInt)
	t.Run("TestCborUintToInt", TestCborUintToInt)
	t.Run("TestMsgpackUintToInt", TestMsgpackUintToInt)
	t.Run("TestBincUintToInt", TestBincUintToInt)
	t.Run("TestSimpleUintToInt", TestSimpleUintToInt)
	t.Run("TestJsonDifferentMapOrSliceType", TestJsonDifferentMapOrSliceType)
	t.Run("TestCborDifferentMapOrSliceType", TestCborDifferentMapOrSliceType)
	t.Run("TestMsgpackDifferentMapOrSliceType", TestMsgpackDifferentMapOrSliceType)
	t.Run("TestBincDifferentMapOrSliceType", TestBincDifferentMapOrSliceType)
	t.Run("TestSimpleDifferentMapOrSliceType", TestSimpleDifferentMapOrSliceType)
	t.Run("TestJsonScalars", TestJsonScalars)
	t.Run("TestCborScalars", TestCborScalars)
	t.Run("TestMsgpackScalars", TestMsgpackScalars)
	t.Run("TestBincScalars", TestBincScalars)
	t.Run("TestSimpleScalars", TestSimpleScalars)
	t.Run("TestJsonIntfMapping", TestJsonIntfMapping)
	t.Run("TestCborIntfMapping", TestCborIntfMapping)
	t.Run("TestMsgpackIntfMapping", TestMsgpackIntfMapping)
	t.Run("TestBincIntfMapping", TestBincIntfMapping)
	t.Run("TestSimpleIntfMapping", TestSimpleIntfMapping)

	t.Run("TestJsonInvalidUnicode", TestJsonInvalidUnicode)
	t.Run("TestCborHalfFloat", TestCborHalfFloat)
	// <tear-down code>
}

func testJsonGroup(t *testing.T) {
	t.Run("TestJsonCodecsTable", TestJsonCodecsTable)
	t.Run("TestJsonCodecsMisc", TestJsonCodecsMisc)
	t.Run("TestJsonCodecsEmbeddedPointer", TestJsonCodecsEmbeddedPointer)
	t.Run("TestJsonCodecChan", TestJsonCodecChan)
	t.Run("TestJsonStdEncIntf", TestJsonStdEncIntf)
	t.Run("TestJsonMammoth", TestJsonMammoth)
	t.Run("TestJsonRaw", TestJsonRaw)
	t.Run("TestJsonRpcGo", TestJsonRpcGo)
	t.Run("TestJsonLargeInteger", TestJsonLargeInteger)
	t.Run("TestJsonDecodeNonStringScalarInStringContext", TestJsonDecodeNonStringScalarInStringContext)
	t.Run("TestJsonEncodeIndent", TestJsonEncodeIndent)

	t.Run("TestJsonSwallowAndZero", TestJsonSwallowAndZero)
	t.Run("TestJsonRawExt", TestJsonRawExt)
	t.Run("TestJsonMapStructKey", TestJsonMapStructKey)
	t.Run("TestJsonDecodeNilMapValue", TestJsonDecodeNilMapValue)
	t.Run("TestJsonEmbeddedFieldPrecedence", TestJsonEmbeddedFieldPrecedence)
	t.Run("TestJsonLargeContainerLen", TestJsonLargeContainerLen)
	t.Run("TestJsonMammothMapsAndSlices", TestJsonMammothMapsAndSlices)
	t.Run("TestJsonInvalidUnicode", TestJsonInvalidUnicode)
	t.Run("TestJsonTime", TestJsonTime)
	t.Run("TestJsonUintToInt", TestJsonUintToInt)
	t.Run("TestJsonDifferentMapOrSliceType", TestJsonDifferentMapOrSliceType)
	t.Run("TestJsonScalars", TestJsonScalars)
	t.Run("TestJsonIntfMapping", TestJsonIntfMapping)
}

func testBincGroup(t *testing.T) {
	t.Run("TestBincCodecsTable", TestBincCodecsTable)
	t.Run("TestBincCodecsMisc", TestBincCodecsMisc)
	t.Run("TestBincCodecsEmbeddedPointer", TestBincCodecsEmbeddedPointer)
	t.Run("TestBincStdEncIntf", TestBincStdEncIntf)
	t.Run("TestBincMammoth", TestBincMammoth)
	t.Run("TestBincRaw", TestBincRaw)
	t.Run("TestSimpleRpcGo", TestSimpleRpcGo)
	t.Run("TestBincUnderlyingType", TestBincUnderlyingType)

	t.Run("TestBincSwallowAndZero", TestBincSwallowAndZero)
	t.Run("TestBincRawExt", TestBincRawExt)
	t.Run("TestBincMapStructKey", TestBincMapStructKey)
	t.Run("TestBincDecodeNilMapValue", TestBincDecodeNilMapValue)
	t.Run("TestBincEmbeddedFieldPrecedence", TestBincEmbeddedFieldPrecedence)
	t.Run("TestBincLargeContainerLen", TestBincLargeContainerLen)
	t.Run("TestBincMammothMapsAndSlices", TestBincMammothMapsAndSlices)
	t.Run("TestBincTime", TestBincTime)
	t.Run("TestBincUintToInt", TestBincUintToInt)
	t.Run("TestBincDifferentMapOrSliceType", TestBincDifferentMapOrSliceType)
	t.Run("TestBincScalars", TestBincScalars)
}

func testCborGroup(t *testing.T) {
	t.Run("TestCborCodecsTable", TestCborCodecsTable)
	t.Run("TestCborCodecsMisc", TestCborCodecsMisc)
	t.Run("TestCborCodecsEmbeddedPointer", TestCborCodecsEmbeddedPointer)
	t.Run("TestCborMapEncodeForCanonical", TestCborMapEncodeForCanonical)
	t.Run("TestCborCodecChan", TestCborCodecChan)
	t.Run("TestCborStdEncIntf", TestCborStdEncIntf)
	t.Run("TestCborMammoth", TestCborMammoth)
	t.Run("TestCborRaw", TestCborRaw)
	t.Run("TestCborRpcGo", TestCborRpcGo)

	t.Run("TestCborSwallowAndZero", TestCborSwallowAndZero)
	t.Run("TestCborRawExt", TestCborRawExt)
	t.Run("TestCborMapStructKey", TestCborMapStructKey)
	t.Run("TestCborDecodeNilMapValue", TestCborDecodeNilMapValue)
	t.Run("TestCborEmbeddedFieldPrecedence", TestCborEmbeddedFieldPrecedence)
	t.Run("TestCborLargeContainerLen", TestCborLargeContainerLen)
	t.Run("TestCborMammothMapsAndSlices", TestCborMammothMapsAndSlices)
	t.Run("TestCborTime", TestCborTime)
	t.Run("TestCborUintToInt", TestCborUintToInt)
	t.Run("TestCborDifferentMapOrSliceType", TestCborDifferentMapOrSliceType)
	t.Run("TestCborScalars", TestCborScalars)

	t.Run("TestCborHalfFloat", TestCborHalfFloat)
}

func testMsgpackGroup(t *testing.T) {
	t.Run("TestMsgpackCodecsTable", TestMsgpackCodecsTable)
	t.Run("TestMsgpackCodecsMisc", TestMsgpackCodecsMisc)
	t.Run("TestMsgpackCodecsEmbeddedPointer", TestMsgpackCodecsEmbeddedPointer)
	t.Run("TestMsgpackStdEncIntf", TestMsgpackStdEncIntf)
	t.Run("TestMsgpackMammoth", TestMsgpackMammoth)
	t.Run("TestMsgpackRaw", TestMsgpackRaw)
	t.Run("TestMsgpackRpcGo", TestMsgpackRpcGo)
	t.Run("TestMsgpackRpcSpec", TestMsgpackRpcSpec)
	t.Run("TestMsgpackSwallowAndZero", TestMsgpackSwallowAndZero)
	t.Run("TestMsgpackRawExt", TestMsgpackRawExt)
	t.Run("TestMsgpackMapStructKey", TestMsgpackMapStructKey)
	t.Run("TestMsgpackDecodeNilMapValue", TestMsgpackDecodeNilMapValue)
	t.Run("TestMsgpackEmbeddedFieldPrecedence", TestMsgpackEmbeddedFieldPrecedence)
	t.Run("TestMsgpackLargeContainerLen", TestMsgpackLargeContainerLen)
	t.Run("TestMsgpackMammothMapsAndSlices", TestMsgpackMammothMapsAndSlices)
	t.Run("TestMsgpackTime", TestMsgpackTime)
	t.Run("TestMsgpackUintToInt", TestMsgpackUintToInt)
	t.Run("TestMsgpackDifferentMapOrSliceType", TestMsgpackDifferentMapOrSliceType)
	t.Run("TestMsgpackScalars", TestMsgpackScalars)
}

func testSimpleGroup(t *testing.T) {
	t.Run("TestSimpleCodecsTable", TestSimpleCodecsTable)
	t.Run("TestSimpleCodecsMisc", TestSimpleCodecsMisc)
	t.Run("TestSimpleCodecsEmbeddedPointer", TestSimpleCodecsEmbeddedPointer)
	t.Run("TestSimpleStdEncIntf", TestSimpleStdEncIntf)
	t.Run("TestSimpleMammoth", TestSimpleMammoth)
	t.Run("TestSimpleRaw", TestSimpleRaw)
	t.Run("TestSimpleRpcGo", TestSimpleRpcGo)
	t.Run("TestSimpleSwallowAndZero", TestSimpleSwallowAndZero)
	t.Run("TestSimpleRawExt", TestSimpleRawExt)
	t.Run("TestSimpleMapStructKey", TestSimpleMapStructKey)
	t.Run("TestSimpleDecodeNilMapValue", TestSimpleDecodeNilMapValue)
	t.Run("TestSimpleEmbeddedFieldPrecedence", TestSimpleEmbeddedFieldPrecedence)
	t.Run("TestSimpleLargeContainerLen", TestSimpleLargeContainerLen)
	t.Run("TestSimpleMammothMapsAndSlices", TestSimpleMammothMapsAndSlices)
	t.Run("TestSimpleTime", TestSimpleTime)
	t.Run("TestSimpleUintToInt", TestSimpleUintToInt)
	t.Run("TestSimpleDifferentMapOrSliceType", TestSimpleDifferentMapOrSliceType)
	t.Run("TestSimpleScalars", TestSimpleScalars)
}

func testSimpleMammothGroup(t *testing.T) {
	t.Run("TestSimpleMammothMapsAndSlices", TestSimpleMammothMapsAndSlices)
}

func testRpcGroup(t *testing.T) {
	t.Run("TestBincRpcGo", TestBincRpcGo)
	t.Run("TestSimpleRpcGo", TestSimpleRpcGo)
	t.Run("TestMsgpackRpcGo", TestMsgpackRpcGo)
	t.Run("TestCborRpcGo", TestCborRpcGo)
	t.Run("TestJsonRpcGo", TestJsonRpcGo)
	t.Run("TestMsgpackRpcSpec", TestMsgpackRpcSpec)
}

func TestCodecSuite(t *testing.T) {
	testSuite(t, testCodecGroup)

	testGroupResetFlags()

	oldIndent, oldCharsAsis, oldPreferFloat, oldMapKeyAsString :=
		testJsonH.Indent, testJsonH.HTMLCharsAsIs, testJsonH.PreferFloat, testJsonH.MapKeyAsString

	testMaxInitLen = 10
	testJsonH.Indent = 8
	testJsonH.HTMLCharsAsIs = true
	testJsonH.MapKeyAsString = true
	// testJsonH.PreferFloat = true
	testReinit()
	t.Run("json-spaces-htmlcharsasis-initLen10", testJsonGroup)

	testMaxInitLen = 10
	testJsonH.Indent = -1
	testJsonH.HTMLCharsAsIs = false
	testJsonH.MapKeyAsString = true
	// testJsonH.PreferFloat = false
	testReinit()
	t.Run("json-tabs-initLen10", testJsonGroup)

	testJsonH.Indent, testJsonH.HTMLCharsAsIs, testJsonH.PreferFloat, testJsonH.MapKeyAsString =
		oldIndent, oldCharsAsis, oldPreferFloat, oldMapKeyAsString

	oldIndefLen := testCborH.IndefiniteLength

	testCborH.IndefiniteLength = true
	testReinit()
	t.Run("cbor-indefinitelength", testCborGroup)

	testCborH.IndefiniteLength = oldIndefLen

	oldTimeRFC3339 := testCborH.TimeRFC3339
	testCborH.TimeRFC3339 = !testCborH.TimeRFC3339
	testReinit()
	t.Run("cbor-rfc3339", testCborGroup)
	testCborH.TimeRFC3339 = oldTimeRFC3339

	oldSymbols := testBincH.AsSymbols

	testBincH.AsSymbols = 2 // AsSymbolNone
	testReinit()
	t.Run("binc-no-symbols", testBincGroup)

	testBincH.AsSymbols = 1 // AsSymbolAll
	testReinit()
	t.Run("binc-all-symbols", testBincGroup)

	testBincH.AsSymbols = oldSymbols

	oldWriteExt := testMsgpackH.WriteExt
	oldNoFixedNum := testMsgpackH.NoFixedNum

	testMsgpackH.WriteExt = !testMsgpackH.WriteExt
	testReinit()
	t.Run("msgpack-inverse-writeext", testMsgpackGroup)

	testMsgpackH.WriteExt = oldWriteExt

	testMsgpackH.NoFixedNum = !testMsgpackH.NoFixedNum
	testReinit()
	t.Run("msgpack-fixednum", testMsgpackGroup)

	testMsgpackH.NoFixedNum = oldNoFixedNum

	oldEncZeroValuesAsNil := testSimpleH.EncZeroValuesAsNil
	testSimpleH.EncZeroValuesAsNil = !testSimpleH.EncZeroValuesAsNil
	testUseMust = true
	testReinit()
	t.Run("simple-enczeroasnil", testSimpleMammothGroup) // testSimpleGroup
	testSimpleH.EncZeroValuesAsNil = oldEncZeroValuesAsNil

	oldRpcBufsize := testRpcBufsize
	testRpcBufsize = 0
	t.Run("rpc-buf-0", testRpcGroup)
	testRpcBufsize = 0
	t.Run("rpc-buf-00", testRpcGroup)
	testRpcBufsize = 0
	t.Run("rpc-buf-000", testRpcGroup)
	testRpcBufsize = 16
	t.Run("rpc-buf-16", testRpcGroup)
	testRpcBufsize = 2048
	t.Run("rpc-buf-2048", testRpcGroup)
	testRpcBufsize = oldRpcBufsize

	testGroupResetFlags()
}

// func TestCodecSuite(t *testing.T) {
// 	testReinit() // so flag.Parse() is called first, and never called again
// 	testDecodeOptions, testEncodeOptions = DecodeOptions{}, EncodeOptions{}
// 	testGroupResetFlags()
// 	testReinit()
// 	t.Run("optionsFalse", func(t *testing.T) {
// 		t.Run("TestJsonMammothMapsAndSlices", TestJsonMammothMapsAndSlices)
// 	})
// }
