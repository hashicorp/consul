// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	testPreInitFns = append(testPreInitFns, testInit)
	// fmt.Printf("sizeof: Decoder: %v, Encoder: %v, decNaked: %v\n",
	// 	reflect.TypeOf((*Decoder)(nil)).Elem().Size(),
	// 	reflect.TypeOf((*Encoder)(nil)).Elem().Size(),
	// 	reflect.TypeOf((*decNaked)(nil)).Elem().Size(),
	// )
}

type testCustomStringT string

// make this a mapbyslice
type testMbsT []interface{}

func (testMbsT) MapBySlice() {}

type testMbsCustStrT []testCustomStringT

func (testMbsCustStrT) MapBySlice() {}

type testIntfMapI interface {
	GetIntfMapV() string
}

type testIntfMapT1 struct {
	IntfMapV string
}

func (x *testIntfMapT1) GetIntfMapV() string { return x.IntfMapV }

type testIntfMapT2 struct {
	IntfMapV string
}

func (x testIntfMapT2) GetIntfMapV() string { return x.IntfMapV }

// ----

type testVerifyFlag uint8

const (
	_ testVerifyFlag = 1 << iota
	testVerifyMapTypeSame
	testVerifyMapTypeStrIntf
	testVerifyMapTypeIntfIntf
	// testVerifySliceIntf
	testVerifyForPython
	testVerifyDoNil
	testVerifyTimeAsInteger
)

func (f testVerifyFlag) isset(v testVerifyFlag) bool {
	return f&v == v
}

// const testSkipRPCTests = false

var (
	testTableNumPrimitives int
	testTableIdxTime       int
	testTableNumMaps       int

	// set this when running using bufio, etc
	testSkipRPCTests = false
)

var (
	skipVerifyVal interface{} = &(struct{}{})

	testMapStrIntfTyp = reflect.TypeOf(map[string]interface{}(nil))

	// For Go Time, do not use a descriptive timezone.
	// It's unnecessary, and makes it harder to do a reflect.DeepEqual.
	// The Offset already tells what the offset should be, if not on UTC and unknown zone name.
	timeLoc        = time.FixedZone("", -8*60*60) // UTC-08:00 //time.UTC-8
	timeToCompare1 = time.Date(2012, 2, 2, 2, 2, 2, 2000, timeLoc).UTC()
	timeToCompare2 = time.Date(1900, 2, 2, 2, 2, 2, 2000, timeLoc).UTC()
	timeToCompare3 = time.Unix(0, 270).UTC() // use value that must be encoded as uint64 for nanoseconds (for cbor/msgpack comparison)
	//timeToCompare4 = time.Time{}.UTC() // does not work well with simple cbor time encoding (overflow)
	timeToCompare4 = time.Unix(-2013855848, 4223).UTC()

	table []interface{} // main items we encode
	// will encode a float32 as float64, or large int as uint
	testRpcInt = new(TestRpcInt)
)

var wrapInt64Typ = reflect.TypeOf(wrapInt64(0))
var wrapBytesTyp = reflect.TypeOf(wrapBytes(nil))

func testByteBuf(in []byte) *bytes.Buffer {
	return bytes.NewBuffer(in)
}

type TestABC struct {
	A, B, C string
}

func (x *TestABC) MarshalBinary() ([]byte, error) {
	return []byte(fmt.Sprintf("%s %s %s", x.A, x.B, x.C)), nil
}
func (x *TestABC) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s %s %s", x.A, x.B, x.C)), nil
}
func (x *TestABC) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s %s %s"`, x.A, x.B, x.C)), nil
}

func (x *TestABC) UnmarshalBinary(data []byte) (err error) {
	ss := strings.Split(string(data), " ")
	x.A, x.B, x.C = ss[0], ss[1], ss[2]
	return
}
func (x *TestABC) UnmarshalText(data []byte) (err error) {
	return x.UnmarshalBinary(data)
}
func (x *TestABC) UnmarshalJSON(data []byte) (err error) {
	return x.UnmarshalBinary(data[1 : len(data)-1])
}

type TestABC2 struct {
	A, B, C string
}

func (x TestABC2) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s %s %s", x.A, x.B, x.C)), nil
}
func (x *TestABC2) UnmarshalText(data []byte) (err error) {
	ss := strings.Split(string(data), " ")
	x.A, x.B, x.C = ss[0], ss[1], ss[2]
	return
	// _, err = fmt.Sscanf(string(data), "%s %s %s", &x.A, &x.B, &x.C)
}

type TestSimplish struct {
	Ii int
	Ss string
	Ar [2]*TestSimplish
	Sl []*TestSimplish
	Mm map[string]*TestSimplish
}

type TestRpcABC struct {
	A, B, C string
}

type TestRpcInt struct {
	i int
}

func (r *TestRpcInt) Update(n int, res *int) error      { r.i = n; *res = r.i; return nil }
func (r *TestRpcInt) Square(ignore int, res *int) error { *res = r.i * r.i; return nil }
func (r *TestRpcInt) Mult(n int, res *int) error        { *res = r.i * n; return nil }
func (r *TestRpcInt) EchoStruct(arg TestRpcABC, res *string) error {
	*res = fmt.Sprintf("%#v", arg)
	return nil
}
func (r *TestRpcInt) Echo123(args []string, res *string) error {
	*res = fmt.Sprintf("%#v", args)
	return nil
}

type TestRawValue struct {
	R Raw
	I int
}

// ----

type testUnixNanoTimeExt struct {
	// keep timestamp here, so that do not incur interface-conversion costs
	// ts int64
}

func (x *testUnixNanoTimeExt) WriteExt(v interface{}) []byte {
	v2 := v.(*time.Time)
	bs := make([]byte, 8)
	bigen.PutUint64(bs, uint64(v2.UnixNano()))
	return bs
}
func (x *testUnixNanoTimeExt) ReadExt(v interface{}, bs []byte) {
	v2 := v.(*time.Time)
	ui := bigen.Uint64(bs)
	*v2 = time.Unix(0, int64(ui)).UTC()
}
func (x *testUnixNanoTimeExt) ConvertExt(v interface{}) interface{} {
	v2 := v.(*time.Time) // structs are encoded by passing the ptr
	return v2.UTC().UnixNano()
}

func (x *testUnixNanoTimeExt) UpdateExt(dest interface{}, v interface{}) {
	tt := dest.(*time.Time)
	switch v2 := v.(type) {
	case int64:
		*tt = time.Unix(0, v2).UTC()
	case uint64:
		*tt = time.Unix(0, int64(v2)).UTC()
	//case float64:
	//case string:
	default:
		panic(fmt.Sprintf("unsupported format for time conversion: expecting int64/uint64; got %T", v))
	}
}

// ----

type wrapInt64Ext int64

func (x *wrapInt64Ext) WriteExt(v interface{}) []byte {
	v2 := uint64(int64(v.(wrapInt64)))
	bs := make([]byte, 8)
	bigen.PutUint64(bs, v2)
	return bs
}
func (x *wrapInt64Ext) ReadExt(v interface{}, bs []byte) {
	v2 := v.(*wrapInt64)
	ui := bigen.Uint64(bs)
	*v2 = wrapInt64(int64(ui))
}
func (x *wrapInt64Ext) ConvertExt(v interface{}) interface{} {
	return int64(v.(wrapInt64))
}
func (x *wrapInt64Ext) UpdateExt(dest interface{}, v interface{}) {
	v2 := dest.(*wrapInt64)
	*v2 = wrapInt64(v.(int64))
}

// ----

type wrapBytesExt struct{}

func (x *wrapBytesExt) WriteExt(v interface{}) []byte {
	return ([]byte)(v.(wrapBytes))
}
func (x *wrapBytesExt) ReadExt(v interface{}, bs []byte) {
	v2 := v.(*wrapBytes)
	*v2 = wrapBytes(bs)
}
func (x *wrapBytesExt) ConvertExt(v interface{}) interface{} {
	return ([]byte)(v.(wrapBytes))
}
func (x *wrapBytesExt) UpdateExt(dest interface{}, v interface{}) {
	v2 := dest.(*wrapBytes)
	// some formats (e.g. json) cannot nakedly determine []byte from string, so expect both
	switch v3 := v.(type) {
	case []byte:
		*v2 = wrapBytes(v3)
	case string:
		*v2 = wrapBytes([]byte(v3))
	default:
		panic("UpdateExt for wrapBytesExt expects string or []byte")
	}
	// *v2 = wrapBytes(v.([]byte))
}

// ----

// timeExt is an extension handler for time.Time, that uses binc model for encoding/decoding time.
// we used binc model, as that is the only custom time representation that we designed ourselves.
type timeExt struct{}

func (x timeExt) WriteExt(v interface{}) (bs []byte) {
	switch v2 := v.(type) {
	case time.Time:
		bs = bincEncodeTime(v2)
	case *time.Time:
		bs = bincEncodeTime(*v2)
	default:
		panic(fmt.Errorf("unsupported format for time conversion: expecting time.Time; got %T", v2))
	}
	return
}
func (x timeExt) ReadExt(v interface{}, bs []byte) {
	tt, err := bincDecodeTime(bs)
	if err != nil {
		panic(err)
	}
	*(v.(*time.Time)) = tt
}

func (x timeExt) ConvertExt(v interface{}) interface{} {
	return x.WriteExt(v)
}
func (x timeExt) UpdateExt(v interface{}, src interface{}) {
	x.ReadExt(v, src.([]byte))
}

// ----

func testCodecEncode(ts interface{}, bsIn []byte,
	fn func([]byte) *bytes.Buffer, h Handle) (bs []byte, err error) {
	return sTestCodecEncode(ts, bsIn, fn, h, h.getBasicHandle())
}

func testCodecDecode(bs []byte, ts interface{}, h Handle) (err error) {
	return sTestCodecDecode(bs, ts, h, h.getBasicHandle())
}

func checkErrT(t *testing.T, err error) {
	if err != nil {
		failT(t, err.Error())
	}
}

func checkEqualT(t *testing.T, v1 interface{}, v2 interface{}, desc string) {
	if err := deepEqual(v1, v2); err != nil {
		failT(t, "Not Equal: %s: %v. v1: %v, v2: %v", desc, err, v1, v2)
	}
}

func failT(t *testing.T, args ...interface{}) {
	if len(args) > 0 {
		if format, isstr := args[0].(string); isstr {
			logT(t, format, args[1:]...)
		}
	}
	t.FailNow()
}

