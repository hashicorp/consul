package test

import (
	"testing"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
	"github.com/v2pro/plz/dump"
	"reflect"
	"fmt"
	"unsafe"
)

type nameOff int32 // offset to a name
type typeOff int32 // offset to an *rtype
type textOff int32 // offset from top of text section

// a copy of runtime.typeAlg
type typeAlg struct {
	// function for hashing objects of this type
	// (ptr to object, seed) -> hash
	hash func(unsafe.Pointer, uintptr) uintptr
	// function for comparing objects of this type
	// (ptr to object A, ptr to object B) -> ==?
	equal func(unsafe.Pointer, unsafe.Pointer) bool
}

// tflag is used by an rtype to signal what extra type information is
// available in the memory directly following the rtype value.
//
// tflag values must be kept in sync with copies in:
//	cmd/compile/internal/gc/reflect.go
//	cmd/link/internal/ld/decodesym.go
//	runtime/type.go
type tflag uint8

// rtype is the common implementation of most values.
// It is embedded in other, public struct types, but always
// with a unique tag like `reflect:"array"` or `reflect:"ptr"`
// so that code cannot convert from, say, *arrayType to *ptrType.
//
// rtype must be kept in sync with ../runtime/type.go:/^type._type.
type rtype struct {
	size       uintptr
	ptrdata    uintptr  // number of bytes in the type that can contain pointers
	hash       uint32   // hash of type; avoids computation in hash tables
	tflag      tflag    // extra type information flags
	align      uint8    // alignment of variable with this type
	fieldAlign uint8    // alignment of struct field with this type
	kind       uint8    // enumeration for C
	alg        *typeAlg // algorithm table
	gcdata     *byte    // garbage collection data
	str        nameOff  // string form
	ptrToThis  typeOff  // type for pointer to this type, may be zero
}

type iface struct {
	itab unsafe.Pointer
	data unsafe.Pointer
}

// mapType represents a map type.
type mapType struct {
	rtype         `reflect:"map"`
	key           *rtype // map key type
	elem          *rtype // map element (value) type
	bucket        *rtype // internal bucket structure
	hmap          *rtype // internal map header
	keysize       uint8  // size of key slot
	indirectkey   uint8  // store ptr to key instead of key itself
	valuesize     uint8  // size of value slot
	indirectvalue uint8  // store ptr to value instead of value itself
	bucketsize    uint16 // size of bucket
	reflexivekey  bool   // true if k==k for all keys
	needkeyupdate bool   // true if we need to update key on an overwrite
}

func Test_map(t *testing.T) {
	t.Run("map int to int", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
		"__root__": {
			"type": "map[int]int",
			"data": {
				"__ptr__": "{ptr1}"
			}
		},
		"{ptr1}": {
			"count": 2,
			"flags": 0,
			"B": 0,
			"noverflow": 0,
			"hash0": "{ANYTHING}",
			"buckets": {"__ptr__":"{ptr2}"},
			"oldbuckets": {"__ptr__":"0"},
			"nevacuate": 0,
			"extra": {"__ptr__":"0"}
		},
		"{ptr2}": [{
			"tophash": "{ANYTHING}",
			"keys": [9,8,0,0,0,0,0,0],
			"elems": [7,6,0,0,0,0,0,0]
		}]}`, dump.Var{map[int]int{
			9: 7,
			8: 6,
		}}.String())
	}))
	t.Run("map string to string", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
		"__root__": {
			"type": "map[string]string",
			"data": {
				"__ptr__": "{ptr1}"
			}
		},
		"{ptr1}": {
			"count": 2,
			"flags": 0,
			"B": 0,
			"noverflow": 0,
			"hash0": "{ANYTHING}",
			"buckets": {"__ptr__":"{ptr2}"},
			"oldbuckets": {"__ptr__":"0"},
			"nevacuate": 0,
			"extra": {"__ptr__":"0"}
		},
		"{ptr2}": [{
			"tophash": "{ANYTHING}",
			"keys": [
				{"data":{"__ptr__":"{key1}"},"len":1},
				{"data":{"__ptr__":"{key2}"},"len":1},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0}
			],
			"elems": [
				{"data":{"__ptr__":"{elem1}"},"len":1},
				{"data":{"__ptr__":"{elem2}"},"len":1},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0},
				{"data":{"__ptr__":"0"},"len":0}
			]
		}],
		"{key1}":"a",
		"{key2}":"c",
		"{elem1}":"b",
		"{elem2}":"d"
		}`, dump.Var{map[string]string{
			"a": "b",
			"c": "d",
		}}.String())
	}))
	t.Run("map type", test.Case(func(ctx *countlog.Context) {
		m := map[int]*int{}
		mType := reflect.TypeOf(m)
		mIFace := (*iface)(unsafe.Pointer(&mType))
		mapType := (*mapType)(mIFace.data)
		fmt.Println(mapType.bucket.ptrdata)
	}))
}
