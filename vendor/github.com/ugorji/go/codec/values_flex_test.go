/* // +build testing */

// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import "time"

// This file contains values used by tests alone.
// This is where we may try out different things,
// that other engines may not support or may barf upon
// e.g. custom extensions for wrapped types, maps with non-string keys, etc.

// Some unused types just stored here
type Bbool bool
type Sstring string
type Sstructsmall struct {
	A int
}

type Sstructbig struct {
	A int
	B bool
	c string
	// Sval Sstruct
	Ssmallptr *Sstructsmall
	Ssmall    *Sstructsmall
	Sptr      *Sstructbig
}

type SstructbigMapBySlice struct {
	_struct struct{} `codec:",toarray"`
	A       int
	B       bool
	c       string
	// Sval Sstruct
	Ssmallptr *Sstructsmall
	Ssmall    *Sstructsmall
	Sptr      *Sstructbig
}

type Sinterface interface {
	Noop()
}

// small struct for testing that codecgen works for unexported types
type tLowerFirstLetter struct {
	I int
	u uint64
	S string
	b []byte
}

// Some used types
type wrapInt64 int64
type wrapUint8 uint8
type wrapBytes []uint8

type AnonInTestStrucIntf struct {
	Islice []interface{}
	Ms     map[string]interface{}
	Nintf  interface{} //don't set this, so we can test for nil
	T      time.Time
}

var testWRepeated512 wrapBytes
var testStrucTime = time.Date(2012, 2, 2, 2, 2, 2, 2000, time.UTC).UTC()

func init() {
	var testARepeated512 [512]byte
	for i := range testARepeated512 {
		testARepeated512[i] = 'A'
	}
	testWRepeated512 = wrapBytes(testARepeated512[:])
}

type TestStrucFlex struct {
	_struct struct{} `codec:",omitempty"` //set omitempty for every field
	TestStrucCommon

	Mis     map[int]string
	Mbu64   map[bool]struct{}
	Miwu64s map[int]wrapUint64Slice
	Mfwss   map[float64]wrapStringSlice
	Mf32wss map[float32]wrapStringSlice
	Mui2wss map[uint64]wrapStringSlice
	Msu2wss map[stringUint64T]wrapStringSlice

	Ci64       wrapInt64
	Swrapbytes []wrapBytes
	Swrapuint8 []wrapUint8

	ArrStrUi64T [4]stringUint64T

	Ui64array      [4]uint64
	Ui64slicearray []*[4]uint64

	// make this a ptr, so that it could be set or not.
	// for comparison (e.g. with msgp), give it a struct tag (so it is not inlined),
	// make this one omitempty (so it is excluded if nil).
	*AnonInTestStrucIntf `json:",omitempty"`

	//M map[interface{}]interface{}  `json:"-",bson:"-"`
	Mtsptr     map[string]*TestStrucFlex
	Mts        map[string]TestStrucFlex
	Its        []*TestStrucFlex
	Nteststruc *TestStrucFlex
}

func newTestStrucFlex(depth, n int, bench, useInterface, useStringKeyOnly bool) (ts *TestStrucFlex) {
	ts = &TestStrucFlex{
		Miwu64s: map[int]wrapUint64Slice{
			5: []wrapUint64{1, 2, 3, 4, 5},
			3: []wrapUint64{1, 2, 3},
		},

		Mf32wss: map[float32]wrapStringSlice{
			5.0: []wrapString{"1.0", "2.0", "3.0", "4.0", "5.0"},
			3.0: []wrapString{"1.0", "2.0", "3.0"},
		},

		Mui2wss: map[uint64]wrapStringSlice{
			5: []wrapString{"1.0", "2.0", "3.0", "4.0", "5.0"},
			3: []wrapString{"1.0", "2.0", "3.0"},
		},

		Mfwss: map[float64]wrapStringSlice{
			5.0: []wrapString{"1.0", "2.0", "3.0", "4.0", "5.0"},
			3.0: []wrapString{"1.0", "2.0", "3.0"},
		},
		Mis: map[int]string{
			1:   "one",
			22:  "twenty two",
			-44: "minus forty four",
		},
		Mbu64: map[bool]struct{}{false: {}, true: {}},

		Ci64: -22,
		Swrapbytes: []wrapBytes{ // lengths of 1, 2, 4, 8, 16, 32, 64, 128, 256,
			testWRepeated512[:1],
			testWRepeated512[:2],
			testWRepeated512[:4],
			testWRepeated512[:8],
			testWRepeated512[:16],
			testWRepeated512[:32],
			testWRepeated512[:64],
			testWRepeated512[:128],
			testWRepeated512[:256],
			testWRepeated512[:512],
		},
		Swrapuint8: []wrapUint8{
			'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J',
		},
		Ui64array:   [4]uint64{4, 16, 64, 256},
		ArrStrUi64T: [4]stringUint64T{{"4", 4}, {"3", 3}, {"2", 2}, {"1", 1}},
	}

	ts.Ui64slicearray = []*[4]uint64{&ts.Ui64array, &ts.Ui64array}

	if useInterface {
		ts.AnonInTestStrucIntf = &AnonInTestStrucIntf{
			Islice: []interface{}{strRpt(n, "true"), true, strRpt(n, "no"), false, uint64(288), float64(0.4)},
			Ms: map[string]interface{}{
				strRpt(n, "true"):     strRpt(n, "true"),
				strRpt(n, "int64(9)"): false,
			},
			T: testStrucTime,
		}
	}

	populateTestStrucCommon(&ts.TestStrucCommon, n, bench, useInterface, useStringKeyOnly)
	if depth > 0 {
		depth--
		if ts.Mtsptr == nil {
			ts.Mtsptr = make(map[string]*TestStrucFlex)
		}
		if ts.Mts == nil {
			ts.Mts = make(map[string]TestStrucFlex)
		}
		ts.Mtsptr["0"] = newTestStrucFlex(depth, n, bench, useInterface, useStringKeyOnly)
		ts.Mts["0"] = *(ts.Mtsptr["0"])
		ts.Its = append(ts.Its, ts.Mtsptr["0"])
	}
	return
}