func testInit() {
	gob.Register(new(TestStrucFlex))
	if testInitDebug {
		ts0 := newTestStrucFlex(2, testNumRepeatString, false, !testSkipIntf, false)
		logT(nil, "====> depth: %v, ts: %#v\n", 2, ts0)
	}

	for _, v := range testHandles {
		bh := v.getBasicHandle()
		// pre-fill them first
		bh.EncodeOptions = testEncodeOptions
		bh.DecodeOptions = testDecodeOptions
		// bh.InterfaceReset = true
		// bh.PreferArrayOverSlice = true
		// modify from flag'ish things
		bh.InternString = testInternStr
		bh.Canonical = testCanonical
		bh.CheckCircularRef = testCheckCircRef
		bh.StructToArray = testStructToArray
		bh.MaxInitLen = testMaxInitLen
	}

	testMsgpackH.RawToString = true

	var tTimeExt timeExt
	var tBytesExt wrapBytesExt
	var tI64Ext wrapInt64Ext

	// create legacy functions suitable for deprecated AddExt functionality,
	// and use on some places for testSimpleH e.g. for time.Time and wrapInt64
	var (
		myExtEncFn = func(x BytesExt, rv reflect.Value) (bs []byte, err error) {
			defer panicToErr(errstrDecoratorDef{}, &err)
			bs = x.WriteExt(rv.Interface())
			return
		}
		myExtDecFn = func(x BytesExt, rv reflect.Value, bs []byte) (err error) {
			defer panicToErr(errstrDecoratorDef{}, &err)
			x.ReadExt(rv.Interface(), bs)
			return
		}
		timeExtEncFn      = func(rv reflect.Value) (bs []byte, err error) { return myExtEncFn(tTimeExt, rv) }
		timeExtDecFn      = func(rv reflect.Value, bs []byte) (err error) { return myExtDecFn(tTimeExt, rv, bs) }
		wrapInt64ExtEncFn = func(rv reflect.Value) (bs []byte, err error) { return myExtEncFn(&tI64Ext, rv) }
		wrapInt64ExtDecFn = func(rv reflect.Value, bs []byte) (err error) { return myExtDecFn(&tI64Ext, rv, bs) }
	)

	chkErr := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	// time.Time is a native type, so extensions will have no effect.
	// However, we add these here to ensure nothing happens.
	chkErr(testSimpleH.AddExt(timeTyp, 1, timeExtEncFn, timeExtDecFn))
	// testBincH.SetBytesExt(timeTyp, 1, timeExt{}) // time is builtin for binc
	chkErr(testMsgpackH.SetBytesExt(timeTyp, 1, timeExt{}))
	chkErr(testCborH.SetInterfaceExt(timeTyp, 1, &testUnixNanoTimeExt{}))
	// testJsonH.SetInterfaceExt(timeTyp, 1, &testUnixNanoTimeExt{})

	// Now, add extensions for the type wrapInt64 and wrapBytes,
	// so we can execute the Encode/Decode Ext paths.

	chkErr(testSimpleH.SetBytesExt(wrapBytesTyp, 32, &tBytesExt))
	chkErr(testMsgpackH.SetBytesExt(wrapBytesTyp, 32, &tBytesExt))
	chkErr(testBincH.SetBytesExt(wrapBytesTyp, 32, &tBytesExt))
	chkErr(testJsonH.SetInterfaceExt(wrapBytesTyp, 32, &tBytesExt))
	chkErr(testCborH.SetInterfaceExt(wrapBytesTyp, 32, &tBytesExt))

	chkErr(testSimpleH.AddExt(wrapInt64Typ, 16, wrapInt64ExtEncFn, wrapInt64ExtDecFn))
	// chkErr(testSimpleH.SetBytesExt(wrapInt64Typ, 16, &tI64Ext))
	chkErr(testMsgpackH.SetBytesExt(wrapInt64Typ, 16, &tI64Ext))
	chkErr(testBincH.SetBytesExt(wrapInt64Typ, 16, &tI64Ext))
	chkErr(testJsonH.SetInterfaceExt(wrapInt64Typ, 16, &tI64Ext))
	chkErr(testCborH.SetInterfaceExt(wrapInt64Typ, 16, &tI64Ext))

	// primitives MUST be an even number, so it can be used as a mapBySlice also.
	primitives := []interface{}{
		int8(-8),
		int16(-1616),
		int32(-32323232),
		int64(-6464646464646464),
		uint8(192),
		uint16(1616),
		uint32(32323232),
		uint64(6464646464646464),
		byte(192),
		float32(-3232.0),
		float64(-6464646464.0),
		float32(3232.0),
		float64(6464.0),
		float64(6464646464.0),
		false,
		true,
		"null",
		nil,
		"some&day>some<day",
		timeToCompare1,
		"",
		timeToCompare2,
		"bytestring",
		timeToCompare3,
		"none",
		timeToCompare4,
	}

	maps := []interface{}{
		map[string]bool{
			"true":  true,
			"false": false,
		},
		map[string]interface{}{
			"true":         "True",
			"false":        false,
			"uint16(1616)": uint16(1616),
		},
		//add a complex combo map in here. (map has list which has map)
		//note that after the first thing, everything else should be generic.
		map[string]interface{}{
			"list": []interface{}{
				int16(1616),
				int32(32323232),
				true,
				float32(-3232.0),
				map[string]interface{}{
					"TRUE":  true,
					"FALSE": false,
				},
				[]interface{}{true, false},
			},
			"int32": int32(32323232),
			"bool":  true,
			"LONG STRING": `
1234567890 1234567890 
1234567890 1234567890 
1234567890 1234567890 
ABCDEDFGHIJKLMNOPQRSTUVWXYZ 
abcdedfghijklmnopqrstuvwxyz 
ABCDEDFGHIJKLMNOPQRSTUVWXYZ 
abcdedfghijklmnopqrstuvwxyz 
"ABCDEDFGHIJKLMNOPQRSTUVWXYZ" 
'	a tab	'
\a\b\c\d\e
\b\f\n\r\t all literally
ugorji
`,
			"SHORT STRING": "1234567890",
		},
		map[interface{}]interface{}{
			true:       "true",
			uint8(138): false,
			false:      uint8(200),
		},
	}

	testTableNumPrimitives = len(primitives)
	testTableIdxTime = testTableNumPrimitives - 8
	testTableNumMaps = len(maps)

	table = []interface{}{}
	table = append(table, primitives...)
	table = append(table, primitives)
	table = append(table, testMbsT(primitives))
	table = append(table, maps...)
	table = append(table, newTestStrucFlex(0, testNumRepeatString, false, !testSkipIntf, false))

}

func testTableVerify(f testVerifyFlag, h Handle) (av []interface{}) {
	av = make([]interface{}, len(table))
	lp := testTableNumPrimitives + 4
	// doNil := f & testVerifyDoNil == testVerifyDoNil
	// doPython := f & testVerifyForPython == testVerifyForPython
	switch {
	case f.isset(testVerifyForPython):
		for i, v := range table {
			if i == testTableNumPrimitives+1 || i > lp { // testTableNumPrimitives+1 is the mapBySlice
				av[i] = skipVerifyVal
				continue
			}
			av[i] = testVerifyVal(v, f, h)
		}
		// only do the python verify up to the maps, skipping the last 2 maps.
		av = av[:testTableNumPrimitives+2+testTableNumMaps-2]
	case f.isset(testVerifyDoNil):
		for i, v := range table {
			if i > lp {
				av[i] = skipVerifyVal
				continue
			}
			av[i] = testVerifyVal(v, f, h)
		}
	default:
		for i, v := range table {
			if i == lp {
				av[i] = skipVerifyVal
				continue
			}
			//av[i] = testVerifyVal(v, testVerifyMapTypeSame)
			switch v.(type) {
			case []interface{}:
				av[i] = testVerifyVal(v, f, h)
			case testMbsT:
				av[i] = testVerifyVal(v, f, h)
			case map[string]interface{}:
				av[i] = testVerifyVal(v, f, h)
			case map[interface{}]interface{}:
				av[i] = testVerifyVal(v, f, h)
			case time.Time:
				av[i] = testVerifyVal(v, f, h)
			default:
				av[i] = v
			}
		}
	}
	return
}

func testVerifyValInt(v int64, isMsgp bool) (v2 interface{}) {
	if isMsgp {
		if v >= 0 && v <= 127 {
			v2 = uint64(v)
		} else {
			v2 = int64(v)
		}
	} else if v >= 0 {
		v2 = uint64(v)
	} else {
		v2 = int64(v)
	}
	return
}

func testVerifyVal(v interface{}, f testVerifyFlag, h Handle) (v2 interface{}) {
	//for python msgpack,
	//  - all positive integers are unsigned 64-bit ints
	//  - all floats are float64
	_, isMsgp := h.(*MsgpackHandle)
	_, isCbor := h.(*CborHandle)
	switch iv := v.(type) {
	case int8:
		v2 = testVerifyValInt(int64(iv), isMsgp)
		// fmt.Printf(">>>> is msgp: %v, v: %T, %v ==> v2: %T, %v\n", isMsgp, v, v, v2, v2)
	case int16:
		v2 = testVerifyValInt(int64(iv), isMsgp)
	case int32:
		v2 = testVerifyValInt(int64(iv), isMsgp)
	case int64:
		v2 = testVerifyValInt(int64(iv), isMsgp)
	case uint8:
		v2 = uint64(iv)
	case uint16:
		v2 = uint64(iv)
	case uint32:
		v2 = uint64(iv)
	case uint64:
		v2 = uint64(iv)
	case float32:
		v2 = float64(iv)
	case float64:
		v2 = float64(iv)
	case []interface{}:
		m2 := make([]interface{}, len(iv))
		for j, vj := range iv {
			m2[j] = testVerifyVal(vj, f, h)
		}
		v2 = m2
	case testMbsT:
		m2 := make([]interface{}, len(iv))
		for j, vj := range iv {
			m2[j] = testVerifyVal(vj, f, h)
		}
		v2 = testMbsT(m2)
	case map[string]bool:
		switch {
		case f.isset(testVerifyMapTypeSame):
			m2 := make(map[string]bool)
			for kj, kv := range iv {
				m2[kj] = kv
			}
			v2 = m2
		case f.isset(testVerifyMapTypeStrIntf):
			m2 := make(map[string]interface{})
			for kj, kv := range iv {
				m2[kj] = kv
			}
			v2 = m2
		case f.isset(testVerifyMapTypeIntfIntf):
			m2 := make(map[interface{}]interface{})
			for kj, kv := range iv {
				m2[kj] = kv
			}
			v2 = m2
		}
	case map[string]interface{}:
		switch {
		case f.isset(testVerifyMapTypeSame):
			m2 := make(map[string]interface{})
			for kj, kv := range iv {
				m2[kj] = testVerifyVal(kv, f, h)
			}
			v2 = m2
		case f.isset(testVerifyMapTypeStrIntf):
			m2 := make(map[string]interface{})
			for kj, kv := range iv {
				m2[kj] = testVerifyVal(kv, f, h)
			}
			v2 = m2
		case f.isset(testVerifyMapTypeIntfIntf):
			m2 := make(map[interface{}]interface{})
			for kj, kv := range iv {
				m2[kj] = testVerifyVal(kv, f, h)
			}
			v2 = m2
		}
	case map[interface{}]interface{}:
		m2 := make(map[interface{}]interface{})
		for kj, kv := range iv {
			m2[testVerifyVal(kj, f, h)] = testVerifyVal(kv, f, h)
		}
		v2 = m2
	case time.Time:
		switch {
		case f.isset(testVerifyTimeAsInteger):
			if iv2 := iv.UnixNano(); iv2 >= 0 {
				v2 = uint64(iv2)
			} else {
				v2 = int64(iv2)
			}
		case isMsgp:
			v2 = iv.UTC()
		case isCbor:
			// fmt.Printf("%%%% cbor verifier\n")
			v2 = iv.UTC().Round(time.Microsecond)
		default:
			v2 = v
		}
	default:
		v2 = v
	}
	return
}

func testUnmarshal(v interface{}, data []byte, h Handle) (err error) {
	return testCodecDecode(data, v, h)
}

func testMarshal(v interface{}, h Handle) (bs []byte, err error) {
	return testCodecEncode(v, nil, testByteBuf, h)
}

func testMarshalErr(v interface{}, h Handle, t *testing.T, name string) (bs []byte) {
	bs, err := testMarshal(v, h)
	if err != nil {
		failT(t, "Error encoding %s: %v, Err: %v", name, v, err)
	}
	return
}

func testUnmarshalErr(v interface{}, data []byte, h Handle, t *testing.T, name string) {
	if err := testUnmarshal(v, data, h); err != nil {
		failT(t, "Error Decoding into %s: %v, Err: %v", name, v, err)
	}
}

func testDeepEqualErr(v1, v2 interface{}, t *testing.T, name string) {
	if err := deepEqual(v1, v2); err == nil {
		logT(t, "%s: values equal", name)
	} else {
		failT(t, "%s: values not equal: %v. 1: %v, 2: %v", name, err, v1, v2)
	}
}

func testReadWriteCloser(c io.ReadWriteCloser) io.ReadWriteCloser {
	if testRpcBufsize <= 0 && rand.Int63()%2 == 0 {
		return c
	}
	return struct {
		io.Closer
		*bufio.Reader
		*bufio.Writer
	}{c, bufio.NewReaderSize(c, testRpcBufsize), bufio.NewWriterSize(c, testRpcBufsize)}
}

// doTestCodecTableOne allows us test for different variations based on arguments passed.
func doTestCodecTableOne(t *testing.T, testNil bool, h Handle,
	vs []interface{}, vsVerify []interface{}) {
	//if testNil, then just test for when a pointer to a nil interface{} is passed. It should work.
	//Current setup allows us test (at least manually) the nil interface or typed interface.
	logT(t, "================ TestNil: %v ================\n", testNil)
	for i, v0 := range vs {
		logT(t, "..............................................")
		logT(t, "         Testing: #%d:, %T, %#v\n", i, v0, v0)
		b0 := testMarshalErr(v0, h, t, "v0")
		var b1 = b0
		if len(b1) > 256 {
			b1 = b1[:256]
		}
		if h.isBinary() {
			logT(t, "         Encoded bytes: len: %v, %v\n", len(b0), b1)
		} else {
			logT(t, "         Encoded string: len: %v, %v\n", len(b0), string(b1))
			// println("########### encoded string: " + string(b0))
		}
		var v1 interface{}
		var err error
		if testNil {
			err = testUnmarshal(&v1, b0, h)
		} else {
			if v0 != nil {
				v0rt := reflect.TypeOf(v0) // ptr
				if v0rt.Kind() == reflect.Ptr {
					err = testUnmarshal(v0, b0, h)
					v1 = v0
				} else {
					rv1 := reflect.New(v0rt)
					err = testUnmarshal(rv1.Interface(), b0, h)
					v1 = rv1.Elem().Interface()
					// v1 = reflect.Indirect(reflect.ValueOf(v1)).Interface()
				}
			}
		}

		logT(t, "         v1 returned: %T, %v %#v", v1, v1, v1)
		// if v1 != nil {
		//	logT(t, "         v1 returned: %T, %#v", v1, v1)
		//	//we always indirect, because ptr to typed value may be passed (if not testNil)
		//	v1 = reflect.Indirect(reflect.ValueOf(v1)).Interface()
		// }
		if err != nil {
			failT(t, "-------- Error: %v. Partial return: %v", err, v1)
		}
		v0check := vsVerify[i]
		if v0check == skipVerifyVal {
			logT(t, "        Nil Check skipped: Decoded: %T, %#v\n", v1, v1)
			continue
		}

		if err = deepEqual(v0check, v1); err == nil {
			logT(t, "++++++++ Before and After marshal matched\n")
		} else {
			// logT(t, "-------- Before and After marshal do not match: Error: %v"+
			// 	" ====> GOLDEN: (%T) %#v, DECODED: (%T) %#v\n", err, v0check, v0check, v1, v1)
			logT(t, "-------- FAIL: Before and After marshal do not match: Error: %v", err)
			logT(t, "    ....... GOLDEN:  (%T) %v %#v", v0check, v0check, v0check)
			logT(t, "    ....... DECODED: (%T) %v %#v", v1, v1, v1)
			failT(t)
		}
	}
}

func testCodecTableOne(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	// func TestMsgpackAllExperimental(t *testing.T) {
	// dopts := testDecOpts(nil, nil, false, true, true),

	numPrim, numMap, idxTime, idxMap := testTableNumPrimitives, testTableNumMaps, testTableIdxTime, testTableNumPrimitives+2

	//println("#################")
	tableVerify := testTableVerify(testVerifyMapTypeSame, h)
	tableTestNilVerify := testTableVerify(testVerifyDoNil|testVerifyMapTypeStrIntf, h)
	switch v := h.(type) {
	case *MsgpackHandle:
		var oldWriteExt, oldRawToString bool
		_, _ = oldWriteExt, oldRawToString
		oldWriteExt, v.WriteExt = v.WriteExt, true
		oldRawToString, v.RawToString = v.RawToString, true
		// defer func() { v.WriteExt, v.RawToString = oldWriteExt, oldRawToString }()
		doTestCodecTableOne(t, false, h, table, tableVerify)
		v.WriteExt = oldWriteExt
		v.RawToString = oldRawToString
	case *JsonHandle:
		//skip []interface{} containing time.Time, as it encodes as a number, but cannot decode back to time.Time.
		//As there is no real support for extension tags in json, this must be skipped.
		doTestCodecTableOne(t, false, h, table[:numPrim], tableVerify[:numPrim])
		doTestCodecTableOne(t, false, h, table[idxMap:], tableVerify[idxMap:])
	default:
		doTestCodecTableOne(t, false, h, table, tableVerify)
	}
	// func TestMsgpackAll(t *testing.T) {

	// //skip []interface{} containing time.Time
	// doTestCodecTableOne(t, false, h, table[:numPrim], tableVerify[:numPrim])
	// doTestCodecTableOne(t, false, h, table[numPrim+1:], tableVerify[numPrim+1:])
	// func TestMsgpackNilStringMap(t *testing.T) {
	var oldMapType reflect.Type
	v := h.getBasicHandle()

	oldMapType, v.MapType = v.MapType, testMapStrIntfTyp
	// defer func() { v.MapType = oldMapType }()
	//skip time.Time, []interface{} containing time.Time, last map, and newStruc
	doTestCodecTableOne(t, true, h, table[:idxTime], tableTestNilVerify[:idxTime])
	doTestCodecTableOne(t, true, h, table[idxMap:idxMap+numMap-1], tableTestNilVerify[idxMap:idxMap+numMap-1]) // failing one for msgpack
	v.MapType = oldMapType
	// func TestMsgpackNilIntf(t *testing.T) {

	//do last map and newStruc
	idx2 := idxMap + numMap - 1
	doTestCodecTableOne(t, true, h, table[idx2:], tableTestNilVerify[idx2:])
	//TODO? What is this one?
	//doTestCodecTableOne(t, true, h, table[17:18], tableTestNilVerify[17:18])
}

func testCodecMiscOne(t *testing.T, h Handle) {
	var err error
	testOnce.Do(testInitAll)
	b := testMarshalErr(32, h, t, "32")
	// Cannot do this nil one, because faster type assertion decoding will panic
	// var i *int32
	// if err = testUnmarshal(b, i, nil); err == nil {
	// 	logT(t, "------- Expecting error because we cannot unmarshal to int32 nil ptr")
	// 	failT(t)
	// }
	var i2 int32
	testUnmarshalErr(&i2, b, h, t, "int32-ptr")
	if i2 != int32(32) {
		logT(t, "------- didn't unmarshal to 32: Received: %d", i2)
		failT(t)
	}

	// func TestMsgpackDecodePtr(t *testing.T) {
	ts := newTestStrucFlex(testDepth, testNumRepeatString, false, !testSkipIntf, false)
	b = testMarshalErr(ts, h, t, "pointer-to-struct")
	if len(b) < 40 {
		logT(t, "------- Size must be > 40. Size: %d", len(b))
		failT(t)
	}
	var b1 = b
	if len(b1) > 256 {
		b1 = b1[:256]
	}
	if h.isBinary() {
		logT(t, "------- b: size: %v, value: %v", len(b), b1)
	} else {
		logT(t, "------- b: size: %v, value: %s", len(b), b1)
	}
	ts2 := emptyTestStrucFlex()
	testUnmarshalErr(ts2, b, h, t, "pointer-to-struct")
	if ts2.I64 != math.MaxInt64*2/3 {
		logT(t, "------- Unmarshal wrong. Expect I64 = 64. Got: %v", ts2.I64)
		failT(t)
	}

	// func TestMsgpackIntfDecode(t *testing.T) {
	m := map[string]int{"A": 2, "B": 3}
	p := []interface{}{m}
	bs := testMarshalErr(p, h, t, "p")

	m2 := map[string]int{}
	p2 := []interface{}{m2}
	testUnmarshalErr(&p2, bs, h, t, "&p2")

	if m2["A"] != 2 || m2["B"] != 3 {
		logT(t, "FAIL: m2 not as expected: expecting: %v, got: %v", m, m2)
		failT(t)
	}

	// log("m: %v, m2: %v, p: %v, p2: %v", m, m2, p, p2)
	checkEqualT(t, p, p2, "p=p2")
	checkEqualT(t, m, m2, "m=m2")
	if err = deepEqual(p, p2); err == nil {
		logT(t, "p and p2 match")
	} else {
		logT(t, "Not Equal: %v. p: %v, p2: %v", err, p, p2)
		failT(t)
	}
	if err = deepEqual(m, m2); err == nil {
		logT(t, "m and m2 match")
	} else {
		logT(t, "Not Equal: %v. m: %v, m2: %v", err, m, m2)
		failT(t)
	}

	// func TestMsgpackDecodeStructSubset(t *testing.T) {
	// test that we can decode a subset of the stream
	mm := map[string]interface{}{"A": 5, "B": 99, "C": 333}
	bs = testMarshalErr(mm, h, t, "mm")
	type ttt struct {
		A uint8
		C int32
	}
	var t2 ttt
	testUnmarshalErr(&t2, bs, h, t, "t2")
	t3 := ttt{5, 333}
	checkEqualT(t, t2, t3, "t2=t3")

	// println(">>>>>")
	// test simple arrays, non-addressable arrays, slices
	type tarr struct {
		A int64
		B [3]int64
		C []byte
		D [3]byte
	}
	var tarr0 = tarr{1, [3]int64{2, 3, 4}, []byte{4, 5, 6}, [3]byte{7, 8, 9}}
	// test both pointer and non-pointer (value)
	for _, tarr1 := range []interface{}{tarr0, &tarr0} {
		bs = testMarshalErr(tarr1, h, t, "tarr1")
		if _, ok := h.(*JsonHandle); ok {
			logT(t, "Marshal as: %s", bs)
		}
		var tarr2 tarr
		testUnmarshalErr(&tarr2, bs, h, t, "tarr2")
		checkEqualT(t, tarr0, tarr2, "tarr0=tarr2")
	}

	// test byte array, even if empty (msgpack only)
	if h == testMsgpackH {
		type ystruct struct {
			Anarray []byte
		}
		var ya = ystruct{}
		testUnmarshalErr(&ya, []byte{0x91, 0x90}, h, t, "ya")
	}

	var tt1, tt2 time.Time
	tt2 = time.Now()
	bs = testMarshalErr(tt1, h, t, "zero-time-enc")
	testUnmarshalErr(&tt2, bs, h, t, "zero-time-dec")
	testDeepEqualErr(tt1, tt2, t, "zero-time-eq")

	// test encoding a slice of byte (but not []byte) and decoding into a []byte
	var sw = []wrapUint8{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'}
	var bw []byte // ("ABCDEFGHIJ")
	bs = testMarshalErr(sw, h, t, "wrap-bytes-enc")
	testUnmarshalErr(&bw, bs, h, t, "wrap-bytes-dec")
	testDeepEqualErr(bw, []byte("ABCDEFGHIJ"), t, "wrap-bytes-eq")
}

func testCodecEmbeddedPointer(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	type Z int
	type A struct {
		AnInt int
	}
	type B struct {
		*Z
		*A
		MoreInt int
	}
	var z Z = 4
	x1 := &B{&z, &A{5}, 6}
	bs := testMarshalErr(x1, h, t, "x1")
	var x2 = new(B)
	testUnmarshalErr(x2, bs, h, t, "x2")
	checkEqualT(t, x1, x2, "x1=x2")
}

func testCodecUnderlyingType(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	// Manual Test.
	// Run by hand, with accompanying print statements in fast-path.go
	// to ensure that the fast functions are called.
	type T1 map[string]string
	v := T1{"1": "1s", "2": "2s"}
	var bs []byte
	var err error
	NewEncoderBytes(&bs, h).MustEncode(v)
	if err != nil {
		logT(t, "Error during encode: %v", err)
		failT(t)
	}
	var v2 T1
	NewDecoderBytes(bs, h).MustDecode(&v2)
	if err != nil {
		logT(t, "Error during decode: %v", err)
		failT(t)
	}
}

func testCodecChan(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	// - send a slice []*int64 (sl1) into an chan (ch1) with cap > len(s1)
	// - encode ch1 as a stream array
	// - decode a chan (ch2), with cap > len(s1) from the stream array
	// - receive from ch2 into slice sl2
	// - compare sl1 and sl2
	// - do this for codecs: json, cbor (covers all types)

	if true {
		logT(t, "*int64")
		sl1 := make([]*int64, 4)
		for i := range sl1 {
			var j int64 = int64(i)
			sl1[i] = &j
		}
		ch1 := make(chan *int64, 4)
		for _, j := range sl1 {
			ch1 <- j
		}
		var bs []byte
		NewEncoderBytes(&bs, h).MustEncode(ch1)
		ch2 := make(chan *int64, 8)
		NewDecoderBytes(bs, h).MustDecode(&ch2)
		close(ch2)
		var sl2 []*int64
		for j := range ch2 {
			sl2 = append(sl2, j)
		}
		if err := deepEqual(sl1, sl2); err != nil {
			logT(t, "FAIL: Not Match: %v; len: %v, %v", err, len(sl1), len(sl2))
			failT(t)
		}
	}

	if true {
		logT(t, "testBytesT []byte - input []byte")
		type testBytesT []byte
		sl1 := make([]testBytesT, 4)
		for i := range sl1 {
			var j = []byte(strings.Repeat(strconv.FormatInt(int64(i), 10), i))
			sl1[i] = j
		}
		ch1 := make(chan testBytesT, 4)
		for _, j := range sl1 {
			ch1 <- j
		}
		var bs []byte
		NewEncoderBytes(&bs, h).MustEncode(ch1)
		ch2 := make(chan testBytesT, 8)
		NewDecoderBytes(bs, h).MustDecode(&ch2)
		close(ch2)
		var sl2 []testBytesT
		for j := range ch2 {
			// logT(t, ">>>> from chan: is nil? %v, %v", j == nil, j)
			sl2 = append(sl2, j)
		}
		if err := deepEqual(sl1, sl2); err != nil {
			logT(t, "FAIL: Not Match: %v; len: %v, %v", err, len(sl1), len(sl2))
			failT(t)
		}
	}
	if true {
		logT(t, "testBytesT byte - input string/testBytesT")
		type testBytesT byte
		sl1 := make([]testBytesT, 4)
		for i := range sl1 {
			var j = strconv.FormatInt(int64(i), 10)[0]
			sl1[i] = testBytesT(j)
		}
		ch1 := make(chan testBytesT, 4)
		for _, j := range sl1 {
			ch1 <- j
		}
		var bs []byte
		NewEncoderBytes(&bs, h).MustEncode(ch1)
		ch2 := make(chan testBytesT, 8)
		NewDecoderBytes(bs, h).MustDecode(&ch2)
		close(ch2)
		var sl2 []testBytesT
		for j := range ch2 {
			sl2 = append(sl2, j)
		}
		if err := deepEqual(sl1, sl2); err != nil {
			logT(t, "FAIL: Not Match: %v; len: %v, %v", err, len(sl1), len(sl2))
			failT(t)
		}
	}

	if true {
		logT(t, "*[]byte")
		sl1 := make([]byte, 4)
		for i := range sl1 {
			var j = strconv.FormatInt(int64(i), 10)[0]
			sl1[i] = byte(j)
		}
		ch1 := make(chan byte, 4)
		for _, j := range sl1 {
			ch1 <- j
		}
		var bs []byte
		NewEncoderBytes(&bs, h).MustEncode(ch1)
		ch2 := make(chan byte, 8)
		NewDecoderBytes(bs, h).MustDecode(&ch2)
		close(ch2)
		var sl2 []byte
		for j := range ch2 {
			sl2 = append(sl2, j)
		}
		if err := deepEqual(sl1, sl2); err != nil {
			logT(t, "FAIL: Not Match: %v; len: %v, %v", err, len(sl1), len(sl2))
			failT(t)
		}
	}

}

func testCodecRpcOne(t *testing.T, rr Rpc, h Handle, doRequest bool, exitSleepMs time.Duration,
) (port int) {
	testOnce.Do(testInitAll)
	if testSkipRPCTests {
		return
	}
	// rpc needs EOF, which is sent via a panic, and so must be recovered.
	if !recoverPanicToErr {
		logT(t, "EXPECTED. set recoverPanicToErr=true, since rpc needs EOF")
		failT(t)
	}

	if jsonH, ok := h.(*JsonHandle); ok && !jsonH.TermWhitespace {
		jsonH.TermWhitespace = true
		defer func() { jsonH.TermWhitespace = false }()
	}
	srv := rpc.NewServer()
	srv.Register(testRpcInt)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	// log("listener: %v", ln.Addr())
	checkErrT(t, err)
	port = (ln.Addr().(*net.TCPAddr)).Port
	// var opts *DecoderOptions
	// opts := testDecOpts
	// opts.MapType = mapStrIntfTyp
	// opts.RawToString = false
	serverExitChan := make(chan bool, 1)
	var serverExitFlag uint64
	serverFn := func() {
		for {
			conn1, err1 := ln.Accept()
			// if err1 != nil {
			// 	//fmt.Printf("accept err1: %v\n", err1)
			// 	continue
			// }
			if atomic.LoadUint64(&serverExitFlag) == 1 {
				serverExitChan <- true
				conn1.Close()
				return // exit serverFn goroutine
			}
			if err1 == nil {
				sc := rr.ServerCodec(testReadWriteCloser(conn1), h)
				srv.ServeCodec(sc)
			}
		}
	}

	clientFn := func(cc rpc.ClientCodec) {
		cl := rpc.NewClientWithCodec(cc)
		defer cl.Close()
		//	defer func() { println("##### client closing"); cl.Close() }()
		var up, sq, mult int
		var rstr string
		// log("Calling client")
		checkErrT(t, cl.Call("TestRpcInt.Update", 5, &up))
		// log("Called TestRpcInt.Update")
		checkEqualT(t, testRpcInt.i, 5, "testRpcInt.i=5")
		checkEqualT(t, up, 5, "up=5")
		checkErrT(t, cl.Call("TestRpcInt.Square", 1, &sq))
		checkEqualT(t, sq, 25, "sq=25")
		checkErrT(t, cl.Call("TestRpcInt.Mult", 20, &mult))
		checkEqualT(t, mult, 100, "mult=100")
		checkErrT(t, cl.Call("TestRpcInt.EchoStruct", TestRpcABC{"Aa", "Bb", "Cc"}, &rstr))
		checkEqualT(t, rstr, fmt.Sprintf("%#v", TestRpcABC{"Aa", "Bb", "Cc"}), "rstr=")
		checkErrT(t, cl.Call("TestRpcInt.Echo123", []string{"A1", "B2", "C3"}, &rstr))
		checkEqualT(t, rstr, fmt.Sprintf("%#v", []string{"A1", "B2", "C3"}), "rstr=")
	}

	connFn := func() (bs net.Conn) {
		// log("calling f1")
		bs, err2 := net.Dial(ln.Addr().Network(), ln.Addr().String())
		checkErrT(t, err2)
		return
	}

	exitFn := func() {
		atomic.StoreUint64(&serverExitFlag, 1)
		bs := connFn()
		<-serverExitChan
		bs.Close()
		// serverExitChan <- true
	}

	go serverFn()
	runtime.Gosched()
	//time.Sleep(100 * time.Millisecond)
	if exitSleepMs == 0 {
		defer ln.Close()
		defer exitFn()
	}
	if doRequest {
		bs := connFn()
		cc := rr.ClientCodec(testReadWriteCloser(bs), h)
		clientFn(cc)
	}
	if exitSleepMs != 0 {
		go func() {
			defer ln.Close()
			time.Sleep(exitSleepMs)
			exitFn()
		}()
	}
	return
}

func doTestMapEncodeForCanonical(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	// println("doTestMapEncodeForCanonical")
	v1 := map[stringUint64T]interface{}{
		{"a", 1}: 1,
		{"b", 2}: "hello",
		{"c", 3}: map[string]interface{}{
			"c/a": 1,
			"c/b": "world",
			"c/c": []int{1, 2, 3, 4},
			"c/d": map[string]interface{}{
				"c/d/a": "fdisajfoidsajfopdjsaopfjdsapofda",
				"c/d/b": "fdsafjdposakfodpsakfopdsakfpodsakfpodksaopfkdsopafkdopsa",
				"c/d/c": "poir02  ir30qif4p03qir0pogjfpoaerfgjp ofke[padfk[ewapf kdp[afep[aw",
				"c/d/d": "fdsopafkd[sa f-32qor-=4qeof -afo-erfo r-eafo 4e-  o r4-qwo ag",
				"c/d/e": "kfep[a sfkr0[paf[a foe-[wq  ewpfao-q ro3-q ro-4qof4-qor 3-e orfkropzjbvoisdb",
				"c/d/f": "",
			},
			"c/e": map[int]string{
				1:     "1",
				22:    "22",
				333:   "333",
				4444:  "4444",
				55555: "55555",
			},
			"c/f": map[string]int{
				"1":     1,
				"22":    22,
				"333":   333,
				"4444":  4444,
				"55555": 55555,
			},
			"c/g": map[bool]int{
				false: 0,
				true:  1,
			},
		},
	}
	var v2 map[stringUint64T]interface{}
	var b1, b2, b3 []byte

	// encode v1 into b1, decode b1 into v2, encode v2 into b2, and compare b1 and b2.
	// OR
	// encode v1 into b1, decode b1 into v2, encode v2 into b2 and b3, and compare b2 and b3.
	//   e.g. when doing cbor indefinite, we may haveto use out-of-band encoding
	//   where each key is encoded as an indefinite length string, which makes it not the same
	//   order as the strings were lexicographically ordered before.

	var cborIndef bool
	if ch, ok := h.(*CborHandle); ok {
		cborIndef = ch.IndefiniteLength
	}
	bh := h.getBasicHandle()
	if !bh.Canonical {
		bh.Canonical = true
		defer func() { bh.Canonical = false }()
	}

	e1 := NewEncoderBytes(&b1, h)
	e1.MustEncode(v1)
	d1 := NewDecoderBytes(b1, h)
	d1.MustDecode(&v2)
	// testDeepEqualErr(v1, v2, t, "huh?")
	e2 := NewEncoderBytes(&b2, h)
	e2.MustEncode(v2)
	var b1t, b2t = b1, b2
	if cborIndef {
		e2 = NewEncoderBytes(&b3, h)
		e2.MustEncode(v2)
		b1t, b2t = b2, b3
	}

	if !bytes.Equal(b1t, b2t) {
		logT(t, "Unequal bytes: %v VS %v", b1t, b2t)
		failT(t)
	}
}

func doTestStdEncIntf(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	args := [][2]interface{}{
		{&TestABC{"A", "BB", "CCC"}, new(TestABC)},
		{&TestABC2{"AAA", "BB", "C"}, new(TestABC2)},
	}
	for _, a := range args {
		var b []byte
		e := NewEncoderBytes(&b, h)
		e.MustEncode(a[0])
		d := NewDecoderBytes(b, h)
		d.MustDecode(a[1])
		if err := deepEqual(a[0], a[1]); err == nil {
			logT(t, "++++ Objects match")
		} else {
			logT(t, "---- FAIL: Objects do not match: y1: %v, err: %v", a[1], err)
			failT(t)
		}
	}
}

func doTestEncCircularRef(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	type T1 struct {
		S string
		B bool
		T interface{}
	}
	type T2 struct {
		S string
		T *T1
	}
	type T3 struct {
		S string
		T *T2
	}
	t1 := T1{"t1", true, nil}
	t2 := T2{"t2", &t1}
	t3 := T3{"t3", &t2}
	t1.T = &t3

	var bs []byte
	var err error

	bh := h.getBasicHandle()
	if !bh.CheckCircularRef {
		bh.CheckCircularRef = true
		defer func() { bh.CheckCircularRef = false }()
	}
	err = NewEncoderBytes(&bs, h).Encode(&t3)
	if err == nil {
		logT(t, "expecting error due to circular reference. found none")
		failT(t)
	}
	if x := err.Error(); strings.Contains(x, "circular") || strings.Contains(x, "cyclic") {
		logT(t, "error detected as expected: %v", x)
	} else {
		logT(t, "FAIL: error detected was not as expected: %v", x)
		failT(t)
	}
}

// TestAnonCycleT{1,2,3} types are used to test anonymous cycles.
// They are top-level, so that they can have circular references.
type (
	TestAnonCycleT1 struct {
		S string
		TestAnonCycleT2
	}
	TestAnonCycleT2 struct {
		S2 string
		TestAnonCycleT3
	}
	TestAnonCycleT3 struct {
		*TestAnonCycleT1
	}
)

func doTestAnonCycle(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	var x TestAnonCycleT1
	x.S = "hello"
	x.TestAnonCycleT2.S2 = "hello.2"
	x.TestAnonCycleT2.TestAnonCycleT3.TestAnonCycleT1 = &x

	// just check that you can get typeInfo for T1
	rt := reflect.TypeOf((*TestAnonCycleT1)(nil)).Elem()
	rtid := rt2id(rt)
	pti := h.getBasicHandle().getTypeInfo(rtid, rt)
	logT(t, "pti: %v", pti)
}

func doTestJsonLargeInteger(t *testing.T, v interface{}, ias uint8) {
	testOnce.Do(testInitAll)
	logT(t, "Running doTestJsonLargeInteger: v: %#v, ias: %c", v, ias)
	oldIAS := testJsonH.IntegerAsString
	defer func() { testJsonH.IntegerAsString = oldIAS }()
	testJsonH.IntegerAsString = ias

	var vu uint
	var vi int
	var vb bool
	var b []byte
	e := NewEncoderBytes(&b, testJsonH)
	e.MustEncode(v)
	e.MustEncode(true)
	d := NewDecoderBytes(b, testJsonH)
	// below, we validate that the json string or number was encoded,
	// then decode, and validate that the correct value was decoded.
	fnStrChk := func() {
		// check that output started with ", and ended with "true
		if !(b[0] == '"' && string(b[len(b)-5:]) == `"true`) {
			logT(t, "Expecting a JSON string, got: %s", b)
			failT(t)
		}
	}

	switch ias {
	case 'L':
		switch v2 := v.(type) {
		case int:
			v2n := int64(v2) // done to work with 32-bit OS
			if v2n > 1<<53 || (v2n < 0 && -v2n > 1<<53) {
				fnStrChk()
			}
		case uint:
			v2n := uint64(v2) // done to work with 32-bit OS
			if v2n > 1<<53 {
				fnStrChk()
			}
		}
	case 'A':
		fnStrChk()
	default:
		// check that output doesn't contain " at all
		for _, i := range b {
			if i == '"' {
				logT(t, "Expecting a JSON Number without quotation: got: %s", b)
				failT(t)
			}
		}
	}
	switch v2 := v.(type) {
	case int:
		d.MustDecode(&vi)
		d.MustDecode(&vb)
		// check that vb = true, and vi == v2
		if !(vb && vi == v2) {
			logT(t, "Expecting equal values from %s: got golden: %v, decoded: %v", b, v2, vi)
			failT(t)
		}
	case uint:
		d.MustDecode(&vu)
		d.MustDecode(&vb)
		// check that vb = true, and vi == v2
		if !(vb && vu == v2) {
			logT(t, "Expecting equal values from %s: got golden: %v, decoded: %v", b, v2, vu)
			failT(t)
		}
	}
}

func doTestRawValue(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	bh := h.getBasicHandle()
	if !bh.Raw {
		bh.Raw = true
		defer func() { bh.Raw = false }()
	}

	var i, i2 int
	var v, v2 TestRawValue
	var bs, bs2 []byte

	i = 1234 //1234567890
	v = TestRawValue{I: i}
	e := NewEncoderBytes(&bs, h)
	e.MustEncode(v.I)
	logT(t, ">>> raw: %v\n", bs)

	v.R = Raw(bs)
	e.ResetBytes(&bs2)
	e.MustEncode(v)

	logT(t, ">>> bs2: %v\n", bs2)
	d := NewDecoderBytes(bs2, h)
	d.MustDecode(&v2)
	d.ResetBytes(v2.R)
	logT(t, ">>> v2.R: %v\n", ([]byte)(v2.R))
	d.MustDecode(&i2)

	logT(t, ">>> Encoded %v, decoded %v\n", i, i2)
	// logT(t, "Encoded %v, decoded %v", i, i2)
	if i != i2 {
		logT(t, "Error: encoded %v, decoded %v", i, i2)
		failT(t)
	}
}

// Comprehensive testing that generates data encoded from python handle (cbor, msgpack),
// and validates that our code can read and write it out accordingly.
// We keep this unexported here, and put actual test in ext_dep_test.go.
// This way, it can be excluded by excluding file completely.
func doTestPythonGenStreams(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	logT(t, "TestPythonGenStreams-%v", name)
	tmpdir, err := ioutil.TempDir("", "golang-"+name+"-test")
	if err != nil {
		logT(t, "-------- Unable to create temp directory\n")
		failT(t)
	}
	defer os.RemoveAll(tmpdir)
	logT(t, "tmpdir: %v", tmpdir)
	cmd := exec.Command("python", "test.py", "testdata", tmpdir)
	//cmd.Stdin = strings.NewReader("some input")
	//cmd.Stdout = &out
	var cmdout []byte
	if cmdout, err = cmd.CombinedOutput(); err != nil {
		logT(t, "-------- Error running test.py testdata. Err: %v", err)
		logT(t, "         %v", string(cmdout))
		failT(t)
	}

	bh := h.getBasicHandle()

	oldMapType := bh.MapType
	tablePythonVerify := testTableVerify(testVerifyForPython|testVerifyTimeAsInteger|testVerifyMapTypeStrIntf, h)
	for i, v := range tablePythonVerify {
		// if v == uint64(0) && h == testMsgpackH {
		// 	v = int64(0)
		// }
		bh.MapType = oldMapType
		//load up the golden file based on number
		//decode it
		//compare to in-mem object
		//encode it again
		//compare to output stream
		logT(t, "..............................................")
		logT(t, "         Testing: #%d: %T, %#v\n", i, v, v)
		var bss []byte
		bss, err = ioutil.ReadFile(filepath.Join(tmpdir, strconv.Itoa(i)+"."+name+".golden"))
		if err != nil {
			logT(t, "-------- Error reading golden file: %d. Err: %v", i, err)
			failT(t)
			continue
		}
		bh.MapType = testMapStrIntfTyp

		var v1 interface{}
		if err = testUnmarshal(&v1, bss, h); err != nil {
			logT(t, "-------- Error decoding stream: %d: Err: %v", i, err)
			failT(t)
			continue
		}
		if v == skipVerifyVal {
			continue
		}
		//no need to indirect, because we pass a nil ptr, so we already have the value
		//if v1 != nil { v1 = reflect.Indirect(reflect.ValueOf(v1)).Interface() }
		if err = deepEqual(v, v1); err == nil {
			logT(t, "++++++++ Objects match: %T, %v", v, v)
		} else {
			logT(t, "-------- FAIL: Objects do not match: %v. Source: %T. Decoded: %T", err, v, v1)
			logT(t, "--------   GOLDEN: %#v", v)
			// logT(t, "--------   DECODED: %#v <====> %#v", v1, reflect.Indirect(reflect.ValueOf(v1)).Interface())
			logT(t, "--------   DECODED: %#v <====> %#v", v1, reflect.Indirect(reflect.ValueOf(v1)).Interface())
			failT(t)
		}
		bsb, err := testMarshal(v1, h)
		if err != nil {
			logT(t, "Error encoding to stream: %d: Err: %v", i, err)
			failT(t)
			continue
		}
		if err = deepEqual(bsb, bss); err == nil {
			logT(t, "++++++++ Bytes match")
		} else {
			logT(t, "???????? FAIL: Bytes do not match. %v.", err)
			xs := "--------"
			if reflect.ValueOf(v).Kind() == reflect.Map {
				xs = "        "
				logT(t, "%s It's a map. Ok that they don't match (dependent on ordering).", xs)
			} else {
				logT(t, "%s It's not a map. They should match.", xs)
				failT(t)
			}
			logT(t, "%s   FROM_FILE: %4d] %v", xs, len(bss), bss)
			logT(t, "%s     ENCODED: %4d] %v", xs, len(bsb), bsb)
		}
	}
	bh.MapType = oldMapType
}

// To test MsgpackSpecRpc, we test 3 scenarios:
//    - Go Client to Go RPC Service (contained within TestMsgpackRpcSpec)
//    - Go client to Python RPC Service (contained within doTestMsgpackRpcSpecGoClientToPythonSvc)
//    - Python Client to Go RPC Service (contained within doTestMsgpackRpcSpecPythonClientToGoSvc)
//
// This allows us test the different calling conventions
//    - Go Service requires only one argument
//    - Python Service allows multiple arguments

func doTestMsgpackRpcSpecGoClientToPythonSvc(t *testing.T) {
	if testSkipRPCTests {
		return
	}
	testOnce.Do(testInitAll)
	// openPorts are between 6700 and 6800
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	openPort := strconv.FormatInt(6700+r.Int63n(99), 10)
	// openPort := "6792"
	cmd := exec.Command("python", "test.py", "rpc-server", openPort, "4")
	checkErrT(t, cmd.Start())
	bs, err2 := net.Dial("tcp", ":"+openPort)
	for i := 0; i < 10 && err2 != nil; i++ {
		time.Sleep(50 * time.Millisecond) // time for python rpc server to start
		bs, err2 = net.Dial("tcp", ":"+openPort)
	}
	checkErrT(t, err2)
	cc := MsgpackSpecRpc.ClientCodec(testReadWriteCloser(bs), testMsgpackH)
	cl := rpc.NewClientWithCodec(cc)
	defer cl.Close()
	var rstr string
	checkErrT(t, cl.Call("EchoStruct", TestRpcABC{"Aa", "Bb", "Cc"}, &rstr))
	//checkEqualT(t, rstr, "{'A': 'Aa', 'B': 'Bb', 'C': 'Cc'}")
	var mArgs MsgpackSpecRpcMultiArgs = []interface{}{"A1", "B2", "C3"}
	checkErrT(t, cl.Call("Echo123", mArgs, &rstr))
	checkEqualT(t, rstr, "1:A1 2:B2 3:C3", "rstr=")
	cmd.Process.Kill()
}

func doTestMsgpackRpcSpecPythonClientToGoSvc(t *testing.T) {
	if testSkipRPCTests {
		return
	}
	testOnce.Do(testInitAll)
	port := testCodecRpcOne(t, MsgpackSpecRpc, testMsgpackH, false, 1*time.Second)
	//time.Sleep(1000 * time.Millisecond)
	cmd := exec.Command("python", "test.py", "rpc-client-go-service", strconv.Itoa(port))
	var cmdout []byte
	var err error
	if cmdout, err = cmd.CombinedOutput(); err != nil {
		logT(t, "-------- Error running test.py rpc-client-go-service. Err: %v", err)
		logT(t, "         %v", string(cmdout))
		failT(t)
	}
	checkEqualT(t, string(cmdout),
		fmt.Sprintf("%#v\n%#v\n", []string{"A1", "B2", "C3"}, TestRpcABC{"Aa", "Bb", "Cc"}), "cmdout=")
}

func doTestSwallowAndZero(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	v1 := newTestStrucFlex(testDepth, testNumRepeatString, false, false, false)
	var b1 []byte

	e1 := NewEncoderBytes(&b1, h)
	e1.MustEncode(v1)
	d1 := NewDecoderBytes(b1, h)
	d1.swallow()
	if d1.r.numread() != len(b1) {
		logT(t, "swallow didn't consume all encoded bytes: %v out of %v", d1.r.numread(), len(b1))
		failT(t)
	}
	setZero(v1)
	testDeepEqualErr(v1, &TestStrucFlex{}, t, "filled-and-zeroed")
}

func doTestRawExt(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	var b []byte
	var v RawExt // interface{}
	_, isJson := h.(*JsonHandle)
	_, isCbor := h.(*CborHandle)
	bh := h.getBasicHandle()
	// isValuer := isJson || isCbor
	// _ = isValuer
	for _, r := range []RawExt{
		{Tag: 99, Value: "9999", Data: []byte("9999")},
	} {
		e := NewEncoderBytes(&b, h)
		e.MustEncode(&r)
		// fmt.Printf(">>>> rawext: isnil? %v, %d - %v\n", b == nil, len(b), b)
		d := NewDecoderBytes(b, h)
		d.MustDecode(&v)
		var r2 = r
		switch {
		case isJson:
			r2.Tag = 0
			r2.Data = nil
		case isCbor:
			r2.Data = nil
		default:
			r2.Value = nil
		}
		testDeepEqualErr(v, r2, t, "rawext-default")
		// switch h.(type) {
		// case *JsonHandle:
		// 	testDeepEqualErr(r.Value, v, t, "rawext-json")
		// default:
		// 	var r2 = r
		// 	if isValuer {
		// 		r2.Data = nil
		// 	} else {
		// 		r2.Value = nil
		// 	}
		// 	testDeepEqualErr(v, r2, t, "rawext-default")
		// }
	}

	// Add testing for Raw also
	if b != nil {
		b = b[:0]
	}
	oldRawMode := bh.Raw
	defer func() { bh.Raw = oldRawMode }()
	bh.Raw = true

	var v2 Raw
	for _, s := range []string{
		"goodbye",
		"hello",
	} {
		e := NewEncoderBytes(&b, h)
		e.MustEncode(&s)
		// fmt.Printf(">>>> rawext: isnil? %v, %d - %v\n", b == nil, len(b), b)
		var r Raw = make([]byte, len(b))
		copy(r, b)
		d := NewDecoderBytes(b, h)
		d.MustDecode(&v2)
		testDeepEqualErr(v2, r, t, "raw-default")
	}

}

// func doTestTimeExt(t *testing.T, h Handle) {
// 	var t = time.Now()
// 	// add time ext to the handle
// }

func doTestMapStructKey(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	var b []byte
	var v interface{} // map[stringUint64T]wrapUint64Slice // interface{}
	bh := h.getBasicHandle()
	m := map[stringUint64T]wrapUint64Slice{
		{"55555", 55555}: []wrapUint64{12345},
		{"333", 333}:     []wrapUint64{123},
	}
	oldCanonical := bh.Canonical
	oldMapType := bh.MapType
	defer func() {
		bh.Canonical = oldCanonical
		bh.MapType = oldMapType
	}()

	bh.MapType = reflect.TypeOf((*map[stringUint64T]wrapUint64Slice)(nil)).Elem()
	for _, bv := range [2]bool{true, false} {
		b, v = nil, nil
		bh.Canonical = bv
		e := NewEncoderBytes(&b, h)
		e.MustEncode(m)
		d := NewDecoderBytes(b, h)
		d.MustDecode(&v)
		testDeepEqualErr(v, m, t, "map-structkey")
	}
}

func doTestDecodeNilMapValue(t *testing.T, handle Handle) {
	testOnce.Do(testInitAll)
	type Struct struct {
		Field map[uint16]map[uint32]struct{}
	}

	bh := handle.getBasicHandle()
	oldMapType := bh.MapType
	oldDeleteOnNilMapValue := bh.DeleteOnNilMapValue
	defer func() {
		bh.MapType = oldMapType
		bh.DeleteOnNilMapValue = oldDeleteOnNilMapValue
	}()
	bh.MapType = reflect.TypeOf(map[interface{}]interface{}(nil))
	bh.DeleteOnNilMapValue = false

	_, isJsonHandle := handle.(*JsonHandle)

	toEncode := Struct{Field: map[uint16]map[uint32]struct{}{
		1: nil,
	}}

	bs, err := testMarshal(toEncode, handle)
	if err != nil {
		logT(t, "Error encoding: %v, Err: %v", toEncode, err)
		failT(t)
	}
	if isJsonHandle {
		logT(t, "json encoded: %s\n", bs)
	}

	var decoded Struct
	err = testUnmarshal(&decoded, bs, handle)
	if err != nil {
		logT(t, "Error decoding: %v", err)
		failT(t)
	}
	if !reflect.DeepEqual(decoded, toEncode) {
		logT(t, "Decoded value %#v != %#v", decoded, toEncode)
		failT(t)
	}
}

func doTestEmbeddedFieldPrecedence(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	type Embedded struct {
		Field byte
	}
	type Struct struct {
		Field byte
		Embedded
	}
	toEncode := Struct{
		Field:    1,
		Embedded: Embedded{Field: 2},
	}
	_, isJsonHandle := h.(*JsonHandle)
	handle := h.getBasicHandle()
	oldMapType := handle.MapType
	defer func() { handle.MapType = oldMapType }()

	handle.MapType = reflect.TypeOf(map[interface{}]interface{}(nil))

	bs, err := testMarshal(toEncode, h)
	if err != nil {
		logT(t, "Error encoding: %v, Err: %v", toEncode, err)
		failT(t)
	}

	var decoded Struct
	err = testUnmarshal(&decoded, bs, h)
	if err != nil {
		logT(t, "Error decoding: %v", err)
		failT(t)
	}

	if decoded.Field != toEncode.Field {
		logT(t, "Decoded result %v != %v", decoded.Field, toEncode.Field) // hex to look at what was encoded
		if isJsonHandle {
			logT(t, "JSON encoded as: %s", bs) // hex to look at what was encoded
		}
		failT(t)
	}
}

func doTestLargeContainerLen(t *testing.T, h Handle) {
	testOnce.Do(testInitAll)
	m := make(map[int][]struct{})
	for i := range []int{
		0, 1,
		math.MaxInt8, math.MaxInt8 + 4, math.MaxInt8 - 4,
		math.MaxInt16, math.MaxInt16 + 4, math.MaxInt16 - 4,
		math.MaxInt32, math.MaxInt32 - 4,
		// math.MaxInt32 + 4, // bombs on 32-bit
		// math.MaxInt64, math.MaxInt64 - 4, // bombs on 32-bit

		math.MaxUint8, math.MaxUint8 + 4, math.MaxUint8 - 4,
		math.MaxUint16, math.MaxUint16 + 4, math.MaxUint16 - 4,
		// math.MaxUint32, math.MaxUint32 + 4, math.MaxUint32 - 4, // bombs on 32-bit
	} {
		m[i] = make([]struct{}, i)
	}
	bs := testMarshalErr(m, h, t, "-")
	var m2 = make(map[int][]struct{})
	testUnmarshalErr(m2, bs, h, t, "-")
	testDeepEqualErr(m, m2, t, "-")

	// do same tests for large strings (encoded as symbols or not)
	// skip if 32-bit or not using unsafe mode
	if safeMode || (32<<(^uint(0)>>63)) < 64 {
		return
	}

	// now, want to do tests for large strings, which
	// could be encoded as symbols.
	// to do this, we create a simple one-field struct,
	// use use flags to switch from symbols to non-symbols

	hbinc, okbinc := h.(*BincHandle)
	if okbinc {
		oldAsSymbols := hbinc.AsSymbols
		defer func() { hbinc.AsSymbols = oldAsSymbols }()
	}
	var out []byte = make([]byte, 0, math.MaxUint16*3/2)
	var in []byte = make([]byte, math.MaxUint16*3/2)
	for i := range in {
		in[i] = 'A'
	}
	e := NewEncoder(nil, h)
	for _, i := range []int{
		0, 1, 4, 8, 12, 16, 28, 32, 36,
		math.MaxInt8, math.MaxInt8 + 4, math.MaxInt8 - 4,
		math.MaxInt16, math.MaxInt16 + 4, math.MaxInt16 - 4,

		math.MaxUint8, math.MaxUint8 + 4, math.MaxUint8 - 4,
		math.MaxUint16, math.MaxUint16 + 4, math.MaxUint16 - 4,
	} {
		var m1, m2 map[string]bool
		m1 = make(map[string]bool, 1)
		var s1 = stringView(in[:i])
		// fmt.Printf("testcontainerlen: large string: i: %v, |%s|\n", i, s1)
		m1[s1] = true

		if okbinc {
			hbinc.AsSymbols = 2
		}
		out = out[:0]
		e.ResetBytes(&out)
		e.MustEncode(m1)
		// bs, _ = testMarshalErr(m1, h, t, "-")
		m2 = make(map[string]bool, 1)
		testUnmarshalErr(m2, out, h, t, "no-symbols")
		testDeepEqualErr(m1, m2, t, "no-symbols")

		if okbinc {
			// now, do as symbols
			hbinc.AsSymbols = 1
			out = out[:0]
			e.ResetBytes(&out)
			e.MustEncode(m1)
			// bs, _ = testMarshalErr(m1, h, t, "-")
			m2 = make(map[string]bool, 1)
			testUnmarshalErr(m2, out, h, t, "symbols")
			testDeepEqualErr(m1, m2, t, "symbols")
		}
	}

}

func testRandomFillRV(v reflect.Value) {
	testOnce.Do(testInitAll)
	fneg := func() int64 {
		i := rand.Intn(1)
		if i == 1 {
			return 1
		}
		return -1
	}

	switch v.Kind() {
	case reflect.Invalid:
	case reflect.Ptr:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		testRandomFillRV(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			v.Set(reflect.ValueOf("nothing"))
		} else {
			testRandomFillRV(v.Elem())
		}
	case reflect.Struct:
		for i, n := 0, v.NumField(); i < n; i++ {
			testRandomFillRV(v.Field(i))
		}
	case reflect.Slice:
		if v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type(), 4, 4))
		}
		fallthrough
	case reflect.Array:
		for i, n := 0, v.Len(); i < n; i++ {
			testRandomFillRV(v.Index(i))
		}
	case reflect.Map:
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		if v.Len() == 0 {
			kt, vt := v.Type().Key(), v.Type().Elem()
			for i := 0; i < 4; i++ {
				k0 := reflect.New(kt).Elem()
				v0 := reflect.New(vt).Elem()
				testRandomFillRV(k0)
				testRandomFillRV(v0)
				v.SetMapIndex(k0, v0)
			}
		} else {
			for _, k := range v.MapKeys() {
				testRandomFillRV(v.MapIndex(k))
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(fneg() * rand.Int63n(127))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(uint64(rand.Int63n(255)))
	case reflect.Bool:
		v.SetBool(fneg() == 1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(fneg()) * float64(rand.Float32()))
	case reflect.String:
		// ensure this string can test the extent of json string decoding
		v.SetString(strings.Repeat(strconv.FormatInt(rand.Int63n(99), 10), rand.Intn(8)) +
			"- ABC \x41=\x42 \u2318 - \r \b \f - \u2028 and \u2029 .")
	default:
		panic(fmt.Errorf("testRandomFillRV: unsupported type: %v", v.Kind()))
	}
}

func testMammoth(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	var b []byte

	var m, m2 TestMammoth
	testRandomFillRV(reflect.ValueOf(&m).Elem())
	b = testMarshalErr(&m, h, t, "mammoth-"+name)
	testUnmarshalErr(&m2, b, h, t, "mammoth-"+name)
	testDeepEqualErr(&m, &m2, t, "mammoth-"+name)

	var mm, mm2 TestMammoth2Wrapper
	testRandomFillRV(reflect.ValueOf(&mm).Elem())
	b = testMarshalErr(&mm, h, t, "mammoth2-"+name)
	testUnmarshalErr(&mm2, b, h, t, "mammoth2-"+name)
	testDeepEqualErr(&mm, &mm2, t, "mammoth2-"+name)
	// testMammoth2(t, name, h)
}

func testTime(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	// test time which uses the time.go implementation (ie Binc)
	var tt, tt2 time.Time
	// time in 1990
	tt = time.Unix(20*366*24*60*60, 1000*900).In(time.FixedZone("UGO", -5*60*60))
	// fmt.Printf("time tt: %v\n", tt)
	b := testMarshalErr(tt, h, t, "time-"+name)
	testUnmarshalErr(&tt2, b, h, t, "time-"+name)
	// per go documentation, test time with .Equal not ==
	if !tt2.Equal(tt) {
		logT(t, "%s: values not equal: 1: %v, 2: %v", name, tt2, tt)
		failT(t)
	}
	// testDeepEqualErr(tt.UTC(), tt2, t, "time-"+name)
}

func testUintToInt(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	var golden = [...]int64{
		0, 1, 22, 333, 4444, 55555, 666666,
		// msgpack ones
		24, 128,
		// standard ones
		math.MaxUint8, math.MaxUint8 + 4, math.MaxUint8 - 4,
		math.MaxUint16, math.MaxUint16 + 4, math.MaxUint16 - 4,
		math.MaxUint32, math.MaxUint32 + 4, math.MaxUint32 - 4,
		math.MaxInt8, math.MaxInt8 + 4, math.MaxInt8 - 4,
		math.MaxInt16, math.MaxInt16 + 4, math.MaxInt16 - 4,
		math.MaxInt32, math.MaxInt32 + 4, math.MaxInt32 - 4,
		math.MaxInt64, math.MaxInt64 - 4,
	}
	var ui uint64
	var fi float64
	var b []byte
	for _, i := range golden {
		ui = 0
		b = testMarshalErr(i, h, t, "int2uint-"+name)
		testUnmarshalErr(&ui, b, h, t, "int2uint-"+name)
		if ui != uint64(i) {
			logT(t, "%s: values not equal: %v, %v", name, ui, uint64(i))
			failT(t)
		}
		i = 0
		b = testMarshalErr(ui, h, t, "uint2int-"+name)
		testUnmarshalErr(&i, b, h, t, "uint2int-"+name)
		if i != int64(ui) {
			logT(t, "%s: values not equal: %v, %v", name, i, int64(ui))
			failT(t)
		}
		fi = 0
		b = testMarshalErr(i, h, t, "int2float-"+name)
		testUnmarshalErr(&fi, b, h, t, "int2float-"+name)
		if fi != float64(i) {
			logT(t, "%s: values not equal: %v, %v", name, fi, float64(i))
			failT(t)
		}
	}
}

func doTestDifferentMapOrSliceType(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)

	// - maptype, slicetype: diff from map[string]intf, map[intf]intf or []intf, etc
	//   include map[interface{}]string where some keys are []byte.
	//   To test, take a sequence of []byte and string, and decode into []string and []interface.
	//   Also, decode into map[string]string, map[string]interface{}, map[interface{}]string

	bh := h.getBasicHandle()
	oldM, oldS := bh.MapType, bh.SliceType
	defer func() { bh.MapType, bh.SliceType = oldM, oldS }()

	var b []byte

	var vi = []interface{}{
		"hello 1",
		[]byte("hello 2"),
		"hello 3",
		[]byte("hello 4"),
		"hello 5",
	}
	var vs []string
	var v2i, v2s testMbsT
	var v2ss testMbsCustStrT
	// encode it as a map or as a slice
	for i, v := range vi {
		vv, ok := v.(string)
		if !ok {
			vv = string(v.([]byte))
		}
		vs = append(vs, vv)
		v2i = append(v2i, v, strconv.FormatInt(int64(i+1), 10))
		v2s = append(v2s, vv, strconv.FormatInt(int64(i+1), 10))
		v2ss = append(v2ss, testCustomStringT(vv), testCustomStringT(strconv.FormatInt(int64(i+1), 10)))
	}

	var v2d interface{}

	// encode vs as a list, and decode into a list and compare
	var goldSliceS = []string{"hello 1", "hello 2", "hello 3", "hello 4", "hello 5"}
	var goldSliceI = []interface{}{"hello 1", "hello 2", "hello 3", "hello 4", "hello 5"}
	var goldSlice = []interface{}{goldSliceS, goldSliceI}
	for j, g := range goldSlice {
		bh.SliceType = reflect.TypeOf(g)
		name := fmt.Sprintf("slice-%s-%v", name, j+1)
		b = testMarshalErr(vs, h, t, name)
		v2d = nil
		// v2d = reflect.New(bh.SliceType).Elem().Interface()
		testUnmarshalErr(&v2d, b, h, t, name)
		testDeepEqualErr(v2d, goldSlice[j], t, name)
	}

	// to ensure that we do not use fast-path for map[intf]string, use a custom string type (for goldMapIS).
	// this will allow us to test out the path that sees a []byte where a map has an interface{} type,
	// and convert it to a string for the decoded map key.

	// encode v2i as a map, and decode into a map and compare
	var goldMapSS = map[string]string{"hello 1": "1", "hello 2": "2", "hello 3": "3", "hello 4": "4", "hello 5": "5"}
	var goldMapSI = map[string]interface{}{"hello 1": "1", "hello 2": "2", "hello 3": "3", "hello 4": "4", "hello 5": "5"}
	var goldMapIS = map[interface{}]testCustomStringT{"hello 1": "1", "hello 2": "2", "hello 3": "3", "hello 4": "4", "hello 5": "5"}
	var goldMap = []interface{}{goldMapSS, goldMapSI, goldMapIS}
	for j, g := range goldMap {
		bh.MapType = reflect.TypeOf(g)
		name := fmt.Sprintf("map-%s-%v", name, j+1)
		// for formats that clearly differentiate binary from string, use v2i
		// else use the v2s (with all strings, no []byte)
		v2d = nil
		// v2d = reflect.New(bh.MapType).Elem().Interface()
		switch h.(type) {
		case *MsgpackHandle, *BincHandle, *CborHandle:
			b = testMarshalErr(v2i, h, t, name)
			testUnmarshalErr(&v2d, b, h, t, name)
			testDeepEqualErr(v2d, goldMap[j], t, name)
		default:
			b = testMarshalErr(v2s, h, t, name)
			testUnmarshalErr(&v2d, b, h, t, name)
			testDeepEqualErr(v2d, goldMap[j], t, name)
			b = testMarshalErr(v2ss, h, t, name)
			v2d = nil
			testUnmarshalErr(&v2d, b, h, t, name)
			testDeepEqualErr(v2d, goldMap[j], t, name)
		}
	}

}

func doTestScalars(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)

	// for each scalar:
	// - encode its ptr
	// - encode it (non-ptr)
	// - check that bytes are same
	// - make a copy (using reflect)
	// - check that same
	// - set zero on it
	// - check that its equal to 0 value
	// - decode into new
	// - compare to original

	bh := h.getBasicHandle()
	if !bh.Canonical {
		bh.Canonical = true
		defer func() { bh.Canonical = false }()
	}

	vi := []interface{}{
		int(0),
		int8(0),
		int16(0),
		int32(0),
		int64(0),
		uint(0),
		uint8(0),
		uint16(0),
		uint32(0),
		uint64(0),
		uintptr(0),
		float32(0),
		float64(0),
		bool(false),
		string(""),
		[]byte(nil),
	}
	for _, v := range fastpathAV {
		vi = append(vi, reflect.Zero(v.rt).Interface())
	}
	for _, v := range vi {
		rv := reflect.New(reflect.TypeOf(v)).Elem()
		testRandomFillRV(rv)
		v = rv.Interface()

		rv2 := reflect.New(rv.Type())
		rv2.Elem().Set(rv)
		vp := rv2.Interface()

		var tname string
		switch rv.Kind() {
		case reflect.Map:
			tname = "map[" + rv.Type().Key().Name() + "]" + rv.Type().Elem().Name()
		case reflect.Slice:
			tname = "[]" + rv.Type().Elem().Name()
		default:
			tname = rv.Type().Name()
		}

		var b, b1, b2 []byte
		b1 = testMarshalErr(v, h, t, tname+"-enc")
		// store b1 into b, as b1 slice is reused for next marshal
		b = make([]byte, len(b1))
		copy(b, b1)
		b2 = testMarshalErr(vp, h, t, tname+"-enc-ptr")
		testDeepEqualErr(b1, b2, t, tname+"-enc-eq")
		setZero(vp)
		testDeepEqualErr(rv2.Elem().Interface(), reflect.Zero(rv.Type()).Interface(), t, tname+"-enc-eq-zero-ref")

		vp = rv2.Interface()
		testUnmarshalErr(vp, b, h, t, tname+"-dec")
		testDeepEqualErr(rv2.Elem().Interface(), v, t, tname+"-dec-eq")
	}
}

func doTestIntfMapping(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	rti := reflect.TypeOf((*testIntfMapI)(nil)).Elem()
	defer func() { h.getBasicHandle().Intf2Impl(rti, nil) }()

	type T9 struct {
		I testIntfMapI
	}

	for i, v := range []testIntfMapI{
		// Use a valid string to test some extents of json string decoding
		&testIntfMapT1{"ABC \x41=\x42 \u2318 - \r \b \f - \u2028 and \u2029 ."},
		testIntfMapT2{"DEF"},
	} {
		if err := h.getBasicHandle().Intf2Impl(rti, reflect.TypeOf(v)); err != nil {
			failT(t, "Error mapping %v to %T", rti, v)
		}
		var v1, v2 T9
		v1 = T9{v}
		b := testMarshalErr(v1, h, t, name+"-enc-"+strconv.Itoa(i))
		testUnmarshalErr(&v2, b, h, t, name+"-dec-"+strconv.Itoa(i))
		testDeepEqualErr(v1, v2, t, name+"-dec-eq-"+strconv.Itoa(i))
	}
}

func doTestOmitempty(t *testing.T, name string, h Handle) {
	testOnce.Do(testInitAll)
	if h.getBasicHandle().StructToArray {
		t.Skipf("Skipping OmitEmpty test when StructToArray=true")
	}
	type T1 struct {
		A int  `codec:"a"`
		B *int `codec:"b,omitempty"`
		C int  `codec:"c,omitempty"`
	}
	type T2 struct {
		A int `codec:"a"`
	}
	var v1 T1
	var v2 T2
	b1 := testMarshalErr(v1, h, t, name+"-omitempty")
	b2 := testMarshalErr(v2, h, t, name+"-no-omitempty-trunc")
	testDeepEqualErr(b1, b2, t, name+"-omitempty-cmp")
}

// -----------------

func TestJsonDecodeNonStringScalarInStringContext(t *testing.T) {
	testOnce.Do(testInitAll)
	var b = `{"s.true": "true", "b.true": true, "s.false": "false", "b.false": false, "s.10": "10", "i.10": 10, "i.-10": -10}`
	var golden = map[string]string{"s.true": "true", "b.true": "true", "s.false": "false", "b.false": "false", "s.10": "10", "i.10": "10", "i.-10": "-10"}

	var m map[string]string
	d := NewDecoderBytes([]byte(b), testJsonH)
	d.MustDecode(&m)
	if err := deepEqual(golden, m); err == nil {
		logT(t, "++++ match: decoded: %#v", m)
	} else {
		logT(t, "---- mismatch: %v ==> golden: %#v, decoded: %#v", err, golden, m)
		failT(t)
	}
}

func TestJsonEncodeIndent(t *testing.T) {
	testOnce.Do(testInitAll)
	v := TestSimplish{
		Ii: -794,
		Ss: `A Man is
after the new line
	after new line and tab
`,
	}
	v2 := v
	v.Mm = make(map[string]*TestSimplish)
	for i := 0; i < len(v.Ar); i++ {
		v3 := v2
		v3.Ii += (i * 4)
		v3.Ss = fmt.Sprintf("%d - %s", v3.Ii, v3.Ss)
		if i%2 == 0 {
			v.Ar[i] = &v3
		}
		// v3 = v2
		v.Sl = append(v.Sl, &v3)
		v.Mm[strconv.FormatInt(int64(i), 10)] = &v3
	}
	oldcan := testJsonH.Canonical
	oldIndent := testJsonH.Indent
	oldS2A := testJsonH.StructToArray
	defer func() {
		testJsonH.Canonical = oldcan
		testJsonH.Indent = oldIndent
		testJsonH.StructToArray = oldS2A
	}()
	testJsonH.Canonical = true
	testJsonH.Indent = -1
	testJsonH.StructToArray = false
	var bs []byte
	NewEncoderBytes(&bs, testJsonH).MustEncode(&v)
	txt1Tab := string(bs)
	bs = nil
	testJsonH.Indent = 120
	NewEncoderBytes(&bs, testJsonH).MustEncode(&v)
	txtSpaces := string(bs)
	// fmt.Printf("\n-----------\n%s\n------------\n%s\n-------------\n", txt1Tab, txtSpaces)

	goldenResultTab := `{
	"Ar": [
		{
			"Ar": [
				null,
				null
			],
			"Ii": -794,
			"Mm": null,
			"Sl": null,
			"Ss": "-794 - A Man is\nafter the new line\n\tafter new line and tab\n"
		},
		null
	],
	"Ii": -794,
	"Mm": {
		"0": {
			"Ar": [
				null,
				null
			],
			"Ii": -794,
			"Mm": null,
			"Sl": null,
			"Ss": "-794 - A Man is\nafter the new line\n\tafter new line and tab\n"
		},
		"1": {
			"Ar": [
				null,
				null
			],
			"Ii": -790,
			"Mm": null,
			"Sl": null,
			"Ss": "-790 - A Man is\nafter the new line\n\tafter new line and tab\n"
		}
	},
	"Sl": [
		{
			"Ar": [
				null,
				null
			],
			"Ii": -794,
			"Mm": null,
			"Sl": null,
			"Ss": "-794 - A Man is\nafter the new line\n\tafter new line and tab\n"
		},
		{
			"Ar": [
				null,
				null
			],
			"Ii": -790,
			"Mm": null,
			"Sl": null,
			"Ss": "-790 - A Man is\nafter the new line\n\tafter new line and tab\n"
		}
	],
	"Ss": "A Man is\nafter the new line\n\tafter new line and tab\n"
}`

	if txt1Tab != goldenResultTab {
		logT(t, "decoded indented with tabs != expected: \nexpected: %s\nencoded: %s", goldenResultTab, txt1Tab)
		failT(t)
	}
	if txtSpaces != strings.Replace(goldenResultTab, "\t", strings.Repeat(" ", 120), -1) {
		logT(t, "decoded indented with spaces != expected: \nexpected: %s\nencoded: %s", goldenResultTab, txtSpaces)
		failT(t)
	}
}

func TestBufioDecReader(t *testing.T) {
	testOnce.Do(testInitAll)
	// try to read 85 bytes in chunks of 7 at a time.
	var s = strings.Repeat("01234'56789      ", 5)
	// fmt.Printf("s: %s\n", s)
	var r = strings.NewReader(s)
	var br = &bufioDecReader{r: r, buf: make([]byte, 0, 13)}
	b, err := ioutil.ReadAll(br)
	if err != nil {
		panic(err)
	}
	var s2 = string(b)
	// fmt.Printf("s==s2: %v, len(s): %v, len(b): %v, len(s2): %v\n", s == s2, len(s), len(b), len(s2))
	if s != s2 {
		logT(t, "not equal: \ns:  %s\ns2: %s", s, s2)
		failT(t)
	}
	// Now, test search functions for skip, readTo and readUntil
	// readUntil ', readTo ', skip whitespace. 3 times in a loop, each time compare the token and/or outs
	// readUntil: see: 56789
	var out []byte
	var token byte
	br = &bufioDecReader{r: strings.NewReader(s), buf: make([]byte, 0, 7)}
	// println()
	for _, v2 := range [...]string{
		`01234'`,
		`56789      01234'`,
		`56789      01234'`,
		`56789      01234'`,
	} {
		out = br.readUntil(nil, '\'')
		testDeepEqualErr(string(out), v2, t, "-")
		// fmt.Printf("readUntil: out: `%s`\n", out)
	}
	br = &bufioDecReader{r: strings.NewReader(s), buf: make([]byte, 0, 7)}
	// println()
	for range [4]struct{}{} {
		out = br.readTo(nil, &jsonNumSet)
		testDeepEqualErr(string(out), `01234`, t, "-")
		// fmt.Printf("readTo: out: `%s`\n", out)
		out = br.readUntil(nil, '\'')
		testDeepEqualErr(string(out), "'", t, "-")
		// fmt.Printf("readUntil: out: `%s`\n", out)
		out = br.readTo(nil, &jsonNumSet)
		testDeepEqualErr(string(out), `56789`, t, "-")
		// fmt.Printf("readTo: out: `%s`\n", out)
		out = br.readUntil(nil, '0')
		testDeepEqualErr(string(out), `      0`, t, "-")
		// fmt.Printf("readUntil: out: `%s`\n", out)
		br.UnreadByte()
	}
	br = &bufioDecReader{r: strings.NewReader(s), buf: make([]byte, 0, 7)}
	// println()
	for range [4]struct{}{} {
		out = br.readUntil(nil, ' ')
		testDeepEqualErr(string(out), `01234'56789 `, t, "-")
		// fmt.Printf("readUntil: out: |%s|\n", out)
		token = br.skip(&jsonCharWhitespaceSet)
		testDeepEqualErr(token, byte('0'), t, "-")
		// fmt.Printf("skip: token: '%c'\n", token)
		br.UnreadByte()
	}
	// println()
}

// -----------

func TestJsonLargeInteger(t *testing.T) {
	testOnce.Do(testInitAll)
	for _, i := range []uint8{'L', 'A', 0} {
		for _, j := range []interface{}{
			int64(1 << 60),
			-int64(1 << 60),
			0,
			1 << 20,
			-(1 << 20),
			uint64(1 << 60),
			uint(0),
			uint(1 << 20),
		} {
			doTestJsonLargeInteger(t, j, i)
		}
	}
}

func TestJsonInvalidUnicode(t *testing.T) {
	testOnce.Do(testInitAll)
	var m = map[string]string{
		`"\udc49\u0430abc"`: "\uFFFDabc",
		`"\udc49\u0430"`:    "\uFFFD",
		`"\udc49abc"`:       "\uFFFDabc",
		`"\udc49"`:          "\uFFFD",
		`"\udZ49\u0430abc"`: "\uFFFD\u0430abc",
		`"\udcG9\u0430"`:    "\uFFFD\u0430",
		`"\uHc49abc"`:       "\uFFFDabc",
		`"\uKc49"`:          "\uFFFD",
		// ``: "",
	}
	for k, v := range m {
		// println("k = ", k)
		var s string
		testUnmarshalErr(&s, []byte(k), testJsonH, t, "-")
		if s != v {
			logT(t, "not equal: %q, %q", v, s)
			failT(t)
		}
	}
}

// ----------

func TestBincCodecsTable(t *testing.T) {
	testCodecTableOne(t, testBincH)
}

func TestBincCodecsMisc(t *testing.T) {
	testCodecMiscOne(t, testBincH)
}

func TestBincCodecsEmbeddedPointer(t *testing.T) {
	testCodecEmbeddedPointer(t, testBincH)
}

func TestBincStdEncIntf(t *testing.T) {
	doTestStdEncIntf(t, "binc", testBincH)
}

func TestBincMammoth(t *testing.T) {
	testMammoth(t, "binc", testBincH)
}

func TestSimpleCodecsTable(t *testing.T) {
	testCodecTableOne(t, testSimpleH)
}

func TestSimpleCodecsMisc(t *testing.T) {
	testCodecMiscOne(t, testSimpleH)
}

func TestSimpleCodecsEmbeddedPointer(t *testing.T) {
	testCodecEmbeddedPointer(t, testSimpleH)
}

func TestSimpleStdEncIntf(t *testing.T) {
	doTestStdEncIntf(t, "simple", testSimpleH)
}

func TestSimpleMammoth(t *testing.T) {
	testMammoth(t, "simple", testSimpleH)
}

func TestMsgpackCodecsTable(t *testing.T) {
	testCodecTableOne(t, testMsgpackH)
}

func TestMsgpackCodecsMisc(t *testing.T) {
	testCodecMiscOne(t, testMsgpackH)
}

func TestMsgpackCodecsEmbeddedPointer(t *testing.T) {
	testCodecEmbeddedPointer(t, testMsgpackH)
}

func TestMsgpackStdEncIntf(t *testing.T) {
	doTestStdEncIntf(t, "msgpack", testMsgpackH)
}

func TestMsgpackMammoth(t *testing.T) {
	testMammoth(t, "msgpack", testMsgpackH)
}

func TestCborCodecsTable(t *testing.T) {
	testCodecTableOne(t, testCborH)
}

func TestCborCodecsMisc(t *testing.T) {
	testCodecMiscOne(t, testCborH)
}

func TestCborCodecsEmbeddedPointer(t *testing.T) {
	testCodecEmbeddedPointer(t, testCborH)
}

func TestCborMapEncodeForCanonical(t *testing.T) {
	doTestMapEncodeForCanonical(t, "cbor", testCborH)
}

func TestCborCodecChan(t *testing.T) {
	testCodecChan(t, testCborH)
}

func TestCborStdEncIntf(t *testing.T) {
	doTestStdEncIntf(t, "cbor", testCborH)
}

func TestCborMammoth(t *testing.T) {
	testMammoth(t, "cbor", testCborH)
}

func TestJsonCodecsTable(t *testing.T) {
	testCodecTableOne(t, testJsonH)
}

func TestJsonCodecsMisc(t *testing.T) {
	testCodecMiscOne(t, testJsonH)
}

func TestJsonCodecsEmbeddedPointer(t *testing.T) {
	testCodecEmbeddedPointer(t, testJsonH)
}

func TestJsonCodecChan(t *testing.T) {
	testCodecChan(t, testJsonH)
}

func TestJsonStdEncIntf(t *testing.T) {
	doTestStdEncIntf(t, "json", testJsonH)
}

func TestJsonMammoth(t *testing.T) {
	testMammoth(t, "json", testJsonH)
}

// ----- Raw ---------
func TestJsonRaw(t *testing.T) {
	doTestRawValue(t, "json", testJsonH)
}
func TestBincRaw(t *testing.T) {
	doTestRawValue(t, "binc", testBincH)
}
func TestMsgpackRaw(t *testing.T) {
	doTestRawValue(t, "msgpack", testMsgpackH)
}
func TestSimpleRaw(t *testing.T) {
	doTestRawValue(t, "simple", testSimpleH)
}
func TestCborRaw(t *testing.T) {
	doTestRawValue(t, "cbor", testCborH)
}

// ----- ALL (framework based) -----

func TestAllEncCircularRef(t *testing.T) {
	doTestEncCircularRef(t, "cbor", testCborH)
}

func TestAllAnonCycle(t *testing.T) {
	doTestAnonCycle(t, "cbor", testCborH)
}

// ----- RPC -----

func TestBincRpcGo(t *testing.T) {
	testCodecRpcOne(t, GoRpc, testBincH, true, 0)
}

func TestSimpleRpcGo(t *testing.T) {
	testCodecRpcOne(t, GoRpc, testSimpleH, true, 0)
}

func TestMsgpackRpcGo(t *testing.T) {
	testCodecRpcOne(t, GoRpc, testMsgpackH, true, 0)
}

func TestCborRpcGo(t *testing.T) {
	testCodecRpcOne(t, GoRpc, testCborH, true, 0)
}

func TestJsonRpcGo(t *testing.T) {
	testCodecRpcOne(t, GoRpc, testJsonH, true, 0)
}

func TestMsgpackRpcSpec(t *testing.T) {
	testCodecRpcOne(t, MsgpackSpecRpc, testMsgpackH, true, 0)
}

func TestBincUnderlyingType(t *testing.T) {
	testCodecUnderlyingType(t, testBincH)
}

func TestJsonSwallowAndZero(t *testing.T) {
	doTestSwallowAndZero(t, testJsonH)
}

func TestCborSwallowAndZero(t *testing.T) {
	doTestSwallowAndZero(t, testCborH)
}

func TestMsgpackSwallowAndZero(t *testing.T) {
	doTestSwallowAndZero(t, testMsgpackH)
}

func TestBincSwallowAndZero(t *testing.T) {
	doTestSwallowAndZero(t, testBincH)
}

func TestSimpleSwallowAndZero(t *testing.T) {
	doTestSwallowAndZero(t, testSimpleH)
}

func TestJsonRawExt(t *testing.T) {
	doTestRawExt(t, testJsonH)
}

func TestCborRawExt(t *testing.T) {
	doTestRawExt(t, testCborH)
}

func TestMsgpackRawExt(t *testing.T) {
	doTestRawExt(t, testMsgpackH)
}

func TestBincRawExt(t *testing.T) {
	doTestRawExt(t, testBincH)
}

func TestSimpleRawExt(t *testing.T) {
	doTestRawExt(t, testSimpleH)
}

func TestJsonMapStructKey(t *testing.T) {
	doTestMapStructKey(t, testJsonH)
}

func TestCborMapStructKey(t *testing.T) {
	doTestMapStructKey(t, testCborH)
}

func TestMsgpackMapStructKey(t *testing.T) {
	doTestMapStructKey(t, testMsgpackH)
}

func TestBincMapStructKey(t *testing.T) {
	doTestMapStructKey(t, testBincH)
}

func TestSimpleMapStructKey(t *testing.T) {
	doTestMapStructKey(t, testSimpleH)
}

func TestJsonDecodeNilMapValue(t *testing.T) {
	doTestDecodeNilMapValue(t, testJsonH)
}

func TestCborDecodeNilMapValue(t *testing.T) {
	doTestDecodeNilMapValue(t, testCborH)
}

func TestMsgpackDecodeNilMapValue(t *testing.T) {
	doTestDecodeNilMapValue(t, testMsgpackH)
}

func TestBincDecodeNilMapValue(t *testing.T) {
	doTestDecodeNilMapValue(t, testBincH)
}

func TestSimpleDecodeNilMapValue(t *testing.T) {
	doTestDecodeNilMapValue(t, testSimpleH)
}

func TestJsonEmbeddedFieldPrecedence(t *testing.T) {
	doTestEmbeddedFieldPrecedence(t, testJsonH)
}

func TestCborEmbeddedFieldPrecedence(t *testing.T) {
	doTestEmbeddedFieldPrecedence(t, testCborH)
}

func TestMsgpackEmbeddedFieldPrecedence(t *testing.T) {
	doTestEmbeddedFieldPrecedence(t, testMsgpackH)
}

func TestBincEmbeddedFieldPrecedence(t *testing.T) {
	doTestEmbeddedFieldPrecedence(t, testBincH)
}

func TestSimpleEmbeddedFieldPrecedence(t *testing.T) {
	doTestEmbeddedFieldPrecedence(t, testSimpleH)
}

func TestJsonLargeContainerLen(t *testing.T) {
	doTestLargeContainerLen(t, testJsonH)
}

func TestCborLargeContainerLen(t *testing.T) {
	doTestLargeContainerLen(t, testCborH)
}

func TestMsgpackLargeContainerLen(t *testing.T) {
	doTestLargeContainerLen(t, testMsgpackH)
}

func TestBincLargeContainerLen(t *testing.T) {
	doTestLargeContainerLen(t, testBincH)
}

func TestSimpleLargeContainerLen(t *testing.T) {
	doTestLargeContainerLen(t, testSimpleH)
}

func TestJsonMammothMapsAndSlices(t *testing.T) {
	doTestMammothMapsAndSlices(t, testJsonH)
}

func TestCborMammothMapsAndSlices(t *testing.T) {
	doTestMammothMapsAndSlices(t, testCborH)
}

func TestMsgpackMammothMapsAndSlices(t *testing.T) {
	old1, old2 := testMsgpackH.RawToString, testMsgpackH.WriteExt
	defer func() { testMsgpackH.RawToString, testMsgpackH.WriteExt = old1, old2 }()
	testMsgpackH.RawToString = true
	testMsgpackH.WriteExt = true

	doTestMammothMapsAndSlices(t, testMsgpackH)
}

func TestBincMammothMapsAndSlices(t *testing.T) {
	doTestMammothMapsAndSlices(t, testBincH)
}

func TestSimpleMammothMapsAndSlices(t *testing.T) {
	doTestMammothMapsAndSlices(t, testSimpleH)
}

func TestJsonTime(t *testing.T) {
	testTime(t, "json", testJsonH)
}

func TestCborTime(t *testing.T) {
	testTime(t, "cbor", testCborH)
}

func TestMsgpackTime(t *testing.T) {
	testTime(t, "msgpack", testMsgpackH)
}

func TestBincTime(t *testing.T) {
	testTime(t, "binc", testBincH)
}

func TestSimpleTime(t *testing.T) {
	testTime(t, "simple", testSimpleH)
}

func TestJsonUintToInt(t *testing.T) {
	testUintToInt(t, "json", testJsonH)
}

func TestCborUintToInt(t *testing.T) {
	testUintToInt(t, "cbor", testCborH)
}

func TestMsgpackUintToInt(t *testing.T) {
	testUintToInt(t, "msgpack", testMsgpackH)
}

func TestBincUintToInt(t *testing.T) {
	testUintToInt(t, "binc", testBincH)
}

func TestSimpleUintToInt(t *testing.T) {
	testUintToInt(t, "simple", testSimpleH)
}

func TestJsonDifferentMapOrSliceType(t *testing.T) {
	doTestDifferentMapOrSliceType(t, "json", testJsonH)
}

func TestCborDifferentMapOrSliceType(t *testing.T) {
	doTestDifferentMapOrSliceType(t, "cbor", testCborH)
}

func TestMsgpackDifferentMapOrSliceType(t *testing.T) {
	doTestDifferentMapOrSliceType(t, "msgpack", testMsgpackH)
}

func TestBincDifferentMapOrSliceType(t *testing.T) {
	doTestDifferentMapOrSliceType(t, "binc", testBincH)
}

func TestSimpleDifferentMapOrSliceType(t *testing.T) {
	doTestDifferentMapOrSliceType(t, "simple", testSimpleH)
}

func TestJsonScalars(t *testing.T) {
	doTestScalars(t, "json", testJsonH)
}

func TestCborScalars(t *testing.T) {
	doTestScalars(t, "cbor", testCborH)
}

func TestMsgpackScalars(t *testing.T) {
	doTestScalars(t, "msgpack", testMsgpackH)
}

func TestBincScalars(t *testing.T) {
	doTestScalars(t, "binc", testBincH)
}

func TestSimpleScalars(t *testing.T) {
	doTestScalars(t, "simple", testSimpleH)
}

func TestJsonOmitempty(t *testing.T) {
	doTestOmitempty(t, "json", testJsonH)
}

func TestCborOmitempty(t *testing.T) {
	doTestOmitempty(t, "cbor", testCborH)
}

func TestMsgpackOmitempty(t *testing.T) {
	doTestOmitempty(t, "msgpack", testMsgpackH)
}

func TestBincOmitempty(t *testing.T) {
	doTestOmitempty(t, "binc", testBincH)
}

func TestSimpleOmitempty(t *testing.T) {
	doTestOmitempty(t, "simple", testSimpleH)
}

func TestJsonIntfMapping(t *testing.T) {
	doTestIntfMapping(t, "json", testJsonH)
}

func TestCborIntfMapping(t *testing.T) {
	doTestIntfMapping(t, "cbor", testCborH)
}

func TestMsgpackIntfMapping(t *testing.T) {
	doTestIntfMapping(t, "msgpack", testMsgpackH)
}

func TestBincIntfMapping(t *testing.T) {
	doTestIntfMapping(t, "binc", testBincH)
}

func TestSimpleIntfMapping(t *testing.T) {
	doTestIntfMapping(t, "simple", testSimpleH)
}

// TODO:
//
// Add Tests for:
// - struct tags: on anonymous fields, _struct (all fields), etc
// - chan to encode and decode (with support for codecgen also)
//
// Add negative tests for failure conditions:
// - bad input with large array length prefix
//
// decode.go (standalone)
// - UnreadByte: only 2 states (z.ls = 2 and z.ls = 1) (0 --> 2 --> 1)
// - track: z.trb: track, stop track, check
// - PreferArrayOverSlice???
// - InterfaceReset
// - (chan byte) to decode []byte (with mapbyslice track)
// - decode slice of len 6, 16 into slice of (len 4, cap 8) and (len ) with maxinitlen=6, 8, 16
// - DeleteOnNilMapValue
// - decnaked: n.l == nil
// - ensureDecodeable (try to decode into a non-decodeable thing e.g. a nil interface{},
//
// encode.go (standalone)
// - nil and 0-len slices and maps for non-fastpath things
