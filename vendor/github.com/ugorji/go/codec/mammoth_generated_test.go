// Copyright (c) 2012-2015 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

// Code generated from mammoth-test.go.tmpl - DO NOT EDIT.

package codec

import "testing"
import "fmt"
import "reflect"

// TestMammoth has all the different paths optimized in fast-path
// It has all the primitives, slices and maps.
//
// For each of those types, it has a pointer and a non-pointer field.

func init() { _ = fmt.Printf } // so we can include fmt as needed

type TestMammoth struct {
	FIntf       interface{}
	FptrIntf    *interface{}
	FString     string
	FptrString  *string
	FFloat32    float32
	FptrFloat32 *float32
	FFloat64    float64
	FptrFloat64 *float64
	FUint       uint
	FptrUint    *uint
	FUint8      uint8
	FptrUint8   *uint8
	FUint16     uint16
	FptrUint16  *uint16
	FUint32     uint32
	FptrUint32  *uint32
	FUint64     uint64
	FptrUint64  *uint64
	FUintptr    uintptr
	FptrUintptr *uintptr
	FInt        int
	FptrInt     *int
	FInt8       int8
	FptrInt8    *int8
	FInt16      int16
	FptrInt16   *int16
	FInt32      int32
	FptrInt32   *int32
	FInt64      int64
	FptrInt64   *int64
	FBool       bool
	FptrBool    *bool

	FSliceIntf       []interface{}
	FptrSliceIntf    *[]interface{}
	FSliceString     []string
	FptrSliceString  *[]string
	FSliceFloat32    []float32
	FptrSliceFloat32 *[]float32
	FSliceFloat64    []float64
	FptrSliceFloat64 *[]float64
	FSliceUint       []uint
	FptrSliceUint    *[]uint
	FSliceUint8      []uint8
	FptrSliceUint8   *[]uint8
	FSliceUint16     []uint16
	FptrSliceUint16  *[]uint16
	FSliceUint32     []uint32
	FptrSliceUint32  *[]uint32
	FSliceUint64     []uint64
	FptrSliceUint64  *[]uint64
	FSliceUintptr    []uintptr
	FptrSliceUintptr *[]uintptr
	FSliceInt        []int
	FptrSliceInt     *[]int
	FSliceInt8       []int8
	FptrSliceInt8    *[]int8
	FSliceInt16      []int16
	FptrSliceInt16   *[]int16
	FSliceInt32      []int32
	FptrSliceInt32   *[]int32
	FSliceInt64      []int64
	FptrSliceInt64   *[]int64
	FSliceBool       []bool
	FptrSliceBool    *[]bool

	FMapIntfIntf          map[interface{}]interface{}
	FptrMapIntfIntf       *map[interface{}]interface{}
	FMapIntfString        map[interface{}]string
	FptrMapIntfString     *map[interface{}]string
	FMapIntfUint          map[interface{}]uint
	FptrMapIntfUint       *map[interface{}]uint
	FMapIntfUint8         map[interface{}]uint8
	FptrMapIntfUint8      *map[interface{}]uint8
	FMapIntfUint16        map[interface{}]uint16
	FptrMapIntfUint16     *map[interface{}]uint16
	FMapIntfUint32        map[interface{}]uint32
	FptrMapIntfUint32     *map[interface{}]uint32
	FMapIntfUint64        map[interface{}]uint64
	FptrMapIntfUint64     *map[interface{}]uint64
	FMapIntfUintptr       map[interface{}]uintptr
	FptrMapIntfUintptr    *map[interface{}]uintptr
	FMapIntfInt           map[interface{}]int
	FptrMapIntfInt        *map[interface{}]int
	FMapIntfInt8          map[interface{}]int8
	FptrMapIntfInt8       *map[interface{}]int8
	FMapIntfInt16         map[interface{}]int16
	FptrMapIntfInt16      *map[interface{}]int16
	FMapIntfInt32         map[interface{}]int32
	FptrMapIntfInt32      *map[interface{}]int32
	FMapIntfInt64         map[interface{}]int64
	FptrMapIntfInt64      *map[interface{}]int64
	FMapIntfFloat32       map[interface{}]float32
	FptrMapIntfFloat32    *map[interface{}]float32
	FMapIntfFloat64       map[interface{}]float64
	FptrMapIntfFloat64    *map[interface{}]float64
	FMapIntfBool          map[interface{}]bool
	FptrMapIntfBool       *map[interface{}]bool
	FMapStringIntf        map[string]interface{}
	FptrMapStringIntf     *map[string]interface{}
	FMapStringString      map[string]string
	FptrMapStringString   *map[string]string
	FMapStringUint        map[string]uint
	FptrMapStringUint     *map[string]uint
	FMapStringUint8       map[string]uint8
	FptrMapStringUint8    *map[string]uint8
	FMapStringUint16      map[string]uint16
	FptrMapStringUint16   *map[string]uint16
	FMapStringUint32      map[string]uint32
	FptrMapStringUint32   *map[string]uint32
	FMapStringUint64      map[string]uint64
	FptrMapStringUint64   *map[string]uint64
	FMapStringUintptr     map[string]uintptr
	FptrMapStringUintptr  *map[string]uintptr
	FMapStringInt         map[string]int
	FptrMapStringInt      *map[string]int
	FMapStringInt8        map[string]int8
	FptrMapStringInt8     *map[string]int8
	FMapStringInt16       map[string]int16
	FptrMapStringInt16    *map[string]int16
	FMapStringInt32       map[string]int32
	FptrMapStringInt32    *map[string]int32
	FMapStringInt64       map[string]int64
	FptrMapStringInt64    *map[string]int64
	FMapStringFloat32     map[string]float32
	FptrMapStringFloat32  *map[string]float32
	FMapStringFloat64     map[string]float64
	FptrMapStringFloat64  *map[string]float64
	FMapStringBool        map[string]bool
	FptrMapStringBool     *map[string]bool
	FMapFloat32Intf       map[float32]interface{}
	FptrMapFloat32Intf    *map[float32]interface{}
	FMapFloat32String     map[float32]string
	FptrMapFloat32String  *map[float32]string
	FMapFloat32Uint       map[float32]uint
	FptrMapFloat32Uint    *map[float32]uint
	FMapFloat32Uint8      map[float32]uint8
	FptrMapFloat32Uint8   *map[float32]uint8
	FMapFloat32Uint16     map[float32]uint16
	FptrMapFloat32Uint16  *map[float32]uint16
	FMapFloat32Uint32     map[float32]uint32
	FptrMapFloat32Uint32  *map[float32]uint32
	FMapFloat32Uint64     map[float32]uint64
	FptrMapFloat32Uint64  *map[float32]uint64
	FMapFloat32Uintptr    map[float32]uintptr
	FptrMapFloat32Uintptr *map[float32]uintptr
	FMapFloat32Int        map[float32]int
	FptrMapFloat32Int     *map[float32]int
	FMapFloat32Int8       map[float32]int8
	FptrMapFloat32Int8    *map[float32]int8
	FMapFloat32Int16      map[float32]int16
	FptrMapFloat32Int16   *map[float32]int16
	FMapFloat32Int32      map[float32]int32
	FptrMapFloat32Int32   *map[float32]int32
	FMapFloat32Int64      map[float32]int64
	FptrMapFloat32Int64   *map[float32]int64
	FMapFloat32Float32    map[float32]float32
	FptrMapFloat32Float32 *map[float32]float32
	FMapFloat32Float64    map[float32]float64
	FptrMapFloat32Float64 *map[float32]float64
	FMapFloat32Bool       map[float32]bool
	FptrMapFloat32Bool    *map[float32]bool
	FMapFloat64Intf       map[float64]interface{}
	FptrMapFloat64Intf    *map[float64]interface{}
	FMapFloat64String     map[float64]string
	FptrMapFloat64String  *map[float64]string
	FMapFloat64Uint       map[float64]uint
	FptrMapFloat64Uint    *map[float64]uint
	FMapFloat64Uint8      map[float64]uint8
	FptrMapFloat64Uint8   *map[float64]uint8
	FMapFloat64Uint16     map[float64]uint16
	FptrMapFloat64Uint16  *map[float64]uint16
	FMapFloat64Uint32     map[float64]uint32
	FptrMapFloat64Uint32  *map[float64]uint32
	FMapFloat64Uint64     map[float64]uint64
	FptrMapFloat64Uint64  *map[float64]uint64
	FMapFloat64Uintptr    map[float64]uintptr
	FptrMapFloat64Uintptr *map[float64]uintptr
	FMapFloat64Int        map[float64]int
	FptrMapFloat64Int     *map[float64]int
	FMapFloat64Int8       map[float64]int8
	FptrMapFloat64Int8    *map[float64]int8
	FMapFloat64Int16      map[float64]int16
	FptrMapFloat64Int16   *map[float64]int16
	FMapFloat64Int32      map[float64]int32
	FptrMapFloat64Int32   *map[float64]int32
	FMapFloat64Int64      map[float64]int64
	FptrMapFloat64Int64   *map[float64]int64
	FMapFloat64Float32    map[float64]float32
	FptrMapFloat64Float32 *map[float64]float32
	FMapFloat64Float64    map[float64]float64
	FptrMapFloat64Float64 *map[float64]float64
	FMapFloat64Bool       map[float64]bool
	FptrMapFloat64Bool    *map[float64]bool
	FMapUintIntf          map[uint]interface{}
	FptrMapUintIntf       *map[uint]interface{}
	FMapUintString        map[uint]string
	FptrMapUintString     *map[uint]string
	FMapUintUint          map[uint]uint
	FptrMapUintUint       *map[uint]uint
	FMapUintUint8         map[uint]uint8
	FptrMapUintUint8      *map[uint]uint8
	FMapUintUint16        map[uint]uint16
	FptrMapUintUint16     *map[uint]uint16
	FMapUintUint32        map[uint]uint32
	FptrMapUintUint32     *map[uint]uint32
	FMapUintUint64        map[uint]uint64
	FptrMapUintUint64     *map[uint]uint64
	FMapUintUintptr       map[uint]uintptr
	FptrMapUintUintptr    *map[uint]uintptr
	FMapUintInt           map[uint]int
	FptrMapUintInt        *map[uint]int
	FMapUintInt8          map[uint]int8
	FptrMapUintInt8       *map[uint]int8
	FMapUintInt16         map[uint]int16
	FptrMapUintInt16      *map[uint]int16
	FMapUintInt32         map[uint]int32
	FptrMapUintInt32      *map[uint]int32
	FMapUintInt64         map[uint]int64
	FptrMapUintInt64      *map[uint]int64
	FMapUintFloat32       map[uint]float32
	FptrMapUintFloat32    *map[uint]float32
	FMapUintFloat64       map[uint]float64
	FptrMapUintFloat64    *map[uint]float64
	FMapUintBool          map[uint]bool
	FptrMapUintBool       *map[uint]bool
	FMapUint8Intf         map[uint8]interface{}
	FptrMapUint8Intf      *map[uint8]interface{}
	FMapUint8String       map[uint8]string
	FptrMapUint8String    *map[uint8]string
	FMapUint8Uint         map[uint8]uint
	FptrMapUint8Uint      *map[uint8]uint
	FMapUint8Uint8        map[uint8]uint8
	FptrMapUint8Uint8     *map[uint8]uint8
	FMapUint8Uint16       map[uint8]uint16
	FptrMapUint8Uint16    *map[uint8]uint16
	FMapUint8Uint32       map[uint8]uint32
	FptrMapUint8Uint32    *map[uint8]uint32
	FMapUint8Uint64       map[uint8]uint64
	FptrMapUint8Uint64    *map[uint8]uint64
	FMapUint8Uintptr      map[uint8]uintptr
	FptrMapUint8Uintptr   *map[uint8]uintptr
	FMapUint8Int          map[uint8]int
	FptrMapUint8Int       *map[uint8]int
	FMapUint8Int8         map[uint8]int8
	FptrMapUint8Int8      *map[uint8]int8
	FMapUint8Int16        map[uint8]int16
	FptrMapUint8Int16     *map[uint8]int16
	FMapUint8Int32        map[uint8]int32
	FptrMapUint8Int32     *map[uint8]int32
	FMapUint8Int64        map[uint8]int64
	FptrMapUint8Int64     *map[uint8]int64
	FMapUint8Float32      map[uint8]float32
	FptrMapUint8Float32   *map[uint8]float32
	FMapUint8Float64      map[uint8]float64
	FptrMapUint8Float64   *map[uint8]float64
	FMapUint8Bool         map[uint8]bool
	FptrMapUint8Bool      *map[uint8]bool
	FMapUint16Intf        map[uint16]interface{}
	FptrMapUint16Intf     *map[uint16]interface{}
	FMapUint16String      map[uint16]string
	FptrMapUint16String   *map[uint16]string
	FMapUint16Uint        map[uint16]uint
	FptrMapUint16Uint     *map[uint16]uint
	FMapUint16Uint8       map[uint16]uint8
	FptrMapUint16Uint8    *map[uint16]uint8
	FMapUint16Uint16      map[uint16]uint16
	FptrMapUint16Uint16   *map[uint16]uint16
	FMapUint16Uint32      map[uint16]uint32
	FptrMapUint16Uint32   *map[uint16]uint32
	FMapUint16Uint64      map[uint16]uint64
	FptrMapUint16Uint64   *map[uint16]uint64
	FMapUint16Uintptr     map[uint16]uintptr
	FptrMapUint16Uintptr  *map[uint16]uintptr
	FMapUint16Int         map[uint16]int
	FptrMapUint16Int      *map[uint16]int
	FMapUint16Int8        map[uint16]int8
	FptrMapUint16Int8     *map[uint16]int8
	FMapUint16Int16       map[uint16]int16
	FptrMapUint16Int16    *map[uint16]int16
	FMapUint16Int32       map[uint16]int32
	FptrMapUint16Int32    *map[uint16]int32
	FMapUint16Int64       map[uint16]int64
	FptrMapUint16Int64    *map[uint16]int64
	FMapUint16Float32     map[uint16]float32
	FptrMapUint16Float32  *map[uint16]float32
	FMapUint16Float64     map[uint16]float64
	FptrMapUint16Float64  *map[uint16]float64
	FMapUint16Bool        map[uint16]bool
	FptrMapUint16Bool     *map[uint16]bool
	FMapUint32Intf        map[uint32]interface{}
	FptrMapUint32Intf     *map[uint32]interface{}
	FMapUint32String      map[uint32]string
	FptrMapUint32String   *map[uint32]string
	FMapUint32Uint        map[uint32]uint
	FptrMapUint32Uint     *map[uint32]uint
	FMapUint32Uint8       map[uint32]uint8
	FptrMapUint32Uint8    *map[uint32]uint8
	FMapUint32Uint16      map[uint32]uint16
	FptrMapUint32Uint16   *map[uint32]uint16
	FMapUint32Uint32      map[uint32]uint32
	FptrMapUint32Uint32   *map[uint32]uint32
	FMapUint32Uint64      map[uint32]uint64
	FptrMapUint32Uint64   *map[uint32]uint64
	FMapUint32Uintptr     map[uint32]uintptr
	FptrMapUint32Uintptr  *map[uint32]uintptr
	FMapUint32Int         map[uint32]int
	FptrMapUint32Int      *map[uint32]int
	FMapUint32Int8        map[uint32]int8
	FptrMapUint32Int8     *map[uint32]int8
	FMapUint32Int16       map[uint32]int16
	FptrMapUint32Int16    *map[uint32]int16
	FMapUint32Int32       map[uint32]int32
	FptrMapUint32Int32    *map[uint32]int32
	FMapUint32Int64       map[uint32]int64
	FptrMapUint32Int64    *map[uint32]int64
	FMapUint32Float32     map[uint32]float32
	FptrMapUint32Float32  *map[uint32]float32
	FMapUint32Float64     map[uint32]float64
	FptrMapUint32Float64  *map[uint32]float64
	FMapUint32Bool        map[uint32]bool
	FptrMapUint32Bool     *map[uint32]bool
	FMapUint64Intf        map[uint64]interface{}
	FptrMapUint64Intf     *map[uint64]interface{}
	FMapUint64String      map[uint64]string
	FptrMapUint64String   *map[uint64]string
	FMapUint64Uint        map[uint64]uint
	FptrMapUint64Uint     *map[uint64]uint
	FMapUint64Uint8       map[uint64]uint8
	FptrMapUint64Uint8    *map[uint64]uint8
	FMapUint64Uint16      map[uint64]uint16
	FptrMapUint64Uint16   *map[uint64]uint16
	FMapUint64Uint32      map[uint64]uint32
	FptrMapUint64Uint32   *map[uint64]uint32
	FMapUint64Uint64      map[uint64]uint64
	FptrMapUint64Uint64   *map[uint64]uint64
	FMapUint64Uintptr     map[uint64]uintptr
	FptrMapUint64Uintptr  *map[uint64]uintptr
	FMapUint64Int         map[uint64]int
	FptrMapUint64Int      *map[uint64]int
	FMapUint64Int8        map[uint64]int8
	FptrMapUint64Int8     *map[uint64]int8
	FMapUint64Int16       map[uint64]int16
	FptrMapUint64Int16    *map[uint64]int16
	FMapUint64Int32       map[uint64]int32
	FptrMapUint64Int32    *map[uint64]int32
	FMapUint64Int64       map[uint64]int64
	FptrMapUint64Int64    *map[uint64]int64
	FMapUint64Float32     map[uint64]float32
	FptrMapUint64Float32  *map[uint64]float32
	FMapUint64Float64     map[uint64]float64
	FptrMapUint64Float64  *map[uint64]float64
	FMapUint64Bool        map[uint64]bool
	FptrMapUint64Bool     *map[uint64]bool
	FMapUintptrIntf       map[uintptr]interface{}
	FptrMapUintptrIntf    *map[uintptr]interface{}
	FMapUintptrString     map[uintptr]string
	FptrMapUintptrString  *map[uintptr]string
	FMapUintptrUint       map[uintptr]uint
	FptrMapUintptrUint    *map[uintptr]uint
	FMapUintptrUint8      map[uintptr]uint8
	FptrMapUintptrUint8   *map[uintptr]uint8
	FMapUintptrUint16     map[uintptr]uint16
	FptrMapUintptrUint16  *map[uintptr]uint16
	FMapUintptrUint32     map[uintptr]uint32
	FptrMapUintptrUint32  *map[uintptr]uint32
	FMapUintptrUint64     map[uintptr]uint64
	FptrMapUintptrUint64  *map[uintptr]uint64
	FMapUintptrUintptr    map[uintptr]uintptr
	FptrMapUintptrUintptr *map[uintptr]uintptr
	FMapUintptrInt        map[uintptr]int
	FptrMapUintptrInt     *map[uintptr]int
	FMapUintptrInt8       map[uintptr]int8
	FptrMapUintptrInt8    *map[uintptr]int8
	FMapUintptrInt16      map[uintptr]int16
	FptrMapUintptrInt16   *map[uintptr]int16
	FMapUintptrInt32      map[uintptr]int32
	FptrMapUintptrInt32   *map[uintptr]int32
	FMapUintptrInt64      map[uintptr]int64
	FptrMapUintptrInt64   *map[uintptr]int64
	FMapUintptrFloat32    map[uintptr]float32
	FptrMapUintptrFloat32 *map[uintptr]float32
	FMapUintptrFloat64    map[uintptr]float64
	FptrMapUintptrFloat64 *map[uintptr]float64
	FMapUintptrBool       map[uintptr]bool
	FptrMapUintptrBool    *map[uintptr]bool
	FMapIntIntf           map[int]interface{}
	FptrMapIntIntf        *map[int]interface{}
	FMapIntString         map[int]string
	FptrMapIntString      *map[int]string
	FMapIntUint           map[int]uint
	FptrMapIntUint        *map[int]uint
	FMapIntUint8          map[int]uint8
	FptrMapIntUint8       *map[int]uint8
	FMapIntUint16         map[int]uint16
	FptrMapIntUint16      *map[int]uint16
	FMapIntUint32         map[int]uint32
	FptrMapIntUint32      *map[int]uint32
	FMapIntUint64         map[int]uint64
	FptrMapIntUint64      *map[int]uint64
	FMapIntUintptr        map[int]uintptr
	FptrMapIntUintptr     *map[int]uintptr
	FMapIntInt            map[int]int
	FptrMapIntInt         *map[int]int
	FMapIntInt8           map[int]int8
	FptrMapIntInt8        *map[int]int8
	FMapIntInt16          map[int]int16
	FptrMapIntInt16       *map[int]int16
	FMapIntInt32          map[int]int32
	FptrMapIntInt32       *map[int]int32
	FMapIntInt64          map[int]int64
	FptrMapIntInt64       *map[int]int64
	FMapIntFloat32        map[int]float32
	FptrMapIntFloat32     *map[int]float32
	FMapIntFloat64        map[int]float64
	FptrMapIntFloat64     *map[int]float64
	FMapIntBool           map[int]bool
	FptrMapIntBool        *map[int]bool
	FMapInt8Intf          map[int8]interface{}
	FptrMapInt8Intf       *map[int8]interface{}
	FMapInt8String        map[int8]string
	FptrMapInt8String     *map[int8]string
	FMapInt8Uint          map[int8]uint
	FptrMapInt8Uint       *map[int8]uint
	FMapInt8Uint8         map[int8]uint8
	FptrMapInt8Uint8      *map[int8]uint8
	FMapInt8Uint16        map[int8]uint16
	FptrMapInt8Uint16     *map[int8]uint16
	FMapInt8Uint32        map[int8]uint32
	FptrMapInt8Uint32     *map[int8]uint32
	FMapInt8Uint64        map[int8]uint64
	FptrMapInt8Uint64     *map[int8]uint64
	FMapInt8Uintptr       map[int8]uintptr
	FptrMapInt8Uintptr    *map[int8]uintptr
	FMapInt8Int           map[int8]int
	FptrMapInt8Int        *map[int8]int
	FMapInt8Int8          map[int8]int8
	FptrMapInt8Int8       *map[int8]int8
	FMapInt8Int16         map[int8]int16
	FptrMapInt8Int16      *map[int8]int16
	FMapInt8Int32         map[int8]int32
	FptrMapInt8Int32      *map[int8]int32
	FMapInt8Int64         map[int8]int64
	FptrMapInt8Int64      *map[int8]int64
	FMapInt8Float32       map[int8]float32
	FptrMapInt8Float32    *map[int8]float32
	FMapInt8Float64       map[int8]float64
	FptrMapInt8Float64    *map[int8]float64
	FMapInt8Bool          map[int8]bool
	FptrMapInt8Bool       *map[int8]bool
	FMapInt16Intf         map[int16]interface{}
	FptrMapInt16Intf      *map[int16]interface{}
	FMapInt16String       map[int16]string
	FptrMapInt16String    *map[int16]string
	FMapInt16Uint         map[int16]uint
	FptrMapInt16Uint      *map[int16]uint
	FMapInt16Uint8        map[int16]uint8
	FptrMapInt16Uint8     *map[int16]uint8
	FMapInt16Uint16       map[int16]uint16
	FptrMapInt16Uint16    *map[int16]uint16
	FMapInt16Uint32       map[int16]uint32
	FptrMapInt16Uint32    *map[int16]uint32
	FMapInt16Uint64       map[int16]uint64
	FptrMapInt16Uint64    *map[int16]uint64
	FMapInt16Uintptr      map[int16]uintptr
	FptrMapInt16Uintptr   *map[int16]uintptr
	FMapInt16Int          map[int16]int
	FptrMapInt16Int       *map[int16]int
	FMapInt16Int8         map[int16]int8
	FptrMapInt16Int8      *map[int16]int8
	FMapInt16Int16        map[int16]int16
	FptrMapInt16Int16     *map[int16]int16
	FMapInt16Int32        map[int16]int32
	FptrMapInt16Int32     *map[int16]int32
	FMapInt16Int64        map[int16]int64
	FptrMapInt16Int64     *map[int16]int64
	FMapInt16Float32      map[int16]float32
	FptrMapInt16Float32   *map[int16]float32
	FMapInt16Float64      map[int16]float64
	FptrMapInt16Float64   *map[int16]float64
	FMapInt16Bool         map[int16]bool
	FptrMapInt16Bool      *map[int16]bool
	FMapInt32Intf         map[int32]interface{}
	FptrMapInt32Intf      *map[int32]interface{}
	FMapInt32String       map[int32]string
	FptrMapInt32String    *map[int32]string
	FMapInt32Uint         map[int32]uint
	FptrMapInt32Uint      *map[int32]uint
	FMapInt32Uint8        map[int32]uint8
	FptrMapInt32Uint8     *map[int32]uint8
	FMapInt32Uint16       map[int32]uint16
	FptrMapInt32Uint16    *map[int32]uint16
	FMapInt32Uint32       map[int32]uint32
	FptrMapInt32Uint32    *map[int32]uint32
	FMapInt32Uint64       map[int32]uint64
	FptrMapInt32Uint64    *map[int32]uint64
	FMapInt32Uintptr      map[int32]uintptr
	FptrMapInt32Uintptr   *map[int32]uintptr
	FMapInt32Int          map[int32]int
	FptrMapInt32Int       *map[int32]int
	FMapInt32Int8         map[int32]int8
	FptrMapInt32Int8      *map[int32]int8
	FMapInt32Int16        map[int32]int16
	FptrMapInt32Int16     *map[int32]int16
	FMapInt32Int32        map[int32]int32
	FptrMapInt32Int32     *map[int32]int32
	FMapInt32Int64        map[int32]int64
	FptrMapInt32Int64     *map[int32]int64
	FMapInt32Float32      map[int32]float32
	FptrMapInt32Float32   *map[int32]float32
	FMapInt32Float64      map[int32]float64
	FptrMapInt32Float64   *map[int32]float64
	FMapInt32Bool         map[int32]bool
	FptrMapInt32Bool      *map[int32]bool
	FMapInt64Intf         map[int64]interface{}
	FptrMapInt64Intf      *map[int64]interface{}
	FMapInt64String       map[int64]string
	FptrMapInt64String    *map[int64]string
	FMapInt64Uint         map[int64]uint
	FptrMapInt64Uint      *map[int64]uint
	FMapInt64Uint8        map[int64]uint8
	FptrMapInt64Uint8     *map[int64]uint8
	FMapInt64Uint16       map[int64]uint16
	FptrMapInt64Uint16    *map[int64]uint16
	FMapInt64Uint32       map[int64]uint32
	FptrMapInt64Uint32    *map[int64]uint32
	FMapInt64Uint64       map[int64]uint64
	FptrMapInt64Uint64    *map[int64]uint64
	FMapInt64Uintptr      map[int64]uintptr
	FptrMapInt64Uintptr   *map[int64]uintptr
	FMapInt64Int          map[int64]int
	FptrMapInt64Int       *map[int64]int
	FMapInt64Int8         map[int64]int8
	FptrMapInt64Int8      *map[int64]int8
	FMapInt64Int16        map[int64]int16
	FptrMapInt64Int16     *map[int64]int16
	FMapInt64Int32        map[int64]int32
	FptrMapInt64Int32     *map[int64]int32
	FMapInt64Int64        map[int64]int64
	FptrMapInt64Int64     *map[int64]int64
	FMapInt64Float32      map[int64]float32
	FptrMapInt64Float32   *map[int64]float32
	FMapInt64Float64      map[int64]float64
	FptrMapInt64Float64   *map[int64]float64
	FMapInt64Bool         map[int64]bool
	FptrMapInt64Bool      *map[int64]bool
	FMapBoolIntf          map[bool]interface{}
	FptrMapBoolIntf       *map[bool]interface{}
	FMapBoolString        map[bool]string
	FptrMapBoolString     *map[bool]string
	FMapBoolUint          map[bool]uint
	FptrMapBoolUint       *map[bool]uint
	FMapBoolUint8         map[bool]uint8
	FptrMapBoolUint8      *map[bool]uint8
	FMapBoolUint16        map[bool]uint16
	FptrMapBoolUint16     *map[bool]uint16
	FMapBoolUint32        map[bool]uint32
	FptrMapBoolUint32     *map[bool]uint32
	FMapBoolUint64        map[bool]uint64
	FptrMapBoolUint64     *map[bool]uint64
	FMapBoolUintptr       map[bool]uintptr
	FptrMapBoolUintptr    *map[bool]uintptr
	FMapBoolInt           map[bool]int
	FptrMapBoolInt        *map[bool]int
	FMapBoolInt8          map[bool]int8
	FptrMapBoolInt8       *map[bool]int8
	FMapBoolInt16         map[bool]int16
	FptrMapBoolInt16      *map[bool]int16
	FMapBoolInt32         map[bool]int32
	FptrMapBoolInt32      *map[bool]int32
	FMapBoolInt64         map[bool]int64
	FptrMapBoolInt64      *map[bool]int64
	FMapBoolFloat32       map[bool]float32
	FptrMapBoolFloat32    *map[bool]float32
	FMapBoolFloat64       map[bool]float64
	FptrMapBoolFloat64    *map[bool]float64
	FMapBoolBool          map[bool]bool
	FptrMapBoolBool       *map[bool]bool
}

type typMbsSliceIntf []interface{}

func (_ typMbsSliceIntf) MapBySlice() {}

type typMbsSliceString []string

func (_ typMbsSliceString) MapBySlice() {}

type typMbsSliceFloat32 []float32

func (_ typMbsSliceFloat32) MapBySlice() {}

type typMbsSliceFloat64 []float64

func (_ typMbsSliceFloat64) MapBySlice() {}

type typMbsSliceUint []uint

func (_ typMbsSliceUint) MapBySlice() {}

type typMbsSliceUint8 []uint8

func (_ typMbsSliceUint8) MapBySlice() {}

type typMbsSliceUint16 []uint16

func (_ typMbsSliceUint16) MapBySlice() {}

type typMbsSliceUint32 []uint32

func (_ typMbsSliceUint32) MapBySlice() {}

type typMbsSliceUint64 []uint64

func (_ typMbsSliceUint64) MapBySlice() {}

type typMbsSliceUintptr []uintptr

func (_ typMbsSliceUintptr) MapBySlice() {}

type typMbsSliceInt []int

func (_ typMbsSliceInt) MapBySlice() {}

type typMbsSliceInt8 []int8

func (_ typMbsSliceInt8) MapBySlice() {}

type typMbsSliceInt16 []int16

func (_ typMbsSliceInt16) MapBySlice() {}

type typMbsSliceInt32 []int32

func (_ typMbsSliceInt32) MapBySlice() {}

type typMbsSliceInt64 []int64

func (_ typMbsSliceInt64) MapBySlice() {}

type typMbsSliceBool []bool

func (_ typMbsSliceBool) MapBySlice() {}

type typMapMapIntfIntf map[interface{}]interface{}
type typMapMapIntfString map[interface{}]string
type typMapMapIntfUint map[interface{}]uint
type typMapMapIntfUint8 map[interface{}]uint8
type typMapMapIntfUint16 map[interface{}]uint16
type typMapMapIntfUint32 map[interface{}]uint32
type typMapMapIntfUint64 map[interface{}]uint64
type typMapMapIntfUintptr map[interface{}]uintptr
type typMapMapIntfInt map[interface{}]int
type typMapMapIntfInt8 map[interface{}]int8
type typMapMapIntfInt16 map[interface{}]int16
type typMapMapIntfInt32 map[interface{}]int32
type typMapMapIntfInt64 map[interface{}]int64
type typMapMapIntfFloat32 map[interface{}]float32
type typMapMapIntfFloat64 map[interface{}]float64
type typMapMapIntfBool map[interface{}]bool
type typMapMapStringIntf map[string]interface{}
type typMapMapStringString map[string]string
type typMapMapStringUint map[string]uint
type typMapMapStringUint8 map[string]uint8
type typMapMapStringUint16 map[string]uint16
type typMapMapStringUint32 map[string]uint32
type typMapMapStringUint64 map[string]uint64
type typMapMapStringUintptr map[string]uintptr
type typMapMapStringInt map[string]int
type typMapMapStringInt8 map[string]int8
type typMapMapStringInt16 map[string]int16
type typMapMapStringInt32 map[string]int32
type typMapMapStringInt64 map[string]int64
type typMapMapStringFloat32 map[string]float32
type typMapMapStringFloat64 map[string]float64
type typMapMapStringBool map[string]bool
type typMapMapFloat32Intf map[float32]interface{}
type typMapMapFloat32String map[float32]string
type typMapMapFloat32Uint map[float32]uint
type typMapMapFloat32Uint8 map[float32]uint8
type typMapMapFloat32Uint16 map[float32]uint16
type typMapMapFloat32Uint32 map[float32]uint32
type typMapMapFloat32Uint64 map[float32]uint64
type typMapMapFloat32Uintptr map[float32]uintptr
type typMapMapFloat32Int map[float32]int
type typMapMapFloat32Int8 map[float32]int8
type typMapMapFloat32Int16 map[float32]int16
type typMapMapFloat32Int32 map[float32]int32
type typMapMapFloat32Int64 map[float32]int64
type typMapMapFloat32Float32 map[float32]float32
type typMapMapFloat32Float64 map[float32]float64
type typMapMapFloat32Bool map[float32]bool
type typMapMapFloat64Intf map[float64]interface{}
type typMapMapFloat64String map[float64]string
type typMapMapFloat64Uint map[float64]uint
type typMapMapFloat64Uint8 map[float64]uint8
type typMapMapFloat64Uint16 map[float64]uint16
type typMapMapFloat64Uint32 map[float64]uint32
type typMapMapFloat64Uint64 map[float64]uint64
type typMapMapFloat64Uintptr map[float64]uintptr
type typMapMapFloat64Int map[float64]int
type typMapMapFloat64Int8 map[float64]int8
type typMapMapFloat64Int16 map[float64]int16
type typMapMapFloat64Int32 map[float64]int32
type typMapMapFloat64Int64 map[float64]int64
type typMapMapFloat64Float32 map[float64]float32
type typMapMapFloat64Float64 map[float64]float64
type typMapMapFloat64Bool map[float64]bool
type typMapMapUintIntf map[uint]interface{}
type typMapMapUintString map[uint]string
type typMapMapUintUint map[uint]uint
type typMapMapUintUint8 map[uint]uint8
type typMapMapUintUint16 map[uint]uint16
type typMapMapUintUint32 map[uint]uint32
type typMapMapUintUint64 map[uint]uint64
type typMapMapUintUintptr map[uint]uintptr
type typMapMapUintInt map[uint]int
type typMapMapUintInt8 map[uint]int8
type typMapMapUintInt16 map[uint]int16
type typMapMapUintInt32 map[uint]int32
type typMapMapUintInt64 map[uint]int64
type typMapMapUintFloat32 map[uint]float32
type typMapMapUintFloat64 map[uint]float64
type typMapMapUintBool map[uint]bool
type typMapMapUint8Intf map[uint8]interface{}
type typMapMapUint8String map[uint8]string
type typMapMapUint8Uint map[uint8]uint
type typMapMapUint8Uint8 map[uint8]uint8
type typMapMapUint8Uint16 map[uint8]uint16
type typMapMapUint8Uint32 map[uint8]uint32
type typMapMapUint8Uint64 map[uint8]uint64
type typMapMapUint8Uintptr map[uint8]uintptr
type typMapMapUint8Int map[uint8]int
type typMapMapUint8Int8 map[uint8]int8
type typMapMapUint8Int16 map[uint8]int16
type typMapMapUint8Int32 map[uint8]int32
type typMapMapUint8Int64 map[uint8]int64
type typMapMapUint8Float32 map[uint8]float32
type typMapMapUint8Float64 map[uint8]float64
type typMapMapUint8Bool map[uint8]bool
type typMapMapUint16Intf map[uint16]interface{}
type typMapMapUint16String map[uint16]string
type typMapMapUint16Uint map[uint16]uint
type typMapMapUint16Uint8 map[uint16]uint8
type typMapMapUint16Uint16 map[uint16]uint16
type typMapMapUint16Uint32 map[uint16]uint32
type typMapMapUint16Uint64 map[uint16]uint64
type typMapMapUint16Uintptr map[uint16]uintptr
type typMapMapUint16Int map[uint16]int
type typMapMapUint16Int8 map[uint16]int8
type typMapMapUint16Int16 map[uint16]int16
type typMapMapUint16Int32 map[uint16]int32
type typMapMapUint16Int64 map[uint16]int64
type typMapMapUint16Float32 map[uint16]float32
type typMapMapUint16Float64 map[uint16]float64
type typMapMapUint16Bool map[uint16]bool
type typMapMapUint32Intf map[uint32]interface{}
type typMapMapUint32String map[uint32]string
type typMapMapUint32Uint map[uint32]uint
type typMapMapUint32Uint8 map[uint32]uint8
type typMapMapUint32Uint16 map[uint32]uint16
type typMapMapUint32Uint32 map[uint32]uint32
type typMapMapUint32Uint64 map[uint32]uint64
type typMapMapUint32Uintptr map[uint32]uintptr
type typMapMapUint32Int map[uint32]int
type typMapMapUint32Int8 map[uint32]int8
type typMapMapUint32Int16 map[uint32]int16
type typMapMapUint32Int32 map[uint32]int32
type typMapMapUint32Int64 map[uint32]int64
type typMapMapUint32Float32 map[uint32]float32
type typMapMapUint32Float64 map[uint32]float64
type typMapMapUint32Bool map[uint32]bool
type typMapMapUint64Intf map[uint64]interface{}
type typMapMapUint64String map[uint64]string
type typMapMapUint64Uint map[uint64]uint
type typMapMapUint64Uint8 map[uint64]uint8
type typMapMapUint64Uint16 map[uint64]uint16
type typMapMapUint64Uint32 map[uint64]uint32
type typMapMapUint64Uint64 map[uint64]uint64
type typMapMapUint64Uintptr map[uint64]uintptr
type typMapMapUint64Int map[uint64]int
type typMapMapUint64Int8 map[uint64]int8
type typMapMapUint64Int16 map[uint64]int16
type typMapMapUint64Int32 map[uint64]int32
type typMapMapUint64Int64 map[uint64]int64
type typMapMapUint64Float32 map[uint64]float32
type typMapMapUint64Float64 map[uint64]float64
type typMapMapUint64Bool map[uint64]bool
type typMapMapUintptrIntf map[uintptr]interface{}
type typMapMapUintptrString map[uintptr]string
type typMapMapUintptrUint map[uintptr]uint
type typMapMapUintptrUint8 map[uintptr]uint8
type typMapMapUintptrUint16 map[uintptr]uint16
type typMapMapUintptrUint32 map[uintptr]uint32
type typMapMapUintptrUint64 map[uintptr]uint64
type typMapMapUintptrUintptr map[uintptr]uintptr
type typMapMapUintptrInt map[uintptr]int
type typMapMapUintptrInt8 map[uintptr]int8
type typMapMapUintptrInt16 map[uintptr]int16
type typMapMapUintptrInt32 map[uintptr]int32
type typMapMapUintptrInt64 map[uintptr]int64
type typMapMapUintptrFloat32 map[uintptr]float32
type typMapMapUintptrFloat64 map[uintptr]float64
type typMapMapUintptrBool map[uintptr]bool
type typMapMapIntIntf map[int]interface{}
type typMapMapIntString map[int]string
type typMapMapIntUint map[int]uint
type typMapMapIntUint8 map[int]uint8
type typMapMapIntUint16 map[int]uint16
type typMapMapIntUint32 map[int]uint32
type typMapMapIntUint64 map[int]uint64
type typMapMapIntUintptr map[int]uintptr
type typMapMapIntInt map[int]int
type typMapMapIntInt8 map[int]int8
type typMapMapIntInt16 map[int]int16
type typMapMapIntInt32 map[int]int32
type typMapMapIntInt64 map[int]int64
type typMapMapIntFloat32 map[int]float32
type typMapMapIntFloat64 map[int]float64
type typMapMapIntBool map[int]bool
type typMapMapInt8Intf map[int8]interface{}
type typMapMapInt8String map[int8]string
type typMapMapInt8Uint map[int8]uint
type typMapMapInt8Uint8 map[int8]uint8
type typMapMapInt8Uint16 map[int8]uint16
type typMapMapInt8Uint32 map[int8]uint32
type typMapMapInt8Uint64 map[int8]uint64
type typMapMapInt8Uintptr map[int8]uintptr
type typMapMapInt8Int map[int8]int
type typMapMapInt8Int8 map[int8]int8
type typMapMapInt8Int16 map[int8]int16
type typMapMapInt8Int32 map[int8]int32
type typMapMapInt8Int64 map[int8]int64
type typMapMapInt8Float32 map[int8]float32
type typMapMapInt8Float64 map[int8]float64
type typMapMapInt8Bool map[int8]bool
type typMapMapInt16Intf map[int16]interface{}
type typMapMapInt16String map[int16]string
type typMapMapInt16Uint map[int16]uint
type typMapMapInt16Uint8 map[int16]uint8
type typMapMapInt16Uint16 map[int16]uint16
type typMapMapInt16Uint32 map[int16]uint32
type typMapMapInt16Uint64 map[int16]uint64
type typMapMapInt16Uintptr map[int16]uintptr
type typMapMapInt16Int map[int16]int
type typMapMapInt16Int8 map[int16]int8
type typMapMapInt16Int16 map[int16]int16
type typMapMapInt16Int32 map[int16]int32
type typMapMapInt16Int64 map[int16]int64
type typMapMapInt16Float32 map[int16]float32
type typMapMapInt16Float64 map[int16]float64
type typMapMapInt16Bool map[int16]bool
type typMapMapInt32Intf map[int32]interface{}
type typMapMapInt32String map[int32]string
type typMapMapInt32Uint map[int32]uint
type typMapMapInt32Uint8 map[int32]uint8
type typMapMapInt32Uint16 map[int32]uint16
type typMapMapInt32Uint32 map[int32]uint32
type typMapMapInt32Uint64 map[int32]uint64
type typMapMapInt32Uintptr map[int32]uintptr
type typMapMapInt32Int map[int32]int
type typMapMapInt32Int8 map[int32]int8
type typMapMapInt32Int16 map[int32]int16
type typMapMapInt32Int32 map[int32]int32
type typMapMapInt32Int64 map[int32]int64
type typMapMapInt32Float32 map[int32]float32
type typMapMapInt32Float64 map[int32]float64
type typMapMapInt32Bool map[int32]bool
type typMapMapInt64Intf map[int64]interface{}
type typMapMapInt64String map[int64]string
type typMapMapInt64Uint map[int64]uint
type typMapMapInt64Uint8 map[int64]uint8
type typMapMapInt64Uint16 map[int64]uint16
type typMapMapInt64Uint32 map[int64]uint32
type typMapMapInt64Uint64 map[int64]uint64
type typMapMapInt64Uintptr map[int64]uintptr
type typMapMapInt64Int map[int64]int
type typMapMapInt64Int8 map[int64]int8
type typMapMapInt64Int16 map[int64]int16
type typMapMapInt64Int32 map[int64]int32
type typMapMapInt64Int64 map[int64]int64
type typMapMapInt64Float32 map[int64]float32
type typMapMapInt64Float64 map[int64]float64
type typMapMapInt64Bool map[int64]bool
type typMapMapBoolIntf map[bool]interface{}
type typMapMapBoolString map[bool]string
type typMapMapBoolUint map[bool]uint
type typMapMapBoolUint8 map[bool]uint8
type typMapMapBoolUint16 map[bool]uint16
type typMapMapBoolUint32 map[bool]uint32
type typMapMapBoolUint64 map[bool]uint64
type typMapMapBoolUintptr map[bool]uintptr
type typMapMapBoolInt map[bool]int
type typMapMapBoolInt8 map[bool]int8
type typMapMapBoolInt16 map[bool]int16
type typMapMapBoolInt32 map[bool]int32
type typMapMapBoolInt64 map[bool]int64
type typMapMapBoolFloat32 map[bool]float32
type typMapMapBoolFloat64 map[bool]float64
type typMapMapBoolBool map[bool]bool

func doTestMammothSlices(t *testing.T, h Handle) {

	var v1va [8]interface{}
	for _, v := range [][]interface{}{nil, {}, {"string-is-an-interface-2", nil, nil, "string-is-an-interface"}} {
		var v1v1, v1v2 []interface{}
		v1v1 = v
		bs1 := testMarshalErr(v1v1, h, t, "enc-slice-v1")
		if v == nil {
			v1v2 = nil
		} else {
			v1v2 = make([]interface{}, len(v))
		}
		testUnmarshalErr(v1v2, bs1, h, t, "dec-slice-v1")
		testDeepEqualErr(v1v1, v1v2, t, "equal-slice-v1")
		if v == nil {
			v1v2 = nil
		} else {
			v1v2 = make([]interface{}, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v1v2), bs1, h, t, "dec-slice-v1-noaddr") // non-addressable value
		testDeepEqualErr(v1v1, v1v2, t, "equal-slice-v1-noaddr")
		// ...
		bs1 = testMarshalErr(&v1v1, h, t, "enc-slice-v1-p")
		v1v2 = nil
		testUnmarshalErr(&v1v2, bs1, h, t, "dec-slice-v1-p")
		testDeepEqualErr(v1v1, v1v2, t, "equal-slice-v1-p")
		v1va = [8]interface{}{} // clear the array
		v1v2 = v1va[:1:1]
		testUnmarshalErr(&v1v2, bs1, h, t, "dec-slice-v1-p-1")
		testDeepEqualErr(v1v1, v1v2, t, "equal-slice-v1-p-1")
		v1va = [8]interface{}{} // clear the array
		v1v2 = v1va[:len(v1v1):len(v1v1)]
		testUnmarshalErr(&v1v2, bs1, h, t, "dec-slice-v1-p-len")
		testDeepEqualErr(v1v1, v1v2, t, "equal-slice-v1-p-len")
		v1va = [8]interface{}{} // clear the array
		v1v2 = v1va[:]
		testUnmarshalErr(&v1v2, bs1, h, t, "dec-slice-v1-p-cap")
		testDeepEqualErr(v1v1, v1v2, t, "equal-slice-v1-p-cap")
		if len(v1v1) > 1 {
			v1va = [8]interface{}{} // clear the array
			testUnmarshalErr((&v1va)[:len(v1v1)], bs1, h, t, "dec-slice-v1-p-len-noaddr")
			testDeepEqualErr(v1v1, v1va[:len(v1v1)], t, "equal-slice-v1-p-len-noaddr")
			v1va = [8]interface{}{} // clear the array
			testUnmarshalErr((&v1va)[:], bs1, h, t, "dec-slice-v1-p-cap-noaddr")
			testDeepEqualErr(v1v1, v1va[:len(v1v1)], t, "equal-slice-v1-p-cap-noaddr")
		}
		// ...
		var v1v3, v1v4 typMbsSliceIntf
		v1v2 = nil
		if v != nil {
			v1v2 = make([]interface{}, len(v))
		}
		v1v3 = typMbsSliceIntf(v1v1)
		v1v4 = typMbsSliceIntf(v1v2)
		bs1 = testMarshalErr(v1v3, h, t, "enc-slice-v1-custom")
		testUnmarshalErr(v1v4, bs1, h, t, "dec-slice-v1-custom")
		testDeepEqualErr(v1v3, v1v4, t, "equal-slice-v1-custom")
		bs1 = testMarshalErr(&v1v3, h, t, "enc-slice-v1-custom-p")
		v1v2 = nil
		v1v4 = typMbsSliceIntf(v1v2)
		testUnmarshalErr(&v1v4, bs1, h, t, "dec-slice-v1-custom-p")
		testDeepEqualErr(v1v3, v1v4, t, "equal-slice-v1-custom-p")
	}

	var v19va [8]string
	for _, v := range [][]string{nil, {}, {"some-string-2", "", "", "some-string"}} {
		var v19v1, v19v2 []string
		v19v1 = v
		bs19 := testMarshalErr(v19v1, h, t, "enc-slice-v19")
		if v == nil {
			v19v2 = nil
		} else {
			v19v2 = make([]string, len(v))
		}
		testUnmarshalErr(v19v2, bs19, h, t, "dec-slice-v19")
		testDeepEqualErr(v19v1, v19v2, t, "equal-slice-v19")
		if v == nil {
			v19v2 = nil
		} else {
			v19v2 = make([]string, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v19v2), bs19, h, t, "dec-slice-v19-noaddr") // non-addressable value
		testDeepEqualErr(v19v1, v19v2, t, "equal-slice-v19-noaddr")
		// ...
		bs19 = testMarshalErr(&v19v1, h, t, "enc-slice-v19-p")
		v19v2 = nil
		testUnmarshalErr(&v19v2, bs19, h, t, "dec-slice-v19-p")
		testDeepEqualErr(v19v1, v19v2, t, "equal-slice-v19-p")
		v19va = [8]string{} // clear the array
		v19v2 = v19va[:1:1]
		testUnmarshalErr(&v19v2, bs19, h, t, "dec-slice-v19-p-1")
		testDeepEqualErr(v19v1, v19v2, t, "equal-slice-v19-p-1")
		v19va = [8]string{} // clear the array
		v19v2 = v19va[:len(v19v1):len(v19v1)]
		testUnmarshalErr(&v19v2, bs19, h, t, "dec-slice-v19-p-len")
		testDeepEqualErr(v19v1, v19v2, t, "equal-slice-v19-p-len")
		v19va = [8]string{} // clear the array
		v19v2 = v19va[:]
		testUnmarshalErr(&v19v2, bs19, h, t, "dec-slice-v19-p-cap")
		testDeepEqualErr(v19v1, v19v2, t, "equal-slice-v19-p-cap")
		if len(v19v1) > 1 {
			v19va = [8]string{} // clear the array
			testUnmarshalErr((&v19va)[:len(v19v1)], bs19, h, t, "dec-slice-v19-p-len-noaddr")
			testDeepEqualErr(v19v1, v19va[:len(v19v1)], t, "equal-slice-v19-p-len-noaddr")
			v19va = [8]string{} // clear the array
			testUnmarshalErr((&v19va)[:], bs19, h, t, "dec-slice-v19-p-cap-noaddr")
			testDeepEqualErr(v19v1, v19va[:len(v19v1)], t, "equal-slice-v19-p-cap-noaddr")
		}
		// ...
		var v19v3, v19v4 typMbsSliceString
		v19v2 = nil
		if v != nil {
			v19v2 = make([]string, len(v))
		}
		v19v3 = typMbsSliceString(v19v1)
		v19v4 = typMbsSliceString(v19v2)
		bs19 = testMarshalErr(v19v3, h, t, "enc-slice-v19-custom")
		testUnmarshalErr(v19v4, bs19, h, t, "dec-slice-v19-custom")
		testDeepEqualErr(v19v3, v19v4, t, "equal-slice-v19-custom")
		bs19 = testMarshalErr(&v19v3, h, t, "enc-slice-v19-custom-p")
		v19v2 = nil
		v19v4 = typMbsSliceString(v19v2)
		testUnmarshalErr(&v19v4, bs19, h, t, "dec-slice-v19-custom-p")
		testDeepEqualErr(v19v3, v19v4, t, "equal-slice-v19-custom-p")
	}

	var v37va [8]float32
	for _, v := range [][]float32{nil, {}, {22.2, 0, 0, 11.1}} {
		var v37v1, v37v2 []float32
		v37v1 = v
		bs37 := testMarshalErr(v37v1, h, t, "enc-slice-v37")
		if v == nil {
			v37v2 = nil
		} else {
			v37v2 = make([]float32, len(v))
		}
		testUnmarshalErr(v37v2, bs37, h, t, "dec-slice-v37")
		testDeepEqualErr(v37v1, v37v2, t, "equal-slice-v37")
		if v == nil {
			v37v2 = nil
		} else {
			v37v2 = make([]float32, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v37v2), bs37, h, t, "dec-slice-v37-noaddr") // non-addressable value
		testDeepEqualErr(v37v1, v37v2, t, "equal-slice-v37-noaddr")
		// ...
		bs37 = testMarshalErr(&v37v1, h, t, "enc-slice-v37-p")
		v37v2 = nil
		testUnmarshalErr(&v37v2, bs37, h, t, "dec-slice-v37-p")
		testDeepEqualErr(v37v1, v37v2, t, "equal-slice-v37-p")
		v37va = [8]float32{} // clear the array
		v37v2 = v37va[:1:1]
		testUnmarshalErr(&v37v2, bs37, h, t, "dec-slice-v37-p-1")
		testDeepEqualErr(v37v1, v37v2, t, "equal-slice-v37-p-1")
		v37va = [8]float32{} // clear the array
		v37v2 = v37va[:len(v37v1):len(v37v1)]
		testUnmarshalErr(&v37v2, bs37, h, t, "dec-slice-v37-p-len")
		testDeepEqualErr(v37v1, v37v2, t, "equal-slice-v37-p-len")
		v37va = [8]float32{} // clear the array
		v37v2 = v37va[:]
		testUnmarshalErr(&v37v2, bs37, h, t, "dec-slice-v37-p-cap")
		testDeepEqualErr(v37v1, v37v2, t, "equal-slice-v37-p-cap")
		if len(v37v1) > 1 {
			v37va = [8]float32{} // clear the array
			testUnmarshalErr((&v37va)[:len(v37v1)], bs37, h, t, "dec-slice-v37-p-len-noaddr")
			testDeepEqualErr(v37v1, v37va[:len(v37v1)], t, "equal-slice-v37-p-len-noaddr")
			v37va = [8]float32{} // clear the array
			testUnmarshalErr((&v37va)[:], bs37, h, t, "dec-slice-v37-p-cap-noaddr")
			testDeepEqualErr(v37v1, v37va[:len(v37v1)], t, "equal-slice-v37-p-cap-noaddr")
		}
		// ...
		var v37v3, v37v4 typMbsSliceFloat32
		v37v2 = nil
		if v != nil {
			v37v2 = make([]float32, len(v))
		}
		v37v3 = typMbsSliceFloat32(v37v1)
		v37v4 = typMbsSliceFloat32(v37v2)
		bs37 = testMarshalErr(v37v3, h, t, "enc-slice-v37-custom")
		testUnmarshalErr(v37v4, bs37, h, t, "dec-slice-v37-custom")
		testDeepEqualErr(v37v3, v37v4, t, "equal-slice-v37-custom")
		bs37 = testMarshalErr(&v37v3, h, t, "enc-slice-v37-custom-p")
		v37v2 = nil
		v37v4 = typMbsSliceFloat32(v37v2)
		testUnmarshalErr(&v37v4, bs37, h, t, "dec-slice-v37-custom-p")
		testDeepEqualErr(v37v3, v37v4, t, "equal-slice-v37-custom-p")
	}

	var v55va [8]float64
	for _, v := range [][]float64{nil, {}, {22.2, 0, 0, 11.1}} {
		var v55v1, v55v2 []float64
		v55v1 = v
		bs55 := testMarshalErr(v55v1, h, t, "enc-slice-v55")
		if v == nil {
			v55v2 = nil
		} else {
			v55v2 = make([]float64, len(v))
		}
		testUnmarshalErr(v55v2, bs55, h, t, "dec-slice-v55")
		testDeepEqualErr(v55v1, v55v2, t, "equal-slice-v55")
		if v == nil {
			v55v2 = nil
		} else {
			v55v2 = make([]float64, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v55v2), bs55, h, t, "dec-slice-v55-noaddr") // non-addressable value
		testDeepEqualErr(v55v1, v55v2, t, "equal-slice-v55-noaddr")
		// ...
		bs55 = testMarshalErr(&v55v1, h, t, "enc-slice-v55-p")
		v55v2 = nil
		testUnmarshalErr(&v55v2, bs55, h, t, "dec-slice-v55-p")
		testDeepEqualErr(v55v1, v55v2, t, "equal-slice-v55-p")
		v55va = [8]float64{} // clear the array
		v55v2 = v55va[:1:1]
		testUnmarshalErr(&v55v2, bs55, h, t, "dec-slice-v55-p-1")
		testDeepEqualErr(v55v1, v55v2, t, "equal-slice-v55-p-1")
		v55va = [8]float64{} // clear the array
		v55v2 = v55va[:len(v55v1):len(v55v1)]
		testUnmarshalErr(&v55v2, bs55, h, t, "dec-slice-v55-p-len")
		testDeepEqualErr(v55v1, v55v2, t, "equal-slice-v55-p-len")
		v55va = [8]float64{} // clear the array
		v55v2 = v55va[:]
		testUnmarshalErr(&v55v2, bs55, h, t, "dec-slice-v55-p-cap")
		testDeepEqualErr(v55v1, v55v2, t, "equal-slice-v55-p-cap")
		if len(v55v1) > 1 {
			v55va = [8]float64{} // clear the array
			testUnmarshalErr((&v55va)[:len(v55v1)], bs55, h, t, "dec-slice-v55-p-len-noaddr")
			testDeepEqualErr(v55v1, v55va[:len(v55v1)], t, "equal-slice-v55-p-len-noaddr")
			v55va = [8]float64{} // clear the array
			testUnmarshalErr((&v55va)[:], bs55, h, t, "dec-slice-v55-p-cap-noaddr")
			testDeepEqualErr(v55v1, v55va[:len(v55v1)], t, "equal-slice-v55-p-cap-noaddr")
		}
		// ...
		var v55v3, v55v4 typMbsSliceFloat64
		v55v2 = nil
		if v != nil {
			v55v2 = make([]float64, len(v))
		}
		v55v3 = typMbsSliceFloat64(v55v1)
		v55v4 = typMbsSliceFloat64(v55v2)
		bs55 = testMarshalErr(v55v3, h, t, "enc-slice-v55-custom")
		testUnmarshalErr(v55v4, bs55, h, t, "dec-slice-v55-custom")
		testDeepEqualErr(v55v3, v55v4, t, "equal-slice-v55-custom")
		bs55 = testMarshalErr(&v55v3, h, t, "enc-slice-v55-custom-p")
		v55v2 = nil
		v55v4 = typMbsSliceFloat64(v55v2)
		testUnmarshalErr(&v55v4, bs55, h, t, "dec-slice-v55-custom-p")
		testDeepEqualErr(v55v3, v55v4, t, "equal-slice-v55-custom-p")
	}

	var v73va [8]uint
	for _, v := range [][]uint{nil, {}, {44, 0, 0, 33}} {
		var v73v1, v73v2 []uint
		v73v1 = v
		bs73 := testMarshalErr(v73v1, h, t, "enc-slice-v73")
		if v == nil {
			v73v2 = nil
		} else {
			v73v2 = make([]uint, len(v))
		}
		testUnmarshalErr(v73v2, bs73, h, t, "dec-slice-v73")
		testDeepEqualErr(v73v1, v73v2, t, "equal-slice-v73")
		if v == nil {
			v73v2 = nil
		} else {
			v73v2 = make([]uint, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v73v2), bs73, h, t, "dec-slice-v73-noaddr") // non-addressable value
		testDeepEqualErr(v73v1, v73v2, t, "equal-slice-v73-noaddr")
		// ...
		bs73 = testMarshalErr(&v73v1, h, t, "enc-slice-v73-p")
		v73v2 = nil
		testUnmarshalErr(&v73v2, bs73, h, t, "dec-slice-v73-p")
		testDeepEqualErr(v73v1, v73v2, t, "equal-slice-v73-p")
		v73va = [8]uint{} // clear the array
		v73v2 = v73va[:1:1]
		testUnmarshalErr(&v73v2, bs73, h, t, "dec-slice-v73-p-1")
		testDeepEqualErr(v73v1, v73v2, t, "equal-slice-v73-p-1")
		v73va = [8]uint{} // clear the array
		v73v2 = v73va[:len(v73v1):len(v73v1)]
		testUnmarshalErr(&v73v2, bs73, h, t, "dec-slice-v73-p-len")
		testDeepEqualErr(v73v1, v73v2, t, "equal-slice-v73-p-len")
		v73va = [8]uint{} // clear the array
		v73v2 = v73va[:]
		testUnmarshalErr(&v73v2, bs73, h, t, "dec-slice-v73-p-cap")
		testDeepEqualErr(v73v1, v73v2, t, "equal-slice-v73-p-cap")
		if len(v73v1) > 1 {
			v73va = [8]uint{} // clear the array
			testUnmarshalErr((&v73va)[:len(v73v1)], bs73, h, t, "dec-slice-v73-p-len-noaddr")
			testDeepEqualErr(v73v1, v73va[:len(v73v1)], t, "equal-slice-v73-p-len-noaddr")
			v73va = [8]uint{} // clear the array
			testUnmarshalErr((&v73va)[:], bs73, h, t, "dec-slice-v73-p-cap-noaddr")
			testDeepEqualErr(v73v1, v73va[:len(v73v1)], t, "equal-slice-v73-p-cap-noaddr")
		}
		// ...
		var v73v3, v73v4 typMbsSliceUint
		v73v2 = nil
		if v != nil {
			v73v2 = make([]uint, len(v))
		}
		v73v3 = typMbsSliceUint(v73v1)
		v73v4 = typMbsSliceUint(v73v2)
		bs73 = testMarshalErr(v73v3, h, t, "enc-slice-v73-custom")
		testUnmarshalErr(v73v4, bs73, h, t, "dec-slice-v73-custom")
		testDeepEqualErr(v73v3, v73v4, t, "equal-slice-v73-custom")
		bs73 = testMarshalErr(&v73v3, h, t, "enc-slice-v73-custom-p")
		v73v2 = nil
		v73v4 = typMbsSliceUint(v73v2)
		testUnmarshalErr(&v73v4, bs73, h, t, "dec-slice-v73-custom-p")
		testDeepEqualErr(v73v3, v73v4, t, "equal-slice-v73-custom-p")
	}

	var v91va [8]uint8
	for _, v := range [][]uint8{nil, {}, {44, 0, 0, 33}} {
		var v91v1, v91v2 []uint8
		v91v1 = v
		bs91 := testMarshalErr(v91v1, h, t, "enc-slice-v91")
		if v == nil {
			v91v2 = nil
		} else {
			v91v2 = make([]uint8, len(v))
		}
		testUnmarshalErr(v91v2, bs91, h, t, "dec-slice-v91")
		testDeepEqualErr(v91v1, v91v2, t, "equal-slice-v91")
		if v == nil {
			v91v2 = nil
		} else {
			v91v2 = make([]uint8, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v91v2), bs91, h, t, "dec-slice-v91-noaddr") // non-addressable value
		testDeepEqualErr(v91v1, v91v2, t, "equal-slice-v91-noaddr")
		// ...
		bs91 = testMarshalErr(&v91v1, h, t, "enc-slice-v91-p")
		v91v2 = nil
		testUnmarshalErr(&v91v2, bs91, h, t, "dec-slice-v91-p")
		testDeepEqualErr(v91v1, v91v2, t, "equal-slice-v91-p")
		v91va = [8]uint8{} // clear the array
		v91v2 = v91va[:1:1]
		testUnmarshalErr(&v91v2, bs91, h, t, "dec-slice-v91-p-1")
		testDeepEqualErr(v91v1, v91v2, t, "equal-slice-v91-p-1")
		v91va = [8]uint8{} // clear the array
		v91v2 = v91va[:len(v91v1):len(v91v1)]
		testUnmarshalErr(&v91v2, bs91, h, t, "dec-slice-v91-p-len")
		testDeepEqualErr(v91v1, v91v2, t, "equal-slice-v91-p-len")
		v91va = [8]uint8{} // clear the array
		v91v2 = v91va[:]
		testUnmarshalErr(&v91v2, bs91, h, t, "dec-slice-v91-p-cap")
		testDeepEqualErr(v91v1, v91v2, t, "equal-slice-v91-p-cap")
		if len(v91v1) > 1 {
			v91va = [8]uint8{} // clear the array
			testUnmarshalErr((&v91va)[:len(v91v1)], bs91, h, t, "dec-slice-v91-p-len-noaddr")
			testDeepEqualErr(v91v1, v91va[:len(v91v1)], t, "equal-slice-v91-p-len-noaddr")
			v91va = [8]uint8{} // clear the array
			testUnmarshalErr((&v91va)[:], bs91, h, t, "dec-slice-v91-p-cap-noaddr")
			testDeepEqualErr(v91v1, v91va[:len(v91v1)], t, "equal-slice-v91-p-cap-noaddr")
		}
		// ...
		var v91v3, v91v4 typMbsSliceUint8
		v91v2 = nil
		if v != nil {
			v91v2 = make([]uint8, len(v))
		}
		v91v3 = typMbsSliceUint8(v91v1)
		v91v4 = typMbsSliceUint8(v91v2)
		bs91 = testMarshalErr(v91v3, h, t, "enc-slice-v91-custom")
		testUnmarshalErr(v91v4, bs91, h, t, "dec-slice-v91-custom")
		testDeepEqualErr(v91v3, v91v4, t, "equal-slice-v91-custom")
		bs91 = testMarshalErr(&v91v3, h, t, "enc-slice-v91-custom-p")
		v91v2 = nil
		v91v4 = typMbsSliceUint8(v91v2)
		testUnmarshalErr(&v91v4, bs91, h, t, "dec-slice-v91-custom-p")
		testDeepEqualErr(v91v3, v91v4, t, "equal-slice-v91-custom-p")
	}

	var v109va [8]uint16
	for _, v := range [][]uint16{nil, {}, {44, 0, 0, 33}} {
		var v109v1, v109v2 []uint16
		v109v1 = v
		bs109 := testMarshalErr(v109v1, h, t, "enc-slice-v109")
		if v == nil {
			v109v2 = nil
		} else {
			v109v2 = make([]uint16, len(v))
		}
		testUnmarshalErr(v109v2, bs109, h, t, "dec-slice-v109")
		testDeepEqualErr(v109v1, v109v2, t, "equal-slice-v109")
		if v == nil {
			v109v2 = nil
		} else {
			v109v2 = make([]uint16, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v109v2), bs109, h, t, "dec-slice-v109-noaddr") // non-addressable value
		testDeepEqualErr(v109v1, v109v2, t, "equal-slice-v109-noaddr")
		// ...
		bs109 = testMarshalErr(&v109v1, h, t, "enc-slice-v109-p")
		v109v2 = nil
		testUnmarshalErr(&v109v2, bs109, h, t, "dec-slice-v109-p")
		testDeepEqualErr(v109v1, v109v2, t, "equal-slice-v109-p")
		v109va = [8]uint16{} // clear the array
		v109v2 = v109va[:1:1]
		testUnmarshalErr(&v109v2, bs109, h, t, "dec-slice-v109-p-1")
		testDeepEqualErr(v109v1, v109v2, t, "equal-slice-v109-p-1")
		v109va = [8]uint16{} // clear the array
		v109v2 = v109va[:len(v109v1):len(v109v1)]
		testUnmarshalErr(&v109v2, bs109, h, t, "dec-slice-v109-p-len")
		testDeepEqualErr(v109v1, v109v2, t, "equal-slice-v109-p-len")
		v109va = [8]uint16{} // clear the array
		v109v2 = v109va[:]
		testUnmarshalErr(&v109v2, bs109, h, t, "dec-slice-v109-p-cap")
		testDeepEqualErr(v109v1, v109v2, t, "equal-slice-v109-p-cap")
		if len(v109v1) > 1 {
			v109va = [8]uint16{} // clear the array
			testUnmarshalErr((&v109va)[:len(v109v1)], bs109, h, t, "dec-slice-v109-p-len-noaddr")
			testDeepEqualErr(v109v1, v109va[:len(v109v1)], t, "equal-slice-v109-p-len-noaddr")
			v109va = [8]uint16{} // clear the array
			testUnmarshalErr((&v109va)[:], bs109, h, t, "dec-slice-v109-p-cap-noaddr")
			testDeepEqualErr(v109v1, v109va[:len(v109v1)], t, "equal-slice-v109-p-cap-noaddr")
		}
		// ...
		var v109v3, v109v4 typMbsSliceUint16
		v109v2 = nil
		if v != nil {
			v109v2 = make([]uint16, len(v))
		}
		v109v3 = typMbsSliceUint16(v109v1)
		v109v4 = typMbsSliceUint16(v109v2)
		bs109 = testMarshalErr(v109v3, h, t, "enc-slice-v109-custom")
		testUnmarshalErr(v109v4, bs109, h, t, "dec-slice-v109-custom")
		testDeepEqualErr(v109v3, v109v4, t, "equal-slice-v109-custom")
		bs109 = testMarshalErr(&v109v3, h, t, "enc-slice-v109-custom-p")
		v109v2 = nil
		v109v4 = typMbsSliceUint16(v109v2)
		testUnmarshalErr(&v109v4, bs109, h, t, "dec-slice-v109-custom-p")
		testDeepEqualErr(v109v3, v109v4, t, "equal-slice-v109-custom-p")
	}

	var v127va [8]uint32
	for _, v := range [][]uint32{nil, {}, {44, 0, 0, 33}} {
		var v127v1, v127v2 []uint32
		v127v1 = v
		bs127 := testMarshalErr(v127v1, h, t, "enc-slice-v127")
		if v == nil {
			v127v2 = nil
		} else {
			v127v2 = make([]uint32, len(v))
		}
		testUnmarshalErr(v127v2, bs127, h, t, "dec-slice-v127")
		testDeepEqualErr(v127v1, v127v2, t, "equal-slice-v127")
		if v == nil {
			v127v2 = nil
		} else {
			v127v2 = make([]uint32, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v127v2), bs127, h, t, "dec-slice-v127-noaddr") // non-addressable value
		testDeepEqualErr(v127v1, v127v2, t, "equal-slice-v127-noaddr")
		// ...
		bs127 = testMarshalErr(&v127v1, h, t, "enc-slice-v127-p")
		v127v2 = nil
		testUnmarshalErr(&v127v2, bs127, h, t, "dec-slice-v127-p")
		testDeepEqualErr(v127v1, v127v2, t, "equal-slice-v127-p")
		v127va = [8]uint32{} // clear the array
		v127v2 = v127va[:1:1]
		testUnmarshalErr(&v127v2, bs127, h, t, "dec-slice-v127-p-1")
		testDeepEqualErr(v127v1, v127v2, t, "equal-slice-v127-p-1")
		v127va = [8]uint32{} // clear the array
		v127v2 = v127va[:len(v127v1):len(v127v1)]
		testUnmarshalErr(&v127v2, bs127, h, t, "dec-slice-v127-p-len")
		testDeepEqualErr(v127v1, v127v2, t, "equal-slice-v127-p-len")
		v127va = [8]uint32{} // clear the array
		v127v2 = v127va[:]
		testUnmarshalErr(&v127v2, bs127, h, t, "dec-slice-v127-p-cap")
		testDeepEqualErr(v127v1, v127v2, t, "equal-slice-v127-p-cap")
		if len(v127v1) > 1 {
			v127va = [8]uint32{} // clear the array
			testUnmarshalErr((&v127va)[:len(v127v1)], bs127, h, t, "dec-slice-v127-p-len-noaddr")
			testDeepEqualErr(v127v1, v127va[:len(v127v1)], t, "equal-slice-v127-p-len-noaddr")
			v127va = [8]uint32{} // clear the array
			testUnmarshalErr((&v127va)[:], bs127, h, t, "dec-slice-v127-p-cap-noaddr")
			testDeepEqualErr(v127v1, v127va[:len(v127v1)], t, "equal-slice-v127-p-cap-noaddr")
		}
		// ...
		var v127v3, v127v4 typMbsSliceUint32
		v127v2 = nil
		if v != nil {
			v127v2 = make([]uint32, len(v))
		}
		v127v3 = typMbsSliceUint32(v127v1)
		v127v4 = typMbsSliceUint32(v127v2)
		bs127 = testMarshalErr(v127v3, h, t, "enc-slice-v127-custom")
		testUnmarshalErr(v127v4, bs127, h, t, "dec-slice-v127-custom")
		testDeepEqualErr(v127v3, v127v4, t, "equal-slice-v127-custom")
		bs127 = testMarshalErr(&v127v3, h, t, "enc-slice-v127-custom-p")
		v127v2 = nil
		v127v4 = typMbsSliceUint32(v127v2)
		testUnmarshalErr(&v127v4, bs127, h, t, "dec-slice-v127-custom-p")
		testDeepEqualErr(v127v3, v127v4, t, "equal-slice-v127-custom-p")
	}

	var v145va [8]uint64
	for _, v := range [][]uint64{nil, {}, {44, 0, 0, 33}} {
		var v145v1, v145v2 []uint64
		v145v1 = v
		bs145 := testMarshalErr(v145v1, h, t, "enc-slice-v145")
		if v == nil {
			v145v2 = nil
		} else {
			v145v2 = make([]uint64, len(v))
		}
		testUnmarshalErr(v145v2, bs145, h, t, "dec-slice-v145")
		testDeepEqualErr(v145v1, v145v2, t, "equal-slice-v145")
		if v == nil {
			v145v2 = nil
		} else {
			v145v2 = make([]uint64, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v145v2), bs145, h, t, "dec-slice-v145-noaddr") // non-addressable value
		testDeepEqualErr(v145v1, v145v2, t, "equal-slice-v145-noaddr")
		// ...
		bs145 = testMarshalErr(&v145v1, h, t, "enc-slice-v145-p")
		v145v2 = nil
		testUnmarshalErr(&v145v2, bs145, h, t, "dec-slice-v145-p")
		testDeepEqualErr(v145v1, v145v2, t, "equal-slice-v145-p")
		v145va = [8]uint64{} // clear the array
		v145v2 = v145va[:1:1]
		testUnmarshalErr(&v145v2, bs145, h, t, "dec-slice-v145-p-1")
		testDeepEqualErr(v145v1, v145v2, t, "equal-slice-v145-p-1")
		v145va = [8]uint64{} // clear the array
		v145v2 = v145va[:len(v145v1):len(v145v1)]
		testUnmarshalErr(&v145v2, bs145, h, t, "dec-slice-v145-p-len")
		testDeepEqualErr(v145v1, v145v2, t, "equal-slice-v145-p-len")
		v145va = [8]uint64{} // clear the array
		v145v2 = v145va[:]
		testUnmarshalErr(&v145v2, bs145, h, t, "dec-slice-v145-p-cap")
		testDeepEqualErr(v145v1, v145v2, t, "equal-slice-v145-p-cap")
		if len(v145v1) > 1 {
			v145va = [8]uint64{} // clear the array
			testUnmarshalErr((&v145va)[:len(v145v1)], bs145, h, t, "dec-slice-v145-p-len-noaddr")
			testDeepEqualErr(v145v1, v145va[:len(v145v1)], t, "equal-slice-v145-p-len-noaddr")
			v145va = [8]uint64{} // clear the array
			testUnmarshalErr((&v145va)[:], bs145, h, t, "dec-slice-v145-p-cap-noaddr")
			testDeepEqualErr(v145v1, v145va[:len(v145v1)], t, "equal-slice-v145-p-cap-noaddr")
		}
		// ...
		var v145v3, v145v4 typMbsSliceUint64
		v145v2 = nil
		if v != nil {
			v145v2 = make([]uint64, len(v))
		}
		v145v3 = typMbsSliceUint64(v145v1)
		v145v4 = typMbsSliceUint64(v145v2)
		bs145 = testMarshalErr(v145v3, h, t, "enc-slice-v145-custom")
		testUnmarshalErr(v145v4, bs145, h, t, "dec-slice-v145-custom")
		testDeepEqualErr(v145v3, v145v4, t, "equal-slice-v145-custom")
		bs145 = testMarshalErr(&v145v3, h, t, "enc-slice-v145-custom-p")
		v145v2 = nil
		v145v4 = typMbsSliceUint64(v145v2)
		testUnmarshalErr(&v145v4, bs145, h, t, "dec-slice-v145-custom-p")
		testDeepEqualErr(v145v3, v145v4, t, "equal-slice-v145-custom-p")
	}

	var v163va [8]uintptr
	for _, v := range [][]uintptr{nil, {}, {44, 0, 0, 33}} {
		var v163v1, v163v2 []uintptr
		v163v1 = v
		bs163 := testMarshalErr(v163v1, h, t, "enc-slice-v163")
		if v == nil {
			v163v2 = nil
		} else {
			v163v2 = make([]uintptr, len(v))
		}
		testUnmarshalErr(v163v2, bs163, h, t, "dec-slice-v163")
		testDeepEqualErr(v163v1, v163v2, t, "equal-slice-v163")
		if v == nil {
			v163v2 = nil
		} else {
			v163v2 = make([]uintptr, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v163v2), bs163, h, t, "dec-slice-v163-noaddr") // non-addressable value
		testDeepEqualErr(v163v1, v163v2, t, "equal-slice-v163-noaddr")
		// ...
		bs163 = testMarshalErr(&v163v1, h, t, "enc-slice-v163-p")
		v163v2 = nil
		testUnmarshalErr(&v163v2, bs163, h, t, "dec-slice-v163-p")
		testDeepEqualErr(v163v1, v163v2, t, "equal-slice-v163-p")
		v163va = [8]uintptr{} // clear the array
		v163v2 = v163va[:1:1]
		testUnmarshalErr(&v163v2, bs163, h, t, "dec-slice-v163-p-1")
		testDeepEqualErr(v163v1, v163v2, t, "equal-slice-v163-p-1")
		v163va = [8]uintptr{} // clear the array
		v163v2 = v163va[:len(v163v1):len(v163v1)]
		testUnmarshalErr(&v163v2, bs163, h, t, "dec-slice-v163-p-len")
		testDeepEqualErr(v163v1, v163v2, t, "equal-slice-v163-p-len")
		v163va = [8]uintptr{} // clear the array
		v163v2 = v163va[:]
		testUnmarshalErr(&v163v2, bs163, h, t, "dec-slice-v163-p-cap")
		testDeepEqualErr(v163v1, v163v2, t, "equal-slice-v163-p-cap")
		if len(v163v1) > 1 {
			v163va = [8]uintptr{} // clear the array
			testUnmarshalErr((&v163va)[:len(v163v1)], bs163, h, t, "dec-slice-v163-p-len-noaddr")
			testDeepEqualErr(v163v1, v163va[:len(v163v1)], t, "equal-slice-v163-p-len-noaddr")
			v163va = [8]uintptr{} // clear the array
			testUnmarshalErr((&v163va)[:], bs163, h, t, "dec-slice-v163-p-cap-noaddr")
			testDeepEqualErr(v163v1, v163va[:len(v163v1)], t, "equal-slice-v163-p-cap-noaddr")
		}
		// ...
		var v163v3, v163v4 typMbsSliceUintptr
		v163v2 = nil
		if v != nil {
			v163v2 = make([]uintptr, len(v))
		}
		v163v3 = typMbsSliceUintptr(v163v1)
		v163v4 = typMbsSliceUintptr(v163v2)
		bs163 = testMarshalErr(v163v3, h, t, "enc-slice-v163-custom")
		testUnmarshalErr(v163v4, bs163, h, t, "dec-slice-v163-custom")
		testDeepEqualErr(v163v3, v163v4, t, "equal-slice-v163-custom")
		bs163 = testMarshalErr(&v163v3, h, t, "enc-slice-v163-custom-p")
		v163v2 = nil
		v163v4 = typMbsSliceUintptr(v163v2)
		testUnmarshalErr(&v163v4, bs163, h, t, "dec-slice-v163-custom-p")
		testDeepEqualErr(v163v3, v163v4, t, "equal-slice-v163-custom-p")
	}

	var v181va [8]int
	for _, v := range [][]int{nil, {}, {44, 0, 0, 33}} {
		var v181v1, v181v2 []int
		v181v1 = v
		bs181 := testMarshalErr(v181v1, h, t, "enc-slice-v181")
		if v == nil {
			v181v2 = nil
		} else {
			v181v2 = make([]int, len(v))
		}
		testUnmarshalErr(v181v2, bs181, h, t, "dec-slice-v181")
		testDeepEqualErr(v181v1, v181v2, t, "equal-slice-v181")
		if v == nil {
			v181v2 = nil
		} else {
			v181v2 = make([]int, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v181v2), bs181, h, t, "dec-slice-v181-noaddr") // non-addressable value
		testDeepEqualErr(v181v1, v181v2, t, "equal-slice-v181-noaddr")
		// ...
		bs181 = testMarshalErr(&v181v1, h, t, "enc-slice-v181-p")
		v181v2 = nil
		testUnmarshalErr(&v181v2, bs181, h, t, "dec-slice-v181-p")
		testDeepEqualErr(v181v1, v181v2, t, "equal-slice-v181-p")
		v181va = [8]int{} // clear the array
		v181v2 = v181va[:1:1]
		testUnmarshalErr(&v181v2, bs181, h, t, "dec-slice-v181-p-1")
		testDeepEqualErr(v181v1, v181v2, t, "equal-slice-v181-p-1")
		v181va = [8]int{} // clear the array
		v181v2 = v181va[:len(v181v1):len(v181v1)]
		testUnmarshalErr(&v181v2, bs181, h, t, "dec-slice-v181-p-len")
		testDeepEqualErr(v181v1, v181v2, t, "equal-slice-v181-p-len")
		v181va = [8]int{} // clear the array
		v181v2 = v181va[:]
		testUnmarshalErr(&v181v2, bs181, h, t, "dec-slice-v181-p-cap")
		testDeepEqualErr(v181v1, v181v2, t, "equal-slice-v181-p-cap")
		if len(v181v1) > 1 {
			v181va = [8]int{} // clear the array
			testUnmarshalErr((&v181va)[:len(v181v1)], bs181, h, t, "dec-slice-v181-p-len-noaddr")
			testDeepEqualErr(v181v1, v181va[:len(v181v1)], t, "equal-slice-v181-p-len-noaddr")
			v181va = [8]int{} // clear the array
			testUnmarshalErr((&v181va)[:], bs181, h, t, "dec-slice-v181-p-cap-noaddr")
			testDeepEqualErr(v181v1, v181va[:len(v181v1)], t, "equal-slice-v181-p-cap-noaddr")
		}
		// ...
		var v181v3, v181v4 typMbsSliceInt
		v181v2 = nil
		if v != nil {
			v181v2 = make([]int, len(v))
		}
		v181v3 = typMbsSliceInt(v181v1)
		v181v4 = typMbsSliceInt(v181v2)
		bs181 = testMarshalErr(v181v3, h, t, "enc-slice-v181-custom")
		testUnmarshalErr(v181v4, bs181, h, t, "dec-slice-v181-custom")
		testDeepEqualErr(v181v3, v181v4, t, "equal-slice-v181-custom")
		bs181 = testMarshalErr(&v181v3, h, t, "enc-slice-v181-custom-p")
		v181v2 = nil
		v181v4 = typMbsSliceInt(v181v2)
		testUnmarshalErr(&v181v4, bs181, h, t, "dec-slice-v181-custom-p")
		testDeepEqualErr(v181v3, v181v4, t, "equal-slice-v181-custom-p")
	}

	var v199va [8]int8
	for _, v := range [][]int8{nil, {}, {44, 0, 0, 33}} {
		var v199v1, v199v2 []int8
		v199v1 = v
		bs199 := testMarshalErr(v199v1, h, t, "enc-slice-v199")
		if v == nil {
			v199v2 = nil
		} else {
			v199v2 = make([]int8, len(v))
		}
		testUnmarshalErr(v199v2, bs199, h, t, "dec-slice-v199")
		testDeepEqualErr(v199v1, v199v2, t, "equal-slice-v199")
		if v == nil {
			v199v2 = nil
		} else {
			v199v2 = make([]int8, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v199v2), bs199, h, t, "dec-slice-v199-noaddr") // non-addressable value
		testDeepEqualErr(v199v1, v199v2, t, "equal-slice-v199-noaddr")
		// ...
		bs199 = testMarshalErr(&v199v1, h, t, "enc-slice-v199-p")
		v199v2 = nil
		testUnmarshalErr(&v199v2, bs199, h, t, "dec-slice-v199-p")
		testDeepEqualErr(v199v1, v199v2, t, "equal-slice-v199-p")
		v199va = [8]int8{} // clear the array
		v199v2 = v199va[:1:1]
		testUnmarshalErr(&v199v2, bs199, h, t, "dec-slice-v199-p-1")
		testDeepEqualErr(v199v1, v199v2, t, "equal-slice-v199-p-1")
		v199va = [8]int8{} // clear the array
		v199v2 = v199va[:len(v199v1):len(v199v1)]
		testUnmarshalErr(&v199v2, bs199, h, t, "dec-slice-v199-p-len")
		testDeepEqualErr(v199v1, v199v2, t, "equal-slice-v199-p-len")
		v199va = [8]int8{} // clear the array
		v199v2 = v199va[:]
		testUnmarshalErr(&v199v2, bs199, h, t, "dec-slice-v199-p-cap")
		testDeepEqualErr(v199v1, v199v2, t, "equal-slice-v199-p-cap")
		if len(v199v1) > 1 {
			v199va = [8]int8{} // clear the array
			testUnmarshalErr((&v199va)[:len(v199v1)], bs199, h, t, "dec-slice-v199-p-len-noaddr")
			testDeepEqualErr(v199v1, v199va[:len(v199v1)], t, "equal-slice-v199-p-len-noaddr")
			v199va = [8]int8{} // clear the array
			testUnmarshalErr((&v199va)[:], bs199, h, t, "dec-slice-v199-p-cap-noaddr")
			testDeepEqualErr(v199v1, v199va[:len(v199v1)], t, "equal-slice-v199-p-cap-noaddr")
		}
		// ...
		var v199v3, v199v4 typMbsSliceInt8
		v199v2 = nil
		if v != nil {
			v199v2 = make([]int8, len(v))
		}
		v199v3 = typMbsSliceInt8(v199v1)
		v199v4 = typMbsSliceInt8(v199v2)
		bs199 = testMarshalErr(v199v3, h, t, "enc-slice-v199-custom")
		testUnmarshalErr(v199v4, bs199, h, t, "dec-slice-v199-custom")
		testDeepEqualErr(v199v3, v199v4, t, "equal-slice-v199-custom")
		bs199 = testMarshalErr(&v199v3, h, t, "enc-slice-v199-custom-p")
		v199v2 = nil
		v199v4 = typMbsSliceInt8(v199v2)
		testUnmarshalErr(&v199v4, bs199, h, t, "dec-slice-v199-custom-p")
		testDeepEqualErr(v199v3, v199v4, t, "equal-slice-v199-custom-p")
	}

	var v217va [8]int16
	for _, v := range [][]int16{nil, {}, {44, 0, 0, 33}} {
		var v217v1, v217v2 []int16
		v217v1 = v
		bs217 := testMarshalErr(v217v1, h, t, "enc-slice-v217")
		if v == nil {
			v217v2 = nil
		} else {
			v217v2 = make([]int16, len(v))
		}
		testUnmarshalErr(v217v2, bs217, h, t, "dec-slice-v217")
		testDeepEqualErr(v217v1, v217v2, t, "equal-slice-v217")
		if v == nil {
			v217v2 = nil
		} else {
			v217v2 = make([]int16, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v217v2), bs217, h, t, "dec-slice-v217-noaddr") // non-addressable value
		testDeepEqualErr(v217v1, v217v2, t, "equal-slice-v217-noaddr")
		// ...
		bs217 = testMarshalErr(&v217v1, h, t, "enc-slice-v217-p")
		v217v2 = nil
		testUnmarshalErr(&v217v2, bs217, h, t, "dec-slice-v217-p")
		testDeepEqualErr(v217v1, v217v2, t, "equal-slice-v217-p")
		v217va = [8]int16{} // clear the array
		v217v2 = v217va[:1:1]
		testUnmarshalErr(&v217v2, bs217, h, t, "dec-slice-v217-p-1")
		testDeepEqualErr(v217v1, v217v2, t, "equal-slice-v217-p-1")
		v217va = [8]int16{} // clear the array
		v217v2 = v217va[:len(v217v1):len(v217v1)]
		testUnmarshalErr(&v217v2, bs217, h, t, "dec-slice-v217-p-len")
		testDeepEqualErr(v217v1, v217v2, t, "equal-slice-v217-p-len")
		v217va = [8]int16{} // clear the array
		v217v2 = v217va[:]
		testUnmarshalErr(&v217v2, bs217, h, t, "dec-slice-v217-p-cap")
		testDeepEqualErr(v217v1, v217v2, t, "equal-slice-v217-p-cap")
		if len(v217v1) > 1 {
			v217va = [8]int16{} // clear the array
			testUnmarshalErr((&v217va)[:len(v217v1)], bs217, h, t, "dec-slice-v217-p-len-noaddr")
			testDeepEqualErr(v217v1, v217va[:len(v217v1)], t, "equal-slice-v217-p-len-noaddr")
			v217va = [8]int16{} // clear the array
			testUnmarshalErr((&v217va)[:], bs217, h, t, "dec-slice-v217-p-cap-noaddr")
			testDeepEqualErr(v217v1, v217va[:len(v217v1)], t, "equal-slice-v217-p-cap-noaddr")
		}
		// ...
		var v217v3, v217v4 typMbsSliceInt16
		v217v2 = nil
		if v != nil {
			v217v2 = make([]int16, len(v))
		}
		v217v3 = typMbsSliceInt16(v217v1)
		v217v4 = typMbsSliceInt16(v217v2)
		bs217 = testMarshalErr(v217v3, h, t, "enc-slice-v217-custom")
		testUnmarshalErr(v217v4, bs217, h, t, "dec-slice-v217-custom")
		testDeepEqualErr(v217v3, v217v4, t, "equal-slice-v217-custom")
		bs217 = testMarshalErr(&v217v3, h, t, "enc-slice-v217-custom-p")
		v217v2 = nil
		v217v4 = typMbsSliceInt16(v217v2)
		testUnmarshalErr(&v217v4, bs217, h, t, "dec-slice-v217-custom-p")
		testDeepEqualErr(v217v3, v217v4, t, "equal-slice-v217-custom-p")
	}

	var v235va [8]int32
	for _, v := range [][]int32{nil, {}, {44, 0, 0, 33}} {
		var v235v1, v235v2 []int32
		v235v1 = v
		bs235 := testMarshalErr(v235v1, h, t, "enc-slice-v235")
		if v == nil {
			v235v2 = nil
		} else {
			v235v2 = make([]int32, len(v))
		}
		testUnmarshalErr(v235v2, bs235, h, t, "dec-slice-v235")
		testDeepEqualErr(v235v1, v235v2, t, "equal-slice-v235")
		if v == nil {
			v235v2 = nil
		} else {
			v235v2 = make([]int32, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v235v2), bs235, h, t, "dec-slice-v235-noaddr") // non-addressable value
		testDeepEqualErr(v235v1, v235v2, t, "equal-slice-v235-noaddr")
		// ...
		bs235 = testMarshalErr(&v235v1, h, t, "enc-slice-v235-p")
		v235v2 = nil
		testUnmarshalErr(&v235v2, bs235, h, t, "dec-slice-v235-p")
		testDeepEqualErr(v235v1, v235v2, t, "equal-slice-v235-p")
		v235va = [8]int32{} // clear the array
		v235v2 = v235va[:1:1]
		testUnmarshalErr(&v235v2, bs235, h, t, "dec-slice-v235-p-1")
		testDeepEqualErr(v235v1, v235v2, t, "equal-slice-v235-p-1")
		v235va = [8]int32{} // clear the array
		v235v2 = v235va[:len(v235v1):len(v235v1)]
		testUnmarshalErr(&v235v2, bs235, h, t, "dec-slice-v235-p-len")
		testDeepEqualErr(v235v1, v235v2, t, "equal-slice-v235-p-len")
		v235va = [8]int32{} // clear the array
		v235v2 = v235va[:]
		testUnmarshalErr(&v235v2, bs235, h, t, "dec-slice-v235-p-cap")
		testDeepEqualErr(v235v1, v235v2, t, "equal-slice-v235-p-cap")
		if len(v235v1) > 1 {
			v235va = [8]int32{} // clear the array
			testUnmarshalErr((&v235va)[:len(v235v1)], bs235, h, t, "dec-slice-v235-p-len-noaddr")
			testDeepEqualErr(v235v1, v235va[:len(v235v1)], t, "equal-slice-v235-p-len-noaddr")
			v235va = [8]int32{} // clear the array
			testUnmarshalErr((&v235va)[:], bs235, h, t, "dec-slice-v235-p-cap-noaddr")
			testDeepEqualErr(v235v1, v235va[:len(v235v1)], t, "equal-slice-v235-p-cap-noaddr")
		}
		// ...
		var v235v3, v235v4 typMbsSliceInt32
		v235v2 = nil
		if v != nil {
			v235v2 = make([]int32, len(v))
		}
		v235v3 = typMbsSliceInt32(v235v1)
		v235v4 = typMbsSliceInt32(v235v2)
		bs235 = testMarshalErr(v235v3, h, t, "enc-slice-v235-custom")
		testUnmarshalErr(v235v4, bs235, h, t, "dec-slice-v235-custom")
		testDeepEqualErr(v235v3, v235v4, t, "equal-slice-v235-custom")
		bs235 = testMarshalErr(&v235v3, h, t, "enc-slice-v235-custom-p")
		v235v2 = nil
		v235v4 = typMbsSliceInt32(v235v2)
		testUnmarshalErr(&v235v4, bs235, h, t, "dec-slice-v235-custom-p")
		testDeepEqualErr(v235v3, v235v4, t, "equal-slice-v235-custom-p")
	}

	var v253va [8]int64
	for _, v := range [][]int64{nil, {}, {44, 0, 0, 33}} {
		var v253v1, v253v2 []int64
		v253v1 = v
		bs253 := testMarshalErr(v253v1, h, t, "enc-slice-v253")
		if v == nil {
			v253v2 = nil
		} else {
			v253v2 = make([]int64, len(v))
		}
		testUnmarshalErr(v253v2, bs253, h, t, "dec-slice-v253")
		testDeepEqualErr(v253v1, v253v2, t, "equal-slice-v253")
		if v == nil {
			v253v2 = nil
		} else {
			v253v2 = make([]int64, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v253v2), bs253, h, t, "dec-slice-v253-noaddr") // non-addressable value
		testDeepEqualErr(v253v1, v253v2, t, "equal-slice-v253-noaddr")
		// ...
		bs253 = testMarshalErr(&v253v1, h, t, "enc-slice-v253-p")
		v253v2 = nil
		testUnmarshalErr(&v253v2, bs253, h, t, "dec-slice-v253-p")
		testDeepEqualErr(v253v1, v253v2, t, "equal-slice-v253-p")
		v253va = [8]int64{} // clear the array
		v253v2 = v253va[:1:1]
		testUnmarshalErr(&v253v2, bs253, h, t, "dec-slice-v253-p-1")
		testDeepEqualErr(v253v1, v253v2, t, "equal-slice-v253-p-1")
		v253va = [8]int64{} // clear the array
		v253v2 = v253va[:len(v253v1):len(v253v1)]
		testUnmarshalErr(&v253v2, bs253, h, t, "dec-slice-v253-p-len")
		testDeepEqualErr(v253v1, v253v2, t, "equal-slice-v253-p-len")
		v253va = [8]int64{} // clear the array
		v253v2 = v253va[:]
		testUnmarshalErr(&v253v2, bs253, h, t, "dec-slice-v253-p-cap")
		testDeepEqualErr(v253v1, v253v2, t, "equal-slice-v253-p-cap")
		if len(v253v1) > 1 {
			v253va = [8]int64{} // clear the array
			testUnmarshalErr((&v253va)[:len(v253v1)], bs253, h, t, "dec-slice-v253-p-len-noaddr")
			testDeepEqualErr(v253v1, v253va[:len(v253v1)], t, "equal-slice-v253-p-len-noaddr")
			v253va = [8]int64{} // clear the array
			testUnmarshalErr((&v253va)[:], bs253, h, t, "dec-slice-v253-p-cap-noaddr")
			testDeepEqualErr(v253v1, v253va[:len(v253v1)], t, "equal-slice-v253-p-cap-noaddr")
		}
		// ...
		var v253v3, v253v4 typMbsSliceInt64
		v253v2 = nil
		if v != nil {
			v253v2 = make([]int64, len(v))
		}
		v253v3 = typMbsSliceInt64(v253v1)
		v253v4 = typMbsSliceInt64(v253v2)
		bs253 = testMarshalErr(v253v3, h, t, "enc-slice-v253-custom")
		testUnmarshalErr(v253v4, bs253, h, t, "dec-slice-v253-custom")
		testDeepEqualErr(v253v3, v253v4, t, "equal-slice-v253-custom")
		bs253 = testMarshalErr(&v253v3, h, t, "enc-slice-v253-custom-p")
		v253v2 = nil
		v253v4 = typMbsSliceInt64(v253v2)
		testUnmarshalErr(&v253v4, bs253, h, t, "dec-slice-v253-custom-p")
		testDeepEqualErr(v253v3, v253v4, t, "equal-slice-v253-custom-p")
	}

	var v271va [8]bool
	for _, v := range [][]bool{nil, {}, {true, false, false, true}} {
		var v271v1, v271v2 []bool
		v271v1 = v
		bs271 := testMarshalErr(v271v1, h, t, "enc-slice-v271")
		if v == nil {
			v271v2 = nil
		} else {
			v271v2 = make([]bool, len(v))
		}
		testUnmarshalErr(v271v2, bs271, h, t, "dec-slice-v271")
		testDeepEqualErr(v271v1, v271v2, t, "equal-slice-v271")
		if v == nil {
			v271v2 = nil
		} else {
			v271v2 = make([]bool, len(v))
		}
		testUnmarshalErr(reflect.ValueOf(v271v2), bs271, h, t, "dec-slice-v271-noaddr") // non-addressable value
		testDeepEqualErr(v271v1, v271v2, t, "equal-slice-v271-noaddr")
		// ...
		bs271 = testMarshalErr(&v271v1, h, t, "enc-slice-v271-p")
		v271v2 = nil
		testUnmarshalErr(&v271v2, bs271, h, t, "dec-slice-v271-p")
		testDeepEqualErr(v271v1, v271v2, t, "equal-slice-v271-p")
		v271va = [8]bool{} // clear the array
		v271v2 = v271va[:1:1]
		testUnmarshalErr(&v271v2, bs271, h, t, "dec-slice-v271-p-1")
		testDeepEqualErr(v271v1, v271v2, t, "equal-slice-v271-p-1")
		v271va = [8]bool{} // clear the array
		v271v2 = v271va[:len(v271v1):len(v271v1)]
		testUnmarshalErr(&v271v2, bs271, h, t, "dec-slice-v271-p-len")
		testDeepEqualErr(v271v1, v271v2, t, "equal-slice-v271-p-len")
		v271va = [8]bool{} // clear the array
		v271v2 = v271va[:]
		testUnmarshalErr(&v271v2, bs271, h, t, "dec-slice-v271-p-cap")
		testDeepEqualErr(v271v1, v271v2, t, "equal-slice-v271-p-cap")
		if len(v271v1) > 1 {
			v271va = [8]bool{} // clear the array
			testUnmarshalErr((&v271va)[:len(v271v1)], bs271, h, t, "dec-slice-v271-p-len-noaddr")
			testDeepEqualErr(v271v1, v271va[:len(v271v1)], t, "equal-slice-v271-p-len-noaddr")
			v271va = [8]bool{} // clear the array
			testUnmarshalErr((&v271va)[:], bs271, h, t, "dec-slice-v271-p-cap-noaddr")
			testDeepEqualErr(v271v1, v271va[:len(v271v1)], t, "equal-slice-v271-p-cap-noaddr")
		}
		// ...
		var v271v3, v271v4 typMbsSliceBool
		v271v2 = nil
		if v != nil {
			v271v2 = make([]bool, len(v))
		}
		v271v3 = typMbsSliceBool(v271v1)
		v271v4 = typMbsSliceBool(v271v2)
		bs271 = testMarshalErr(v271v3, h, t, "enc-slice-v271-custom")
		testUnmarshalErr(v271v4, bs271, h, t, "dec-slice-v271-custom")
		testDeepEqualErr(v271v3, v271v4, t, "equal-slice-v271-custom")
		bs271 = testMarshalErr(&v271v3, h, t, "enc-slice-v271-custom-p")
		v271v2 = nil
		v271v4 = typMbsSliceBool(v271v2)
		testUnmarshalErr(&v271v4, bs271, h, t, "dec-slice-v271-custom-p")
		testDeepEqualErr(v271v3, v271v4, t, "equal-slice-v271-custom-p")
	}

}

func doTestMammothMaps(t *testing.T, h Handle) {

	for _, v := range []map[interface{}]interface{}{nil, {}, {"string-is-an-interface-2": nil, "string-is-an-interface": "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v2: %v\n", v)
		var v2v1, v2v2 map[interface{}]interface{}
		v2v1 = v
		bs2 := testMarshalErr(v2v1, h, t, "enc-map-v2")
		if v == nil {
			v2v2 = nil
		} else {
			v2v2 = make(map[interface{}]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v2v2, bs2, h, t, "dec-map-v2")
		testDeepEqualErr(v2v1, v2v2, t, "equal-map-v2")
		if v == nil {
			v2v2 = nil
		} else {
			v2v2 = make(map[interface{}]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v2v2), bs2, h, t, "dec-map-v2-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v2v1, v2v2, t, "equal-map-v2-noaddr")
		if v == nil {
			v2v2 = nil
		} else {
			v2v2 = make(map[interface{}]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v2v2, bs2, h, t, "dec-map-v2-p-len")
		testDeepEqualErr(v2v1, v2v2, t, "equal-map-v2-p-len")
		bs2 = testMarshalErr(&v2v1, h, t, "enc-map-v2-p")
		v2v2 = nil
		testUnmarshalErr(&v2v2, bs2, h, t, "dec-map-v2-p-nil")
		testDeepEqualErr(v2v1, v2v2, t, "equal-map-v2-p-nil")
		// ...
		if v == nil {
			v2v2 = nil
		} else {
			v2v2 = make(map[interface{}]interface{}, len(v))
		} // reset map
		var v2v3, v2v4 typMapMapIntfIntf
		v2v3 = typMapMapIntfIntf(v2v1)
		v2v4 = typMapMapIntfIntf(v2v2)
		bs2 = testMarshalErr(v2v3, h, t, "enc-map-v2-custom")
		testUnmarshalErr(v2v4, bs2, h, t, "dec-map-v2-p-len")
		testDeepEqualErr(v2v3, v2v4, t, "equal-map-v2-p-len")
	}

	for _, v := range []map[interface{}]string{nil, {}, {"string-is-an-interface": "", "string-is-an-interface-2": "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v3: %v\n", v)
		var v3v1, v3v2 map[interface{}]string
		v3v1 = v
		bs3 := testMarshalErr(v3v1, h, t, "enc-map-v3")
		if v == nil {
			v3v2 = nil
		} else {
			v3v2 = make(map[interface{}]string, len(v))
		} // reset map
		testUnmarshalErr(v3v2, bs3, h, t, "dec-map-v3")
		testDeepEqualErr(v3v1, v3v2, t, "equal-map-v3")
		if v == nil {
			v3v2 = nil
		} else {
			v3v2 = make(map[interface{}]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v3v2), bs3, h, t, "dec-map-v3-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v3v1, v3v2, t, "equal-map-v3-noaddr")
		if v == nil {
			v3v2 = nil
		} else {
			v3v2 = make(map[interface{}]string, len(v))
		} // reset map
		testUnmarshalErr(&v3v2, bs3, h, t, "dec-map-v3-p-len")
		testDeepEqualErr(v3v1, v3v2, t, "equal-map-v3-p-len")
		bs3 = testMarshalErr(&v3v1, h, t, "enc-map-v3-p")
		v3v2 = nil
		testUnmarshalErr(&v3v2, bs3, h, t, "dec-map-v3-p-nil")
		testDeepEqualErr(v3v1, v3v2, t, "equal-map-v3-p-nil")
		// ...
		if v == nil {
			v3v2 = nil
		} else {
			v3v2 = make(map[interface{}]string, len(v))
		} // reset map
		var v3v3, v3v4 typMapMapIntfString
		v3v3 = typMapMapIntfString(v3v1)
		v3v4 = typMapMapIntfString(v3v2)
		bs3 = testMarshalErr(v3v3, h, t, "enc-map-v3-custom")
		testUnmarshalErr(v3v4, bs3, h, t, "dec-map-v3-p-len")
		testDeepEqualErr(v3v3, v3v4, t, "equal-map-v3-p-len")
	}

	for _, v := range []map[interface{}]uint{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 44}} {
		// fmt.Printf(">>>> running mammoth map v4: %v\n", v)
		var v4v1, v4v2 map[interface{}]uint
		v4v1 = v
		bs4 := testMarshalErr(v4v1, h, t, "enc-map-v4")
		if v == nil {
			v4v2 = nil
		} else {
			v4v2 = make(map[interface{}]uint, len(v))
		} // reset map
		testUnmarshalErr(v4v2, bs4, h, t, "dec-map-v4")
		testDeepEqualErr(v4v1, v4v2, t, "equal-map-v4")
		if v == nil {
			v4v2 = nil
		} else {
			v4v2 = make(map[interface{}]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v4v2), bs4, h, t, "dec-map-v4-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v4v1, v4v2, t, "equal-map-v4-noaddr")
		if v == nil {
			v4v2 = nil
		} else {
			v4v2 = make(map[interface{}]uint, len(v))
		} // reset map
		testUnmarshalErr(&v4v2, bs4, h, t, "dec-map-v4-p-len")
		testDeepEqualErr(v4v1, v4v2, t, "equal-map-v4-p-len")
		bs4 = testMarshalErr(&v4v1, h, t, "enc-map-v4-p")
		v4v2 = nil
		testUnmarshalErr(&v4v2, bs4, h, t, "dec-map-v4-p-nil")
		testDeepEqualErr(v4v1, v4v2, t, "equal-map-v4-p-nil")
		// ...
		if v == nil {
			v4v2 = nil
		} else {
			v4v2 = make(map[interface{}]uint, len(v))
		} // reset map
		var v4v3, v4v4 typMapMapIntfUint
		v4v3 = typMapMapIntfUint(v4v1)
		v4v4 = typMapMapIntfUint(v4v2)
		bs4 = testMarshalErr(v4v3, h, t, "enc-map-v4-custom")
		testUnmarshalErr(v4v4, bs4, h, t, "dec-map-v4-p-len")
		testDeepEqualErr(v4v3, v4v4, t, "equal-map-v4-p-len")
	}

	for _, v := range []map[interface{}]uint8{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 33}} {
		// fmt.Printf(">>>> running mammoth map v5: %v\n", v)
		var v5v1, v5v2 map[interface{}]uint8
		v5v1 = v
		bs5 := testMarshalErr(v5v1, h, t, "enc-map-v5")
		if v == nil {
			v5v2 = nil
		} else {
			v5v2 = make(map[interface{}]uint8, len(v))
		} // reset map
		testUnmarshalErr(v5v2, bs5, h, t, "dec-map-v5")
		testDeepEqualErr(v5v1, v5v2, t, "equal-map-v5")
		if v == nil {
			v5v2 = nil
		} else {
			v5v2 = make(map[interface{}]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v5v2), bs5, h, t, "dec-map-v5-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v5v1, v5v2, t, "equal-map-v5-noaddr")
		if v == nil {
			v5v2 = nil
		} else {
			v5v2 = make(map[interface{}]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v5v2, bs5, h, t, "dec-map-v5-p-len")
		testDeepEqualErr(v5v1, v5v2, t, "equal-map-v5-p-len")
		bs5 = testMarshalErr(&v5v1, h, t, "enc-map-v5-p")
		v5v2 = nil
		testUnmarshalErr(&v5v2, bs5, h, t, "dec-map-v5-p-nil")
		testDeepEqualErr(v5v1, v5v2, t, "equal-map-v5-p-nil")
		// ...
		if v == nil {
			v5v2 = nil
		} else {
			v5v2 = make(map[interface{}]uint8, len(v))
		} // reset map
		var v5v3, v5v4 typMapMapIntfUint8
		v5v3 = typMapMapIntfUint8(v5v1)
		v5v4 = typMapMapIntfUint8(v5v2)
		bs5 = testMarshalErr(v5v3, h, t, "enc-map-v5-custom")
		testUnmarshalErr(v5v4, bs5, h, t, "dec-map-v5-p-len")
		testDeepEqualErr(v5v3, v5v4, t, "equal-map-v5-p-len")
	}

	for _, v := range []map[interface{}]uint16{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 44}} {
		// fmt.Printf(">>>> running mammoth map v6: %v\n", v)
		var v6v1, v6v2 map[interface{}]uint16
		v6v1 = v
		bs6 := testMarshalErr(v6v1, h, t, "enc-map-v6")
		if v == nil {
			v6v2 = nil
		} else {
			v6v2 = make(map[interface{}]uint16, len(v))
		} // reset map
		testUnmarshalErr(v6v2, bs6, h, t, "dec-map-v6")
		testDeepEqualErr(v6v1, v6v2, t, "equal-map-v6")
		if v == nil {
			v6v2 = nil
		} else {
			v6v2 = make(map[interface{}]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v6v2), bs6, h, t, "dec-map-v6-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v6v1, v6v2, t, "equal-map-v6-noaddr")
		if v == nil {
			v6v2 = nil
		} else {
			v6v2 = make(map[interface{}]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v6v2, bs6, h, t, "dec-map-v6-p-len")
		testDeepEqualErr(v6v1, v6v2, t, "equal-map-v6-p-len")
		bs6 = testMarshalErr(&v6v1, h, t, "enc-map-v6-p")
		v6v2 = nil
		testUnmarshalErr(&v6v2, bs6, h, t, "dec-map-v6-p-nil")
		testDeepEqualErr(v6v1, v6v2, t, "equal-map-v6-p-nil")
		// ...
		if v == nil {
			v6v2 = nil
		} else {
			v6v2 = make(map[interface{}]uint16, len(v))
		} // reset map
		var v6v3, v6v4 typMapMapIntfUint16
		v6v3 = typMapMapIntfUint16(v6v1)
		v6v4 = typMapMapIntfUint16(v6v2)
		bs6 = testMarshalErr(v6v3, h, t, "enc-map-v6-custom")
		testUnmarshalErr(v6v4, bs6, h, t, "dec-map-v6-p-len")
		testDeepEqualErr(v6v3, v6v4, t, "equal-map-v6-p-len")
	}

	for _, v := range []map[interface{}]uint32{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 33}} {
		// fmt.Printf(">>>> running mammoth map v7: %v\n", v)
		var v7v1, v7v2 map[interface{}]uint32
		v7v1 = v
		bs7 := testMarshalErr(v7v1, h, t, "enc-map-v7")
		if v == nil {
			v7v2 = nil
		} else {
			v7v2 = make(map[interface{}]uint32, len(v))
		} // reset map
		testUnmarshalErr(v7v2, bs7, h, t, "dec-map-v7")
		testDeepEqualErr(v7v1, v7v2, t, "equal-map-v7")
		if v == nil {
			v7v2 = nil
		} else {
			v7v2 = make(map[interface{}]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v7v2), bs7, h, t, "dec-map-v7-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v7v1, v7v2, t, "equal-map-v7-noaddr")
		if v == nil {
			v7v2 = nil
		} else {
			v7v2 = make(map[interface{}]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v7v2, bs7, h, t, "dec-map-v7-p-len")
		testDeepEqualErr(v7v1, v7v2, t, "equal-map-v7-p-len")
		bs7 = testMarshalErr(&v7v1, h, t, "enc-map-v7-p")
		v7v2 = nil
		testUnmarshalErr(&v7v2, bs7, h, t, "dec-map-v7-p-nil")
		testDeepEqualErr(v7v1, v7v2, t, "equal-map-v7-p-nil")
		// ...
		if v == nil {
			v7v2 = nil
		} else {
			v7v2 = make(map[interface{}]uint32, len(v))
		} // reset map
		var v7v3, v7v4 typMapMapIntfUint32
		v7v3 = typMapMapIntfUint32(v7v1)
		v7v4 = typMapMapIntfUint32(v7v2)
		bs7 = testMarshalErr(v7v3, h, t, "enc-map-v7-custom")
		testUnmarshalErr(v7v4, bs7, h, t, "dec-map-v7-p-len")
		testDeepEqualErr(v7v3, v7v4, t, "equal-map-v7-p-len")
	}

	for _, v := range []map[interface{}]uint64{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 44}} {
		// fmt.Printf(">>>> running mammoth map v8: %v\n", v)
		var v8v1, v8v2 map[interface{}]uint64
		v8v1 = v
		bs8 := testMarshalErr(v8v1, h, t, "enc-map-v8")
		if v == nil {
			v8v2 = nil
		} else {
			v8v2 = make(map[interface{}]uint64, len(v))
		} // reset map
		testUnmarshalErr(v8v2, bs8, h, t, "dec-map-v8")
		testDeepEqualErr(v8v1, v8v2, t, "equal-map-v8")
		if v == nil {
			v8v2 = nil
		} else {
			v8v2 = make(map[interface{}]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v8v2), bs8, h, t, "dec-map-v8-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v8v1, v8v2, t, "equal-map-v8-noaddr")
		if v == nil {
			v8v2 = nil
		} else {
			v8v2 = make(map[interface{}]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v8v2, bs8, h, t, "dec-map-v8-p-len")
		testDeepEqualErr(v8v1, v8v2, t, "equal-map-v8-p-len")
		bs8 = testMarshalErr(&v8v1, h, t, "enc-map-v8-p")
		v8v2 = nil
		testUnmarshalErr(&v8v2, bs8, h, t, "dec-map-v8-p-nil")
		testDeepEqualErr(v8v1, v8v2, t, "equal-map-v8-p-nil")
		// ...
		if v == nil {
			v8v2 = nil
		} else {
			v8v2 = make(map[interface{}]uint64, len(v))
		} // reset map
		var v8v3, v8v4 typMapMapIntfUint64
		v8v3 = typMapMapIntfUint64(v8v1)
		v8v4 = typMapMapIntfUint64(v8v2)
		bs8 = testMarshalErr(v8v3, h, t, "enc-map-v8-custom")
		testUnmarshalErr(v8v4, bs8, h, t, "dec-map-v8-p-len")
		testDeepEqualErr(v8v3, v8v4, t, "equal-map-v8-p-len")
	}

	for _, v := range []map[interface{}]uintptr{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 33}} {
		// fmt.Printf(">>>> running mammoth map v9: %v\n", v)
		var v9v1, v9v2 map[interface{}]uintptr
		v9v1 = v
		bs9 := testMarshalErr(v9v1, h, t, "enc-map-v9")
		if v == nil {
			v9v2 = nil
		} else {
			v9v2 = make(map[interface{}]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v9v2, bs9, h, t, "dec-map-v9")
		testDeepEqualErr(v9v1, v9v2, t, "equal-map-v9")
		if v == nil {
			v9v2 = nil
		} else {
			v9v2 = make(map[interface{}]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v9v2), bs9, h, t, "dec-map-v9-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v9v1, v9v2, t, "equal-map-v9-noaddr")
		if v == nil {
			v9v2 = nil
		} else {
			v9v2 = make(map[interface{}]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v9v2, bs9, h, t, "dec-map-v9-p-len")
		testDeepEqualErr(v9v1, v9v2, t, "equal-map-v9-p-len")
		bs9 = testMarshalErr(&v9v1, h, t, "enc-map-v9-p")
		v9v2 = nil
		testUnmarshalErr(&v9v2, bs9, h, t, "dec-map-v9-p-nil")
		testDeepEqualErr(v9v1, v9v2, t, "equal-map-v9-p-nil")
		// ...
		if v == nil {
			v9v2 = nil
		} else {
			v9v2 = make(map[interface{}]uintptr, len(v))
		} // reset map
		var v9v3, v9v4 typMapMapIntfUintptr
		v9v3 = typMapMapIntfUintptr(v9v1)
		v9v4 = typMapMapIntfUintptr(v9v2)
		bs9 = testMarshalErr(v9v3, h, t, "enc-map-v9-custom")
		testUnmarshalErr(v9v4, bs9, h, t, "dec-map-v9-p-len")
		testDeepEqualErr(v9v3, v9v4, t, "equal-map-v9-p-len")
	}

	for _, v := range []map[interface{}]int{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 44}} {
		// fmt.Printf(">>>> running mammoth map v10: %v\n", v)
		var v10v1, v10v2 map[interface{}]int
		v10v1 = v
		bs10 := testMarshalErr(v10v1, h, t, "enc-map-v10")
		if v == nil {
			v10v2 = nil
		} else {
			v10v2 = make(map[interface{}]int, len(v))
		} // reset map
		testUnmarshalErr(v10v2, bs10, h, t, "dec-map-v10")
		testDeepEqualErr(v10v1, v10v2, t, "equal-map-v10")
		if v == nil {
			v10v2 = nil
		} else {
			v10v2 = make(map[interface{}]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v10v2), bs10, h, t, "dec-map-v10-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v10v1, v10v2, t, "equal-map-v10-noaddr")
		if v == nil {
			v10v2 = nil
		} else {
			v10v2 = make(map[interface{}]int, len(v))
		} // reset map
		testUnmarshalErr(&v10v2, bs10, h, t, "dec-map-v10-p-len")
		testDeepEqualErr(v10v1, v10v2, t, "equal-map-v10-p-len")
		bs10 = testMarshalErr(&v10v1, h, t, "enc-map-v10-p")
		v10v2 = nil
		testUnmarshalErr(&v10v2, bs10, h, t, "dec-map-v10-p-nil")
		testDeepEqualErr(v10v1, v10v2, t, "equal-map-v10-p-nil")
		// ...
		if v == nil {
			v10v2 = nil
		} else {
			v10v2 = make(map[interface{}]int, len(v))
		} // reset map
		var v10v3, v10v4 typMapMapIntfInt
		v10v3 = typMapMapIntfInt(v10v1)
		v10v4 = typMapMapIntfInt(v10v2)
		bs10 = testMarshalErr(v10v3, h, t, "enc-map-v10-custom")
		testUnmarshalErr(v10v4, bs10, h, t, "dec-map-v10-p-len")
		testDeepEqualErr(v10v3, v10v4, t, "equal-map-v10-p-len")
	}

	for _, v := range []map[interface{}]int8{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 33}} {
		// fmt.Printf(">>>> running mammoth map v11: %v\n", v)
		var v11v1, v11v2 map[interface{}]int8
		v11v1 = v
		bs11 := testMarshalErr(v11v1, h, t, "enc-map-v11")
		if v == nil {
			v11v2 = nil
		} else {
			v11v2 = make(map[interface{}]int8, len(v))
		} // reset map
		testUnmarshalErr(v11v2, bs11, h, t, "dec-map-v11")
		testDeepEqualErr(v11v1, v11v2, t, "equal-map-v11")
		if v == nil {
			v11v2 = nil
		} else {
			v11v2 = make(map[interface{}]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v11v2), bs11, h, t, "dec-map-v11-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v11v1, v11v2, t, "equal-map-v11-noaddr")
		if v == nil {
			v11v2 = nil
		} else {
			v11v2 = make(map[interface{}]int8, len(v))
		} // reset map
		testUnmarshalErr(&v11v2, bs11, h, t, "dec-map-v11-p-len")
		testDeepEqualErr(v11v1, v11v2, t, "equal-map-v11-p-len")
		bs11 = testMarshalErr(&v11v1, h, t, "enc-map-v11-p")
		v11v2 = nil
		testUnmarshalErr(&v11v2, bs11, h, t, "dec-map-v11-p-nil")
		testDeepEqualErr(v11v1, v11v2, t, "equal-map-v11-p-nil")
		// ...
		if v == nil {
			v11v2 = nil
		} else {
			v11v2 = make(map[interface{}]int8, len(v))
		} // reset map
		var v11v3, v11v4 typMapMapIntfInt8
		v11v3 = typMapMapIntfInt8(v11v1)
		v11v4 = typMapMapIntfInt8(v11v2)
		bs11 = testMarshalErr(v11v3, h, t, "enc-map-v11-custom")
		testUnmarshalErr(v11v4, bs11, h, t, "dec-map-v11-p-len")
		testDeepEqualErr(v11v3, v11v4, t, "equal-map-v11-p-len")
	}

	for _, v := range []map[interface{}]int16{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 44}} {
		// fmt.Printf(">>>> running mammoth map v12: %v\n", v)
		var v12v1, v12v2 map[interface{}]int16
		v12v1 = v
		bs12 := testMarshalErr(v12v1, h, t, "enc-map-v12")
		if v == nil {
			v12v2 = nil
		} else {
			v12v2 = make(map[interface{}]int16, len(v))
		} // reset map
		testUnmarshalErr(v12v2, bs12, h, t, "dec-map-v12")
		testDeepEqualErr(v12v1, v12v2, t, "equal-map-v12")
		if v == nil {
			v12v2 = nil
		} else {
			v12v2 = make(map[interface{}]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v12v2), bs12, h, t, "dec-map-v12-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v12v1, v12v2, t, "equal-map-v12-noaddr")
		if v == nil {
			v12v2 = nil
		} else {
			v12v2 = make(map[interface{}]int16, len(v))
		} // reset map
		testUnmarshalErr(&v12v2, bs12, h, t, "dec-map-v12-p-len")
		testDeepEqualErr(v12v1, v12v2, t, "equal-map-v12-p-len")
		bs12 = testMarshalErr(&v12v1, h, t, "enc-map-v12-p")
		v12v2 = nil
		testUnmarshalErr(&v12v2, bs12, h, t, "dec-map-v12-p-nil")
		testDeepEqualErr(v12v1, v12v2, t, "equal-map-v12-p-nil")
		// ...
		if v == nil {
			v12v2 = nil
		} else {
			v12v2 = make(map[interface{}]int16, len(v))
		} // reset map
		var v12v3, v12v4 typMapMapIntfInt16
		v12v3 = typMapMapIntfInt16(v12v1)
		v12v4 = typMapMapIntfInt16(v12v2)
		bs12 = testMarshalErr(v12v3, h, t, "enc-map-v12-custom")
		testUnmarshalErr(v12v4, bs12, h, t, "dec-map-v12-p-len")
		testDeepEqualErr(v12v3, v12v4, t, "equal-map-v12-p-len")
	}

	for _, v := range []map[interface{}]int32{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 33}} {
		// fmt.Printf(">>>> running mammoth map v13: %v\n", v)
		var v13v1, v13v2 map[interface{}]int32
		v13v1 = v
		bs13 := testMarshalErr(v13v1, h, t, "enc-map-v13")
		if v == nil {
			v13v2 = nil
		} else {
			v13v2 = make(map[interface{}]int32, len(v))
		} // reset map
		testUnmarshalErr(v13v2, bs13, h, t, "dec-map-v13")
		testDeepEqualErr(v13v1, v13v2, t, "equal-map-v13")
		if v == nil {
			v13v2 = nil
		} else {
			v13v2 = make(map[interface{}]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v13v2), bs13, h, t, "dec-map-v13-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v13v1, v13v2, t, "equal-map-v13-noaddr")
		if v == nil {
			v13v2 = nil
		} else {
			v13v2 = make(map[interface{}]int32, len(v))
		} // reset map
		testUnmarshalErr(&v13v2, bs13, h, t, "dec-map-v13-p-len")
		testDeepEqualErr(v13v1, v13v2, t, "equal-map-v13-p-len")
		bs13 = testMarshalErr(&v13v1, h, t, "enc-map-v13-p")
		v13v2 = nil
		testUnmarshalErr(&v13v2, bs13, h, t, "dec-map-v13-p-nil")
		testDeepEqualErr(v13v1, v13v2, t, "equal-map-v13-p-nil")
		// ...
		if v == nil {
			v13v2 = nil
		} else {
			v13v2 = make(map[interface{}]int32, len(v))
		} // reset map
		var v13v3, v13v4 typMapMapIntfInt32
		v13v3 = typMapMapIntfInt32(v13v1)
		v13v4 = typMapMapIntfInt32(v13v2)
		bs13 = testMarshalErr(v13v3, h, t, "enc-map-v13-custom")
		testUnmarshalErr(v13v4, bs13, h, t, "dec-map-v13-p-len")
		testDeepEqualErr(v13v3, v13v4, t, "equal-map-v13-p-len")
	}

	for _, v := range []map[interface{}]int64{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 44}} {
		// fmt.Printf(">>>> running mammoth map v14: %v\n", v)
		var v14v1, v14v2 map[interface{}]int64
		v14v1 = v
		bs14 := testMarshalErr(v14v1, h, t, "enc-map-v14")
		if v == nil {
			v14v2 = nil
		} else {
			v14v2 = make(map[interface{}]int64, len(v))
		} // reset map
		testUnmarshalErr(v14v2, bs14, h, t, "dec-map-v14")
		testDeepEqualErr(v14v1, v14v2, t, "equal-map-v14")
		if v == nil {
			v14v2 = nil
		} else {
			v14v2 = make(map[interface{}]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v14v2), bs14, h, t, "dec-map-v14-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v14v1, v14v2, t, "equal-map-v14-noaddr")
		if v == nil {
			v14v2 = nil
		} else {
			v14v2 = make(map[interface{}]int64, len(v))
		} // reset map
		testUnmarshalErr(&v14v2, bs14, h, t, "dec-map-v14-p-len")
		testDeepEqualErr(v14v1, v14v2, t, "equal-map-v14-p-len")
		bs14 = testMarshalErr(&v14v1, h, t, "enc-map-v14-p")
		v14v2 = nil
		testUnmarshalErr(&v14v2, bs14, h, t, "dec-map-v14-p-nil")
		testDeepEqualErr(v14v1, v14v2, t, "equal-map-v14-p-nil")
		// ...
		if v == nil {
			v14v2 = nil
		} else {
			v14v2 = make(map[interface{}]int64, len(v))
		} // reset map
		var v14v3, v14v4 typMapMapIntfInt64
		v14v3 = typMapMapIntfInt64(v14v1)
		v14v4 = typMapMapIntfInt64(v14v2)
		bs14 = testMarshalErr(v14v3, h, t, "enc-map-v14-custom")
		testUnmarshalErr(v14v4, bs14, h, t, "dec-map-v14-p-len")
		testDeepEqualErr(v14v3, v14v4, t, "equal-map-v14-p-len")
	}

	for _, v := range []map[interface{}]float32{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 22.2}} {
		// fmt.Printf(">>>> running mammoth map v15: %v\n", v)
		var v15v1, v15v2 map[interface{}]float32
		v15v1 = v
		bs15 := testMarshalErr(v15v1, h, t, "enc-map-v15")
		if v == nil {
			v15v2 = nil
		} else {
			v15v2 = make(map[interface{}]float32, len(v))
		} // reset map
		testUnmarshalErr(v15v2, bs15, h, t, "dec-map-v15")
		testDeepEqualErr(v15v1, v15v2, t, "equal-map-v15")
		if v == nil {
			v15v2 = nil
		} else {
			v15v2 = make(map[interface{}]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v15v2), bs15, h, t, "dec-map-v15-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v15v1, v15v2, t, "equal-map-v15-noaddr")
		if v == nil {
			v15v2 = nil
		} else {
			v15v2 = make(map[interface{}]float32, len(v))
		} // reset map
		testUnmarshalErr(&v15v2, bs15, h, t, "dec-map-v15-p-len")
		testDeepEqualErr(v15v1, v15v2, t, "equal-map-v15-p-len")
		bs15 = testMarshalErr(&v15v1, h, t, "enc-map-v15-p")
		v15v2 = nil
		testUnmarshalErr(&v15v2, bs15, h, t, "dec-map-v15-p-nil")
		testDeepEqualErr(v15v1, v15v2, t, "equal-map-v15-p-nil")
		// ...
		if v == nil {
			v15v2 = nil
		} else {
			v15v2 = make(map[interface{}]float32, len(v))
		} // reset map
		var v15v3, v15v4 typMapMapIntfFloat32
		v15v3 = typMapMapIntfFloat32(v15v1)
		v15v4 = typMapMapIntfFloat32(v15v2)
		bs15 = testMarshalErr(v15v3, h, t, "enc-map-v15-custom")
		testUnmarshalErr(v15v4, bs15, h, t, "dec-map-v15-p-len")
		testDeepEqualErr(v15v3, v15v4, t, "equal-map-v15-p-len")
	}

	for _, v := range []map[interface{}]float64{nil, {}, {"string-is-an-interface": 0, "string-is-an-interface-2": 11.1}} {
		// fmt.Printf(">>>> running mammoth map v16: %v\n", v)
		var v16v1, v16v2 map[interface{}]float64
		v16v1 = v
		bs16 := testMarshalErr(v16v1, h, t, "enc-map-v16")
		if v == nil {
			v16v2 = nil
		} else {
			v16v2 = make(map[interface{}]float64, len(v))
		} // reset map
		testUnmarshalErr(v16v2, bs16, h, t, "dec-map-v16")
		testDeepEqualErr(v16v1, v16v2, t, "equal-map-v16")
		if v == nil {
			v16v2 = nil
		} else {
			v16v2 = make(map[interface{}]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v16v2), bs16, h, t, "dec-map-v16-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v16v1, v16v2, t, "equal-map-v16-noaddr")
		if v == nil {
			v16v2 = nil
		} else {
			v16v2 = make(map[interface{}]float64, len(v))
		} // reset map
		testUnmarshalErr(&v16v2, bs16, h, t, "dec-map-v16-p-len")
		testDeepEqualErr(v16v1, v16v2, t, "equal-map-v16-p-len")
		bs16 = testMarshalErr(&v16v1, h, t, "enc-map-v16-p")
		v16v2 = nil
		testUnmarshalErr(&v16v2, bs16, h, t, "dec-map-v16-p-nil")
		testDeepEqualErr(v16v1, v16v2, t, "equal-map-v16-p-nil")
		// ...
		if v == nil {
			v16v2 = nil
		} else {
			v16v2 = make(map[interface{}]float64, len(v))
		} // reset map
		var v16v3, v16v4 typMapMapIntfFloat64
		v16v3 = typMapMapIntfFloat64(v16v1)
		v16v4 = typMapMapIntfFloat64(v16v2)
		bs16 = testMarshalErr(v16v3, h, t, "enc-map-v16-custom")
		testUnmarshalErr(v16v4, bs16, h, t, "dec-map-v16-p-len")
		testDeepEqualErr(v16v3, v16v4, t, "equal-map-v16-p-len")
	}

	for _, v := range []map[interface{}]bool{nil, {}, {"string-is-an-interface": false, "string-is-an-interface-2": true}} {
		// fmt.Printf(">>>> running mammoth map v17: %v\n", v)
		var v17v1, v17v2 map[interface{}]bool
		v17v1 = v
		bs17 := testMarshalErr(v17v1, h, t, "enc-map-v17")
		if v == nil {
			v17v2 = nil
		} else {
			v17v2 = make(map[interface{}]bool, len(v))
		} // reset map
		testUnmarshalErr(v17v2, bs17, h, t, "dec-map-v17")
		testDeepEqualErr(v17v1, v17v2, t, "equal-map-v17")
		if v == nil {
			v17v2 = nil
		} else {
			v17v2 = make(map[interface{}]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v17v2), bs17, h, t, "dec-map-v17-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v17v1, v17v2, t, "equal-map-v17-noaddr")
		if v == nil {
			v17v2 = nil
		} else {
			v17v2 = make(map[interface{}]bool, len(v))
		} // reset map
		testUnmarshalErr(&v17v2, bs17, h, t, "dec-map-v17-p-len")
		testDeepEqualErr(v17v1, v17v2, t, "equal-map-v17-p-len")
		bs17 = testMarshalErr(&v17v1, h, t, "enc-map-v17-p")
		v17v2 = nil
		testUnmarshalErr(&v17v2, bs17, h, t, "dec-map-v17-p-nil")
		testDeepEqualErr(v17v1, v17v2, t, "equal-map-v17-p-nil")
		// ...
		if v == nil {
			v17v2 = nil
		} else {
			v17v2 = make(map[interface{}]bool, len(v))
		} // reset map
		var v17v3, v17v4 typMapMapIntfBool
		v17v3 = typMapMapIntfBool(v17v1)
		v17v4 = typMapMapIntfBool(v17v2)
		bs17 = testMarshalErr(v17v3, h, t, "enc-map-v17-custom")
		testUnmarshalErr(v17v4, bs17, h, t, "dec-map-v17-p-len")
		testDeepEqualErr(v17v3, v17v4, t, "equal-map-v17-p-len")
	}

	for _, v := range []map[string]interface{}{nil, {}, {"some-string": nil, "some-string-2": "string-is-an-interface"}} {
		// fmt.Printf(">>>> running mammoth map v20: %v\n", v)
		var v20v1, v20v2 map[string]interface{}
		v20v1 = v
		bs20 := testMarshalErr(v20v1, h, t, "enc-map-v20")
		if v == nil {
			v20v2 = nil
		} else {
			v20v2 = make(map[string]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v20v2, bs20, h, t, "dec-map-v20")
		testDeepEqualErr(v20v1, v20v2, t, "equal-map-v20")
		if v == nil {
			v20v2 = nil
		} else {
			v20v2 = make(map[string]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v20v2), bs20, h, t, "dec-map-v20-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v20v1, v20v2, t, "equal-map-v20-noaddr")
		if v == nil {
			v20v2 = nil
		} else {
			v20v2 = make(map[string]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v20v2, bs20, h, t, "dec-map-v20-p-len")
		testDeepEqualErr(v20v1, v20v2, t, "equal-map-v20-p-len")
		bs20 = testMarshalErr(&v20v1, h, t, "enc-map-v20-p")
		v20v2 = nil
		testUnmarshalErr(&v20v2, bs20, h, t, "dec-map-v20-p-nil")
		testDeepEqualErr(v20v1, v20v2, t, "equal-map-v20-p-nil")
		// ...
		if v == nil {
			v20v2 = nil
		} else {
			v20v2 = make(map[string]interface{}, len(v))
		} // reset map
		var v20v3, v20v4 typMapMapStringIntf
		v20v3 = typMapMapStringIntf(v20v1)
		v20v4 = typMapMapStringIntf(v20v2)
		bs20 = testMarshalErr(v20v3, h, t, "enc-map-v20-custom")
		testUnmarshalErr(v20v4, bs20, h, t, "dec-map-v20-p-len")
		testDeepEqualErr(v20v3, v20v4, t, "equal-map-v20-p-len")
	}

	for _, v := range []map[string]string{nil, {}, {"some-string": "", "some-string-2": "some-string"}} {
		// fmt.Printf(">>>> running mammoth map v21: %v\n", v)
		var v21v1, v21v2 map[string]string
		v21v1 = v
		bs21 := testMarshalErr(v21v1, h, t, "enc-map-v21")
		if v == nil {
			v21v2 = nil
		} else {
			v21v2 = make(map[string]string, len(v))
		} // reset map
		testUnmarshalErr(v21v2, bs21, h, t, "dec-map-v21")
		testDeepEqualErr(v21v1, v21v2, t, "equal-map-v21")
		if v == nil {
			v21v2 = nil
		} else {
			v21v2 = make(map[string]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v21v2), bs21, h, t, "dec-map-v21-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v21v1, v21v2, t, "equal-map-v21-noaddr")
		if v == nil {
			v21v2 = nil
		} else {
			v21v2 = make(map[string]string, len(v))
		} // reset map
		testUnmarshalErr(&v21v2, bs21, h, t, "dec-map-v21-p-len")
		testDeepEqualErr(v21v1, v21v2, t, "equal-map-v21-p-len")
		bs21 = testMarshalErr(&v21v1, h, t, "enc-map-v21-p")
		v21v2 = nil
		testUnmarshalErr(&v21v2, bs21, h, t, "dec-map-v21-p-nil")
		testDeepEqualErr(v21v1, v21v2, t, "equal-map-v21-p-nil")
		// ...
		if v == nil {
			v21v2 = nil
		} else {
			v21v2 = make(map[string]string, len(v))
		} // reset map
		var v21v3, v21v4 typMapMapStringString
		v21v3 = typMapMapStringString(v21v1)
		v21v4 = typMapMapStringString(v21v2)
		bs21 = testMarshalErr(v21v3, h, t, "enc-map-v21-custom")
		testUnmarshalErr(v21v4, bs21, h, t, "dec-map-v21-p-len")
		testDeepEqualErr(v21v3, v21v4, t, "equal-map-v21-p-len")
	}

	for _, v := range []map[string]uint{nil, {}, {"some-string-2": 0, "some-string": 33}} {
		// fmt.Printf(">>>> running mammoth map v22: %v\n", v)
		var v22v1, v22v2 map[string]uint
		v22v1 = v
		bs22 := testMarshalErr(v22v1, h, t, "enc-map-v22")
		if v == nil {
			v22v2 = nil
		} else {
			v22v2 = make(map[string]uint, len(v))
		} // reset map
		testUnmarshalErr(v22v2, bs22, h, t, "dec-map-v22")
		testDeepEqualErr(v22v1, v22v2, t, "equal-map-v22")
		if v == nil {
			v22v2 = nil
		} else {
			v22v2 = make(map[string]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v22v2), bs22, h, t, "dec-map-v22-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v22v1, v22v2, t, "equal-map-v22-noaddr")
		if v == nil {
			v22v2 = nil
		} else {
			v22v2 = make(map[string]uint, len(v))
		} // reset map
		testUnmarshalErr(&v22v2, bs22, h, t, "dec-map-v22-p-len")
		testDeepEqualErr(v22v1, v22v2, t, "equal-map-v22-p-len")
		bs22 = testMarshalErr(&v22v1, h, t, "enc-map-v22-p")
		v22v2 = nil
		testUnmarshalErr(&v22v2, bs22, h, t, "dec-map-v22-p-nil")
		testDeepEqualErr(v22v1, v22v2, t, "equal-map-v22-p-nil")
		// ...
		if v == nil {
			v22v2 = nil
		} else {
			v22v2 = make(map[string]uint, len(v))
		} // reset map
		var v22v3, v22v4 typMapMapStringUint
		v22v3 = typMapMapStringUint(v22v1)
		v22v4 = typMapMapStringUint(v22v2)
		bs22 = testMarshalErr(v22v3, h, t, "enc-map-v22-custom")
		testUnmarshalErr(v22v4, bs22, h, t, "dec-map-v22-p-len")
		testDeepEqualErr(v22v3, v22v4, t, "equal-map-v22-p-len")
	}

	for _, v := range []map[string]uint8{nil, {}, {"some-string-2": 0, "some-string": 44}} {
		// fmt.Printf(">>>> running mammoth map v23: %v\n", v)
		var v23v1, v23v2 map[string]uint8
		v23v1 = v
		bs23 := testMarshalErr(v23v1, h, t, "enc-map-v23")
		if v == nil {
			v23v2 = nil
		} else {
			v23v2 = make(map[string]uint8, len(v))
		} // reset map
		testUnmarshalErr(v23v2, bs23, h, t, "dec-map-v23")
		testDeepEqualErr(v23v1, v23v2, t, "equal-map-v23")
		if v == nil {
			v23v2 = nil
		} else {
			v23v2 = make(map[string]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v23v2), bs23, h, t, "dec-map-v23-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v23v1, v23v2, t, "equal-map-v23-noaddr")
		if v == nil {
			v23v2 = nil
		} else {
			v23v2 = make(map[string]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v23v2, bs23, h, t, "dec-map-v23-p-len")
		testDeepEqualErr(v23v1, v23v2, t, "equal-map-v23-p-len")
		bs23 = testMarshalErr(&v23v1, h, t, "enc-map-v23-p")
		v23v2 = nil
		testUnmarshalErr(&v23v2, bs23, h, t, "dec-map-v23-p-nil")
		testDeepEqualErr(v23v1, v23v2, t, "equal-map-v23-p-nil")
		// ...
		if v == nil {
			v23v2 = nil
		} else {
			v23v2 = make(map[string]uint8, len(v))
		} // reset map
		var v23v3, v23v4 typMapMapStringUint8
		v23v3 = typMapMapStringUint8(v23v1)
		v23v4 = typMapMapStringUint8(v23v2)
		bs23 = testMarshalErr(v23v3, h, t, "enc-map-v23-custom")
		testUnmarshalErr(v23v4, bs23, h, t, "dec-map-v23-p-len")
		testDeepEqualErr(v23v3, v23v4, t, "equal-map-v23-p-len")
	}

	for _, v := range []map[string]uint16{nil, {}, {"some-string-2": 0, "some-string": 33}} {
		// fmt.Printf(">>>> running mammoth map v24: %v\n", v)
		var v24v1, v24v2 map[string]uint16
		v24v1 = v
		bs24 := testMarshalErr(v24v1, h, t, "enc-map-v24")
		if v == nil {
			v24v2 = nil
		} else {
			v24v2 = make(map[string]uint16, len(v))
		} // reset map
		testUnmarshalErr(v24v2, bs24, h, t, "dec-map-v24")
		testDeepEqualErr(v24v1, v24v2, t, "equal-map-v24")
		if v == nil {
			v24v2 = nil
		} else {
			v24v2 = make(map[string]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v24v2), bs24, h, t, "dec-map-v24-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v24v1, v24v2, t, "equal-map-v24-noaddr")
		if v == nil {
			v24v2 = nil
		} else {
			v24v2 = make(map[string]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v24v2, bs24, h, t, "dec-map-v24-p-len")
		testDeepEqualErr(v24v1, v24v2, t, "equal-map-v24-p-len")
		bs24 = testMarshalErr(&v24v1, h, t, "enc-map-v24-p")
		v24v2 = nil
		testUnmarshalErr(&v24v2, bs24, h, t, "dec-map-v24-p-nil")
		testDeepEqualErr(v24v1, v24v2, t, "equal-map-v24-p-nil")
		// ...
		if v == nil {
			v24v2 = nil
		} else {
			v24v2 = make(map[string]uint16, len(v))
		} // reset map
		var v24v3, v24v4 typMapMapStringUint16
		v24v3 = typMapMapStringUint16(v24v1)
		v24v4 = typMapMapStringUint16(v24v2)
		bs24 = testMarshalErr(v24v3, h, t, "enc-map-v24-custom")
		testUnmarshalErr(v24v4, bs24, h, t, "dec-map-v24-p-len")
		testDeepEqualErr(v24v3, v24v4, t, "equal-map-v24-p-len")
	}

	for _, v := range []map[string]uint32{nil, {}, {"some-string-2": 0, "some-string": 44}} {
		// fmt.Printf(">>>> running mammoth map v25: %v\n", v)
		var v25v1, v25v2 map[string]uint32
		v25v1 = v
		bs25 := testMarshalErr(v25v1, h, t, "enc-map-v25")
		if v == nil {
			v25v2 = nil
		} else {
			v25v2 = make(map[string]uint32, len(v))
		} // reset map
		testUnmarshalErr(v25v2, bs25, h, t, "dec-map-v25")
		testDeepEqualErr(v25v1, v25v2, t, "equal-map-v25")
		if v == nil {
			v25v2 = nil
		} else {
			v25v2 = make(map[string]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v25v2), bs25, h, t, "dec-map-v25-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v25v1, v25v2, t, "equal-map-v25-noaddr")
		if v == nil {
			v25v2 = nil
		} else {
			v25v2 = make(map[string]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v25v2, bs25, h, t, "dec-map-v25-p-len")
		testDeepEqualErr(v25v1, v25v2, t, "equal-map-v25-p-len")
		bs25 = testMarshalErr(&v25v1, h, t, "enc-map-v25-p")
		v25v2 = nil
		testUnmarshalErr(&v25v2, bs25, h, t, "dec-map-v25-p-nil")
		testDeepEqualErr(v25v1, v25v2, t, "equal-map-v25-p-nil")
		// ...
		if v == nil {
			v25v2 = nil
		} else {
			v25v2 = make(map[string]uint32, len(v))
		} // reset map
		var v25v3, v25v4 typMapMapStringUint32
		v25v3 = typMapMapStringUint32(v25v1)
		v25v4 = typMapMapStringUint32(v25v2)
		bs25 = testMarshalErr(v25v3, h, t, "enc-map-v25-custom")
		testUnmarshalErr(v25v4, bs25, h, t, "dec-map-v25-p-len")
		testDeepEqualErr(v25v3, v25v4, t, "equal-map-v25-p-len")
	}

	for _, v := range []map[string]uint64{nil, {}, {"some-string-2": 0, "some-string": 33}} {
		// fmt.Printf(">>>> running mammoth map v26: %v\n", v)
		var v26v1, v26v2 map[string]uint64
		v26v1 = v
		bs26 := testMarshalErr(v26v1, h, t, "enc-map-v26")
		if v == nil {
			v26v2 = nil
		} else {
			v26v2 = make(map[string]uint64, len(v))
		} // reset map
		testUnmarshalErr(v26v2, bs26, h, t, "dec-map-v26")
		testDeepEqualErr(v26v1, v26v2, t, "equal-map-v26")
		if v == nil {
			v26v2 = nil
		} else {
			v26v2 = make(map[string]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v26v2), bs26, h, t, "dec-map-v26-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v26v1, v26v2, t, "equal-map-v26-noaddr")
		if v == nil {
			v26v2 = nil
		} else {
			v26v2 = make(map[string]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v26v2, bs26, h, t, "dec-map-v26-p-len")
		testDeepEqualErr(v26v1, v26v2, t, "equal-map-v26-p-len")
		bs26 = testMarshalErr(&v26v1, h, t, "enc-map-v26-p")
		v26v2 = nil
		testUnmarshalErr(&v26v2, bs26, h, t, "dec-map-v26-p-nil")
		testDeepEqualErr(v26v1, v26v2, t, "equal-map-v26-p-nil")
		// ...
		if v == nil {
			v26v2 = nil
		} else {
			v26v2 = make(map[string]uint64, len(v))
		} // reset map
		var v26v3, v26v4 typMapMapStringUint64
		v26v3 = typMapMapStringUint64(v26v1)
		v26v4 = typMapMapStringUint64(v26v2)
		bs26 = testMarshalErr(v26v3, h, t, "enc-map-v26-custom")
		testUnmarshalErr(v26v4, bs26, h, t, "dec-map-v26-p-len")
		testDeepEqualErr(v26v3, v26v4, t, "equal-map-v26-p-len")
	}

	for _, v := range []map[string]uintptr{nil, {}, {"some-string-2": 0, "some-string": 44}} {
		// fmt.Printf(">>>> running mammoth map v27: %v\n", v)
		var v27v1, v27v2 map[string]uintptr
		v27v1 = v
		bs27 := testMarshalErr(v27v1, h, t, "enc-map-v27")
		if v == nil {
			v27v2 = nil
		} else {
			v27v2 = make(map[string]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v27v2, bs27, h, t, "dec-map-v27")
		testDeepEqualErr(v27v1, v27v2, t, "equal-map-v27")
		if v == nil {
			v27v2 = nil
		} else {
			v27v2 = make(map[string]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v27v2), bs27, h, t, "dec-map-v27-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v27v1, v27v2, t, "equal-map-v27-noaddr")
		if v == nil {
			v27v2 = nil
		} else {
			v27v2 = make(map[string]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v27v2, bs27, h, t, "dec-map-v27-p-len")
		testDeepEqualErr(v27v1, v27v2, t, "equal-map-v27-p-len")
		bs27 = testMarshalErr(&v27v1, h, t, "enc-map-v27-p")
		v27v2 = nil
		testUnmarshalErr(&v27v2, bs27, h, t, "dec-map-v27-p-nil")
		testDeepEqualErr(v27v1, v27v2, t, "equal-map-v27-p-nil")
		// ...
		if v == nil {
			v27v2 = nil
		} else {
			v27v2 = make(map[string]uintptr, len(v))
		} // reset map
		var v27v3, v27v4 typMapMapStringUintptr
		v27v3 = typMapMapStringUintptr(v27v1)
		v27v4 = typMapMapStringUintptr(v27v2)
		bs27 = testMarshalErr(v27v3, h, t, "enc-map-v27-custom")
		testUnmarshalErr(v27v4, bs27, h, t, "dec-map-v27-p-len")
		testDeepEqualErr(v27v3, v27v4, t, "equal-map-v27-p-len")
	}

	for _, v := range []map[string]int{nil, {}, {"some-string-2": 0, "some-string": 33}} {
		// fmt.Printf(">>>> running mammoth map v28: %v\n", v)
		var v28v1, v28v2 map[string]int
		v28v1 = v
		bs28 := testMarshalErr(v28v1, h, t, "enc-map-v28")
		if v == nil {
			v28v2 = nil
		} else {
			v28v2 = make(map[string]int, len(v))
		} // reset map
		testUnmarshalErr(v28v2, bs28, h, t, "dec-map-v28")
		testDeepEqualErr(v28v1, v28v2, t, "equal-map-v28")
		if v == nil {
			v28v2 = nil
		} else {
			v28v2 = make(map[string]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v28v2), bs28, h, t, "dec-map-v28-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v28v1, v28v2, t, "equal-map-v28-noaddr")
		if v == nil {
			v28v2 = nil
		} else {
			v28v2 = make(map[string]int, len(v))
		} // reset map
		testUnmarshalErr(&v28v2, bs28, h, t, "dec-map-v28-p-len")
		testDeepEqualErr(v28v1, v28v2, t, "equal-map-v28-p-len")
		bs28 = testMarshalErr(&v28v1, h, t, "enc-map-v28-p")
		v28v2 = nil
		testUnmarshalErr(&v28v2, bs28, h, t, "dec-map-v28-p-nil")
		testDeepEqualErr(v28v1, v28v2, t, "equal-map-v28-p-nil")
		// ...
		if v == nil {
			v28v2 = nil
		} else {
			v28v2 = make(map[string]int, len(v))
		} // reset map
		var v28v3, v28v4 typMapMapStringInt
		v28v3 = typMapMapStringInt(v28v1)
		v28v4 = typMapMapStringInt(v28v2)
		bs28 = testMarshalErr(v28v3, h, t, "enc-map-v28-custom")
		testUnmarshalErr(v28v4, bs28, h, t, "dec-map-v28-p-len")
		testDeepEqualErr(v28v3, v28v4, t, "equal-map-v28-p-len")
	}

	for _, v := range []map[string]int8{nil, {}, {"some-string-2": 0, "some-string": 44}} {
		// fmt.Printf(">>>> running mammoth map v29: %v\n", v)
		var v29v1, v29v2 map[string]int8
		v29v1 = v
		bs29 := testMarshalErr(v29v1, h, t, "enc-map-v29")
		if v == nil {
			v29v2 = nil
		} else {
			v29v2 = make(map[string]int8, len(v))
		} // reset map
		testUnmarshalErr(v29v2, bs29, h, t, "dec-map-v29")
		testDeepEqualErr(v29v1, v29v2, t, "equal-map-v29")
		if v == nil {
			v29v2 = nil
		} else {
			v29v2 = make(map[string]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v29v2), bs29, h, t, "dec-map-v29-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v29v1, v29v2, t, "equal-map-v29-noaddr")
		if v == nil {
			v29v2 = nil
		} else {
			v29v2 = make(map[string]int8, len(v))
		} // reset map
		testUnmarshalErr(&v29v2, bs29, h, t, "dec-map-v29-p-len")
		testDeepEqualErr(v29v1, v29v2, t, "equal-map-v29-p-len")
		bs29 = testMarshalErr(&v29v1, h, t, "enc-map-v29-p")
		v29v2 = nil
		testUnmarshalErr(&v29v2, bs29, h, t, "dec-map-v29-p-nil")
		testDeepEqualErr(v29v1, v29v2, t, "equal-map-v29-p-nil")
		// ...
		if v == nil {
			v29v2 = nil
		} else {
			v29v2 = make(map[string]int8, len(v))
		} // reset map
		var v29v3, v29v4 typMapMapStringInt8
		v29v3 = typMapMapStringInt8(v29v1)
		v29v4 = typMapMapStringInt8(v29v2)
		bs29 = testMarshalErr(v29v3, h, t, "enc-map-v29-custom")
		testUnmarshalErr(v29v4, bs29, h, t, "dec-map-v29-p-len")
		testDeepEqualErr(v29v3, v29v4, t, "equal-map-v29-p-len")
	}

	for _, v := range []map[string]int16{nil, {}, {"some-string-2": 0, "some-string": 33}} {
		// fmt.Printf(">>>> running mammoth map v30: %v\n", v)
		var v30v1, v30v2 map[string]int16
		v30v1 = v
		bs30 := testMarshalErr(v30v1, h, t, "enc-map-v30")
		if v == nil {
			v30v2 = nil
		} else {
			v30v2 = make(map[string]int16, len(v))
		} // reset map
		testUnmarshalErr(v30v2, bs30, h, t, "dec-map-v30")
		testDeepEqualErr(v30v1, v30v2, t, "equal-map-v30")
		if v == nil {
			v30v2 = nil
		} else {
			v30v2 = make(map[string]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v30v2), bs30, h, t, "dec-map-v30-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v30v1, v30v2, t, "equal-map-v30-noaddr")
		if v == nil {
			v30v2 = nil
		} else {
			v30v2 = make(map[string]int16, len(v))
		} // reset map
		testUnmarshalErr(&v30v2, bs30, h, t, "dec-map-v30-p-len")
		testDeepEqualErr(v30v1, v30v2, t, "equal-map-v30-p-len")
		bs30 = testMarshalErr(&v30v1, h, t, "enc-map-v30-p")
		v30v2 = nil
		testUnmarshalErr(&v30v2, bs30, h, t, "dec-map-v30-p-nil")
		testDeepEqualErr(v30v1, v30v2, t, "equal-map-v30-p-nil")
		// ...
		if v == nil {
			v30v2 = nil
		} else {
			v30v2 = make(map[string]int16, len(v))
		} // reset map
		var v30v3, v30v4 typMapMapStringInt16
		v30v3 = typMapMapStringInt16(v30v1)
		v30v4 = typMapMapStringInt16(v30v2)
		bs30 = testMarshalErr(v30v3, h, t, "enc-map-v30-custom")
		testUnmarshalErr(v30v4, bs30, h, t, "dec-map-v30-p-len")
		testDeepEqualErr(v30v3, v30v4, t, "equal-map-v30-p-len")
	}

	for _, v := range []map[string]int32{nil, {}, {"some-string-2": 0, "some-string": 44}} {
		// fmt.Printf(">>>> running mammoth map v31: %v\n", v)
		var v31v1, v31v2 map[string]int32
		v31v1 = v
		bs31 := testMarshalErr(v31v1, h, t, "enc-map-v31")
		if v == nil {
			v31v2 = nil
		} else {
			v31v2 = make(map[string]int32, len(v))
		} // reset map
		testUnmarshalErr(v31v2, bs31, h, t, "dec-map-v31")
		testDeepEqualErr(v31v1, v31v2, t, "equal-map-v31")
		if v == nil {
			v31v2 = nil
		} else {
			v31v2 = make(map[string]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v31v2), bs31, h, t, "dec-map-v31-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v31v1, v31v2, t, "equal-map-v31-noaddr")
		if v == nil {
			v31v2 = nil
		} else {
			v31v2 = make(map[string]int32, len(v))
		} // reset map
		testUnmarshalErr(&v31v2, bs31, h, t, "dec-map-v31-p-len")
		testDeepEqualErr(v31v1, v31v2, t, "equal-map-v31-p-len")
		bs31 = testMarshalErr(&v31v1, h, t, "enc-map-v31-p")
		v31v2 = nil
		testUnmarshalErr(&v31v2, bs31, h, t, "dec-map-v31-p-nil")
		testDeepEqualErr(v31v1, v31v2, t, "equal-map-v31-p-nil")
		// ...
		if v == nil {
			v31v2 = nil
		} else {
			v31v2 = make(map[string]int32, len(v))
		} // reset map
		var v31v3, v31v4 typMapMapStringInt32
		v31v3 = typMapMapStringInt32(v31v1)
		v31v4 = typMapMapStringInt32(v31v2)
		bs31 = testMarshalErr(v31v3, h, t, "enc-map-v31-custom")
		testUnmarshalErr(v31v4, bs31, h, t, "dec-map-v31-p-len")
		testDeepEqualErr(v31v3, v31v4, t, "equal-map-v31-p-len")
	}

	for _, v := range []map[string]int64{nil, {}, {"some-string-2": 0, "some-string": 33}} {
		// fmt.Printf(">>>> running mammoth map v32: %v\n", v)
		var v32v1, v32v2 map[string]int64
		v32v1 = v
		bs32 := testMarshalErr(v32v1, h, t, "enc-map-v32")
		if v == nil {
			v32v2 = nil
		} else {
			v32v2 = make(map[string]int64, len(v))
		} // reset map
		testUnmarshalErr(v32v2, bs32, h, t, "dec-map-v32")
		testDeepEqualErr(v32v1, v32v2, t, "equal-map-v32")
		if v == nil {
			v32v2 = nil
		} else {
			v32v2 = make(map[string]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v32v2), bs32, h, t, "dec-map-v32-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v32v1, v32v2, t, "equal-map-v32-noaddr")
		if v == nil {
			v32v2 = nil
		} else {
			v32v2 = make(map[string]int64, len(v))
		} // reset map
		testUnmarshalErr(&v32v2, bs32, h, t, "dec-map-v32-p-len")
		testDeepEqualErr(v32v1, v32v2, t, "equal-map-v32-p-len")
		bs32 = testMarshalErr(&v32v1, h, t, "enc-map-v32-p")
		v32v2 = nil
		testUnmarshalErr(&v32v2, bs32, h, t, "dec-map-v32-p-nil")
		testDeepEqualErr(v32v1, v32v2, t, "equal-map-v32-p-nil")
		// ...
		if v == nil {
			v32v2 = nil
		} else {
			v32v2 = make(map[string]int64, len(v))
		} // reset map
		var v32v3, v32v4 typMapMapStringInt64
		v32v3 = typMapMapStringInt64(v32v1)
		v32v4 = typMapMapStringInt64(v32v2)
		bs32 = testMarshalErr(v32v3, h, t, "enc-map-v32-custom")
		testUnmarshalErr(v32v4, bs32, h, t, "dec-map-v32-p-len")
		testDeepEqualErr(v32v3, v32v4, t, "equal-map-v32-p-len")
	}

	for _, v := range []map[string]float32{nil, {}, {"some-string-2": 0, "some-string": 22.2}} {
		// fmt.Printf(">>>> running mammoth map v33: %v\n", v)
		var v33v1, v33v2 map[string]float32
		v33v1 = v
		bs33 := testMarshalErr(v33v1, h, t, "enc-map-v33")
		if v == nil {
			v33v2 = nil
		} else {
			v33v2 = make(map[string]float32, len(v))
		} // reset map
		testUnmarshalErr(v33v2, bs33, h, t, "dec-map-v33")
		testDeepEqualErr(v33v1, v33v2, t, "equal-map-v33")
		if v == nil {
			v33v2 = nil
		} else {
			v33v2 = make(map[string]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v33v2), bs33, h, t, "dec-map-v33-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v33v1, v33v2, t, "equal-map-v33-noaddr")
		if v == nil {
			v33v2 = nil
		} else {
			v33v2 = make(map[string]float32, len(v))
		} // reset map
		testUnmarshalErr(&v33v2, bs33, h, t, "dec-map-v33-p-len")
		testDeepEqualErr(v33v1, v33v2, t, "equal-map-v33-p-len")
		bs33 = testMarshalErr(&v33v1, h, t, "enc-map-v33-p")
		v33v2 = nil
		testUnmarshalErr(&v33v2, bs33, h, t, "dec-map-v33-p-nil")
		testDeepEqualErr(v33v1, v33v2, t, "equal-map-v33-p-nil")
		// ...
		if v == nil {
			v33v2 = nil
		} else {
			v33v2 = make(map[string]float32, len(v))
		} // reset map
		var v33v3, v33v4 typMapMapStringFloat32
		v33v3 = typMapMapStringFloat32(v33v1)
		v33v4 = typMapMapStringFloat32(v33v2)
		bs33 = testMarshalErr(v33v3, h, t, "enc-map-v33-custom")
		testUnmarshalErr(v33v4, bs33, h, t, "dec-map-v33-p-len")
		testDeepEqualErr(v33v3, v33v4, t, "equal-map-v33-p-len")
	}

	for _, v := range []map[string]float64{nil, {}, {"some-string-2": 0, "some-string": 11.1}} {
		// fmt.Printf(">>>> running mammoth map v34: %v\n", v)
		var v34v1, v34v2 map[string]float64
		v34v1 = v
		bs34 := testMarshalErr(v34v1, h, t, "enc-map-v34")
		if v == nil {
			v34v2 = nil
		} else {
			v34v2 = make(map[string]float64, len(v))
		} // reset map
		testUnmarshalErr(v34v2, bs34, h, t, "dec-map-v34")
		testDeepEqualErr(v34v1, v34v2, t, "equal-map-v34")
		if v == nil {
			v34v2 = nil
		} else {
			v34v2 = make(map[string]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v34v2), bs34, h, t, "dec-map-v34-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v34v1, v34v2, t, "equal-map-v34-noaddr")
		if v == nil {
			v34v2 = nil
		} else {
			v34v2 = make(map[string]float64, len(v))
		} // reset map
		testUnmarshalErr(&v34v2, bs34, h, t, "dec-map-v34-p-len")
		testDeepEqualErr(v34v1, v34v2, t, "equal-map-v34-p-len")
		bs34 = testMarshalErr(&v34v1, h, t, "enc-map-v34-p")
		v34v2 = nil
		testUnmarshalErr(&v34v2, bs34, h, t, "dec-map-v34-p-nil")
		testDeepEqualErr(v34v1, v34v2, t, "equal-map-v34-p-nil")
		// ...
		if v == nil {
			v34v2 = nil
		} else {
			v34v2 = make(map[string]float64, len(v))
		} // reset map
		var v34v3, v34v4 typMapMapStringFloat64
		v34v3 = typMapMapStringFloat64(v34v1)
		v34v4 = typMapMapStringFloat64(v34v2)
		bs34 = testMarshalErr(v34v3, h, t, "enc-map-v34-custom")
		testUnmarshalErr(v34v4, bs34, h, t, "dec-map-v34-p-len")
		testDeepEqualErr(v34v3, v34v4, t, "equal-map-v34-p-len")
	}

	for _, v := range []map[string]bool{nil, {}, {"some-string-2": false, "some-string": true}} {
		// fmt.Printf(">>>> running mammoth map v35: %v\n", v)
		var v35v1, v35v2 map[string]bool
		v35v1 = v
		bs35 := testMarshalErr(v35v1, h, t, "enc-map-v35")
		if v == nil {
			v35v2 = nil
		} else {
			v35v2 = make(map[string]bool, len(v))
		} // reset map
		testUnmarshalErr(v35v2, bs35, h, t, "dec-map-v35")
		testDeepEqualErr(v35v1, v35v2, t, "equal-map-v35")
		if v == nil {
			v35v2 = nil
		} else {
			v35v2 = make(map[string]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v35v2), bs35, h, t, "dec-map-v35-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v35v1, v35v2, t, "equal-map-v35-noaddr")
		if v == nil {
			v35v2 = nil
		} else {
			v35v2 = make(map[string]bool, len(v))
		} // reset map
		testUnmarshalErr(&v35v2, bs35, h, t, "dec-map-v35-p-len")
		testDeepEqualErr(v35v1, v35v2, t, "equal-map-v35-p-len")
		bs35 = testMarshalErr(&v35v1, h, t, "enc-map-v35-p")
		v35v2 = nil
		testUnmarshalErr(&v35v2, bs35, h, t, "dec-map-v35-p-nil")
		testDeepEqualErr(v35v1, v35v2, t, "equal-map-v35-p-nil")
		// ...
		if v == nil {
			v35v2 = nil
		} else {
			v35v2 = make(map[string]bool, len(v))
		} // reset map
		var v35v3, v35v4 typMapMapStringBool
		v35v3 = typMapMapStringBool(v35v1)
		v35v4 = typMapMapStringBool(v35v2)
		bs35 = testMarshalErr(v35v3, h, t, "enc-map-v35-custom")
		testUnmarshalErr(v35v4, bs35, h, t, "dec-map-v35-p-len")
		testDeepEqualErr(v35v3, v35v4, t, "equal-map-v35-p-len")
	}

	for _, v := range []map[float32]interface{}{nil, {}, {22.2: nil, 11.1: "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v38: %v\n", v)
		var v38v1, v38v2 map[float32]interface{}
		v38v1 = v
		bs38 := testMarshalErr(v38v1, h, t, "enc-map-v38")
		if v == nil {
			v38v2 = nil
		} else {
			v38v2 = make(map[float32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v38v2, bs38, h, t, "dec-map-v38")
		testDeepEqualErr(v38v1, v38v2, t, "equal-map-v38")
		if v == nil {
			v38v2 = nil
		} else {
			v38v2 = make(map[float32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v38v2), bs38, h, t, "dec-map-v38-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v38v1, v38v2, t, "equal-map-v38-noaddr")
		if v == nil {
			v38v2 = nil
		} else {
			v38v2 = make(map[float32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v38v2, bs38, h, t, "dec-map-v38-p-len")
		testDeepEqualErr(v38v1, v38v2, t, "equal-map-v38-p-len")
		bs38 = testMarshalErr(&v38v1, h, t, "enc-map-v38-p")
		v38v2 = nil
		testUnmarshalErr(&v38v2, bs38, h, t, "dec-map-v38-p-nil")
		testDeepEqualErr(v38v1, v38v2, t, "equal-map-v38-p-nil")
		// ...
		if v == nil {
			v38v2 = nil
		} else {
			v38v2 = make(map[float32]interface{}, len(v))
		} // reset map
		var v38v3, v38v4 typMapMapFloat32Intf
		v38v3 = typMapMapFloat32Intf(v38v1)
		v38v4 = typMapMapFloat32Intf(v38v2)
		bs38 = testMarshalErr(v38v3, h, t, "enc-map-v38-custom")
		testUnmarshalErr(v38v4, bs38, h, t, "dec-map-v38-p-len")
		testDeepEqualErr(v38v3, v38v4, t, "equal-map-v38-p-len")
	}

	for _, v := range []map[float32]string{nil, {}, {22.2: "", 11.1: "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v39: %v\n", v)
		var v39v1, v39v2 map[float32]string
		v39v1 = v
		bs39 := testMarshalErr(v39v1, h, t, "enc-map-v39")
		if v == nil {
			v39v2 = nil
		} else {
			v39v2 = make(map[float32]string, len(v))
		} // reset map
		testUnmarshalErr(v39v2, bs39, h, t, "dec-map-v39")
		testDeepEqualErr(v39v1, v39v2, t, "equal-map-v39")
		if v == nil {
			v39v2 = nil
		} else {
			v39v2 = make(map[float32]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v39v2), bs39, h, t, "dec-map-v39-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v39v1, v39v2, t, "equal-map-v39-noaddr")
		if v == nil {
			v39v2 = nil
		} else {
			v39v2 = make(map[float32]string, len(v))
		} // reset map
		testUnmarshalErr(&v39v2, bs39, h, t, "dec-map-v39-p-len")
		testDeepEqualErr(v39v1, v39v2, t, "equal-map-v39-p-len")
		bs39 = testMarshalErr(&v39v1, h, t, "enc-map-v39-p")
		v39v2 = nil
		testUnmarshalErr(&v39v2, bs39, h, t, "dec-map-v39-p-nil")
		testDeepEqualErr(v39v1, v39v2, t, "equal-map-v39-p-nil")
		// ...
		if v == nil {
			v39v2 = nil
		} else {
			v39v2 = make(map[float32]string, len(v))
		} // reset map
		var v39v3, v39v4 typMapMapFloat32String
		v39v3 = typMapMapFloat32String(v39v1)
		v39v4 = typMapMapFloat32String(v39v2)
		bs39 = testMarshalErr(v39v3, h, t, "enc-map-v39-custom")
		testUnmarshalErr(v39v4, bs39, h, t, "dec-map-v39-p-len")
		testDeepEqualErr(v39v3, v39v4, t, "equal-map-v39-p-len")
	}

	for _, v := range []map[float32]uint{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v40: %v\n", v)
		var v40v1, v40v2 map[float32]uint
		v40v1 = v
		bs40 := testMarshalErr(v40v1, h, t, "enc-map-v40")
		if v == nil {
			v40v2 = nil
		} else {
			v40v2 = make(map[float32]uint, len(v))
		} // reset map
		testUnmarshalErr(v40v2, bs40, h, t, "dec-map-v40")
		testDeepEqualErr(v40v1, v40v2, t, "equal-map-v40")
		if v == nil {
			v40v2 = nil
		} else {
			v40v2 = make(map[float32]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v40v2), bs40, h, t, "dec-map-v40-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v40v1, v40v2, t, "equal-map-v40-noaddr")
		if v == nil {
			v40v2 = nil
		} else {
			v40v2 = make(map[float32]uint, len(v))
		} // reset map
		testUnmarshalErr(&v40v2, bs40, h, t, "dec-map-v40-p-len")
		testDeepEqualErr(v40v1, v40v2, t, "equal-map-v40-p-len")
		bs40 = testMarshalErr(&v40v1, h, t, "enc-map-v40-p")
		v40v2 = nil
		testUnmarshalErr(&v40v2, bs40, h, t, "dec-map-v40-p-nil")
		testDeepEqualErr(v40v1, v40v2, t, "equal-map-v40-p-nil")
		// ...
		if v == nil {
			v40v2 = nil
		} else {
			v40v2 = make(map[float32]uint, len(v))
		} // reset map
		var v40v3, v40v4 typMapMapFloat32Uint
		v40v3 = typMapMapFloat32Uint(v40v1)
		v40v4 = typMapMapFloat32Uint(v40v2)
		bs40 = testMarshalErr(v40v3, h, t, "enc-map-v40-custom")
		testUnmarshalErr(v40v4, bs40, h, t, "dec-map-v40-p-len")
		testDeepEqualErr(v40v3, v40v4, t, "equal-map-v40-p-len")
	}

	for _, v := range []map[float32]uint8{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v41: %v\n", v)
		var v41v1, v41v2 map[float32]uint8
		v41v1 = v
		bs41 := testMarshalErr(v41v1, h, t, "enc-map-v41")
		if v == nil {
			v41v2 = nil
		} else {
			v41v2 = make(map[float32]uint8, len(v))
		} // reset map
		testUnmarshalErr(v41v2, bs41, h, t, "dec-map-v41")
		testDeepEqualErr(v41v1, v41v2, t, "equal-map-v41")
		if v == nil {
			v41v2 = nil
		} else {
			v41v2 = make(map[float32]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v41v2), bs41, h, t, "dec-map-v41-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v41v1, v41v2, t, "equal-map-v41-noaddr")
		if v == nil {
			v41v2 = nil
		} else {
			v41v2 = make(map[float32]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v41v2, bs41, h, t, "dec-map-v41-p-len")
		testDeepEqualErr(v41v1, v41v2, t, "equal-map-v41-p-len")
		bs41 = testMarshalErr(&v41v1, h, t, "enc-map-v41-p")
		v41v2 = nil
		testUnmarshalErr(&v41v2, bs41, h, t, "dec-map-v41-p-nil")
		testDeepEqualErr(v41v1, v41v2, t, "equal-map-v41-p-nil")
		// ...
		if v == nil {
			v41v2 = nil
		} else {
			v41v2 = make(map[float32]uint8, len(v))
		} // reset map
		var v41v3, v41v4 typMapMapFloat32Uint8
		v41v3 = typMapMapFloat32Uint8(v41v1)
		v41v4 = typMapMapFloat32Uint8(v41v2)
		bs41 = testMarshalErr(v41v3, h, t, "enc-map-v41-custom")
		testUnmarshalErr(v41v4, bs41, h, t, "dec-map-v41-p-len")
		testDeepEqualErr(v41v3, v41v4, t, "equal-map-v41-p-len")
	}

	for _, v := range []map[float32]uint16{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v42: %v\n", v)
		var v42v1, v42v2 map[float32]uint16
		v42v1 = v
		bs42 := testMarshalErr(v42v1, h, t, "enc-map-v42")
		if v == nil {
			v42v2 = nil
		} else {
			v42v2 = make(map[float32]uint16, len(v))
		} // reset map
		testUnmarshalErr(v42v2, bs42, h, t, "dec-map-v42")
		testDeepEqualErr(v42v1, v42v2, t, "equal-map-v42")
		if v == nil {
			v42v2 = nil
		} else {
			v42v2 = make(map[float32]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v42v2), bs42, h, t, "dec-map-v42-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v42v1, v42v2, t, "equal-map-v42-noaddr")
		if v == nil {
			v42v2 = nil
		} else {
			v42v2 = make(map[float32]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v42v2, bs42, h, t, "dec-map-v42-p-len")
		testDeepEqualErr(v42v1, v42v2, t, "equal-map-v42-p-len")
		bs42 = testMarshalErr(&v42v1, h, t, "enc-map-v42-p")
		v42v2 = nil
		testUnmarshalErr(&v42v2, bs42, h, t, "dec-map-v42-p-nil")
		testDeepEqualErr(v42v1, v42v2, t, "equal-map-v42-p-nil")
		// ...
		if v == nil {
			v42v2 = nil
		} else {
			v42v2 = make(map[float32]uint16, len(v))
		} // reset map
		var v42v3, v42v4 typMapMapFloat32Uint16
		v42v3 = typMapMapFloat32Uint16(v42v1)
		v42v4 = typMapMapFloat32Uint16(v42v2)
		bs42 = testMarshalErr(v42v3, h, t, "enc-map-v42-custom")
		testUnmarshalErr(v42v4, bs42, h, t, "dec-map-v42-p-len")
		testDeepEqualErr(v42v3, v42v4, t, "equal-map-v42-p-len")
	}

	for _, v := range []map[float32]uint32{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v43: %v\n", v)
		var v43v1, v43v2 map[float32]uint32
		v43v1 = v
		bs43 := testMarshalErr(v43v1, h, t, "enc-map-v43")
		if v == nil {
			v43v2 = nil
		} else {
			v43v2 = make(map[float32]uint32, len(v))
		} // reset map
		testUnmarshalErr(v43v2, bs43, h, t, "dec-map-v43")
		testDeepEqualErr(v43v1, v43v2, t, "equal-map-v43")
		if v == nil {
			v43v2 = nil
		} else {
			v43v2 = make(map[float32]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v43v2), bs43, h, t, "dec-map-v43-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v43v1, v43v2, t, "equal-map-v43-noaddr")
		if v == nil {
			v43v2 = nil
		} else {
			v43v2 = make(map[float32]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v43v2, bs43, h, t, "dec-map-v43-p-len")
		testDeepEqualErr(v43v1, v43v2, t, "equal-map-v43-p-len")
		bs43 = testMarshalErr(&v43v1, h, t, "enc-map-v43-p")
		v43v2 = nil
		testUnmarshalErr(&v43v2, bs43, h, t, "dec-map-v43-p-nil")
		testDeepEqualErr(v43v1, v43v2, t, "equal-map-v43-p-nil")
		// ...
		if v == nil {
			v43v2 = nil
		} else {
			v43v2 = make(map[float32]uint32, len(v))
		} // reset map
		var v43v3, v43v4 typMapMapFloat32Uint32
		v43v3 = typMapMapFloat32Uint32(v43v1)
		v43v4 = typMapMapFloat32Uint32(v43v2)
		bs43 = testMarshalErr(v43v3, h, t, "enc-map-v43-custom")
		testUnmarshalErr(v43v4, bs43, h, t, "dec-map-v43-p-len")
		testDeepEqualErr(v43v3, v43v4, t, "equal-map-v43-p-len")
	}

	for _, v := range []map[float32]uint64{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v44: %v\n", v)
		var v44v1, v44v2 map[float32]uint64
		v44v1 = v
		bs44 := testMarshalErr(v44v1, h, t, "enc-map-v44")
		if v == nil {
			v44v2 = nil
		} else {
			v44v2 = make(map[float32]uint64, len(v))
		} // reset map
		testUnmarshalErr(v44v2, bs44, h, t, "dec-map-v44")
		testDeepEqualErr(v44v1, v44v2, t, "equal-map-v44")
		if v == nil {
			v44v2 = nil
		} else {
			v44v2 = make(map[float32]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v44v2), bs44, h, t, "dec-map-v44-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v44v1, v44v2, t, "equal-map-v44-noaddr")
		if v == nil {
			v44v2 = nil
		} else {
			v44v2 = make(map[float32]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v44v2, bs44, h, t, "dec-map-v44-p-len")
		testDeepEqualErr(v44v1, v44v2, t, "equal-map-v44-p-len")
		bs44 = testMarshalErr(&v44v1, h, t, "enc-map-v44-p")
		v44v2 = nil
		testUnmarshalErr(&v44v2, bs44, h, t, "dec-map-v44-p-nil")
		testDeepEqualErr(v44v1, v44v2, t, "equal-map-v44-p-nil")
		// ...
		if v == nil {
			v44v2 = nil
		} else {
			v44v2 = make(map[float32]uint64, len(v))
		} // reset map
		var v44v3, v44v4 typMapMapFloat32Uint64
		v44v3 = typMapMapFloat32Uint64(v44v1)
		v44v4 = typMapMapFloat32Uint64(v44v2)
		bs44 = testMarshalErr(v44v3, h, t, "enc-map-v44-custom")
		testUnmarshalErr(v44v4, bs44, h, t, "dec-map-v44-p-len")
		testDeepEqualErr(v44v3, v44v4, t, "equal-map-v44-p-len")
	}

	for _, v := range []map[float32]uintptr{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v45: %v\n", v)
		var v45v1, v45v2 map[float32]uintptr
		v45v1 = v
		bs45 := testMarshalErr(v45v1, h, t, "enc-map-v45")
		if v == nil {
			v45v2 = nil
		} else {
			v45v2 = make(map[float32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v45v2, bs45, h, t, "dec-map-v45")
		testDeepEqualErr(v45v1, v45v2, t, "equal-map-v45")
		if v == nil {
			v45v2 = nil
		} else {
			v45v2 = make(map[float32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v45v2), bs45, h, t, "dec-map-v45-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v45v1, v45v2, t, "equal-map-v45-noaddr")
		if v == nil {
			v45v2 = nil
		} else {
			v45v2 = make(map[float32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v45v2, bs45, h, t, "dec-map-v45-p-len")
		testDeepEqualErr(v45v1, v45v2, t, "equal-map-v45-p-len")
		bs45 = testMarshalErr(&v45v1, h, t, "enc-map-v45-p")
		v45v2 = nil
		testUnmarshalErr(&v45v2, bs45, h, t, "dec-map-v45-p-nil")
		testDeepEqualErr(v45v1, v45v2, t, "equal-map-v45-p-nil")
		// ...
		if v == nil {
			v45v2 = nil
		} else {
			v45v2 = make(map[float32]uintptr, len(v))
		} // reset map
		var v45v3, v45v4 typMapMapFloat32Uintptr
		v45v3 = typMapMapFloat32Uintptr(v45v1)
		v45v4 = typMapMapFloat32Uintptr(v45v2)
		bs45 = testMarshalErr(v45v3, h, t, "enc-map-v45-custom")
		testUnmarshalErr(v45v4, bs45, h, t, "dec-map-v45-p-len")
		testDeepEqualErr(v45v3, v45v4, t, "equal-map-v45-p-len")
	}

	for _, v := range []map[float32]int{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v46: %v\n", v)
		var v46v1, v46v2 map[float32]int
		v46v1 = v
		bs46 := testMarshalErr(v46v1, h, t, "enc-map-v46")
		if v == nil {
			v46v2 = nil
		} else {
			v46v2 = make(map[float32]int, len(v))
		} // reset map
		testUnmarshalErr(v46v2, bs46, h, t, "dec-map-v46")
		testDeepEqualErr(v46v1, v46v2, t, "equal-map-v46")
		if v == nil {
			v46v2 = nil
		} else {
			v46v2 = make(map[float32]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v46v2), bs46, h, t, "dec-map-v46-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v46v1, v46v2, t, "equal-map-v46-noaddr")
		if v == nil {
			v46v2 = nil
		} else {
			v46v2 = make(map[float32]int, len(v))
		} // reset map
		testUnmarshalErr(&v46v2, bs46, h, t, "dec-map-v46-p-len")
		testDeepEqualErr(v46v1, v46v2, t, "equal-map-v46-p-len")
		bs46 = testMarshalErr(&v46v1, h, t, "enc-map-v46-p")
		v46v2 = nil
		testUnmarshalErr(&v46v2, bs46, h, t, "dec-map-v46-p-nil")
		testDeepEqualErr(v46v1, v46v2, t, "equal-map-v46-p-nil")
		// ...
		if v == nil {
			v46v2 = nil
		} else {
			v46v2 = make(map[float32]int, len(v))
		} // reset map
		var v46v3, v46v4 typMapMapFloat32Int
		v46v3 = typMapMapFloat32Int(v46v1)
		v46v4 = typMapMapFloat32Int(v46v2)
		bs46 = testMarshalErr(v46v3, h, t, "enc-map-v46-custom")
		testUnmarshalErr(v46v4, bs46, h, t, "dec-map-v46-p-len")
		testDeepEqualErr(v46v3, v46v4, t, "equal-map-v46-p-len")
	}

	for _, v := range []map[float32]int8{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v47: %v\n", v)
		var v47v1, v47v2 map[float32]int8
		v47v1 = v
		bs47 := testMarshalErr(v47v1, h, t, "enc-map-v47")
		if v == nil {
			v47v2 = nil
		} else {
			v47v2 = make(map[float32]int8, len(v))
		} // reset map
		testUnmarshalErr(v47v2, bs47, h, t, "dec-map-v47")
		testDeepEqualErr(v47v1, v47v2, t, "equal-map-v47")
		if v == nil {
			v47v2 = nil
		} else {
			v47v2 = make(map[float32]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v47v2), bs47, h, t, "dec-map-v47-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v47v1, v47v2, t, "equal-map-v47-noaddr")
		if v == nil {
			v47v2 = nil
		} else {
			v47v2 = make(map[float32]int8, len(v))
		} // reset map
		testUnmarshalErr(&v47v2, bs47, h, t, "dec-map-v47-p-len")
		testDeepEqualErr(v47v1, v47v2, t, "equal-map-v47-p-len")
		bs47 = testMarshalErr(&v47v1, h, t, "enc-map-v47-p")
		v47v2 = nil
		testUnmarshalErr(&v47v2, bs47, h, t, "dec-map-v47-p-nil")
		testDeepEqualErr(v47v1, v47v2, t, "equal-map-v47-p-nil")
		// ...
		if v == nil {
			v47v2 = nil
		} else {
			v47v2 = make(map[float32]int8, len(v))
		} // reset map
		var v47v3, v47v4 typMapMapFloat32Int8
		v47v3 = typMapMapFloat32Int8(v47v1)
		v47v4 = typMapMapFloat32Int8(v47v2)
		bs47 = testMarshalErr(v47v3, h, t, "enc-map-v47-custom")
		testUnmarshalErr(v47v4, bs47, h, t, "dec-map-v47-p-len")
		testDeepEqualErr(v47v3, v47v4, t, "equal-map-v47-p-len")
	}

	for _, v := range []map[float32]int16{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v48: %v\n", v)
		var v48v1, v48v2 map[float32]int16
		v48v1 = v
		bs48 := testMarshalErr(v48v1, h, t, "enc-map-v48")
		if v == nil {
			v48v2 = nil
		} else {
			v48v2 = make(map[float32]int16, len(v))
		} // reset map
		testUnmarshalErr(v48v2, bs48, h, t, "dec-map-v48")
		testDeepEqualErr(v48v1, v48v2, t, "equal-map-v48")
		if v == nil {
			v48v2 = nil
		} else {
			v48v2 = make(map[float32]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v48v2), bs48, h, t, "dec-map-v48-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v48v1, v48v2, t, "equal-map-v48-noaddr")
		if v == nil {
			v48v2 = nil
		} else {
			v48v2 = make(map[float32]int16, len(v))
		} // reset map
		testUnmarshalErr(&v48v2, bs48, h, t, "dec-map-v48-p-len")
		testDeepEqualErr(v48v1, v48v2, t, "equal-map-v48-p-len")
		bs48 = testMarshalErr(&v48v1, h, t, "enc-map-v48-p")
		v48v2 = nil
		testUnmarshalErr(&v48v2, bs48, h, t, "dec-map-v48-p-nil")
		testDeepEqualErr(v48v1, v48v2, t, "equal-map-v48-p-nil")
		// ...
		if v == nil {
			v48v2 = nil
		} else {
			v48v2 = make(map[float32]int16, len(v))
		} // reset map
		var v48v3, v48v4 typMapMapFloat32Int16
		v48v3 = typMapMapFloat32Int16(v48v1)
		v48v4 = typMapMapFloat32Int16(v48v2)
		bs48 = testMarshalErr(v48v3, h, t, "enc-map-v48-custom")
		testUnmarshalErr(v48v4, bs48, h, t, "dec-map-v48-p-len")
		testDeepEqualErr(v48v3, v48v4, t, "equal-map-v48-p-len")
	}

	for _, v := range []map[float32]int32{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v49: %v\n", v)
		var v49v1, v49v2 map[float32]int32
		v49v1 = v
		bs49 := testMarshalErr(v49v1, h, t, "enc-map-v49")
		if v == nil {
			v49v2 = nil
		} else {
			v49v2 = make(map[float32]int32, len(v))
		} // reset map
		testUnmarshalErr(v49v2, bs49, h, t, "dec-map-v49")
		testDeepEqualErr(v49v1, v49v2, t, "equal-map-v49")
		if v == nil {
			v49v2 = nil
		} else {
			v49v2 = make(map[float32]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v49v2), bs49, h, t, "dec-map-v49-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v49v1, v49v2, t, "equal-map-v49-noaddr")
		if v == nil {
			v49v2 = nil
		} else {
			v49v2 = make(map[float32]int32, len(v))
		} // reset map
		testUnmarshalErr(&v49v2, bs49, h, t, "dec-map-v49-p-len")
		testDeepEqualErr(v49v1, v49v2, t, "equal-map-v49-p-len")
		bs49 = testMarshalErr(&v49v1, h, t, "enc-map-v49-p")
		v49v2 = nil
		testUnmarshalErr(&v49v2, bs49, h, t, "dec-map-v49-p-nil")
		testDeepEqualErr(v49v1, v49v2, t, "equal-map-v49-p-nil")
		// ...
		if v == nil {
			v49v2 = nil
		} else {
			v49v2 = make(map[float32]int32, len(v))
		} // reset map
		var v49v3, v49v4 typMapMapFloat32Int32
		v49v3 = typMapMapFloat32Int32(v49v1)
		v49v4 = typMapMapFloat32Int32(v49v2)
		bs49 = testMarshalErr(v49v3, h, t, "enc-map-v49-custom")
		testUnmarshalErr(v49v4, bs49, h, t, "dec-map-v49-p-len")
		testDeepEqualErr(v49v3, v49v4, t, "equal-map-v49-p-len")
	}

	for _, v := range []map[float32]int64{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v50: %v\n", v)
		var v50v1, v50v2 map[float32]int64
		v50v1 = v
		bs50 := testMarshalErr(v50v1, h, t, "enc-map-v50")
		if v == nil {
			v50v2 = nil
		} else {
			v50v2 = make(map[float32]int64, len(v))
		} // reset map
		testUnmarshalErr(v50v2, bs50, h, t, "dec-map-v50")
		testDeepEqualErr(v50v1, v50v2, t, "equal-map-v50")
		if v == nil {
			v50v2 = nil
		} else {
			v50v2 = make(map[float32]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v50v2), bs50, h, t, "dec-map-v50-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v50v1, v50v2, t, "equal-map-v50-noaddr")
		if v == nil {
			v50v2 = nil
		} else {
			v50v2 = make(map[float32]int64, len(v))
		} // reset map
		testUnmarshalErr(&v50v2, bs50, h, t, "dec-map-v50-p-len")
		testDeepEqualErr(v50v1, v50v2, t, "equal-map-v50-p-len")
		bs50 = testMarshalErr(&v50v1, h, t, "enc-map-v50-p")
		v50v2 = nil
		testUnmarshalErr(&v50v2, bs50, h, t, "dec-map-v50-p-nil")
		testDeepEqualErr(v50v1, v50v2, t, "equal-map-v50-p-nil")
		// ...
		if v == nil {
			v50v2 = nil
		} else {
			v50v2 = make(map[float32]int64, len(v))
		} // reset map
		var v50v3, v50v4 typMapMapFloat32Int64
		v50v3 = typMapMapFloat32Int64(v50v1)
		v50v4 = typMapMapFloat32Int64(v50v2)
		bs50 = testMarshalErr(v50v3, h, t, "enc-map-v50-custom")
		testUnmarshalErr(v50v4, bs50, h, t, "dec-map-v50-p-len")
		testDeepEqualErr(v50v3, v50v4, t, "equal-map-v50-p-len")
	}

	for _, v := range []map[float32]float32{nil, {}, {22.2: 0, 11.1: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v51: %v\n", v)
		var v51v1, v51v2 map[float32]float32
		v51v1 = v
		bs51 := testMarshalErr(v51v1, h, t, "enc-map-v51")
		if v == nil {
			v51v2 = nil
		} else {
			v51v2 = make(map[float32]float32, len(v))
		} // reset map
		testUnmarshalErr(v51v2, bs51, h, t, "dec-map-v51")
		testDeepEqualErr(v51v1, v51v2, t, "equal-map-v51")
		if v == nil {
			v51v2 = nil
		} else {
			v51v2 = make(map[float32]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v51v2), bs51, h, t, "dec-map-v51-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v51v1, v51v2, t, "equal-map-v51-noaddr")
		if v == nil {
			v51v2 = nil
		} else {
			v51v2 = make(map[float32]float32, len(v))
		} // reset map
		testUnmarshalErr(&v51v2, bs51, h, t, "dec-map-v51-p-len")
		testDeepEqualErr(v51v1, v51v2, t, "equal-map-v51-p-len")
		bs51 = testMarshalErr(&v51v1, h, t, "enc-map-v51-p")
		v51v2 = nil
		testUnmarshalErr(&v51v2, bs51, h, t, "dec-map-v51-p-nil")
		testDeepEqualErr(v51v1, v51v2, t, "equal-map-v51-p-nil")
		// ...
		if v == nil {
			v51v2 = nil
		} else {
			v51v2 = make(map[float32]float32, len(v))
		} // reset map
		var v51v3, v51v4 typMapMapFloat32Float32
		v51v3 = typMapMapFloat32Float32(v51v1)
		v51v4 = typMapMapFloat32Float32(v51v2)
		bs51 = testMarshalErr(v51v3, h, t, "enc-map-v51-custom")
		testUnmarshalErr(v51v4, bs51, h, t, "dec-map-v51-p-len")
		testDeepEqualErr(v51v3, v51v4, t, "equal-map-v51-p-len")
	}

	for _, v := range []map[float32]float64{nil, {}, {11.1: 0, 22.2: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v52: %v\n", v)
		var v52v1, v52v2 map[float32]float64
		v52v1 = v
		bs52 := testMarshalErr(v52v1, h, t, "enc-map-v52")
		if v == nil {
			v52v2 = nil
		} else {
			v52v2 = make(map[float32]float64, len(v))
		} // reset map
		testUnmarshalErr(v52v2, bs52, h, t, "dec-map-v52")
		testDeepEqualErr(v52v1, v52v2, t, "equal-map-v52")
		if v == nil {
			v52v2 = nil
		} else {
			v52v2 = make(map[float32]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v52v2), bs52, h, t, "dec-map-v52-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v52v1, v52v2, t, "equal-map-v52-noaddr")
		if v == nil {
			v52v2 = nil
		} else {
			v52v2 = make(map[float32]float64, len(v))
		} // reset map
		testUnmarshalErr(&v52v2, bs52, h, t, "dec-map-v52-p-len")
		testDeepEqualErr(v52v1, v52v2, t, "equal-map-v52-p-len")
		bs52 = testMarshalErr(&v52v1, h, t, "enc-map-v52-p")
		v52v2 = nil
		testUnmarshalErr(&v52v2, bs52, h, t, "dec-map-v52-p-nil")
		testDeepEqualErr(v52v1, v52v2, t, "equal-map-v52-p-nil")
		// ...
		if v == nil {
			v52v2 = nil
		} else {
			v52v2 = make(map[float32]float64, len(v))
		} // reset map
		var v52v3, v52v4 typMapMapFloat32Float64
		v52v3 = typMapMapFloat32Float64(v52v1)
		v52v4 = typMapMapFloat32Float64(v52v2)
		bs52 = testMarshalErr(v52v3, h, t, "enc-map-v52-custom")
		testUnmarshalErr(v52v4, bs52, h, t, "dec-map-v52-p-len")
		testDeepEqualErr(v52v3, v52v4, t, "equal-map-v52-p-len")
	}

	for _, v := range []map[float32]bool{nil, {}, {22.2: false, 11.1: true}} {
		// fmt.Printf(">>>> running mammoth map v53: %v\n", v)
		var v53v1, v53v2 map[float32]bool
		v53v1 = v
		bs53 := testMarshalErr(v53v1, h, t, "enc-map-v53")
		if v == nil {
			v53v2 = nil
		} else {
			v53v2 = make(map[float32]bool, len(v))
		} // reset map
		testUnmarshalErr(v53v2, bs53, h, t, "dec-map-v53")
		testDeepEqualErr(v53v1, v53v2, t, "equal-map-v53")
		if v == nil {
			v53v2 = nil
		} else {
			v53v2 = make(map[float32]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v53v2), bs53, h, t, "dec-map-v53-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v53v1, v53v2, t, "equal-map-v53-noaddr")
		if v == nil {
			v53v2 = nil
		} else {
			v53v2 = make(map[float32]bool, len(v))
		} // reset map
		testUnmarshalErr(&v53v2, bs53, h, t, "dec-map-v53-p-len")
		testDeepEqualErr(v53v1, v53v2, t, "equal-map-v53-p-len")
		bs53 = testMarshalErr(&v53v1, h, t, "enc-map-v53-p")
		v53v2 = nil
		testUnmarshalErr(&v53v2, bs53, h, t, "dec-map-v53-p-nil")
		testDeepEqualErr(v53v1, v53v2, t, "equal-map-v53-p-nil")
		// ...
		if v == nil {
			v53v2 = nil
		} else {
			v53v2 = make(map[float32]bool, len(v))
		} // reset map
		var v53v3, v53v4 typMapMapFloat32Bool
		v53v3 = typMapMapFloat32Bool(v53v1)
		v53v4 = typMapMapFloat32Bool(v53v2)
		bs53 = testMarshalErr(v53v3, h, t, "enc-map-v53-custom")
		testUnmarshalErr(v53v4, bs53, h, t, "dec-map-v53-p-len")
		testDeepEqualErr(v53v3, v53v4, t, "equal-map-v53-p-len")
	}

	for _, v := range []map[float64]interface{}{nil, {}, {22.2: nil, 11.1: "string-is-an-interface"}} {
		// fmt.Printf(">>>> running mammoth map v56: %v\n", v)
		var v56v1, v56v2 map[float64]interface{}
		v56v1 = v
		bs56 := testMarshalErr(v56v1, h, t, "enc-map-v56")
		if v == nil {
			v56v2 = nil
		} else {
			v56v2 = make(map[float64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v56v2, bs56, h, t, "dec-map-v56")
		testDeepEqualErr(v56v1, v56v2, t, "equal-map-v56")
		if v == nil {
			v56v2 = nil
		} else {
			v56v2 = make(map[float64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v56v2), bs56, h, t, "dec-map-v56-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v56v1, v56v2, t, "equal-map-v56-noaddr")
		if v == nil {
			v56v2 = nil
		} else {
			v56v2 = make(map[float64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v56v2, bs56, h, t, "dec-map-v56-p-len")
		testDeepEqualErr(v56v1, v56v2, t, "equal-map-v56-p-len")
		bs56 = testMarshalErr(&v56v1, h, t, "enc-map-v56-p")
		v56v2 = nil
		testUnmarshalErr(&v56v2, bs56, h, t, "dec-map-v56-p-nil")
		testDeepEqualErr(v56v1, v56v2, t, "equal-map-v56-p-nil")
		// ...
		if v == nil {
			v56v2 = nil
		} else {
			v56v2 = make(map[float64]interface{}, len(v))
		} // reset map
		var v56v3, v56v4 typMapMapFloat64Intf
		v56v3 = typMapMapFloat64Intf(v56v1)
		v56v4 = typMapMapFloat64Intf(v56v2)
		bs56 = testMarshalErr(v56v3, h, t, "enc-map-v56-custom")
		testUnmarshalErr(v56v4, bs56, h, t, "dec-map-v56-p-len")
		testDeepEqualErr(v56v3, v56v4, t, "equal-map-v56-p-len")
	}

	for _, v := range []map[float64]string{nil, {}, {22.2: "", 11.1: "some-string"}} {
		// fmt.Printf(">>>> running mammoth map v57: %v\n", v)
		var v57v1, v57v2 map[float64]string
		v57v1 = v
		bs57 := testMarshalErr(v57v1, h, t, "enc-map-v57")
		if v == nil {
			v57v2 = nil
		} else {
			v57v2 = make(map[float64]string, len(v))
		} // reset map
		testUnmarshalErr(v57v2, bs57, h, t, "dec-map-v57")
		testDeepEqualErr(v57v1, v57v2, t, "equal-map-v57")
		if v == nil {
			v57v2 = nil
		} else {
			v57v2 = make(map[float64]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v57v2), bs57, h, t, "dec-map-v57-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v57v1, v57v2, t, "equal-map-v57-noaddr")
		if v == nil {
			v57v2 = nil
		} else {
			v57v2 = make(map[float64]string, len(v))
		} // reset map
		testUnmarshalErr(&v57v2, bs57, h, t, "dec-map-v57-p-len")
		testDeepEqualErr(v57v1, v57v2, t, "equal-map-v57-p-len")
		bs57 = testMarshalErr(&v57v1, h, t, "enc-map-v57-p")
		v57v2 = nil
		testUnmarshalErr(&v57v2, bs57, h, t, "dec-map-v57-p-nil")
		testDeepEqualErr(v57v1, v57v2, t, "equal-map-v57-p-nil")
		// ...
		if v == nil {
			v57v2 = nil
		} else {
			v57v2 = make(map[float64]string, len(v))
		} // reset map
		var v57v3, v57v4 typMapMapFloat64String
		v57v3 = typMapMapFloat64String(v57v1)
		v57v4 = typMapMapFloat64String(v57v2)
		bs57 = testMarshalErr(v57v3, h, t, "enc-map-v57-custom")
		testUnmarshalErr(v57v4, bs57, h, t, "dec-map-v57-p-len")
		testDeepEqualErr(v57v3, v57v4, t, "equal-map-v57-p-len")
	}

	for _, v := range []map[float64]uint{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v58: %v\n", v)
		var v58v1, v58v2 map[float64]uint
		v58v1 = v
		bs58 := testMarshalErr(v58v1, h, t, "enc-map-v58")
		if v == nil {
			v58v2 = nil
		} else {
			v58v2 = make(map[float64]uint, len(v))
		} // reset map
		testUnmarshalErr(v58v2, bs58, h, t, "dec-map-v58")
		testDeepEqualErr(v58v1, v58v2, t, "equal-map-v58")
		if v == nil {
			v58v2 = nil
		} else {
			v58v2 = make(map[float64]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v58v2), bs58, h, t, "dec-map-v58-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v58v1, v58v2, t, "equal-map-v58-noaddr")
		if v == nil {
			v58v2 = nil
		} else {
			v58v2 = make(map[float64]uint, len(v))
		} // reset map
		testUnmarshalErr(&v58v2, bs58, h, t, "dec-map-v58-p-len")
		testDeepEqualErr(v58v1, v58v2, t, "equal-map-v58-p-len")
		bs58 = testMarshalErr(&v58v1, h, t, "enc-map-v58-p")
		v58v2 = nil
		testUnmarshalErr(&v58v2, bs58, h, t, "dec-map-v58-p-nil")
		testDeepEqualErr(v58v1, v58v2, t, "equal-map-v58-p-nil")
		// ...
		if v == nil {
			v58v2 = nil
		} else {
			v58v2 = make(map[float64]uint, len(v))
		} // reset map
		var v58v3, v58v4 typMapMapFloat64Uint
		v58v3 = typMapMapFloat64Uint(v58v1)
		v58v4 = typMapMapFloat64Uint(v58v2)
		bs58 = testMarshalErr(v58v3, h, t, "enc-map-v58-custom")
		testUnmarshalErr(v58v4, bs58, h, t, "dec-map-v58-p-len")
		testDeepEqualErr(v58v3, v58v4, t, "equal-map-v58-p-len")
	}

	for _, v := range []map[float64]uint8{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v59: %v\n", v)
		var v59v1, v59v2 map[float64]uint8
		v59v1 = v
		bs59 := testMarshalErr(v59v1, h, t, "enc-map-v59")
		if v == nil {
			v59v2 = nil
		} else {
			v59v2 = make(map[float64]uint8, len(v))
		} // reset map
		testUnmarshalErr(v59v2, bs59, h, t, "dec-map-v59")
		testDeepEqualErr(v59v1, v59v2, t, "equal-map-v59")
		if v == nil {
			v59v2 = nil
		} else {
			v59v2 = make(map[float64]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v59v2), bs59, h, t, "dec-map-v59-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v59v1, v59v2, t, "equal-map-v59-noaddr")
		if v == nil {
			v59v2 = nil
		} else {
			v59v2 = make(map[float64]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v59v2, bs59, h, t, "dec-map-v59-p-len")
		testDeepEqualErr(v59v1, v59v2, t, "equal-map-v59-p-len")
		bs59 = testMarshalErr(&v59v1, h, t, "enc-map-v59-p")
		v59v2 = nil
		testUnmarshalErr(&v59v2, bs59, h, t, "dec-map-v59-p-nil")
		testDeepEqualErr(v59v1, v59v2, t, "equal-map-v59-p-nil")
		// ...
		if v == nil {
			v59v2 = nil
		} else {
			v59v2 = make(map[float64]uint8, len(v))
		} // reset map
		var v59v3, v59v4 typMapMapFloat64Uint8
		v59v3 = typMapMapFloat64Uint8(v59v1)
		v59v4 = typMapMapFloat64Uint8(v59v2)
		bs59 = testMarshalErr(v59v3, h, t, "enc-map-v59-custom")
		testUnmarshalErr(v59v4, bs59, h, t, "dec-map-v59-p-len")
		testDeepEqualErr(v59v3, v59v4, t, "equal-map-v59-p-len")
	}

	for _, v := range []map[float64]uint16{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v60: %v\n", v)
		var v60v1, v60v2 map[float64]uint16
		v60v1 = v
		bs60 := testMarshalErr(v60v1, h, t, "enc-map-v60")
		if v == nil {
			v60v2 = nil
		} else {
			v60v2 = make(map[float64]uint16, len(v))
		} // reset map
		testUnmarshalErr(v60v2, bs60, h, t, "dec-map-v60")
		testDeepEqualErr(v60v1, v60v2, t, "equal-map-v60")
		if v == nil {
			v60v2 = nil
		} else {
			v60v2 = make(map[float64]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v60v2), bs60, h, t, "dec-map-v60-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v60v1, v60v2, t, "equal-map-v60-noaddr")
		if v == nil {
			v60v2 = nil
		} else {
			v60v2 = make(map[float64]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v60v2, bs60, h, t, "dec-map-v60-p-len")
		testDeepEqualErr(v60v1, v60v2, t, "equal-map-v60-p-len")
		bs60 = testMarshalErr(&v60v1, h, t, "enc-map-v60-p")
		v60v2 = nil
		testUnmarshalErr(&v60v2, bs60, h, t, "dec-map-v60-p-nil")
		testDeepEqualErr(v60v1, v60v2, t, "equal-map-v60-p-nil")
		// ...
		if v == nil {
			v60v2 = nil
		} else {
			v60v2 = make(map[float64]uint16, len(v))
		} // reset map
		var v60v3, v60v4 typMapMapFloat64Uint16
		v60v3 = typMapMapFloat64Uint16(v60v1)
		v60v4 = typMapMapFloat64Uint16(v60v2)
		bs60 = testMarshalErr(v60v3, h, t, "enc-map-v60-custom")
		testUnmarshalErr(v60v4, bs60, h, t, "dec-map-v60-p-len")
		testDeepEqualErr(v60v3, v60v4, t, "equal-map-v60-p-len")
	}

	for _, v := range []map[float64]uint32{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v61: %v\n", v)
		var v61v1, v61v2 map[float64]uint32
		v61v1 = v
		bs61 := testMarshalErr(v61v1, h, t, "enc-map-v61")
		if v == nil {
			v61v2 = nil
		} else {
			v61v2 = make(map[float64]uint32, len(v))
		} // reset map
		testUnmarshalErr(v61v2, bs61, h, t, "dec-map-v61")
		testDeepEqualErr(v61v1, v61v2, t, "equal-map-v61")
		if v == nil {
			v61v2 = nil
		} else {
			v61v2 = make(map[float64]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v61v2), bs61, h, t, "dec-map-v61-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v61v1, v61v2, t, "equal-map-v61-noaddr")
		if v == nil {
			v61v2 = nil
		} else {
			v61v2 = make(map[float64]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v61v2, bs61, h, t, "dec-map-v61-p-len")
		testDeepEqualErr(v61v1, v61v2, t, "equal-map-v61-p-len")
		bs61 = testMarshalErr(&v61v1, h, t, "enc-map-v61-p")
		v61v2 = nil
		testUnmarshalErr(&v61v2, bs61, h, t, "dec-map-v61-p-nil")
		testDeepEqualErr(v61v1, v61v2, t, "equal-map-v61-p-nil")
		// ...
		if v == nil {
			v61v2 = nil
		} else {
			v61v2 = make(map[float64]uint32, len(v))
		} // reset map
		var v61v3, v61v4 typMapMapFloat64Uint32
		v61v3 = typMapMapFloat64Uint32(v61v1)
		v61v4 = typMapMapFloat64Uint32(v61v2)
		bs61 = testMarshalErr(v61v3, h, t, "enc-map-v61-custom")
		testUnmarshalErr(v61v4, bs61, h, t, "dec-map-v61-p-len")
		testDeepEqualErr(v61v3, v61v4, t, "equal-map-v61-p-len")
	}

	for _, v := range []map[float64]uint64{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v62: %v\n", v)
		var v62v1, v62v2 map[float64]uint64
		v62v1 = v
		bs62 := testMarshalErr(v62v1, h, t, "enc-map-v62")
		if v == nil {
			v62v2 = nil
		} else {
			v62v2 = make(map[float64]uint64, len(v))
		} // reset map
		testUnmarshalErr(v62v2, bs62, h, t, "dec-map-v62")
		testDeepEqualErr(v62v1, v62v2, t, "equal-map-v62")
		if v == nil {
			v62v2 = nil
		} else {
			v62v2 = make(map[float64]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v62v2), bs62, h, t, "dec-map-v62-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v62v1, v62v2, t, "equal-map-v62-noaddr")
		if v == nil {
			v62v2 = nil
		} else {
			v62v2 = make(map[float64]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v62v2, bs62, h, t, "dec-map-v62-p-len")
		testDeepEqualErr(v62v1, v62v2, t, "equal-map-v62-p-len")
		bs62 = testMarshalErr(&v62v1, h, t, "enc-map-v62-p")
		v62v2 = nil
		testUnmarshalErr(&v62v2, bs62, h, t, "dec-map-v62-p-nil")
		testDeepEqualErr(v62v1, v62v2, t, "equal-map-v62-p-nil")
		// ...
		if v == nil {
			v62v2 = nil
		} else {
			v62v2 = make(map[float64]uint64, len(v))
		} // reset map
		var v62v3, v62v4 typMapMapFloat64Uint64
		v62v3 = typMapMapFloat64Uint64(v62v1)
		v62v4 = typMapMapFloat64Uint64(v62v2)
		bs62 = testMarshalErr(v62v3, h, t, "enc-map-v62-custom")
		testUnmarshalErr(v62v4, bs62, h, t, "dec-map-v62-p-len")
		testDeepEqualErr(v62v3, v62v4, t, "equal-map-v62-p-len")
	}

	for _, v := range []map[float64]uintptr{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v63: %v\n", v)
		var v63v1, v63v2 map[float64]uintptr
		v63v1 = v
		bs63 := testMarshalErr(v63v1, h, t, "enc-map-v63")
		if v == nil {
			v63v2 = nil
		} else {
			v63v2 = make(map[float64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v63v2, bs63, h, t, "dec-map-v63")
		testDeepEqualErr(v63v1, v63v2, t, "equal-map-v63")
		if v == nil {
			v63v2 = nil
		} else {
			v63v2 = make(map[float64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v63v2), bs63, h, t, "dec-map-v63-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v63v1, v63v2, t, "equal-map-v63-noaddr")
		if v == nil {
			v63v2 = nil
		} else {
			v63v2 = make(map[float64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v63v2, bs63, h, t, "dec-map-v63-p-len")
		testDeepEqualErr(v63v1, v63v2, t, "equal-map-v63-p-len")
		bs63 = testMarshalErr(&v63v1, h, t, "enc-map-v63-p")
		v63v2 = nil
		testUnmarshalErr(&v63v2, bs63, h, t, "dec-map-v63-p-nil")
		testDeepEqualErr(v63v1, v63v2, t, "equal-map-v63-p-nil")
		// ...
		if v == nil {
			v63v2 = nil
		} else {
			v63v2 = make(map[float64]uintptr, len(v))
		} // reset map
		var v63v3, v63v4 typMapMapFloat64Uintptr
		v63v3 = typMapMapFloat64Uintptr(v63v1)
		v63v4 = typMapMapFloat64Uintptr(v63v2)
		bs63 = testMarshalErr(v63v3, h, t, "enc-map-v63-custom")
		testUnmarshalErr(v63v4, bs63, h, t, "dec-map-v63-p-len")
		testDeepEqualErr(v63v3, v63v4, t, "equal-map-v63-p-len")
	}

	for _, v := range []map[float64]int{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v64: %v\n", v)
		var v64v1, v64v2 map[float64]int
		v64v1 = v
		bs64 := testMarshalErr(v64v1, h, t, "enc-map-v64")
		if v == nil {
			v64v2 = nil
		} else {
			v64v2 = make(map[float64]int, len(v))
		} // reset map
		testUnmarshalErr(v64v2, bs64, h, t, "dec-map-v64")
		testDeepEqualErr(v64v1, v64v2, t, "equal-map-v64")
		if v == nil {
			v64v2 = nil
		} else {
			v64v2 = make(map[float64]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v64v2), bs64, h, t, "dec-map-v64-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v64v1, v64v2, t, "equal-map-v64-noaddr")
		if v == nil {
			v64v2 = nil
		} else {
			v64v2 = make(map[float64]int, len(v))
		} // reset map
		testUnmarshalErr(&v64v2, bs64, h, t, "dec-map-v64-p-len")
		testDeepEqualErr(v64v1, v64v2, t, "equal-map-v64-p-len")
		bs64 = testMarshalErr(&v64v1, h, t, "enc-map-v64-p")
		v64v2 = nil
		testUnmarshalErr(&v64v2, bs64, h, t, "dec-map-v64-p-nil")
		testDeepEqualErr(v64v1, v64v2, t, "equal-map-v64-p-nil")
		// ...
		if v == nil {
			v64v2 = nil
		} else {
			v64v2 = make(map[float64]int, len(v))
		} // reset map
		var v64v3, v64v4 typMapMapFloat64Int
		v64v3 = typMapMapFloat64Int(v64v1)
		v64v4 = typMapMapFloat64Int(v64v2)
		bs64 = testMarshalErr(v64v3, h, t, "enc-map-v64-custom")
		testUnmarshalErr(v64v4, bs64, h, t, "dec-map-v64-p-len")
		testDeepEqualErr(v64v3, v64v4, t, "equal-map-v64-p-len")
	}

	for _, v := range []map[float64]int8{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v65: %v\n", v)
		var v65v1, v65v2 map[float64]int8
		v65v1 = v
		bs65 := testMarshalErr(v65v1, h, t, "enc-map-v65")
		if v == nil {
			v65v2 = nil
		} else {
			v65v2 = make(map[float64]int8, len(v))
		} // reset map
		testUnmarshalErr(v65v2, bs65, h, t, "dec-map-v65")
		testDeepEqualErr(v65v1, v65v2, t, "equal-map-v65")
		if v == nil {
			v65v2 = nil
		} else {
			v65v2 = make(map[float64]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v65v2), bs65, h, t, "dec-map-v65-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v65v1, v65v2, t, "equal-map-v65-noaddr")
		if v == nil {
			v65v2 = nil
		} else {
			v65v2 = make(map[float64]int8, len(v))
		} // reset map
		testUnmarshalErr(&v65v2, bs65, h, t, "dec-map-v65-p-len")
		testDeepEqualErr(v65v1, v65v2, t, "equal-map-v65-p-len")
		bs65 = testMarshalErr(&v65v1, h, t, "enc-map-v65-p")
		v65v2 = nil
		testUnmarshalErr(&v65v2, bs65, h, t, "dec-map-v65-p-nil")
		testDeepEqualErr(v65v1, v65v2, t, "equal-map-v65-p-nil")
		// ...
		if v == nil {
			v65v2 = nil
		} else {
			v65v2 = make(map[float64]int8, len(v))
		} // reset map
		var v65v3, v65v4 typMapMapFloat64Int8
		v65v3 = typMapMapFloat64Int8(v65v1)
		v65v4 = typMapMapFloat64Int8(v65v2)
		bs65 = testMarshalErr(v65v3, h, t, "enc-map-v65-custom")
		testUnmarshalErr(v65v4, bs65, h, t, "dec-map-v65-p-len")
		testDeepEqualErr(v65v3, v65v4, t, "equal-map-v65-p-len")
	}

	for _, v := range []map[float64]int16{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v66: %v\n", v)
		var v66v1, v66v2 map[float64]int16
		v66v1 = v
		bs66 := testMarshalErr(v66v1, h, t, "enc-map-v66")
		if v == nil {
			v66v2 = nil
		} else {
			v66v2 = make(map[float64]int16, len(v))
		} // reset map
		testUnmarshalErr(v66v2, bs66, h, t, "dec-map-v66")
		testDeepEqualErr(v66v1, v66v2, t, "equal-map-v66")
		if v == nil {
			v66v2 = nil
		} else {
			v66v2 = make(map[float64]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v66v2), bs66, h, t, "dec-map-v66-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v66v1, v66v2, t, "equal-map-v66-noaddr")
		if v == nil {
			v66v2 = nil
		} else {
			v66v2 = make(map[float64]int16, len(v))
		} // reset map
		testUnmarshalErr(&v66v2, bs66, h, t, "dec-map-v66-p-len")
		testDeepEqualErr(v66v1, v66v2, t, "equal-map-v66-p-len")
		bs66 = testMarshalErr(&v66v1, h, t, "enc-map-v66-p")
		v66v2 = nil
		testUnmarshalErr(&v66v2, bs66, h, t, "dec-map-v66-p-nil")
		testDeepEqualErr(v66v1, v66v2, t, "equal-map-v66-p-nil")
		// ...
		if v == nil {
			v66v2 = nil
		} else {
			v66v2 = make(map[float64]int16, len(v))
		} // reset map
		var v66v3, v66v4 typMapMapFloat64Int16
		v66v3 = typMapMapFloat64Int16(v66v1)
		v66v4 = typMapMapFloat64Int16(v66v2)
		bs66 = testMarshalErr(v66v3, h, t, "enc-map-v66-custom")
		testUnmarshalErr(v66v4, bs66, h, t, "dec-map-v66-p-len")
		testDeepEqualErr(v66v3, v66v4, t, "equal-map-v66-p-len")
	}

	for _, v := range []map[float64]int32{nil, {}, {22.2: 0, 11.1: 44}} {
		// fmt.Printf(">>>> running mammoth map v67: %v\n", v)
		var v67v1, v67v2 map[float64]int32
		v67v1 = v
		bs67 := testMarshalErr(v67v1, h, t, "enc-map-v67")
		if v == nil {
			v67v2 = nil
		} else {
			v67v2 = make(map[float64]int32, len(v))
		} // reset map
		testUnmarshalErr(v67v2, bs67, h, t, "dec-map-v67")
		testDeepEqualErr(v67v1, v67v2, t, "equal-map-v67")
		if v == nil {
			v67v2 = nil
		} else {
			v67v2 = make(map[float64]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v67v2), bs67, h, t, "dec-map-v67-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v67v1, v67v2, t, "equal-map-v67-noaddr")
		if v == nil {
			v67v2 = nil
		} else {
			v67v2 = make(map[float64]int32, len(v))
		} // reset map
		testUnmarshalErr(&v67v2, bs67, h, t, "dec-map-v67-p-len")
		testDeepEqualErr(v67v1, v67v2, t, "equal-map-v67-p-len")
		bs67 = testMarshalErr(&v67v1, h, t, "enc-map-v67-p")
		v67v2 = nil
		testUnmarshalErr(&v67v2, bs67, h, t, "dec-map-v67-p-nil")
		testDeepEqualErr(v67v1, v67v2, t, "equal-map-v67-p-nil")
		// ...
		if v == nil {
			v67v2 = nil
		} else {
			v67v2 = make(map[float64]int32, len(v))
		} // reset map
		var v67v3, v67v4 typMapMapFloat64Int32
		v67v3 = typMapMapFloat64Int32(v67v1)
		v67v4 = typMapMapFloat64Int32(v67v2)
		bs67 = testMarshalErr(v67v3, h, t, "enc-map-v67-custom")
		testUnmarshalErr(v67v4, bs67, h, t, "dec-map-v67-p-len")
		testDeepEqualErr(v67v3, v67v4, t, "equal-map-v67-p-len")
	}

	for _, v := range []map[float64]int64{nil, {}, {22.2: 0, 11.1: 33}} {
		// fmt.Printf(">>>> running mammoth map v68: %v\n", v)
		var v68v1, v68v2 map[float64]int64
		v68v1 = v
		bs68 := testMarshalErr(v68v1, h, t, "enc-map-v68")
		if v == nil {
			v68v2 = nil
		} else {
			v68v2 = make(map[float64]int64, len(v))
		} // reset map
		testUnmarshalErr(v68v2, bs68, h, t, "dec-map-v68")
		testDeepEqualErr(v68v1, v68v2, t, "equal-map-v68")
		if v == nil {
			v68v2 = nil
		} else {
			v68v2 = make(map[float64]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v68v2), bs68, h, t, "dec-map-v68-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v68v1, v68v2, t, "equal-map-v68-noaddr")
		if v == nil {
			v68v2 = nil
		} else {
			v68v2 = make(map[float64]int64, len(v))
		} // reset map
		testUnmarshalErr(&v68v2, bs68, h, t, "dec-map-v68-p-len")
		testDeepEqualErr(v68v1, v68v2, t, "equal-map-v68-p-len")
		bs68 = testMarshalErr(&v68v1, h, t, "enc-map-v68-p")
		v68v2 = nil
		testUnmarshalErr(&v68v2, bs68, h, t, "dec-map-v68-p-nil")
		testDeepEqualErr(v68v1, v68v2, t, "equal-map-v68-p-nil")
		// ...
		if v == nil {
			v68v2 = nil
		} else {
			v68v2 = make(map[float64]int64, len(v))
		} // reset map
		var v68v3, v68v4 typMapMapFloat64Int64
		v68v3 = typMapMapFloat64Int64(v68v1)
		v68v4 = typMapMapFloat64Int64(v68v2)
		bs68 = testMarshalErr(v68v3, h, t, "enc-map-v68-custom")
		testUnmarshalErr(v68v4, bs68, h, t, "dec-map-v68-p-len")
		testDeepEqualErr(v68v3, v68v4, t, "equal-map-v68-p-len")
	}

	for _, v := range []map[float64]float32{nil, {}, {22.2: 0, 11.1: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v69: %v\n", v)
		var v69v1, v69v2 map[float64]float32
		v69v1 = v
		bs69 := testMarshalErr(v69v1, h, t, "enc-map-v69")
		if v == nil {
			v69v2 = nil
		} else {
			v69v2 = make(map[float64]float32, len(v))
		} // reset map
		testUnmarshalErr(v69v2, bs69, h, t, "dec-map-v69")
		testDeepEqualErr(v69v1, v69v2, t, "equal-map-v69")
		if v == nil {
			v69v2 = nil
		} else {
			v69v2 = make(map[float64]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v69v2), bs69, h, t, "dec-map-v69-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v69v1, v69v2, t, "equal-map-v69-noaddr")
		if v == nil {
			v69v2 = nil
		} else {
			v69v2 = make(map[float64]float32, len(v))
		} // reset map
		testUnmarshalErr(&v69v2, bs69, h, t, "dec-map-v69-p-len")
		testDeepEqualErr(v69v1, v69v2, t, "equal-map-v69-p-len")
		bs69 = testMarshalErr(&v69v1, h, t, "enc-map-v69-p")
		v69v2 = nil
		testUnmarshalErr(&v69v2, bs69, h, t, "dec-map-v69-p-nil")
		testDeepEqualErr(v69v1, v69v2, t, "equal-map-v69-p-nil")
		// ...
		if v == nil {
			v69v2 = nil
		} else {
			v69v2 = make(map[float64]float32, len(v))
		} // reset map
		var v69v3, v69v4 typMapMapFloat64Float32
		v69v3 = typMapMapFloat64Float32(v69v1)
		v69v4 = typMapMapFloat64Float32(v69v2)
		bs69 = testMarshalErr(v69v3, h, t, "enc-map-v69-custom")
		testUnmarshalErr(v69v4, bs69, h, t, "dec-map-v69-p-len")
		testDeepEqualErr(v69v3, v69v4, t, "equal-map-v69-p-len")
	}

	for _, v := range []map[float64]float64{nil, {}, {11.1: 0, 22.2: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v70: %v\n", v)
		var v70v1, v70v2 map[float64]float64
		v70v1 = v
		bs70 := testMarshalErr(v70v1, h, t, "enc-map-v70")
		if v == nil {
			v70v2 = nil
		} else {
			v70v2 = make(map[float64]float64, len(v))
		} // reset map
		testUnmarshalErr(v70v2, bs70, h, t, "dec-map-v70")
		testDeepEqualErr(v70v1, v70v2, t, "equal-map-v70")
		if v == nil {
			v70v2 = nil
		} else {
			v70v2 = make(map[float64]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v70v2), bs70, h, t, "dec-map-v70-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v70v1, v70v2, t, "equal-map-v70-noaddr")
		if v == nil {
			v70v2 = nil
		} else {
			v70v2 = make(map[float64]float64, len(v))
		} // reset map
		testUnmarshalErr(&v70v2, bs70, h, t, "dec-map-v70-p-len")
		testDeepEqualErr(v70v1, v70v2, t, "equal-map-v70-p-len")
		bs70 = testMarshalErr(&v70v1, h, t, "enc-map-v70-p")
		v70v2 = nil
		testUnmarshalErr(&v70v2, bs70, h, t, "dec-map-v70-p-nil")
		testDeepEqualErr(v70v1, v70v2, t, "equal-map-v70-p-nil")
		// ...
		if v == nil {
			v70v2 = nil
		} else {
			v70v2 = make(map[float64]float64, len(v))
		} // reset map
		var v70v3, v70v4 typMapMapFloat64Float64
		v70v3 = typMapMapFloat64Float64(v70v1)
		v70v4 = typMapMapFloat64Float64(v70v2)
		bs70 = testMarshalErr(v70v3, h, t, "enc-map-v70-custom")
		testUnmarshalErr(v70v4, bs70, h, t, "dec-map-v70-p-len")
		testDeepEqualErr(v70v3, v70v4, t, "equal-map-v70-p-len")
	}

	for _, v := range []map[float64]bool{nil, {}, {22.2: false, 11.1: true}} {
		// fmt.Printf(">>>> running mammoth map v71: %v\n", v)
		var v71v1, v71v2 map[float64]bool
		v71v1 = v
		bs71 := testMarshalErr(v71v1, h, t, "enc-map-v71")
		if v == nil {
			v71v2 = nil
		} else {
			v71v2 = make(map[float64]bool, len(v))
		} // reset map
		testUnmarshalErr(v71v2, bs71, h, t, "dec-map-v71")
		testDeepEqualErr(v71v1, v71v2, t, "equal-map-v71")
		if v == nil {
			v71v2 = nil
		} else {
			v71v2 = make(map[float64]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v71v2), bs71, h, t, "dec-map-v71-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v71v1, v71v2, t, "equal-map-v71-noaddr")
		if v == nil {
			v71v2 = nil
		} else {
			v71v2 = make(map[float64]bool, len(v))
		} // reset map
		testUnmarshalErr(&v71v2, bs71, h, t, "dec-map-v71-p-len")
		testDeepEqualErr(v71v1, v71v2, t, "equal-map-v71-p-len")
		bs71 = testMarshalErr(&v71v1, h, t, "enc-map-v71-p")
		v71v2 = nil
		testUnmarshalErr(&v71v2, bs71, h, t, "dec-map-v71-p-nil")
		testDeepEqualErr(v71v1, v71v2, t, "equal-map-v71-p-nil")
		// ...
		if v == nil {
			v71v2 = nil
		} else {
			v71v2 = make(map[float64]bool, len(v))
		} // reset map
		var v71v3, v71v4 typMapMapFloat64Bool
		v71v3 = typMapMapFloat64Bool(v71v1)
		v71v4 = typMapMapFloat64Bool(v71v2)
		bs71 = testMarshalErr(v71v3, h, t, "enc-map-v71-custom")
		testUnmarshalErr(v71v4, bs71, h, t, "dec-map-v71-p-len")
		testDeepEqualErr(v71v3, v71v4, t, "equal-map-v71-p-len")
	}

	for _, v := range []map[uint]interface{}{nil, {}, {44: nil, 33: "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v74: %v\n", v)
		var v74v1, v74v2 map[uint]interface{}
		v74v1 = v
		bs74 := testMarshalErr(v74v1, h, t, "enc-map-v74")
		if v == nil {
			v74v2 = nil
		} else {
			v74v2 = make(map[uint]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v74v2, bs74, h, t, "dec-map-v74")
		testDeepEqualErr(v74v1, v74v2, t, "equal-map-v74")
		if v == nil {
			v74v2 = nil
		} else {
			v74v2 = make(map[uint]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v74v2), bs74, h, t, "dec-map-v74-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v74v1, v74v2, t, "equal-map-v74-noaddr")
		if v == nil {
			v74v2 = nil
		} else {
			v74v2 = make(map[uint]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v74v2, bs74, h, t, "dec-map-v74-p-len")
		testDeepEqualErr(v74v1, v74v2, t, "equal-map-v74-p-len")
		bs74 = testMarshalErr(&v74v1, h, t, "enc-map-v74-p")
		v74v2 = nil
		testUnmarshalErr(&v74v2, bs74, h, t, "dec-map-v74-p-nil")
		testDeepEqualErr(v74v1, v74v2, t, "equal-map-v74-p-nil")
		// ...
		if v == nil {
			v74v2 = nil
		} else {
			v74v2 = make(map[uint]interface{}, len(v))
		} // reset map
		var v74v3, v74v4 typMapMapUintIntf
		v74v3 = typMapMapUintIntf(v74v1)
		v74v4 = typMapMapUintIntf(v74v2)
		bs74 = testMarshalErr(v74v3, h, t, "enc-map-v74-custom")
		testUnmarshalErr(v74v4, bs74, h, t, "dec-map-v74-p-len")
		testDeepEqualErr(v74v3, v74v4, t, "equal-map-v74-p-len")
	}

	for _, v := range []map[uint]string{nil, {}, {44: "", 33: "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v75: %v\n", v)
		var v75v1, v75v2 map[uint]string
		v75v1 = v
		bs75 := testMarshalErr(v75v1, h, t, "enc-map-v75")
		if v == nil {
			v75v2 = nil
		} else {
			v75v2 = make(map[uint]string, len(v))
		} // reset map
		testUnmarshalErr(v75v2, bs75, h, t, "dec-map-v75")
		testDeepEqualErr(v75v1, v75v2, t, "equal-map-v75")
		if v == nil {
			v75v2 = nil
		} else {
			v75v2 = make(map[uint]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v75v2), bs75, h, t, "dec-map-v75-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v75v1, v75v2, t, "equal-map-v75-noaddr")
		if v == nil {
			v75v2 = nil
		} else {
			v75v2 = make(map[uint]string, len(v))
		} // reset map
		testUnmarshalErr(&v75v2, bs75, h, t, "dec-map-v75-p-len")
		testDeepEqualErr(v75v1, v75v2, t, "equal-map-v75-p-len")
		bs75 = testMarshalErr(&v75v1, h, t, "enc-map-v75-p")
		v75v2 = nil
		testUnmarshalErr(&v75v2, bs75, h, t, "dec-map-v75-p-nil")
		testDeepEqualErr(v75v1, v75v2, t, "equal-map-v75-p-nil")
		// ...
		if v == nil {
			v75v2 = nil
		} else {
			v75v2 = make(map[uint]string, len(v))
		} // reset map
		var v75v3, v75v4 typMapMapUintString
		v75v3 = typMapMapUintString(v75v1)
		v75v4 = typMapMapUintString(v75v2)
		bs75 = testMarshalErr(v75v3, h, t, "enc-map-v75-custom")
		testUnmarshalErr(v75v4, bs75, h, t, "dec-map-v75-p-len")
		testDeepEqualErr(v75v3, v75v4, t, "equal-map-v75-p-len")
	}

	for _, v := range []map[uint]uint{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v76: %v\n", v)
		var v76v1, v76v2 map[uint]uint
		v76v1 = v
		bs76 := testMarshalErr(v76v1, h, t, "enc-map-v76")
		if v == nil {
			v76v2 = nil
		} else {
			v76v2 = make(map[uint]uint, len(v))
		} // reset map
		testUnmarshalErr(v76v2, bs76, h, t, "dec-map-v76")
		testDeepEqualErr(v76v1, v76v2, t, "equal-map-v76")
		if v == nil {
			v76v2 = nil
		} else {
			v76v2 = make(map[uint]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v76v2), bs76, h, t, "dec-map-v76-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v76v1, v76v2, t, "equal-map-v76-noaddr")
		if v == nil {
			v76v2 = nil
		} else {
			v76v2 = make(map[uint]uint, len(v))
		} // reset map
		testUnmarshalErr(&v76v2, bs76, h, t, "dec-map-v76-p-len")
		testDeepEqualErr(v76v1, v76v2, t, "equal-map-v76-p-len")
		bs76 = testMarshalErr(&v76v1, h, t, "enc-map-v76-p")
		v76v2 = nil
		testUnmarshalErr(&v76v2, bs76, h, t, "dec-map-v76-p-nil")
		testDeepEqualErr(v76v1, v76v2, t, "equal-map-v76-p-nil")
		// ...
		if v == nil {
			v76v2 = nil
		} else {
			v76v2 = make(map[uint]uint, len(v))
		} // reset map
		var v76v3, v76v4 typMapMapUintUint
		v76v3 = typMapMapUintUint(v76v1)
		v76v4 = typMapMapUintUint(v76v2)
		bs76 = testMarshalErr(v76v3, h, t, "enc-map-v76-custom")
		testUnmarshalErr(v76v4, bs76, h, t, "dec-map-v76-p-len")
		testDeepEqualErr(v76v3, v76v4, t, "equal-map-v76-p-len")
	}

	for _, v := range []map[uint]uint8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v77: %v\n", v)
		var v77v1, v77v2 map[uint]uint8
		v77v1 = v
		bs77 := testMarshalErr(v77v1, h, t, "enc-map-v77")
		if v == nil {
			v77v2 = nil
		} else {
			v77v2 = make(map[uint]uint8, len(v))
		} // reset map
		testUnmarshalErr(v77v2, bs77, h, t, "dec-map-v77")
		testDeepEqualErr(v77v1, v77v2, t, "equal-map-v77")
		if v == nil {
			v77v2 = nil
		} else {
			v77v2 = make(map[uint]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v77v2), bs77, h, t, "dec-map-v77-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v77v1, v77v2, t, "equal-map-v77-noaddr")
		if v == nil {
			v77v2 = nil
		} else {
			v77v2 = make(map[uint]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v77v2, bs77, h, t, "dec-map-v77-p-len")
		testDeepEqualErr(v77v1, v77v2, t, "equal-map-v77-p-len")
		bs77 = testMarshalErr(&v77v1, h, t, "enc-map-v77-p")
		v77v2 = nil
		testUnmarshalErr(&v77v2, bs77, h, t, "dec-map-v77-p-nil")
		testDeepEqualErr(v77v1, v77v2, t, "equal-map-v77-p-nil")
		// ...
		if v == nil {
			v77v2 = nil
		} else {
			v77v2 = make(map[uint]uint8, len(v))
		} // reset map
		var v77v3, v77v4 typMapMapUintUint8
		v77v3 = typMapMapUintUint8(v77v1)
		v77v4 = typMapMapUintUint8(v77v2)
		bs77 = testMarshalErr(v77v3, h, t, "enc-map-v77-custom")
		testUnmarshalErr(v77v4, bs77, h, t, "dec-map-v77-p-len")
		testDeepEqualErr(v77v3, v77v4, t, "equal-map-v77-p-len")
	}

	for _, v := range []map[uint]uint16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v78: %v\n", v)
		var v78v1, v78v2 map[uint]uint16
		v78v1 = v
		bs78 := testMarshalErr(v78v1, h, t, "enc-map-v78")
		if v == nil {
			v78v2 = nil
		} else {
			v78v2 = make(map[uint]uint16, len(v))
		} // reset map
		testUnmarshalErr(v78v2, bs78, h, t, "dec-map-v78")
		testDeepEqualErr(v78v1, v78v2, t, "equal-map-v78")
		if v == nil {
			v78v2 = nil
		} else {
			v78v2 = make(map[uint]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v78v2), bs78, h, t, "dec-map-v78-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v78v1, v78v2, t, "equal-map-v78-noaddr")
		if v == nil {
			v78v2 = nil
		} else {
			v78v2 = make(map[uint]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v78v2, bs78, h, t, "dec-map-v78-p-len")
		testDeepEqualErr(v78v1, v78v2, t, "equal-map-v78-p-len")
		bs78 = testMarshalErr(&v78v1, h, t, "enc-map-v78-p")
		v78v2 = nil
		testUnmarshalErr(&v78v2, bs78, h, t, "dec-map-v78-p-nil")
		testDeepEqualErr(v78v1, v78v2, t, "equal-map-v78-p-nil")
		// ...
		if v == nil {
			v78v2 = nil
		} else {
			v78v2 = make(map[uint]uint16, len(v))
		} // reset map
		var v78v3, v78v4 typMapMapUintUint16
		v78v3 = typMapMapUintUint16(v78v1)
		v78v4 = typMapMapUintUint16(v78v2)
		bs78 = testMarshalErr(v78v3, h, t, "enc-map-v78-custom")
		testUnmarshalErr(v78v4, bs78, h, t, "dec-map-v78-p-len")
		testDeepEqualErr(v78v3, v78v4, t, "equal-map-v78-p-len")
	}

	for _, v := range []map[uint]uint32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v79: %v\n", v)
		var v79v1, v79v2 map[uint]uint32
		v79v1 = v
		bs79 := testMarshalErr(v79v1, h, t, "enc-map-v79")
		if v == nil {
			v79v2 = nil
		} else {
			v79v2 = make(map[uint]uint32, len(v))
		} // reset map
		testUnmarshalErr(v79v2, bs79, h, t, "dec-map-v79")
		testDeepEqualErr(v79v1, v79v2, t, "equal-map-v79")
		if v == nil {
			v79v2 = nil
		} else {
			v79v2 = make(map[uint]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v79v2), bs79, h, t, "dec-map-v79-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v79v1, v79v2, t, "equal-map-v79-noaddr")
		if v == nil {
			v79v2 = nil
		} else {
			v79v2 = make(map[uint]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v79v2, bs79, h, t, "dec-map-v79-p-len")
		testDeepEqualErr(v79v1, v79v2, t, "equal-map-v79-p-len")
		bs79 = testMarshalErr(&v79v1, h, t, "enc-map-v79-p")
		v79v2 = nil
		testUnmarshalErr(&v79v2, bs79, h, t, "dec-map-v79-p-nil")
		testDeepEqualErr(v79v1, v79v2, t, "equal-map-v79-p-nil")
		// ...
		if v == nil {
			v79v2 = nil
		} else {
			v79v2 = make(map[uint]uint32, len(v))
		} // reset map
		var v79v3, v79v4 typMapMapUintUint32
		v79v3 = typMapMapUintUint32(v79v1)
		v79v4 = typMapMapUintUint32(v79v2)
		bs79 = testMarshalErr(v79v3, h, t, "enc-map-v79-custom")
		testUnmarshalErr(v79v4, bs79, h, t, "dec-map-v79-p-len")
		testDeepEqualErr(v79v3, v79v4, t, "equal-map-v79-p-len")
	}

	for _, v := range []map[uint]uint64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v80: %v\n", v)
		var v80v1, v80v2 map[uint]uint64
		v80v1 = v
		bs80 := testMarshalErr(v80v1, h, t, "enc-map-v80")
		if v == nil {
			v80v2 = nil
		} else {
			v80v2 = make(map[uint]uint64, len(v))
		} // reset map
		testUnmarshalErr(v80v2, bs80, h, t, "dec-map-v80")
		testDeepEqualErr(v80v1, v80v2, t, "equal-map-v80")
		if v == nil {
			v80v2 = nil
		} else {
			v80v2 = make(map[uint]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v80v2), bs80, h, t, "dec-map-v80-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v80v1, v80v2, t, "equal-map-v80-noaddr")
		if v == nil {
			v80v2 = nil
		} else {
			v80v2 = make(map[uint]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v80v2, bs80, h, t, "dec-map-v80-p-len")
		testDeepEqualErr(v80v1, v80v2, t, "equal-map-v80-p-len")
		bs80 = testMarshalErr(&v80v1, h, t, "enc-map-v80-p")
		v80v2 = nil
		testUnmarshalErr(&v80v2, bs80, h, t, "dec-map-v80-p-nil")
		testDeepEqualErr(v80v1, v80v2, t, "equal-map-v80-p-nil")
		// ...
		if v == nil {
			v80v2 = nil
		} else {
			v80v2 = make(map[uint]uint64, len(v))
		} // reset map
		var v80v3, v80v4 typMapMapUintUint64
		v80v3 = typMapMapUintUint64(v80v1)
		v80v4 = typMapMapUintUint64(v80v2)
		bs80 = testMarshalErr(v80v3, h, t, "enc-map-v80-custom")
		testUnmarshalErr(v80v4, bs80, h, t, "dec-map-v80-p-len")
		testDeepEqualErr(v80v3, v80v4, t, "equal-map-v80-p-len")
	}

	for _, v := range []map[uint]uintptr{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v81: %v\n", v)
		var v81v1, v81v2 map[uint]uintptr
		v81v1 = v
		bs81 := testMarshalErr(v81v1, h, t, "enc-map-v81")
		if v == nil {
			v81v2 = nil
		} else {
			v81v2 = make(map[uint]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v81v2, bs81, h, t, "dec-map-v81")
		testDeepEqualErr(v81v1, v81v2, t, "equal-map-v81")
		if v == nil {
			v81v2 = nil
		} else {
			v81v2 = make(map[uint]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v81v2), bs81, h, t, "dec-map-v81-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v81v1, v81v2, t, "equal-map-v81-noaddr")
		if v == nil {
			v81v2 = nil
		} else {
			v81v2 = make(map[uint]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v81v2, bs81, h, t, "dec-map-v81-p-len")
		testDeepEqualErr(v81v1, v81v2, t, "equal-map-v81-p-len")
		bs81 = testMarshalErr(&v81v1, h, t, "enc-map-v81-p")
		v81v2 = nil
		testUnmarshalErr(&v81v2, bs81, h, t, "dec-map-v81-p-nil")
		testDeepEqualErr(v81v1, v81v2, t, "equal-map-v81-p-nil")
		// ...
		if v == nil {
			v81v2 = nil
		} else {
			v81v2 = make(map[uint]uintptr, len(v))
		} // reset map
		var v81v3, v81v4 typMapMapUintUintptr
		v81v3 = typMapMapUintUintptr(v81v1)
		v81v4 = typMapMapUintUintptr(v81v2)
		bs81 = testMarshalErr(v81v3, h, t, "enc-map-v81-custom")
		testUnmarshalErr(v81v4, bs81, h, t, "dec-map-v81-p-len")
		testDeepEqualErr(v81v3, v81v4, t, "equal-map-v81-p-len")
	}

	for _, v := range []map[uint]int{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v82: %v\n", v)
		var v82v1, v82v2 map[uint]int
		v82v1 = v
		bs82 := testMarshalErr(v82v1, h, t, "enc-map-v82")
		if v == nil {
			v82v2 = nil
		} else {
			v82v2 = make(map[uint]int, len(v))
		} // reset map
		testUnmarshalErr(v82v2, bs82, h, t, "dec-map-v82")
		testDeepEqualErr(v82v1, v82v2, t, "equal-map-v82")
		if v == nil {
			v82v2 = nil
		} else {
			v82v2 = make(map[uint]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v82v2), bs82, h, t, "dec-map-v82-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v82v1, v82v2, t, "equal-map-v82-noaddr")
		if v == nil {
			v82v2 = nil
		} else {
			v82v2 = make(map[uint]int, len(v))
		} // reset map
		testUnmarshalErr(&v82v2, bs82, h, t, "dec-map-v82-p-len")
		testDeepEqualErr(v82v1, v82v2, t, "equal-map-v82-p-len")
		bs82 = testMarshalErr(&v82v1, h, t, "enc-map-v82-p")
		v82v2 = nil
		testUnmarshalErr(&v82v2, bs82, h, t, "dec-map-v82-p-nil")
		testDeepEqualErr(v82v1, v82v2, t, "equal-map-v82-p-nil")
		// ...
		if v == nil {
			v82v2 = nil
		} else {
			v82v2 = make(map[uint]int, len(v))
		} // reset map
		var v82v3, v82v4 typMapMapUintInt
		v82v3 = typMapMapUintInt(v82v1)
		v82v4 = typMapMapUintInt(v82v2)
		bs82 = testMarshalErr(v82v3, h, t, "enc-map-v82-custom")
		testUnmarshalErr(v82v4, bs82, h, t, "dec-map-v82-p-len")
		testDeepEqualErr(v82v3, v82v4, t, "equal-map-v82-p-len")
	}

	for _, v := range []map[uint]int8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v83: %v\n", v)
		var v83v1, v83v2 map[uint]int8
		v83v1 = v
		bs83 := testMarshalErr(v83v1, h, t, "enc-map-v83")
		if v == nil {
			v83v2 = nil
		} else {
			v83v2 = make(map[uint]int8, len(v))
		} // reset map
		testUnmarshalErr(v83v2, bs83, h, t, "dec-map-v83")
		testDeepEqualErr(v83v1, v83v2, t, "equal-map-v83")
		if v == nil {
			v83v2 = nil
		} else {
			v83v2 = make(map[uint]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v83v2), bs83, h, t, "dec-map-v83-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v83v1, v83v2, t, "equal-map-v83-noaddr")
		if v == nil {
			v83v2 = nil
		} else {
			v83v2 = make(map[uint]int8, len(v))
		} // reset map
		testUnmarshalErr(&v83v2, bs83, h, t, "dec-map-v83-p-len")
		testDeepEqualErr(v83v1, v83v2, t, "equal-map-v83-p-len")
		bs83 = testMarshalErr(&v83v1, h, t, "enc-map-v83-p")
		v83v2 = nil
		testUnmarshalErr(&v83v2, bs83, h, t, "dec-map-v83-p-nil")
		testDeepEqualErr(v83v1, v83v2, t, "equal-map-v83-p-nil")
		// ...
		if v == nil {
			v83v2 = nil
		} else {
			v83v2 = make(map[uint]int8, len(v))
		} // reset map
		var v83v3, v83v4 typMapMapUintInt8
		v83v3 = typMapMapUintInt8(v83v1)
		v83v4 = typMapMapUintInt8(v83v2)
		bs83 = testMarshalErr(v83v3, h, t, "enc-map-v83-custom")
		testUnmarshalErr(v83v4, bs83, h, t, "dec-map-v83-p-len")
		testDeepEqualErr(v83v3, v83v4, t, "equal-map-v83-p-len")
	}

	for _, v := range []map[uint]int16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v84: %v\n", v)
		var v84v1, v84v2 map[uint]int16
		v84v1 = v
		bs84 := testMarshalErr(v84v1, h, t, "enc-map-v84")
		if v == nil {
			v84v2 = nil
		} else {
			v84v2 = make(map[uint]int16, len(v))
		} // reset map
		testUnmarshalErr(v84v2, bs84, h, t, "dec-map-v84")
		testDeepEqualErr(v84v1, v84v2, t, "equal-map-v84")
		if v == nil {
			v84v2 = nil
		} else {
			v84v2 = make(map[uint]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v84v2), bs84, h, t, "dec-map-v84-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v84v1, v84v2, t, "equal-map-v84-noaddr")
		if v == nil {
			v84v2 = nil
		} else {
			v84v2 = make(map[uint]int16, len(v))
		} // reset map
		testUnmarshalErr(&v84v2, bs84, h, t, "dec-map-v84-p-len")
		testDeepEqualErr(v84v1, v84v2, t, "equal-map-v84-p-len")
		bs84 = testMarshalErr(&v84v1, h, t, "enc-map-v84-p")
		v84v2 = nil
		testUnmarshalErr(&v84v2, bs84, h, t, "dec-map-v84-p-nil")
		testDeepEqualErr(v84v1, v84v2, t, "equal-map-v84-p-nil")
		// ...
		if v == nil {
			v84v2 = nil
		} else {
			v84v2 = make(map[uint]int16, len(v))
		} // reset map
		var v84v3, v84v4 typMapMapUintInt16
		v84v3 = typMapMapUintInt16(v84v1)
		v84v4 = typMapMapUintInt16(v84v2)
		bs84 = testMarshalErr(v84v3, h, t, "enc-map-v84-custom")
		testUnmarshalErr(v84v4, bs84, h, t, "dec-map-v84-p-len")
		testDeepEqualErr(v84v3, v84v4, t, "equal-map-v84-p-len")
	}

	for _, v := range []map[uint]int32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v85: %v\n", v)
		var v85v1, v85v2 map[uint]int32
		v85v1 = v
		bs85 := testMarshalErr(v85v1, h, t, "enc-map-v85")
		if v == nil {
			v85v2 = nil
		} else {
			v85v2 = make(map[uint]int32, len(v))
		} // reset map
		testUnmarshalErr(v85v2, bs85, h, t, "dec-map-v85")
		testDeepEqualErr(v85v1, v85v2, t, "equal-map-v85")
		if v == nil {
			v85v2 = nil
		} else {
			v85v2 = make(map[uint]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v85v2), bs85, h, t, "dec-map-v85-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v85v1, v85v2, t, "equal-map-v85-noaddr")
		if v == nil {
			v85v2 = nil
		} else {
			v85v2 = make(map[uint]int32, len(v))
		} // reset map
		testUnmarshalErr(&v85v2, bs85, h, t, "dec-map-v85-p-len")
		testDeepEqualErr(v85v1, v85v2, t, "equal-map-v85-p-len")
		bs85 = testMarshalErr(&v85v1, h, t, "enc-map-v85-p")
		v85v2 = nil
		testUnmarshalErr(&v85v2, bs85, h, t, "dec-map-v85-p-nil")
		testDeepEqualErr(v85v1, v85v2, t, "equal-map-v85-p-nil")
		// ...
		if v == nil {
			v85v2 = nil
		} else {
			v85v2 = make(map[uint]int32, len(v))
		} // reset map
		var v85v3, v85v4 typMapMapUintInt32
		v85v3 = typMapMapUintInt32(v85v1)
		v85v4 = typMapMapUintInt32(v85v2)
		bs85 = testMarshalErr(v85v3, h, t, "enc-map-v85-custom")
		testUnmarshalErr(v85v4, bs85, h, t, "dec-map-v85-p-len")
		testDeepEqualErr(v85v3, v85v4, t, "equal-map-v85-p-len")
	}

	for _, v := range []map[uint]int64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v86: %v\n", v)
		var v86v1, v86v2 map[uint]int64
		v86v1 = v
		bs86 := testMarshalErr(v86v1, h, t, "enc-map-v86")
		if v == nil {
			v86v2 = nil
		} else {
			v86v2 = make(map[uint]int64, len(v))
		} // reset map
		testUnmarshalErr(v86v2, bs86, h, t, "dec-map-v86")
		testDeepEqualErr(v86v1, v86v2, t, "equal-map-v86")
		if v == nil {
			v86v2 = nil
		} else {
			v86v2 = make(map[uint]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v86v2), bs86, h, t, "dec-map-v86-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v86v1, v86v2, t, "equal-map-v86-noaddr")
		if v == nil {
			v86v2 = nil
		} else {
			v86v2 = make(map[uint]int64, len(v))
		} // reset map
		testUnmarshalErr(&v86v2, bs86, h, t, "dec-map-v86-p-len")
		testDeepEqualErr(v86v1, v86v2, t, "equal-map-v86-p-len")
		bs86 = testMarshalErr(&v86v1, h, t, "enc-map-v86-p")
		v86v2 = nil
		testUnmarshalErr(&v86v2, bs86, h, t, "dec-map-v86-p-nil")
		testDeepEqualErr(v86v1, v86v2, t, "equal-map-v86-p-nil")
		// ...
		if v == nil {
			v86v2 = nil
		} else {
			v86v2 = make(map[uint]int64, len(v))
		} // reset map
		var v86v3, v86v4 typMapMapUintInt64
		v86v3 = typMapMapUintInt64(v86v1)
		v86v4 = typMapMapUintInt64(v86v2)
		bs86 = testMarshalErr(v86v3, h, t, "enc-map-v86-custom")
		testUnmarshalErr(v86v4, bs86, h, t, "dec-map-v86-p-len")
		testDeepEqualErr(v86v3, v86v4, t, "equal-map-v86-p-len")
	}

	for _, v := range []map[uint]float32{nil, {}, {33: 0, 44: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v87: %v\n", v)
		var v87v1, v87v2 map[uint]float32
		v87v1 = v
		bs87 := testMarshalErr(v87v1, h, t, "enc-map-v87")
		if v == nil {
			v87v2 = nil
		} else {
			v87v2 = make(map[uint]float32, len(v))
		} // reset map
		testUnmarshalErr(v87v2, bs87, h, t, "dec-map-v87")
		testDeepEqualErr(v87v1, v87v2, t, "equal-map-v87")
		if v == nil {
			v87v2 = nil
		} else {
			v87v2 = make(map[uint]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v87v2), bs87, h, t, "dec-map-v87-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v87v1, v87v2, t, "equal-map-v87-noaddr")
		if v == nil {
			v87v2 = nil
		} else {
			v87v2 = make(map[uint]float32, len(v))
		} // reset map
		testUnmarshalErr(&v87v2, bs87, h, t, "dec-map-v87-p-len")
		testDeepEqualErr(v87v1, v87v2, t, "equal-map-v87-p-len")
		bs87 = testMarshalErr(&v87v1, h, t, "enc-map-v87-p")
		v87v2 = nil
		testUnmarshalErr(&v87v2, bs87, h, t, "dec-map-v87-p-nil")
		testDeepEqualErr(v87v1, v87v2, t, "equal-map-v87-p-nil")
		// ...
		if v == nil {
			v87v2 = nil
		} else {
			v87v2 = make(map[uint]float32, len(v))
		} // reset map
		var v87v3, v87v4 typMapMapUintFloat32
		v87v3 = typMapMapUintFloat32(v87v1)
		v87v4 = typMapMapUintFloat32(v87v2)
		bs87 = testMarshalErr(v87v3, h, t, "enc-map-v87-custom")
		testUnmarshalErr(v87v4, bs87, h, t, "dec-map-v87-p-len")
		testDeepEqualErr(v87v3, v87v4, t, "equal-map-v87-p-len")
	}

	for _, v := range []map[uint]float64{nil, {}, {33: 0, 44: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v88: %v\n", v)
		var v88v1, v88v2 map[uint]float64
		v88v1 = v
		bs88 := testMarshalErr(v88v1, h, t, "enc-map-v88")
		if v == nil {
			v88v2 = nil
		} else {
			v88v2 = make(map[uint]float64, len(v))
		} // reset map
		testUnmarshalErr(v88v2, bs88, h, t, "dec-map-v88")
		testDeepEqualErr(v88v1, v88v2, t, "equal-map-v88")
		if v == nil {
			v88v2 = nil
		} else {
			v88v2 = make(map[uint]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v88v2), bs88, h, t, "dec-map-v88-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v88v1, v88v2, t, "equal-map-v88-noaddr")
		if v == nil {
			v88v2 = nil
		} else {
			v88v2 = make(map[uint]float64, len(v))
		} // reset map
		testUnmarshalErr(&v88v2, bs88, h, t, "dec-map-v88-p-len")
		testDeepEqualErr(v88v1, v88v2, t, "equal-map-v88-p-len")
		bs88 = testMarshalErr(&v88v1, h, t, "enc-map-v88-p")
		v88v2 = nil
		testUnmarshalErr(&v88v2, bs88, h, t, "dec-map-v88-p-nil")
		testDeepEqualErr(v88v1, v88v2, t, "equal-map-v88-p-nil")
		// ...
		if v == nil {
			v88v2 = nil
		} else {
			v88v2 = make(map[uint]float64, len(v))
		} // reset map
		var v88v3, v88v4 typMapMapUintFloat64
		v88v3 = typMapMapUintFloat64(v88v1)
		v88v4 = typMapMapUintFloat64(v88v2)
		bs88 = testMarshalErr(v88v3, h, t, "enc-map-v88-custom")
		testUnmarshalErr(v88v4, bs88, h, t, "dec-map-v88-p-len")
		testDeepEqualErr(v88v3, v88v4, t, "equal-map-v88-p-len")
	}

	for _, v := range []map[uint]bool{nil, {}, {33: false, 44: true}} {
		// fmt.Printf(">>>> running mammoth map v89: %v\n", v)
		var v89v1, v89v2 map[uint]bool
		v89v1 = v
		bs89 := testMarshalErr(v89v1, h, t, "enc-map-v89")
		if v == nil {
			v89v2 = nil
		} else {
			v89v2 = make(map[uint]bool, len(v))
		} // reset map
		testUnmarshalErr(v89v2, bs89, h, t, "dec-map-v89")
		testDeepEqualErr(v89v1, v89v2, t, "equal-map-v89")
		if v == nil {
			v89v2 = nil
		} else {
			v89v2 = make(map[uint]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v89v2), bs89, h, t, "dec-map-v89-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v89v1, v89v2, t, "equal-map-v89-noaddr")
		if v == nil {
			v89v2 = nil
		} else {
			v89v2 = make(map[uint]bool, len(v))
		} // reset map
		testUnmarshalErr(&v89v2, bs89, h, t, "dec-map-v89-p-len")
		testDeepEqualErr(v89v1, v89v2, t, "equal-map-v89-p-len")
		bs89 = testMarshalErr(&v89v1, h, t, "enc-map-v89-p")
		v89v2 = nil
		testUnmarshalErr(&v89v2, bs89, h, t, "dec-map-v89-p-nil")
		testDeepEqualErr(v89v1, v89v2, t, "equal-map-v89-p-nil")
		// ...
		if v == nil {
			v89v2 = nil
		} else {
			v89v2 = make(map[uint]bool, len(v))
		} // reset map
		var v89v3, v89v4 typMapMapUintBool
		v89v3 = typMapMapUintBool(v89v1)
		v89v4 = typMapMapUintBool(v89v2)
		bs89 = testMarshalErr(v89v3, h, t, "enc-map-v89-custom")
		testUnmarshalErr(v89v4, bs89, h, t, "dec-map-v89-p-len")
		testDeepEqualErr(v89v3, v89v4, t, "equal-map-v89-p-len")
	}

	for _, v := range []map[uint8]interface{}{nil, {}, {33: nil, 44: "string-is-an-interface"}} {
		// fmt.Printf(">>>> running mammoth map v92: %v\n", v)
		var v92v1, v92v2 map[uint8]interface{}
		v92v1 = v
		bs92 := testMarshalErr(v92v1, h, t, "enc-map-v92")
		if v == nil {
			v92v2 = nil
		} else {
			v92v2 = make(map[uint8]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v92v2, bs92, h, t, "dec-map-v92")
		testDeepEqualErr(v92v1, v92v2, t, "equal-map-v92")
		if v == nil {
			v92v2 = nil
		} else {
			v92v2 = make(map[uint8]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v92v2), bs92, h, t, "dec-map-v92-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v92v1, v92v2, t, "equal-map-v92-noaddr")
		if v == nil {
			v92v2 = nil
		} else {
			v92v2 = make(map[uint8]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v92v2, bs92, h, t, "dec-map-v92-p-len")
		testDeepEqualErr(v92v1, v92v2, t, "equal-map-v92-p-len")
		bs92 = testMarshalErr(&v92v1, h, t, "enc-map-v92-p")
		v92v2 = nil
		testUnmarshalErr(&v92v2, bs92, h, t, "dec-map-v92-p-nil")
		testDeepEqualErr(v92v1, v92v2, t, "equal-map-v92-p-nil")
		// ...
		if v == nil {
			v92v2 = nil
		} else {
			v92v2 = make(map[uint8]interface{}, len(v))
		} // reset map
		var v92v3, v92v4 typMapMapUint8Intf
		v92v3 = typMapMapUint8Intf(v92v1)
		v92v4 = typMapMapUint8Intf(v92v2)
		bs92 = testMarshalErr(v92v3, h, t, "enc-map-v92-custom")
		testUnmarshalErr(v92v4, bs92, h, t, "dec-map-v92-p-len")
		testDeepEqualErr(v92v3, v92v4, t, "equal-map-v92-p-len")
	}

	for _, v := range []map[uint8]string{nil, {}, {33: "", 44: "some-string"}} {
		// fmt.Printf(">>>> running mammoth map v93: %v\n", v)
		var v93v1, v93v2 map[uint8]string
		v93v1 = v
		bs93 := testMarshalErr(v93v1, h, t, "enc-map-v93")
		if v == nil {
			v93v2 = nil
		} else {
			v93v2 = make(map[uint8]string, len(v))
		} // reset map
		testUnmarshalErr(v93v2, bs93, h, t, "dec-map-v93")
		testDeepEqualErr(v93v1, v93v2, t, "equal-map-v93")
		if v == nil {
			v93v2 = nil
		} else {
			v93v2 = make(map[uint8]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v93v2), bs93, h, t, "dec-map-v93-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v93v1, v93v2, t, "equal-map-v93-noaddr")
		if v == nil {
			v93v2 = nil
		} else {
			v93v2 = make(map[uint8]string, len(v))
		} // reset map
		testUnmarshalErr(&v93v2, bs93, h, t, "dec-map-v93-p-len")
		testDeepEqualErr(v93v1, v93v2, t, "equal-map-v93-p-len")
		bs93 = testMarshalErr(&v93v1, h, t, "enc-map-v93-p")
		v93v2 = nil
		testUnmarshalErr(&v93v2, bs93, h, t, "dec-map-v93-p-nil")
		testDeepEqualErr(v93v1, v93v2, t, "equal-map-v93-p-nil")
		// ...
		if v == nil {
			v93v2 = nil
		} else {
			v93v2 = make(map[uint8]string, len(v))
		} // reset map
		var v93v3, v93v4 typMapMapUint8String
		v93v3 = typMapMapUint8String(v93v1)
		v93v4 = typMapMapUint8String(v93v2)
		bs93 = testMarshalErr(v93v3, h, t, "enc-map-v93-custom")
		testUnmarshalErr(v93v4, bs93, h, t, "dec-map-v93-p-len")
		testDeepEqualErr(v93v3, v93v4, t, "equal-map-v93-p-len")
	}

	for _, v := range []map[uint8]uint{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v94: %v\n", v)
		var v94v1, v94v2 map[uint8]uint
		v94v1 = v
		bs94 := testMarshalErr(v94v1, h, t, "enc-map-v94")
		if v == nil {
			v94v2 = nil
		} else {
			v94v2 = make(map[uint8]uint, len(v))
		} // reset map
		testUnmarshalErr(v94v2, bs94, h, t, "dec-map-v94")
		testDeepEqualErr(v94v1, v94v2, t, "equal-map-v94")
		if v == nil {
			v94v2 = nil
		} else {
			v94v2 = make(map[uint8]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v94v2), bs94, h, t, "dec-map-v94-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v94v1, v94v2, t, "equal-map-v94-noaddr")
		if v == nil {
			v94v2 = nil
		} else {
			v94v2 = make(map[uint8]uint, len(v))
		} // reset map
		testUnmarshalErr(&v94v2, bs94, h, t, "dec-map-v94-p-len")
		testDeepEqualErr(v94v1, v94v2, t, "equal-map-v94-p-len")
		bs94 = testMarshalErr(&v94v1, h, t, "enc-map-v94-p")
		v94v2 = nil
		testUnmarshalErr(&v94v2, bs94, h, t, "dec-map-v94-p-nil")
		testDeepEqualErr(v94v1, v94v2, t, "equal-map-v94-p-nil")
		// ...
		if v == nil {
			v94v2 = nil
		} else {
			v94v2 = make(map[uint8]uint, len(v))
		} // reset map
		var v94v3, v94v4 typMapMapUint8Uint
		v94v3 = typMapMapUint8Uint(v94v1)
		v94v4 = typMapMapUint8Uint(v94v2)
		bs94 = testMarshalErr(v94v3, h, t, "enc-map-v94-custom")
		testUnmarshalErr(v94v4, bs94, h, t, "dec-map-v94-p-len")
		testDeepEqualErr(v94v3, v94v4, t, "equal-map-v94-p-len")
	}

	for _, v := range []map[uint8]uint8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v95: %v\n", v)
		var v95v1, v95v2 map[uint8]uint8
		v95v1 = v
		bs95 := testMarshalErr(v95v1, h, t, "enc-map-v95")
		if v == nil {
			v95v2 = nil
		} else {
			v95v2 = make(map[uint8]uint8, len(v))
		} // reset map
		testUnmarshalErr(v95v2, bs95, h, t, "dec-map-v95")
		testDeepEqualErr(v95v1, v95v2, t, "equal-map-v95")
		if v == nil {
			v95v2 = nil
		} else {
			v95v2 = make(map[uint8]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v95v2), bs95, h, t, "dec-map-v95-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v95v1, v95v2, t, "equal-map-v95-noaddr")
		if v == nil {
			v95v2 = nil
		} else {
			v95v2 = make(map[uint8]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v95v2, bs95, h, t, "dec-map-v95-p-len")
		testDeepEqualErr(v95v1, v95v2, t, "equal-map-v95-p-len")
		bs95 = testMarshalErr(&v95v1, h, t, "enc-map-v95-p")
		v95v2 = nil
		testUnmarshalErr(&v95v2, bs95, h, t, "dec-map-v95-p-nil")
		testDeepEqualErr(v95v1, v95v2, t, "equal-map-v95-p-nil")
		// ...
		if v == nil {
			v95v2 = nil
		} else {
			v95v2 = make(map[uint8]uint8, len(v))
		} // reset map
		var v95v3, v95v4 typMapMapUint8Uint8
		v95v3 = typMapMapUint8Uint8(v95v1)
		v95v4 = typMapMapUint8Uint8(v95v2)
		bs95 = testMarshalErr(v95v3, h, t, "enc-map-v95-custom")
		testUnmarshalErr(v95v4, bs95, h, t, "dec-map-v95-p-len")
		testDeepEqualErr(v95v3, v95v4, t, "equal-map-v95-p-len")
	}

	for _, v := range []map[uint8]uint16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v96: %v\n", v)
		var v96v1, v96v2 map[uint8]uint16
		v96v1 = v
		bs96 := testMarshalErr(v96v1, h, t, "enc-map-v96")
		if v == nil {
			v96v2 = nil
		} else {
			v96v2 = make(map[uint8]uint16, len(v))
		} // reset map
		testUnmarshalErr(v96v2, bs96, h, t, "dec-map-v96")
		testDeepEqualErr(v96v1, v96v2, t, "equal-map-v96")
		if v == nil {
			v96v2 = nil
		} else {
			v96v2 = make(map[uint8]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v96v2), bs96, h, t, "dec-map-v96-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v96v1, v96v2, t, "equal-map-v96-noaddr")
		if v == nil {
			v96v2 = nil
		} else {
			v96v2 = make(map[uint8]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v96v2, bs96, h, t, "dec-map-v96-p-len")
		testDeepEqualErr(v96v1, v96v2, t, "equal-map-v96-p-len")
		bs96 = testMarshalErr(&v96v1, h, t, "enc-map-v96-p")
		v96v2 = nil
		testUnmarshalErr(&v96v2, bs96, h, t, "dec-map-v96-p-nil")
		testDeepEqualErr(v96v1, v96v2, t, "equal-map-v96-p-nil")
		// ...
		if v == nil {
			v96v2 = nil
		} else {
			v96v2 = make(map[uint8]uint16, len(v))
		} // reset map
		var v96v3, v96v4 typMapMapUint8Uint16
		v96v3 = typMapMapUint8Uint16(v96v1)
		v96v4 = typMapMapUint8Uint16(v96v2)
		bs96 = testMarshalErr(v96v3, h, t, "enc-map-v96-custom")
		testUnmarshalErr(v96v4, bs96, h, t, "dec-map-v96-p-len")
		testDeepEqualErr(v96v3, v96v4, t, "equal-map-v96-p-len")
	}

	for _, v := range []map[uint8]uint32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v97: %v\n", v)
		var v97v1, v97v2 map[uint8]uint32
		v97v1 = v
		bs97 := testMarshalErr(v97v1, h, t, "enc-map-v97")
		if v == nil {
			v97v2 = nil
		} else {
			v97v2 = make(map[uint8]uint32, len(v))
		} // reset map
		testUnmarshalErr(v97v2, bs97, h, t, "dec-map-v97")
		testDeepEqualErr(v97v1, v97v2, t, "equal-map-v97")
		if v == nil {
			v97v2 = nil
		} else {
			v97v2 = make(map[uint8]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v97v2), bs97, h, t, "dec-map-v97-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v97v1, v97v2, t, "equal-map-v97-noaddr")
		if v == nil {
			v97v2 = nil
		} else {
			v97v2 = make(map[uint8]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v97v2, bs97, h, t, "dec-map-v97-p-len")
		testDeepEqualErr(v97v1, v97v2, t, "equal-map-v97-p-len")
		bs97 = testMarshalErr(&v97v1, h, t, "enc-map-v97-p")
		v97v2 = nil
		testUnmarshalErr(&v97v2, bs97, h, t, "dec-map-v97-p-nil")
		testDeepEqualErr(v97v1, v97v2, t, "equal-map-v97-p-nil")
		// ...
		if v == nil {
			v97v2 = nil
		} else {
			v97v2 = make(map[uint8]uint32, len(v))
		} // reset map
		var v97v3, v97v4 typMapMapUint8Uint32
		v97v3 = typMapMapUint8Uint32(v97v1)
		v97v4 = typMapMapUint8Uint32(v97v2)
		bs97 = testMarshalErr(v97v3, h, t, "enc-map-v97-custom")
		testUnmarshalErr(v97v4, bs97, h, t, "dec-map-v97-p-len")
		testDeepEqualErr(v97v3, v97v4, t, "equal-map-v97-p-len")
	}

	for _, v := range []map[uint8]uint64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v98: %v\n", v)
		var v98v1, v98v2 map[uint8]uint64
		v98v1 = v
		bs98 := testMarshalErr(v98v1, h, t, "enc-map-v98")
		if v == nil {
			v98v2 = nil
		} else {
			v98v2 = make(map[uint8]uint64, len(v))
		} // reset map
		testUnmarshalErr(v98v2, bs98, h, t, "dec-map-v98")
		testDeepEqualErr(v98v1, v98v2, t, "equal-map-v98")
		if v == nil {
			v98v2 = nil
		} else {
			v98v2 = make(map[uint8]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v98v2), bs98, h, t, "dec-map-v98-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v98v1, v98v2, t, "equal-map-v98-noaddr")
		if v == nil {
			v98v2 = nil
		} else {
			v98v2 = make(map[uint8]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v98v2, bs98, h, t, "dec-map-v98-p-len")
		testDeepEqualErr(v98v1, v98v2, t, "equal-map-v98-p-len")
		bs98 = testMarshalErr(&v98v1, h, t, "enc-map-v98-p")
		v98v2 = nil
		testUnmarshalErr(&v98v2, bs98, h, t, "dec-map-v98-p-nil")
		testDeepEqualErr(v98v1, v98v2, t, "equal-map-v98-p-nil")
		// ...
		if v == nil {
			v98v2 = nil
		} else {
			v98v2 = make(map[uint8]uint64, len(v))
		} // reset map
		var v98v3, v98v4 typMapMapUint8Uint64
		v98v3 = typMapMapUint8Uint64(v98v1)
		v98v4 = typMapMapUint8Uint64(v98v2)
		bs98 = testMarshalErr(v98v3, h, t, "enc-map-v98-custom")
		testUnmarshalErr(v98v4, bs98, h, t, "dec-map-v98-p-len")
		testDeepEqualErr(v98v3, v98v4, t, "equal-map-v98-p-len")
	}

	for _, v := range []map[uint8]uintptr{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v99: %v\n", v)
		var v99v1, v99v2 map[uint8]uintptr
		v99v1 = v
		bs99 := testMarshalErr(v99v1, h, t, "enc-map-v99")
		if v == nil {
			v99v2 = nil
		} else {
			v99v2 = make(map[uint8]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v99v2, bs99, h, t, "dec-map-v99")
		testDeepEqualErr(v99v1, v99v2, t, "equal-map-v99")
		if v == nil {
			v99v2 = nil
		} else {
			v99v2 = make(map[uint8]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v99v2), bs99, h, t, "dec-map-v99-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v99v1, v99v2, t, "equal-map-v99-noaddr")
		if v == nil {
			v99v2 = nil
		} else {
			v99v2 = make(map[uint8]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v99v2, bs99, h, t, "dec-map-v99-p-len")
		testDeepEqualErr(v99v1, v99v2, t, "equal-map-v99-p-len")
		bs99 = testMarshalErr(&v99v1, h, t, "enc-map-v99-p")
		v99v2 = nil
		testUnmarshalErr(&v99v2, bs99, h, t, "dec-map-v99-p-nil")
		testDeepEqualErr(v99v1, v99v2, t, "equal-map-v99-p-nil")
		// ...
		if v == nil {
			v99v2 = nil
		} else {
			v99v2 = make(map[uint8]uintptr, len(v))
		} // reset map
		var v99v3, v99v4 typMapMapUint8Uintptr
		v99v3 = typMapMapUint8Uintptr(v99v1)
		v99v4 = typMapMapUint8Uintptr(v99v2)
		bs99 = testMarshalErr(v99v3, h, t, "enc-map-v99-custom")
		testUnmarshalErr(v99v4, bs99, h, t, "dec-map-v99-p-len")
		testDeepEqualErr(v99v3, v99v4, t, "equal-map-v99-p-len")
	}

	for _, v := range []map[uint8]int{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v100: %v\n", v)
		var v100v1, v100v2 map[uint8]int
		v100v1 = v
		bs100 := testMarshalErr(v100v1, h, t, "enc-map-v100")
		if v == nil {
			v100v2 = nil
		} else {
			v100v2 = make(map[uint8]int, len(v))
		} // reset map
		testUnmarshalErr(v100v2, bs100, h, t, "dec-map-v100")
		testDeepEqualErr(v100v1, v100v2, t, "equal-map-v100")
		if v == nil {
			v100v2 = nil
		} else {
			v100v2 = make(map[uint8]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v100v2), bs100, h, t, "dec-map-v100-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v100v1, v100v2, t, "equal-map-v100-noaddr")
		if v == nil {
			v100v2 = nil
		} else {
			v100v2 = make(map[uint8]int, len(v))
		} // reset map
		testUnmarshalErr(&v100v2, bs100, h, t, "dec-map-v100-p-len")
		testDeepEqualErr(v100v1, v100v2, t, "equal-map-v100-p-len")
		bs100 = testMarshalErr(&v100v1, h, t, "enc-map-v100-p")
		v100v2 = nil
		testUnmarshalErr(&v100v2, bs100, h, t, "dec-map-v100-p-nil")
		testDeepEqualErr(v100v1, v100v2, t, "equal-map-v100-p-nil")
		// ...
		if v == nil {
			v100v2 = nil
		} else {
			v100v2 = make(map[uint8]int, len(v))
		} // reset map
		var v100v3, v100v4 typMapMapUint8Int
		v100v3 = typMapMapUint8Int(v100v1)
		v100v4 = typMapMapUint8Int(v100v2)
		bs100 = testMarshalErr(v100v3, h, t, "enc-map-v100-custom")
		testUnmarshalErr(v100v4, bs100, h, t, "dec-map-v100-p-len")
		testDeepEqualErr(v100v3, v100v4, t, "equal-map-v100-p-len")
	}

	for _, v := range []map[uint8]int8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v101: %v\n", v)
		var v101v1, v101v2 map[uint8]int8
		v101v1 = v
		bs101 := testMarshalErr(v101v1, h, t, "enc-map-v101")
		if v == nil {
			v101v2 = nil
		} else {
			v101v2 = make(map[uint8]int8, len(v))
		} // reset map
		testUnmarshalErr(v101v2, bs101, h, t, "dec-map-v101")
		testDeepEqualErr(v101v1, v101v2, t, "equal-map-v101")
		if v == nil {
			v101v2 = nil
		} else {
			v101v2 = make(map[uint8]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v101v2), bs101, h, t, "dec-map-v101-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v101v1, v101v2, t, "equal-map-v101-noaddr")
		if v == nil {
			v101v2 = nil
		} else {
			v101v2 = make(map[uint8]int8, len(v))
		} // reset map
		testUnmarshalErr(&v101v2, bs101, h, t, "dec-map-v101-p-len")
		testDeepEqualErr(v101v1, v101v2, t, "equal-map-v101-p-len")
		bs101 = testMarshalErr(&v101v1, h, t, "enc-map-v101-p")
		v101v2 = nil
		testUnmarshalErr(&v101v2, bs101, h, t, "dec-map-v101-p-nil")
		testDeepEqualErr(v101v1, v101v2, t, "equal-map-v101-p-nil")
		// ...
		if v == nil {
			v101v2 = nil
		} else {
			v101v2 = make(map[uint8]int8, len(v))
		} // reset map
		var v101v3, v101v4 typMapMapUint8Int8
		v101v3 = typMapMapUint8Int8(v101v1)
		v101v4 = typMapMapUint8Int8(v101v2)
		bs101 = testMarshalErr(v101v3, h, t, "enc-map-v101-custom")
		testUnmarshalErr(v101v4, bs101, h, t, "dec-map-v101-p-len")
		testDeepEqualErr(v101v3, v101v4, t, "equal-map-v101-p-len")
	}

	for _, v := range []map[uint8]int16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v102: %v\n", v)
		var v102v1, v102v2 map[uint8]int16
		v102v1 = v
		bs102 := testMarshalErr(v102v1, h, t, "enc-map-v102")
		if v == nil {
			v102v2 = nil
		} else {
			v102v2 = make(map[uint8]int16, len(v))
		} // reset map
		testUnmarshalErr(v102v2, bs102, h, t, "dec-map-v102")
		testDeepEqualErr(v102v1, v102v2, t, "equal-map-v102")
		if v == nil {
			v102v2 = nil
		} else {
			v102v2 = make(map[uint8]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v102v2), bs102, h, t, "dec-map-v102-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v102v1, v102v2, t, "equal-map-v102-noaddr")
		if v == nil {
			v102v2 = nil
		} else {
			v102v2 = make(map[uint8]int16, len(v))
		} // reset map
		testUnmarshalErr(&v102v2, bs102, h, t, "dec-map-v102-p-len")
		testDeepEqualErr(v102v1, v102v2, t, "equal-map-v102-p-len")
		bs102 = testMarshalErr(&v102v1, h, t, "enc-map-v102-p")
		v102v2 = nil
		testUnmarshalErr(&v102v2, bs102, h, t, "dec-map-v102-p-nil")
		testDeepEqualErr(v102v1, v102v2, t, "equal-map-v102-p-nil")
		// ...
		if v == nil {
			v102v2 = nil
		} else {
			v102v2 = make(map[uint8]int16, len(v))
		} // reset map
		var v102v3, v102v4 typMapMapUint8Int16
		v102v3 = typMapMapUint8Int16(v102v1)
		v102v4 = typMapMapUint8Int16(v102v2)
		bs102 = testMarshalErr(v102v3, h, t, "enc-map-v102-custom")
		testUnmarshalErr(v102v4, bs102, h, t, "dec-map-v102-p-len")
		testDeepEqualErr(v102v3, v102v4, t, "equal-map-v102-p-len")
	}

	for _, v := range []map[uint8]int32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v103: %v\n", v)
		var v103v1, v103v2 map[uint8]int32
		v103v1 = v
		bs103 := testMarshalErr(v103v1, h, t, "enc-map-v103")
		if v == nil {
			v103v2 = nil
		} else {
			v103v2 = make(map[uint8]int32, len(v))
		} // reset map
		testUnmarshalErr(v103v2, bs103, h, t, "dec-map-v103")
		testDeepEqualErr(v103v1, v103v2, t, "equal-map-v103")
		if v == nil {
			v103v2 = nil
		} else {
			v103v2 = make(map[uint8]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v103v2), bs103, h, t, "dec-map-v103-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v103v1, v103v2, t, "equal-map-v103-noaddr")
		if v == nil {
			v103v2 = nil
		} else {
			v103v2 = make(map[uint8]int32, len(v))
		} // reset map
		testUnmarshalErr(&v103v2, bs103, h, t, "dec-map-v103-p-len")
		testDeepEqualErr(v103v1, v103v2, t, "equal-map-v103-p-len")
		bs103 = testMarshalErr(&v103v1, h, t, "enc-map-v103-p")
		v103v2 = nil
		testUnmarshalErr(&v103v2, bs103, h, t, "dec-map-v103-p-nil")
		testDeepEqualErr(v103v1, v103v2, t, "equal-map-v103-p-nil")
		// ...
		if v == nil {
			v103v2 = nil
		} else {
			v103v2 = make(map[uint8]int32, len(v))
		} // reset map
		var v103v3, v103v4 typMapMapUint8Int32
		v103v3 = typMapMapUint8Int32(v103v1)
		v103v4 = typMapMapUint8Int32(v103v2)
		bs103 = testMarshalErr(v103v3, h, t, "enc-map-v103-custom")
		testUnmarshalErr(v103v4, bs103, h, t, "dec-map-v103-p-len")
		testDeepEqualErr(v103v3, v103v4, t, "equal-map-v103-p-len")
	}

	for _, v := range []map[uint8]int64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v104: %v\n", v)
		var v104v1, v104v2 map[uint8]int64
		v104v1 = v
		bs104 := testMarshalErr(v104v1, h, t, "enc-map-v104")
		if v == nil {
			v104v2 = nil
		} else {
			v104v2 = make(map[uint8]int64, len(v))
		} // reset map
		testUnmarshalErr(v104v2, bs104, h, t, "dec-map-v104")
		testDeepEqualErr(v104v1, v104v2, t, "equal-map-v104")
		if v == nil {
			v104v2 = nil
		} else {
			v104v2 = make(map[uint8]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v104v2), bs104, h, t, "dec-map-v104-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v104v1, v104v2, t, "equal-map-v104-noaddr")
		if v == nil {
			v104v2 = nil
		} else {
			v104v2 = make(map[uint8]int64, len(v))
		} // reset map
		testUnmarshalErr(&v104v2, bs104, h, t, "dec-map-v104-p-len")
		testDeepEqualErr(v104v1, v104v2, t, "equal-map-v104-p-len")
		bs104 = testMarshalErr(&v104v1, h, t, "enc-map-v104-p")
		v104v2 = nil
		testUnmarshalErr(&v104v2, bs104, h, t, "dec-map-v104-p-nil")
		testDeepEqualErr(v104v1, v104v2, t, "equal-map-v104-p-nil")
		// ...
		if v == nil {
			v104v2 = nil
		} else {
			v104v2 = make(map[uint8]int64, len(v))
		} // reset map
		var v104v3, v104v4 typMapMapUint8Int64
		v104v3 = typMapMapUint8Int64(v104v1)
		v104v4 = typMapMapUint8Int64(v104v2)
		bs104 = testMarshalErr(v104v3, h, t, "enc-map-v104-custom")
		testUnmarshalErr(v104v4, bs104, h, t, "dec-map-v104-p-len")
		testDeepEqualErr(v104v3, v104v4, t, "equal-map-v104-p-len")
	}

	for _, v := range []map[uint8]float32{nil, {}, {44: 0, 33: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v105: %v\n", v)
		var v105v1, v105v2 map[uint8]float32
		v105v1 = v
		bs105 := testMarshalErr(v105v1, h, t, "enc-map-v105")
		if v == nil {
			v105v2 = nil
		} else {
			v105v2 = make(map[uint8]float32, len(v))
		} // reset map
		testUnmarshalErr(v105v2, bs105, h, t, "dec-map-v105")
		testDeepEqualErr(v105v1, v105v2, t, "equal-map-v105")
		if v == nil {
			v105v2 = nil
		} else {
			v105v2 = make(map[uint8]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v105v2), bs105, h, t, "dec-map-v105-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v105v1, v105v2, t, "equal-map-v105-noaddr")
		if v == nil {
			v105v2 = nil
		} else {
			v105v2 = make(map[uint8]float32, len(v))
		} // reset map
		testUnmarshalErr(&v105v2, bs105, h, t, "dec-map-v105-p-len")
		testDeepEqualErr(v105v1, v105v2, t, "equal-map-v105-p-len")
		bs105 = testMarshalErr(&v105v1, h, t, "enc-map-v105-p")
		v105v2 = nil
		testUnmarshalErr(&v105v2, bs105, h, t, "dec-map-v105-p-nil")
		testDeepEqualErr(v105v1, v105v2, t, "equal-map-v105-p-nil")
		// ...
		if v == nil {
			v105v2 = nil
		} else {
			v105v2 = make(map[uint8]float32, len(v))
		} // reset map
		var v105v3, v105v4 typMapMapUint8Float32
		v105v3 = typMapMapUint8Float32(v105v1)
		v105v4 = typMapMapUint8Float32(v105v2)
		bs105 = testMarshalErr(v105v3, h, t, "enc-map-v105-custom")
		testUnmarshalErr(v105v4, bs105, h, t, "dec-map-v105-p-len")
		testDeepEqualErr(v105v3, v105v4, t, "equal-map-v105-p-len")
	}

	for _, v := range []map[uint8]float64{nil, {}, {44: 0, 33: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v106: %v\n", v)
		var v106v1, v106v2 map[uint8]float64
		v106v1 = v
		bs106 := testMarshalErr(v106v1, h, t, "enc-map-v106")
		if v == nil {
			v106v2 = nil
		} else {
			v106v2 = make(map[uint8]float64, len(v))
		} // reset map
		testUnmarshalErr(v106v2, bs106, h, t, "dec-map-v106")
		testDeepEqualErr(v106v1, v106v2, t, "equal-map-v106")
		if v == nil {
			v106v2 = nil
		} else {
			v106v2 = make(map[uint8]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v106v2), bs106, h, t, "dec-map-v106-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v106v1, v106v2, t, "equal-map-v106-noaddr")
		if v == nil {
			v106v2 = nil
		} else {
			v106v2 = make(map[uint8]float64, len(v))
		} // reset map
		testUnmarshalErr(&v106v2, bs106, h, t, "dec-map-v106-p-len")
		testDeepEqualErr(v106v1, v106v2, t, "equal-map-v106-p-len")
		bs106 = testMarshalErr(&v106v1, h, t, "enc-map-v106-p")
		v106v2 = nil
		testUnmarshalErr(&v106v2, bs106, h, t, "dec-map-v106-p-nil")
		testDeepEqualErr(v106v1, v106v2, t, "equal-map-v106-p-nil")
		// ...
		if v == nil {
			v106v2 = nil
		} else {
			v106v2 = make(map[uint8]float64, len(v))
		} // reset map
		var v106v3, v106v4 typMapMapUint8Float64
		v106v3 = typMapMapUint8Float64(v106v1)
		v106v4 = typMapMapUint8Float64(v106v2)
		bs106 = testMarshalErr(v106v3, h, t, "enc-map-v106-custom")
		testUnmarshalErr(v106v4, bs106, h, t, "dec-map-v106-p-len")
		testDeepEqualErr(v106v3, v106v4, t, "equal-map-v106-p-len")
	}

	for _, v := range []map[uint8]bool{nil, {}, {44: false, 33: true}} {
		// fmt.Printf(">>>> running mammoth map v107: %v\n", v)
		var v107v1, v107v2 map[uint8]bool
		v107v1 = v
		bs107 := testMarshalErr(v107v1, h, t, "enc-map-v107")
		if v == nil {
			v107v2 = nil
		} else {
			v107v2 = make(map[uint8]bool, len(v))
		} // reset map
		testUnmarshalErr(v107v2, bs107, h, t, "dec-map-v107")
		testDeepEqualErr(v107v1, v107v2, t, "equal-map-v107")
		if v == nil {
			v107v2 = nil
		} else {
			v107v2 = make(map[uint8]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v107v2), bs107, h, t, "dec-map-v107-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v107v1, v107v2, t, "equal-map-v107-noaddr")
		if v == nil {
			v107v2 = nil
		} else {
			v107v2 = make(map[uint8]bool, len(v))
		} // reset map
		testUnmarshalErr(&v107v2, bs107, h, t, "dec-map-v107-p-len")
		testDeepEqualErr(v107v1, v107v2, t, "equal-map-v107-p-len")
		bs107 = testMarshalErr(&v107v1, h, t, "enc-map-v107-p")
		v107v2 = nil
		testUnmarshalErr(&v107v2, bs107, h, t, "dec-map-v107-p-nil")
		testDeepEqualErr(v107v1, v107v2, t, "equal-map-v107-p-nil")
		// ...
		if v == nil {
			v107v2 = nil
		} else {
			v107v2 = make(map[uint8]bool, len(v))
		} // reset map
		var v107v3, v107v4 typMapMapUint8Bool
		v107v3 = typMapMapUint8Bool(v107v1)
		v107v4 = typMapMapUint8Bool(v107v2)
		bs107 = testMarshalErr(v107v3, h, t, "enc-map-v107-custom")
		testUnmarshalErr(v107v4, bs107, h, t, "dec-map-v107-p-len")
		testDeepEqualErr(v107v3, v107v4, t, "equal-map-v107-p-len")
	}

	for _, v := range []map[uint16]interface{}{nil, {}, {44: nil, 33: "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v110: %v\n", v)
		var v110v1, v110v2 map[uint16]interface{}
		v110v1 = v
		bs110 := testMarshalErr(v110v1, h, t, "enc-map-v110")
		if v == nil {
			v110v2 = nil
		} else {
			v110v2 = make(map[uint16]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v110v2, bs110, h, t, "dec-map-v110")
		testDeepEqualErr(v110v1, v110v2, t, "equal-map-v110")
		if v == nil {
			v110v2 = nil
		} else {
			v110v2 = make(map[uint16]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v110v2), bs110, h, t, "dec-map-v110-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v110v1, v110v2, t, "equal-map-v110-noaddr")
		if v == nil {
			v110v2 = nil
		} else {
			v110v2 = make(map[uint16]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v110v2, bs110, h, t, "dec-map-v110-p-len")
		testDeepEqualErr(v110v1, v110v2, t, "equal-map-v110-p-len")
		bs110 = testMarshalErr(&v110v1, h, t, "enc-map-v110-p")
		v110v2 = nil
		testUnmarshalErr(&v110v2, bs110, h, t, "dec-map-v110-p-nil")
		testDeepEqualErr(v110v1, v110v2, t, "equal-map-v110-p-nil")
		// ...
		if v == nil {
			v110v2 = nil
		} else {
			v110v2 = make(map[uint16]interface{}, len(v))
		} // reset map
		var v110v3, v110v4 typMapMapUint16Intf
		v110v3 = typMapMapUint16Intf(v110v1)
		v110v4 = typMapMapUint16Intf(v110v2)
		bs110 = testMarshalErr(v110v3, h, t, "enc-map-v110-custom")
		testUnmarshalErr(v110v4, bs110, h, t, "dec-map-v110-p-len")
		testDeepEqualErr(v110v3, v110v4, t, "equal-map-v110-p-len")
	}

	for _, v := range []map[uint16]string{nil, {}, {44: "", 33: "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v111: %v\n", v)
		var v111v1, v111v2 map[uint16]string
		v111v1 = v
		bs111 := testMarshalErr(v111v1, h, t, "enc-map-v111")
		if v == nil {
			v111v2 = nil
		} else {
			v111v2 = make(map[uint16]string, len(v))
		} // reset map
		testUnmarshalErr(v111v2, bs111, h, t, "dec-map-v111")
		testDeepEqualErr(v111v1, v111v2, t, "equal-map-v111")
		if v == nil {
			v111v2 = nil
		} else {
			v111v2 = make(map[uint16]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v111v2), bs111, h, t, "dec-map-v111-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v111v1, v111v2, t, "equal-map-v111-noaddr")
		if v == nil {
			v111v2 = nil
		} else {
			v111v2 = make(map[uint16]string, len(v))
		} // reset map
		testUnmarshalErr(&v111v2, bs111, h, t, "dec-map-v111-p-len")
		testDeepEqualErr(v111v1, v111v2, t, "equal-map-v111-p-len")
		bs111 = testMarshalErr(&v111v1, h, t, "enc-map-v111-p")
		v111v2 = nil
		testUnmarshalErr(&v111v2, bs111, h, t, "dec-map-v111-p-nil")
		testDeepEqualErr(v111v1, v111v2, t, "equal-map-v111-p-nil")
		// ...
		if v == nil {
			v111v2 = nil
		} else {
			v111v2 = make(map[uint16]string, len(v))
		} // reset map
		var v111v3, v111v4 typMapMapUint16String
		v111v3 = typMapMapUint16String(v111v1)
		v111v4 = typMapMapUint16String(v111v2)
		bs111 = testMarshalErr(v111v3, h, t, "enc-map-v111-custom")
		testUnmarshalErr(v111v4, bs111, h, t, "dec-map-v111-p-len")
		testDeepEqualErr(v111v3, v111v4, t, "equal-map-v111-p-len")
	}

	for _, v := range []map[uint16]uint{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v112: %v\n", v)
		var v112v1, v112v2 map[uint16]uint
		v112v1 = v
		bs112 := testMarshalErr(v112v1, h, t, "enc-map-v112")
		if v == nil {
			v112v2 = nil
		} else {
			v112v2 = make(map[uint16]uint, len(v))
		} // reset map
		testUnmarshalErr(v112v2, bs112, h, t, "dec-map-v112")
		testDeepEqualErr(v112v1, v112v2, t, "equal-map-v112")
		if v == nil {
			v112v2 = nil
		} else {
			v112v2 = make(map[uint16]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v112v2), bs112, h, t, "dec-map-v112-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v112v1, v112v2, t, "equal-map-v112-noaddr")
		if v == nil {
			v112v2 = nil
		} else {
			v112v2 = make(map[uint16]uint, len(v))
		} // reset map
		testUnmarshalErr(&v112v2, bs112, h, t, "dec-map-v112-p-len")
		testDeepEqualErr(v112v1, v112v2, t, "equal-map-v112-p-len")
		bs112 = testMarshalErr(&v112v1, h, t, "enc-map-v112-p")
		v112v2 = nil
		testUnmarshalErr(&v112v2, bs112, h, t, "dec-map-v112-p-nil")
		testDeepEqualErr(v112v1, v112v2, t, "equal-map-v112-p-nil")
		// ...
		if v == nil {
			v112v2 = nil
		} else {
			v112v2 = make(map[uint16]uint, len(v))
		} // reset map
		var v112v3, v112v4 typMapMapUint16Uint
		v112v3 = typMapMapUint16Uint(v112v1)
		v112v4 = typMapMapUint16Uint(v112v2)
		bs112 = testMarshalErr(v112v3, h, t, "enc-map-v112-custom")
		testUnmarshalErr(v112v4, bs112, h, t, "dec-map-v112-p-len")
		testDeepEqualErr(v112v3, v112v4, t, "equal-map-v112-p-len")
	}

	for _, v := range []map[uint16]uint8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v113: %v\n", v)
		var v113v1, v113v2 map[uint16]uint8
		v113v1 = v
		bs113 := testMarshalErr(v113v1, h, t, "enc-map-v113")
		if v == nil {
			v113v2 = nil
		} else {
			v113v2 = make(map[uint16]uint8, len(v))
		} // reset map
		testUnmarshalErr(v113v2, bs113, h, t, "dec-map-v113")
		testDeepEqualErr(v113v1, v113v2, t, "equal-map-v113")
		if v == nil {
			v113v2 = nil
		} else {
			v113v2 = make(map[uint16]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v113v2), bs113, h, t, "dec-map-v113-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v113v1, v113v2, t, "equal-map-v113-noaddr")
		if v == nil {
			v113v2 = nil
		} else {
			v113v2 = make(map[uint16]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v113v2, bs113, h, t, "dec-map-v113-p-len")
		testDeepEqualErr(v113v1, v113v2, t, "equal-map-v113-p-len")
		bs113 = testMarshalErr(&v113v1, h, t, "enc-map-v113-p")
		v113v2 = nil
		testUnmarshalErr(&v113v2, bs113, h, t, "dec-map-v113-p-nil")
		testDeepEqualErr(v113v1, v113v2, t, "equal-map-v113-p-nil")
		// ...
		if v == nil {
			v113v2 = nil
		} else {
			v113v2 = make(map[uint16]uint8, len(v))
		} // reset map
		var v113v3, v113v4 typMapMapUint16Uint8
		v113v3 = typMapMapUint16Uint8(v113v1)
		v113v4 = typMapMapUint16Uint8(v113v2)
		bs113 = testMarshalErr(v113v3, h, t, "enc-map-v113-custom")
		testUnmarshalErr(v113v4, bs113, h, t, "dec-map-v113-p-len")
		testDeepEqualErr(v113v3, v113v4, t, "equal-map-v113-p-len")
	}

	for _, v := range []map[uint16]uint16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v114: %v\n", v)
		var v114v1, v114v2 map[uint16]uint16
		v114v1 = v
		bs114 := testMarshalErr(v114v1, h, t, "enc-map-v114")
		if v == nil {
			v114v2 = nil
		} else {
			v114v2 = make(map[uint16]uint16, len(v))
		} // reset map
		testUnmarshalErr(v114v2, bs114, h, t, "dec-map-v114")
		testDeepEqualErr(v114v1, v114v2, t, "equal-map-v114")
		if v == nil {
			v114v2 = nil
		} else {
			v114v2 = make(map[uint16]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v114v2), bs114, h, t, "dec-map-v114-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v114v1, v114v2, t, "equal-map-v114-noaddr")
		if v == nil {
			v114v2 = nil
		} else {
			v114v2 = make(map[uint16]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v114v2, bs114, h, t, "dec-map-v114-p-len")
		testDeepEqualErr(v114v1, v114v2, t, "equal-map-v114-p-len")
		bs114 = testMarshalErr(&v114v1, h, t, "enc-map-v114-p")
		v114v2 = nil
		testUnmarshalErr(&v114v2, bs114, h, t, "dec-map-v114-p-nil")
		testDeepEqualErr(v114v1, v114v2, t, "equal-map-v114-p-nil")
		// ...
		if v == nil {
			v114v2 = nil
		} else {
			v114v2 = make(map[uint16]uint16, len(v))
		} // reset map
		var v114v3, v114v4 typMapMapUint16Uint16
		v114v3 = typMapMapUint16Uint16(v114v1)
		v114v4 = typMapMapUint16Uint16(v114v2)
		bs114 = testMarshalErr(v114v3, h, t, "enc-map-v114-custom")
		testUnmarshalErr(v114v4, bs114, h, t, "dec-map-v114-p-len")
		testDeepEqualErr(v114v3, v114v4, t, "equal-map-v114-p-len")
	}

	for _, v := range []map[uint16]uint32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v115: %v\n", v)
		var v115v1, v115v2 map[uint16]uint32
		v115v1 = v
		bs115 := testMarshalErr(v115v1, h, t, "enc-map-v115")
		if v == nil {
			v115v2 = nil
		} else {
			v115v2 = make(map[uint16]uint32, len(v))
		} // reset map
		testUnmarshalErr(v115v2, bs115, h, t, "dec-map-v115")
		testDeepEqualErr(v115v1, v115v2, t, "equal-map-v115")
		if v == nil {
			v115v2 = nil
		} else {
			v115v2 = make(map[uint16]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v115v2), bs115, h, t, "dec-map-v115-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v115v1, v115v2, t, "equal-map-v115-noaddr")
		if v == nil {
			v115v2 = nil
		} else {
			v115v2 = make(map[uint16]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v115v2, bs115, h, t, "dec-map-v115-p-len")
		testDeepEqualErr(v115v1, v115v2, t, "equal-map-v115-p-len")
		bs115 = testMarshalErr(&v115v1, h, t, "enc-map-v115-p")
		v115v2 = nil
		testUnmarshalErr(&v115v2, bs115, h, t, "dec-map-v115-p-nil")
		testDeepEqualErr(v115v1, v115v2, t, "equal-map-v115-p-nil")
		// ...
		if v == nil {
			v115v2 = nil
		} else {
			v115v2 = make(map[uint16]uint32, len(v))
		} // reset map
		var v115v3, v115v4 typMapMapUint16Uint32
		v115v3 = typMapMapUint16Uint32(v115v1)
		v115v4 = typMapMapUint16Uint32(v115v2)
		bs115 = testMarshalErr(v115v3, h, t, "enc-map-v115-custom")
		testUnmarshalErr(v115v4, bs115, h, t, "dec-map-v115-p-len")
		testDeepEqualErr(v115v3, v115v4, t, "equal-map-v115-p-len")
	}

	for _, v := range []map[uint16]uint64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v116: %v\n", v)
		var v116v1, v116v2 map[uint16]uint64
		v116v1 = v
		bs116 := testMarshalErr(v116v1, h, t, "enc-map-v116")
		if v == nil {
			v116v2 = nil
		} else {
			v116v2 = make(map[uint16]uint64, len(v))
		} // reset map
		testUnmarshalErr(v116v2, bs116, h, t, "dec-map-v116")
		testDeepEqualErr(v116v1, v116v2, t, "equal-map-v116")
		if v == nil {
			v116v2 = nil
		} else {
			v116v2 = make(map[uint16]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v116v2), bs116, h, t, "dec-map-v116-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v116v1, v116v2, t, "equal-map-v116-noaddr")
		if v == nil {
			v116v2 = nil
		} else {
			v116v2 = make(map[uint16]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v116v2, bs116, h, t, "dec-map-v116-p-len")
		testDeepEqualErr(v116v1, v116v2, t, "equal-map-v116-p-len")
		bs116 = testMarshalErr(&v116v1, h, t, "enc-map-v116-p")
		v116v2 = nil
		testUnmarshalErr(&v116v2, bs116, h, t, "dec-map-v116-p-nil")
		testDeepEqualErr(v116v1, v116v2, t, "equal-map-v116-p-nil")
		// ...
		if v == nil {
			v116v2 = nil
		} else {
			v116v2 = make(map[uint16]uint64, len(v))
		} // reset map
		var v116v3, v116v4 typMapMapUint16Uint64
		v116v3 = typMapMapUint16Uint64(v116v1)
		v116v4 = typMapMapUint16Uint64(v116v2)
		bs116 = testMarshalErr(v116v3, h, t, "enc-map-v116-custom")
		testUnmarshalErr(v116v4, bs116, h, t, "dec-map-v116-p-len")
		testDeepEqualErr(v116v3, v116v4, t, "equal-map-v116-p-len")
	}

	for _, v := range []map[uint16]uintptr{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v117: %v\n", v)
		var v117v1, v117v2 map[uint16]uintptr
		v117v1 = v
		bs117 := testMarshalErr(v117v1, h, t, "enc-map-v117")
		if v == nil {
			v117v2 = nil
		} else {
			v117v2 = make(map[uint16]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v117v2, bs117, h, t, "dec-map-v117")
		testDeepEqualErr(v117v1, v117v2, t, "equal-map-v117")
		if v == nil {
			v117v2 = nil
		} else {
			v117v2 = make(map[uint16]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v117v2), bs117, h, t, "dec-map-v117-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v117v1, v117v2, t, "equal-map-v117-noaddr")
		if v == nil {
			v117v2 = nil
		} else {
			v117v2 = make(map[uint16]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v117v2, bs117, h, t, "dec-map-v117-p-len")
		testDeepEqualErr(v117v1, v117v2, t, "equal-map-v117-p-len")
		bs117 = testMarshalErr(&v117v1, h, t, "enc-map-v117-p")
		v117v2 = nil
		testUnmarshalErr(&v117v2, bs117, h, t, "dec-map-v117-p-nil")
		testDeepEqualErr(v117v1, v117v2, t, "equal-map-v117-p-nil")
		// ...
		if v == nil {
			v117v2 = nil
		} else {
			v117v2 = make(map[uint16]uintptr, len(v))
		} // reset map
		var v117v3, v117v4 typMapMapUint16Uintptr
		v117v3 = typMapMapUint16Uintptr(v117v1)
		v117v4 = typMapMapUint16Uintptr(v117v2)
		bs117 = testMarshalErr(v117v3, h, t, "enc-map-v117-custom")
		testUnmarshalErr(v117v4, bs117, h, t, "dec-map-v117-p-len")
		testDeepEqualErr(v117v3, v117v4, t, "equal-map-v117-p-len")
	}

	for _, v := range []map[uint16]int{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v118: %v\n", v)
		var v118v1, v118v2 map[uint16]int
		v118v1 = v
		bs118 := testMarshalErr(v118v1, h, t, "enc-map-v118")
		if v == nil {
			v118v2 = nil
		} else {
			v118v2 = make(map[uint16]int, len(v))
		} // reset map
		testUnmarshalErr(v118v2, bs118, h, t, "dec-map-v118")
		testDeepEqualErr(v118v1, v118v2, t, "equal-map-v118")
		if v == nil {
			v118v2 = nil
		} else {
			v118v2 = make(map[uint16]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v118v2), bs118, h, t, "dec-map-v118-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v118v1, v118v2, t, "equal-map-v118-noaddr")
		if v == nil {
			v118v2 = nil
		} else {
			v118v2 = make(map[uint16]int, len(v))
		} // reset map
		testUnmarshalErr(&v118v2, bs118, h, t, "dec-map-v118-p-len")
		testDeepEqualErr(v118v1, v118v2, t, "equal-map-v118-p-len")
		bs118 = testMarshalErr(&v118v1, h, t, "enc-map-v118-p")
		v118v2 = nil
		testUnmarshalErr(&v118v2, bs118, h, t, "dec-map-v118-p-nil")
		testDeepEqualErr(v118v1, v118v2, t, "equal-map-v118-p-nil")
		// ...
		if v == nil {
			v118v2 = nil
		} else {
			v118v2 = make(map[uint16]int, len(v))
		} // reset map
		var v118v3, v118v4 typMapMapUint16Int
		v118v3 = typMapMapUint16Int(v118v1)
		v118v4 = typMapMapUint16Int(v118v2)
		bs118 = testMarshalErr(v118v3, h, t, "enc-map-v118-custom")
		testUnmarshalErr(v118v4, bs118, h, t, "dec-map-v118-p-len")
		testDeepEqualErr(v118v3, v118v4, t, "equal-map-v118-p-len")
	}

	for _, v := range []map[uint16]int8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v119: %v\n", v)
		var v119v1, v119v2 map[uint16]int8
		v119v1 = v
		bs119 := testMarshalErr(v119v1, h, t, "enc-map-v119")
		if v == nil {
			v119v2 = nil
		} else {
			v119v2 = make(map[uint16]int8, len(v))
		} // reset map
		testUnmarshalErr(v119v2, bs119, h, t, "dec-map-v119")
		testDeepEqualErr(v119v1, v119v2, t, "equal-map-v119")
		if v == nil {
			v119v2 = nil
		} else {
			v119v2 = make(map[uint16]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v119v2), bs119, h, t, "dec-map-v119-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v119v1, v119v2, t, "equal-map-v119-noaddr")
		if v == nil {
			v119v2 = nil
		} else {
			v119v2 = make(map[uint16]int8, len(v))
		} // reset map
		testUnmarshalErr(&v119v2, bs119, h, t, "dec-map-v119-p-len")
		testDeepEqualErr(v119v1, v119v2, t, "equal-map-v119-p-len")
		bs119 = testMarshalErr(&v119v1, h, t, "enc-map-v119-p")
		v119v2 = nil
		testUnmarshalErr(&v119v2, bs119, h, t, "dec-map-v119-p-nil")
		testDeepEqualErr(v119v1, v119v2, t, "equal-map-v119-p-nil")
		// ...
		if v == nil {
			v119v2 = nil
		} else {
			v119v2 = make(map[uint16]int8, len(v))
		} // reset map
		var v119v3, v119v4 typMapMapUint16Int8
		v119v3 = typMapMapUint16Int8(v119v1)
		v119v4 = typMapMapUint16Int8(v119v2)
		bs119 = testMarshalErr(v119v3, h, t, "enc-map-v119-custom")
		testUnmarshalErr(v119v4, bs119, h, t, "dec-map-v119-p-len")
		testDeepEqualErr(v119v3, v119v4, t, "equal-map-v119-p-len")
	}

	for _, v := range []map[uint16]int16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v120: %v\n", v)
		var v120v1, v120v2 map[uint16]int16
		v120v1 = v
		bs120 := testMarshalErr(v120v1, h, t, "enc-map-v120")
		if v == nil {
			v120v2 = nil
		} else {
			v120v2 = make(map[uint16]int16, len(v))
		} // reset map
		testUnmarshalErr(v120v2, bs120, h, t, "dec-map-v120")
		testDeepEqualErr(v120v1, v120v2, t, "equal-map-v120")
		if v == nil {
			v120v2 = nil
		} else {
			v120v2 = make(map[uint16]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v120v2), bs120, h, t, "dec-map-v120-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v120v1, v120v2, t, "equal-map-v120-noaddr")
		if v == nil {
			v120v2 = nil
		} else {
			v120v2 = make(map[uint16]int16, len(v))
		} // reset map
		testUnmarshalErr(&v120v2, bs120, h, t, "dec-map-v120-p-len")
		testDeepEqualErr(v120v1, v120v2, t, "equal-map-v120-p-len")
		bs120 = testMarshalErr(&v120v1, h, t, "enc-map-v120-p")
		v120v2 = nil
		testUnmarshalErr(&v120v2, bs120, h, t, "dec-map-v120-p-nil")
		testDeepEqualErr(v120v1, v120v2, t, "equal-map-v120-p-nil")
		// ...
		if v == nil {
			v120v2 = nil
		} else {
			v120v2 = make(map[uint16]int16, len(v))
		} // reset map
		var v120v3, v120v4 typMapMapUint16Int16
		v120v3 = typMapMapUint16Int16(v120v1)
		v120v4 = typMapMapUint16Int16(v120v2)
		bs120 = testMarshalErr(v120v3, h, t, "enc-map-v120-custom")
		testUnmarshalErr(v120v4, bs120, h, t, "dec-map-v120-p-len")
		testDeepEqualErr(v120v3, v120v4, t, "equal-map-v120-p-len")
	}

	for _, v := range []map[uint16]int32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v121: %v\n", v)
		var v121v1, v121v2 map[uint16]int32
		v121v1 = v
		bs121 := testMarshalErr(v121v1, h, t, "enc-map-v121")
		if v == nil {
			v121v2 = nil
		} else {
			v121v2 = make(map[uint16]int32, len(v))
		} // reset map
		testUnmarshalErr(v121v2, bs121, h, t, "dec-map-v121")
		testDeepEqualErr(v121v1, v121v2, t, "equal-map-v121")
		if v == nil {
			v121v2 = nil
		} else {
			v121v2 = make(map[uint16]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v121v2), bs121, h, t, "dec-map-v121-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v121v1, v121v2, t, "equal-map-v121-noaddr")
		if v == nil {
			v121v2 = nil
		} else {
			v121v2 = make(map[uint16]int32, len(v))
		} // reset map
		testUnmarshalErr(&v121v2, bs121, h, t, "dec-map-v121-p-len")
		testDeepEqualErr(v121v1, v121v2, t, "equal-map-v121-p-len")
		bs121 = testMarshalErr(&v121v1, h, t, "enc-map-v121-p")
		v121v2 = nil
		testUnmarshalErr(&v121v2, bs121, h, t, "dec-map-v121-p-nil")
		testDeepEqualErr(v121v1, v121v2, t, "equal-map-v121-p-nil")
		// ...
		if v == nil {
			v121v2 = nil
		} else {
			v121v2 = make(map[uint16]int32, len(v))
		} // reset map
		var v121v3, v121v4 typMapMapUint16Int32
		v121v3 = typMapMapUint16Int32(v121v1)
		v121v4 = typMapMapUint16Int32(v121v2)
		bs121 = testMarshalErr(v121v3, h, t, "enc-map-v121-custom")
		testUnmarshalErr(v121v4, bs121, h, t, "dec-map-v121-p-len")
		testDeepEqualErr(v121v3, v121v4, t, "equal-map-v121-p-len")
	}

	for _, v := range []map[uint16]int64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v122: %v\n", v)
		var v122v1, v122v2 map[uint16]int64
		v122v1 = v
		bs122 := testMarshalErr(v122v1, h, t, "enc-map-v122")
		if v == nil {
			v122v2 = nil
		} else {
			v122v2 = make(map[uint16]int64, len(v))
		} // reset map
		testUnmarshalErr(v122v2, bs122, h, t, "dec-map-v122")
		testDeepEqualErr(v122v1, v122v2, t, "equal-map-v122")
		if v == nil {
			v122v2 = nil
		} else {
			v122v2 = make(map[uint16]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v122v2), bs122, h, t, "dec-map-v122-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v122v1, v122v2, t, "equal-map-v122-noaddr")
		if v == nil {
			v122v2 = nil
		} else {
			v122v2 = make(map[uint16]int64, len(v))
		} // reset map
		testUnmarshalErr(&v122v2, bs122, h, t, "dec-map-v122-p-len")
		testDeepEqualErr(v122v1, v122v2, t, "equal-map-v122-p-len")
		bs122 = testMarshalErr(&v122v1, h, t, "enc-map-v122-p")
		v122v2 = nil
		testUnmarshalErr(&v122v2, bs122, h, t, "dec-map-v122-p-nil")
		testDeepEqualErr(v122v1, v122v2, t, "equal-map-v122-p-nil")
		// ...
		if v == nil {
			v122v2 = nil
		} else {
			v122v2 = make(map[uint16]int64, len(v))
		} // reset map
		var v122v3, v122v4 typMapMapUint16Int64
		v122v3 = typMapMapUint16Int64(v122v1)
		v122v4 = typMapMapUint16Int64(v122v2)
		bs122 = testMarshalErr(v122v3, h, t, "enc-map-v122-custom")
		testUnmarshalErr(v122v4, bs122, h, t, "dec-map-v122-p-len")
		testDeepEqualErr(v122v3, v122v4, t, "equal-map-v122-p-len")
	}

	for _, v := range []map[uint16]float32{nil, {}, {33: 0, 44: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v123: %v\n", v)
		var v123v1, v123v2 map[uint16]float32
		v123v1 = v
		bs123 := testMarshalErr(v123v1, h, t, "enc-map-v123")
		if v == nil {
			v123v2 = nil
		} else {
			v123v2 = make(map[uint16]float32, len(v))
		} // reset map
		testUnmarshalErr(v123v2, bs123, h, t, "dec-map-v123")
		testDeepEqualErr(v123v1, v123v2, t, "equal-map-v123")
		if v == nil {
			v123v2 = nil
		} else {
			v123v2 = make(map[uint16]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v123v2), bs123, h, t, "dec-map-v123-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v123v1, v123v2, t, "equal-map-v123-noaddr")
		if v == nil {
			v123v2 = nil
		} else {
			v123v2 = make(map[uint16]float32, len(v))
		} // reset map
		testUnmarshalErr(&v123v2, bs123, h, t, "dec-map-v123-p-len")
		testDeepEqualErr(v123v1, v123v2, t, "equal-map-v123-p-len")
		bs123 = testMarshalErr(&v123v1, h, t, "enc-map-v123-p")
		v123v2 = nil
		testUnmarshalErr(&v123v2, bs123, h, t, "dec-map-v123-p-nil")
		testDeepEqualErr(v123v1, v123v2, t, "equal-map-v123-p-nil")
		// ...
		if v == nil {
			v123v2 = nil
		} else {
			v123v2 = make(map[uint16]float32, len(v))
		} // reset map
		var v123v3, v123v4 typMapMapUint16Float32
		v123v3 = typMapMapUint16Float32(v123v1)
		v123v4 = typMapMapUint16Float32(v123v2)
		bs123 = testMarshalErr(v123v3, h, t, "enc-map-v123-custom")
		testUnmarshalErr(v123v4, bs123, h, t, "dec-map-v123-p-len")
		testDeepEqualErr(v123v3, v123v4, t, "equal-map-v123-p-len")
	}

	for _, v := range []map[uint16]float64{nil, {}, {33: 0, 44: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v124: %v\n", v)
		var v124v1, v124v2 map[uint16]float64
		v124v1 = v
		bs124 := testMarshalErr(v124v1, h, t, "enc-map-v124")
		if v == nil {
			v124v2 = nil
		} else {
			v124v2 = make(map[uint16]float64, len(v))
		} // reset map
		testUnmarshalErr(v124v2, bs124, h, t, "dec-map-v124")
		testDeepEqualErr(v124v1, v124v2, t, "equal-map-v124")
		if v == nil {
			v124v2 = nil
		} else {
			v124v2 = make(map[uint16]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v124v2), bs124, h, t, "dec-map-v124-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v124v1, v124v2, t, "equal-map-v124-noaddr")
		if v == nil {
			v124v2 = nil
		} else {
			v124v2 = make(map[uint16]float64, len(v))
		} // reset map
		testUnmarshalErr(&v124v2, bs124, h, t, "dec-map-v124-p-len")
		testDeepEqualErr(v124v1, v124v2, t, "equal-map-v124-p-len")
		bs124 = testMarshalErr(&v124v1, h, t, "enc-map-v124-p")
		v124v2 = nil
		testUnmarshalErr(&v124v2, bs124, h, t, "dec-map-v124-p-nil")
		testDeepEqualErr(v124v1, v124v2, t, "equal-map-v124-p-nil")
		// ...
		if v == nil {
			v124v2 = nil
		} else {
			v124v2 = make(map[uint16]float64, len(v))
		} // reset map
		var v124v3, v124v4 typMapMapUint16Float64
		v124v3 = typMapMapUint16Float64(v124v1)
		v124v4 = typMapMapUint16Float64(v124v2)
		bs124 = testMarshalErr(v124v3, h, t, "enc-map-v124-custom")
		testUnmarshalErr(v124v4, bs124, h, t, "dec-map-v124-p-len")
		testDeepEqualErr(v124v3, v124v4, t, "equal-map-v124-p-len")
	}

	for _, v := range []map[uint16]bool{nil, {}, {33: false, 44: true}} {
		// fmt.Printf(">>>> running mammoth map v125: %v\n", v)
		var v125v1, v125v2 map[uint16]bool
		v125v1 = v
		bs125 := testMarshalErr(v125v1, h, t, "enc-map-v125")
		if v == nil {
			v125v2 = nil
		} else {
			v125v2 = make(map[uint16]bool, len(v))
		} // reset map
		testUnmarshalErr(v125v2, bs125, h, t, "dec-map-v125")
		testDeepEqualErr(v125v1, v125v2, t, "equal-map-v125")
		if v == nil {
			v125v2 = nil
		} else {
			v125v2 = make(map[uint16]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v125v2), bs125, h, t, "dec-map-v125-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v125v1, v125v2, t, "equal-map-v125-noaddr")
		if v == nil {
			v125v2 = nil
		} else {
			v125v2 = make(map[uint16]bool, len(v))
		} // reset map
		testUnmarshalErr(&v125v2, bs125, h, t, "dec-map-v125-p-len")
		testDeepEqualErr(v125v1, v125v2, t, "equal-map-v125-p-len")
		bs125 = testMarshalErr(&v125v1, h, t, "enc-map-v125-p")
		v125v2 = nil
		testUnmarshalErr(&v125v2, bs125, h, t, "dec-map-v125-p-nil")
		testDeepEqualErr(v125v1, v125v2, t, "equal-map-v125-p-nil")
		// ...
		if v == nil {
			v125v2 = nil
		} else {
			v125v2 = make(map[uint16]bool, len(v))
		} // reset map
		var v125v3, v125v4 typMapMapUint16Bool
		v125v3 = typMapMapUint16Bool(v125v1)
		v125v4 = typMapMapUint16Bool(v125v2)
		bs125 = testMarshalErr(v125v3, h, t, "enc-map-v125-custom")
		testUnmarshalErr(v125v4, bs125, h, t, "dec-map-v125-p-len")
		testDeepEqualErr(v125v3, v125v4, t, "equal-map-v125-p-len")
	}

	for _, v := range []map[uint32]interface{}{nil, {}, {33: nil, 44: "string-is-an-interface"}} {
		// fmt.Printf(">>>> running mammoth map v128: %v\n", v)
		var v128v1, v128v2 map[uint32]interface{}
		v128v1 = v
		bs128 := testMarshalErr(v128v1, h, t, "enc-map-v128")
		if v == nil {
			v128v2 = nil
		} else {
			v128v2 = make(map[uint32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v128v2, bs128, h, t, "dec-map-v128")
		testDeepEqualErr(v128v1, v128v2, t, "equal-map-v128")
		if v == nil {
			v128v2 = nil
		} else {
			v128v2 = make(map[uint32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v128v2), bs128, h, t, "dec-map-v128-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v128v1, v128v2, t, "equal-map-v128-noaddr")
		if v == nil {
			v128v2 = nil
		} else {
			v128v2 = make(map[uint32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v128v2, bs128, h, t, "dec-map-v128-p-len")
		testDeepEqualErr(v128v1, v128v2, t, "equal-map-v128-p-len")
		bs128 = testMarshalErr(&v128v1, h, t, "enc-map-v128-p")
		v128v2 = nil
		testUnmarshalErr(&v128v2, bs128, h, t, "dec-map-v128-p-nil")
		testDeepEqualErr(v128v1, v128v2, t, "equal-map-v128-p-nil")
		// ...
		if v == nil {
			v128v2 = nil
		} else {
			v128v2 = make(map[uint32]interface{}, len(v))
		} // reset map
		var v128v3, v128v4 typMapMapUint32Intf
		v128v3 = typMapMapUint32Intf(v128v1)
		v128v4 = typMapMapUint32Intf(v128v2)
		bs128 = testMarshalErr(v128v3, h, t, "enc-map-v128-custom")
		testUnmarshalErr(v128v4, bs128, h, t, "dec-map-v128-p-len")
		testDeepEqualErr(v128v3, v128v4, t, "equal-map-v128-p-len")
	}

	for _, v := range []map[uint32]string{nil, {}, {33: "", 44: "some-string"}} {
		// fmt.Printf(">>>> running mammoth map v129: %v\n", v)
		var v129v1, v129v2 map[uint32]string
		v129v1 = v
		bs129 := testMarshalErr(v129v1, h, t, "enc-map-v129")
		if v == nil {
			v129v2 = nil
		} else {
			v129v2 = make(map[uint32]string, len(v))
		} // reset map
		testUnmarshalErr(v129v2, bs129, h, t, "dec-map-v129")
		testDeepEqualErr(v129v1, v129v2, t, "equal-map-v129")
		if v == nil {
			v129v2 = nil
		} else {
			v129v2 = make(map[uint32]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v129v2), bs129, h, t, "dec-map-v129-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v129v1, v129v2, t, "equal-map-v129-noaddr")
		if v == nil {
			v129v2 = nil
		} else {
			v129v2 = make(map[uint32]string, len(v))
		} // reset map
		testUnmarshalErr(&v129v2, bs129, h, t, "dec-map-v129-p-len")
		testDeepEqualErr(v129v1, v129v2, t, "equal-map-v129-p-len")
		bs129 = testMarshalErr(&v129v1, h, t, "enc-map-v129-p")
		v129v2 = nil
		testUnmarshalErr(&v129v2, bs129, h, t, "dec-map-v129-p-nil")
		testDeepEqualErr(v129v1, v129v2, t, "equal-map-v129-p-nil")
		// ...
		if v == nil {
			v129v2 = nil
		} else {
			v129v2 = make(map[uint32]string, len(v))
		} // reset map
		var v129v3, v129v4 typMapMapUint32String
		v129v3 = typMapMapUint32String(v129v1)
		v129v4 = typMapMapUint32String(v129v2)
		bs129 = testMarshalErr(v129v3, h, t, "enc-map-v129-custom")
		testUnmarshalErr(v129v4, bs129, h, t, "dec-map-v129-p-len")
		testDeepEqualErr(v129v3, v129v4, t, "equal-map-v129-p-len")
	}

	for _, v := range []map[uint32]uint{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v130: %v\n", v)
		var v130v1, v130v2 map[uint32]uint
		v130v1 = v
		bs130 := testMarshalErr(v130v1, h, t, "enc-map-v130")
		if v == nil {
			v130v2 = nil
		} else {
			v130v2 = make(map[uint32]uint, len(v))
		} // reset map
		testUnmarshalErr(v130v2, bs130, h, t, "dec-map-v130")
		testDeepEqualErr(v130v1, v130v2, t, "equal-map-v130")
		if v == nil {
			v130v2 = nil
		} else {
			v130v2 = make(map[uint32]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v130v2), bs130, h, t, "dec-map-v130-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v130v1, v130v2, t, "equal-map-v130-noaddr")
		if v == nil {
			v130v2 = nil
		} else {
			v130v2 = make(map[uint32]uint, len(v))
		} // reset map
		testUnmarshalErr(&v130v2, bs130, h, t, "dec-map-v130-p-len")
		testDeepEqualErr(v130v1, v130v2, t, "equal-map-v130-p-len")
		bs130 = testMarshalErr(&v130v1, h, t, "enc-map-v130-p")
		v130v2 = nil
		testUnmarshalErr(&v130v2, bs130, h, t, "dec-map-v130-p-nil")
		testDeepEqualErr(v130v1, v130v2, t, "equal-map-v130-p-nil")
		// ...
		if v == nil {
			v130v2 = nil
		} else {
			v130v2 = make(map[uint32]uint, len(v))
		} // reset map
		var v130v3, v130v4 typMapMapUint32Uint
		v130v3 = typMapMapUint32Uint(v130v1)
		v130v4 = typMapMapUint32Uint(v130v2)
		bs130 = testMarshalErr(v130v3, h, t, "enc-map-v130-custom")
		testUnmarshalErr(v130v4, bs130, h, t, "dec-map-v130-p-len")
		testDeepEqualErr(v130v3, v130v4, t, "equal-map-v130-p-len")
	}

	for _, v := range []map[uint32]uint8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v131: %v\n", v)
		var v131v1, v131v2 map[uint32]uint8
		v131v1 = v
		bs131 := testMarshalErr(v131v1, h, t, "enc-map-v131")
		if v == nil {
			v131v2 = nil
		} else {
			v131v2 = make(map[uint32]uint8, len(v))
		} // reset map
		testUnmarshalErr(v131v2, bs131, h, t, "dec-map-v131")
		testDeepEqualErr(v131v1, v131v2, t, "equal-map-v131")
		if v == nil {
			v131v2 = nil
		} else {
			v131v2 = make(map[uint32]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v131v2), bs131, h, t, "dec-map-v131-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v131v1, v131v2, t, "equal-map-v131-noaddr")
		if v == nil {
			v131v2 = nil
		} else {
			v131v2 = make(map[uint32]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v131v2, bs131, h, t, "dec-map-v131-p-len")
		testDeepEqualErr(v131v1, v131v2, t, "equal-map-v131-p-len")
		bs131 = testMarshalErr(&v131v1, h, t, "enc-map-v131-p")
		v131v2 = nil
		testUnmarshalErr(&v131v2, bs131, h, t, "dec-map-v131-p-nil")
		testDeepEqualErr(v131v1, v131v2, t, "equal-map-v131-p-nil")
		// ...
		if v == nil {
			v131v2 = nil
		} else {
			v131v2 = make(map[uint32]uint8, len(v))
		} // reset map
		var v131v3, v131v4 typMapMapUint32Uint8
		v131v3 = typMapMapUint32Uint8(v131v1)
		v131v4 = typMapMapUint32Uint8(v131v2)
		bs131 = testMarshalErr(v131v3, h, t, "enc-map-v131-custom")
		testUnmarshalErr(v131v4, bs131, h, t, "dec-map-v131-p-len")
		testDeepEqualErr(v131v3, v131v4, t, "equal-map-v131-p-len")
	}

	for _, v := range []map[uint32]uint16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v132: %v\n", v)
		var v132v1, v132v2 map[uint32]uint16
		v132v1 = v
		bs132 := testMarshalErr(v132v1, h, t, "enc-map-v132")
		if v == nil {
			v132v2 = nil
		} else {
			v132v2 = make(map[uint32]uint16, len(v))
		} // reset map
		testUnmarshalErr(v132v2, bs132, h, t, "dec-map-v132")
		testDeepEqualErr(v132v1, v132v2, t, "equal-map-v132")
		if v == nil {
			v132v2 = nil
		} else {
			v132v2 = make(map[uint32]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v132v2), bs132, h, t, "dec-map-v132-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v132v1, v132v2, t, "equal-map-v132-noaddr")
		if v == nil {
			v132v2 = nil
		} else {
			v132v2 = make(map[uint32]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v132v2, bs132, h, t, "dec-map-v132-p-len")
		testDeepEqualErr(v132v1, v132v2, t, "equal-map-v132-p-len")
		bs132 = testMarshalErr(&v132v1, h, t, "enc-map-v132-p")
		v132v2 = nil
		testUnmarshalErr(&v132v2, bs132, h, t, "dec-map-v132-p-nil")
		testDeepEqualErr(v132v1, v132v2, t, "equal-map-v132-p-nil")
		// ...
		if v == nil {
			v132v2 = nil
		} else {
			v132v2 = make(map[uint32]uint16, len(v))
		} // reset map
		var v132v3, v132v4 typMapMapUint32Uint16
		v132v3 = typMapMapUint32Uint16(v132v1)
		v132v4 = typMapMapUint32Uint16(v132v2)
		bs132 = testMarshalErr(v132v3, h, t, "enc-map-v132-custom")
		testUnmarshalErr(v132v4, bs132, h, t, "dec-map-v132-p-len")
		testDeepEqualErr(v132v3, v132v4, t, "equal-map-v132-p-len")
	}

	for _, v := range []map[uint32]uint32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v133: %v\n", v)
		var v133v1, v133v2 map[uint32]uint32
		v133v1 = v
		bs133 := testMarshalErr(v133v1, h, t, "enc-map-v133")
		if v == nil {
			v133v2 = nil
		} else {
			v133v2 = make(map[uint32]uint32, len(v))
		} // reset map
		testUnmarshalErr(v133v2, bs133, h, t, "dec-map-v133")
		testDeepEqualErr(v133v1, v133v2, t, "equal-map-v133")
		if v == nil {
			v133v2 = nil
		} else {
			v133v2 = make(map[uint32]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v133v2), bs133, h, t, "dec-map-v133-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v133v1, v133v2, t, "equal-map-v133-noaddr")
		if v == nil {
			v133v2 = nil
		} else {
			v133v2 = make(map[uint32]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v133v2, bs133, h, t, "dec-map-v133-p-len")
		testDeepEqualErr(v133v1, v133v2, t, "equal-map-v133-p-len")
		bs133 = testMarshalErr(&v133v1, h, t, "enc-map-v133-p")
		v133v2 = nil
		testUnmarshalErr(&v133v2, bs133, h, t, "dec-map-v133-p-nil")
		testDeepEqualErr(v133v1, v133v2, t, "equal-map-v133-p-nil")
		// ...
		if v == nil {
			v133v2 = nil
		} else {
			v133v2 = make(map[uint32]uint32, len(v))
		} // reset map
		var v133v3, v133v4 typMapMapUint32Uint32
		v133v3 = typMapMapUint32Uint32(v133v1)
		v133v4 = typMapMapUint32Uint32(v133v2)
		bs133 = testMarshalErr(v133v3, h, t, "enc-map-v133-custom")
		testUnmarshalErr(v133v4, bs133, h, t, "dec-map-v133-p-len")
		testDeepEqualErr(v133v3, v133v4, t, "equal-map-v133-p-len")
	}

	for _, v := range []map[uint32]uint64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v134: %v\n", v)
		var v134v1, v134v2 map[uint32]uint64
		v134v1 = v
		bs134 := testMarshalErr(v134v1, h, t, "enc-map-v134")
		if v == nil {
			v134v2 = nil
		} else {
			v134v2 = make(map[uint32]uint64, len(v))
		} // reset map
		testUnmarshalErr(v134v2, bs134, h, t, "dec-map-v134")
		testDeepEqualErr(v134v1, v134v2, t, "equal-map-v134")
		if v == nil {
			v134v2 = nil
		} else {
			v134v2 = make(map[uint32]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v134v2), bs134, h, t, "dec-map-v134-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v134v1, v134v2, t, "equal-map-v134-noaddr")
		if v == nil {
			v134v2 = nil
		} else {
			v134v2 = make(map[uint32]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v134v2, bs134, h, t, "dec-map-v134-p-len")
		testDeepEqualErr(v134v1, v134v2, t, "equal-map-v134-p-len")
		bs134 = testMarshalErr(&v134v1, h, t, "enc-map-v134-p")
		v134v2 = nil
		testUnmarshalErr(&v134v2, bs134, h, t, "dec-map-v134-p-nil")
		testDeepEqualErr(v134v1, v134v2, t, "equal-map-v134-p-nil")
		// ...
		if v == nil {
			v134v2 = nil
		} else {
			v134v2 = make(map[uint32]uint64, len(v))
		} // reset map
		var v134v3, v134v4 typMapMapUint32Uint64
		v134v3 = typMapMapUint32Uint64(v134v1)
		v134v4 = typMapMapUint32Uint64(v134v2)
		bs134 = testMarshalErr(v134v3, h, t, "enc-map-v134-custom")
		testUnmarshalErr(v134v4, bs134, h, t, "dec-map-v134-p-len")
		testDeepEqualErr(v134v3, v134v4, t, "equal-map-v134-p-len")
	}

	for _, v := range []map[uint32]uintptr{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v135: %v\n", v)
		var v135v1, v135v2 map[uint32]uintptr
		v135v1 = v
		bs135 := testMarshalErr(v135v1, h, t, "enc-map-v135")
		if v == nil {
			v135v2 = nil
		} else {
			v135v2 = make(map[uint32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v135v2, bs135, h, t, "dec-map-v135")
		testDeepEqualErr(v135v1, v135v2, t, "equal-map-v135")
		if v == nil {
			v135v2 = nil
		} else {
			v135v2 = make(map[uint32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v135v2), bs135, h, t, "dec-map-v135-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v135v1, v135v2, t, "equal-map-v135-noaddr")
		if v == nil {
			v135v2 = nil
		} else {
			v135v2 = make(map[uint32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v135v2, bs135, h, t, "dec-map-v135-p-len")
		testDeepEqualErr(v135v1, v135v2, t, "equal-map-v135-p-len")
		bs135 = testMarshalErr(&v135v1, h, t, "enc-map-v135-p")
		v135v2 = nil
		testUnmarshalErr(&v135v2, bs135, h, t, "dec-map-v135-p-nil")
		testDeepEqualErr(v135v1, v135v2, t, "equal-map-v135-p-nil")
		// ...
		if v == nil {
			v135v2 = nil
		} else {
			v135v2 = make(map[uint32]uintptr, len(v))
		} // reset map
		var v135v3, v135v4 typMapMapUint32Uintptr
		v135v3 = typMapMapUint32Uintptr(v135v1)
		v135v4 = typMapMapUint32Uintptr(v135v2)
		bs135 = testMarshalErr(v135v3, h, t, "enc-map-v135-custom")
		testUnmarshalErr(v135v4, bs135, h, t, "dec-map-v135-p-len")
		testDeepEqualErr(v135v3, v135v4, t, "equal-map-v135-p-len")
	}

	for _, v := range []map[uint32]int{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v136: %v\n", v)
		var v136v1, v136v2 map[uint32]int
		v136v1 = v
		bs136 := testMarshalErr(v136v1, h, t, "enc-map-v136")
		if v == nil {
			v136v2 = nil
		} else {
			v136v2 = make(map[uint32]int, len(v))
		} // reset map
		testUnmarshalErr(v136v2, bs136, h, t, "dec-map-v136")
		testDeepEqualErr(v136v1, v136v2, t, "equal-map-v136")
		if v == nil {
			v136v2 = nil
		} else {
			v136v2 = make(map[uint32]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v136v2), bs136, h, t, "dec-map-v136-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v136v1, v136v2, t, "equal-map-v136-noaddr")
		if v == nil {
			v136v2 = nil
		} else {
			v136v2 = make(map[uint32]int, len(v))
		} // reset map
		testUnmarshalErr(&v136v2, bs136, h, t, "dec-map-v136-p-len")
		testDeepEqualErr(v136v1, v136v2, t, "equal-map-v136-p-len")
		bs136 = testMarshalErr(&v136v1, h, t, "enc-map-v136-p")
		v136v2 = nil
		testUnmarshalErr(&v136v2, bs136, h, t, "dec-map-v136-p-nil")
		testDeepEqualErr(v136v1, v136v2, t, "equal-map-v136-p-nil")
		// ...
		if v == nil {
			v136v2 = nil
		} else {
			v136v2 = make(map[uint32]int, len(v))
		} // reset map
		var v136v3, v136v4 typMapMapUint32Int
		v136v3 = typMapMapUint32Int(v136v1)
		v136v4 = typMapMapUint32Int(v136v2)
		bs136 = testMarshalErr(v136v3, h, t, "enc-map-v136-custom")
		testUnmarshalErr(v136v4, bs136, h, t, "dec-map-v136-p-len")
		testDeepEqualErr(v136v3, v136v4, t, "equal-map-v136-p-len")
	}

	for _, v := range []map[uint32]int8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v137: %v\n", v)
		var v137v1, v137v2 map[uint32]int8
		v137v1 = v
		bs137 := testMarshalErr(v137v1, h, t, "enc-map-v137")
		if v == nil {
			v137v2 = nil
		} else {
			v137v2 = make(map[uint32]int8, len(v))
		} // reset map
		testUnmarshalErr(v137v2, bs137, h, t, "dec-map-v137")
		testDeepEqualErr(v137v1, v137v2, t, "equal-map-v137")
		if v == nil {
			v137v2 = nil
		} else {
			v137v2 = make(map[uint32]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v137v2), bs137, h, t, "dec-map-v137-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v137v1, v137v2, t, "equal-map-v137-noaddr")
		if v == nil {
			v137v2 = nil
		} else {
			v137v2 = make(map[uint32]int8, len(v))
		} // reset map
		testUnmarshalErr(&v137v2, bs137, h, t, "dec-map-v137-p-len")
		testDeepEqualErr(v137v1, v137v2, t, "equal-map-v137-p-len")
		bs137 = testMarshalErr(&v137v1, h, t, "enc-map-v137-p")
		v137v2 = nil
		testUnmarshalErr(&v137v2, bs137, h, t, "dec-map-v137-p-nil")
		testDeepEqualErr(v137v1, v137v2, t, "equal-map-v137-p-nil")
		// ...
		if v == nil {
			v137v2 = nil
		} else {
			v137v2 = make(map[uint32]int8, len(v))
		} // reset map
		var v137v3, v137v4 typMapMapUint32Int8
		v137v3 = typMapMapUint32Int8(v137v1)
		v137v4 = typMapMapUint32Int8(v137v2)
		bs137 = testMarshalErr(v137v3, h, t, "enc-map-v137-custom")
		testUnmarshalErr(v137v4, bs137, h, t, "dec-map-v137-p-len")
		testDeepEqualErr(v137v3, v137v4, t, "equal-map-v137-p-len")
	}

	for _, v := range []map[uint32]int16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v138: %v\n", v)
		var v138v1, v138v2 map[uint32]int16
		v138v1 = v
		bs138 := testMarshalErr(v138v1, h, t, "enc-map-v138")
		if v == nil {
			v138v2 = nil
		} else {
			v138v2 = make(map[uint32]int16, len(v))
		} // reset map
		testUnmarshalErr(v138v2, bs138, h, t, "dec-map-v138")
		testDeepEqualErr(v138v1, v138v2, t, "equal-map-v138")
		if v == nil {
			v138v2 = nil
		} else {
			v138v2 = make(map[uint32]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v138v2), bs138, h, t, "dec-map-v138-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v138v1, v138v2, t, "equal-map-v138-noaddr")
		if v == nil {
			v138v2 = nil
		} else {
			v138v2 = make(map[uint32]int16, len(v))
		} // reset map
		testUnmarshalErr(&v138v2, bs138, h, t, "dec-map-v138-p-len")
		testDeepEqualErr(v138v1, v138v2, t, "equal-map-v138-p-len")
		bs138 = testMarshalErr(&v138v1, h, t, "enc-map-v138-p")
		v138v2 = nil
		testUnmarshalErr(&v138v2, bs138, h, t, "dec-map-v138-p-nil")
		testDeepEqualErr(v138v1, v138v2, t, "equal-map-v138-p-nil")
		// ...
		if v == nil {
			v138v2 = nil
		} else {
			v138v2 = make(map[uint32]int16, len(v))
		} // reset map
		var v138v3, v138v4 typMapMapUint32Int16
		v138v3 = typMapMapUint32Int16(v138v1)
		v138v4 = typMapMapUint32Int16(v138v2)
		bs138 = testMarshalErr(v138v3, h, t, "enc-map-v138-custom")
		testUnmarshalErr(v138v4, bs138, h, t, "dec-map-v138-p-len")
		testDeepEqualErr(v138v3, v138v4, t, "equal-map-v138-p-len")
	}

	for _, v := range []map[uint32]int32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v139: %v\n", v)
		var v139v1, v139v2 map[uint32]int32
		v139v1 = v
		bs139 := testMarshalErr(v139v1, h, t, "enc-map-v139")
		if v == nil {
			v139v2 = nil
		} else {
			v139v2 = make(map[uint32]int32, len(v))
		} // reset map
		testUnmarshalErr(v139v2, bs139, h, t, "dec-map-v139")
		testDeepEqualErr(v139v1, v139v2, t, "equal-map-v139")
		if v == nil {
			v139v2 = nil
		} else {
			v139v2 = make(map[uint32]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v139v2), bs139, h, t, "dec-map-v139-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v139v1, v139v2, t, "equal-map-v139-noaddr")
		if v == nil {
			v139v2 = nil
		} else {
			v139v2 = make(map[uint32]int32, len(v))
		} // reset map
		testUnmarshalErr(&v139v2, bs139, h, t, "dec-map-v139-p-len")
		testDeepEqualErr(v139v1, v139v2, t, "equal-map-v139-p-len")
		bs139 = testMarshalErr(&v139v1, h, t, "enc-map-v139-p")
		v139v2 = nil
		testUnmarshalErr(&v139v2, bs139, h, t, "dec-map-v139-p-nil")
		testDeepEqualErr(v139v1, v139v2, t, "equal-map-v139-p-nil")
		// ...
		if v == nil {
			v139v2 = nil
		} else {
			v139v2 = make(map[uint32]int32, len(v))
		} // reset map
		var v139v3, v139v4 typMapMapUint32Int32
		v139v3 = typMapMapUint32Int32(v139v1)
		v139v4 = typMapMapUint32Int32(v139v2)
		bs139 = testMarshalErr(v139v3, h, t, "enc-map-v139-custom")
		testUnmarshalErr(v139v4, bs139, h, t, "dec-map-v139-p-len")
		testDeepEqualErr(v139v3, v139v4, t, "equal-map-v139-p-len")
	}

	for _, v := range []map[uint32]int64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v140: %v\n", v)
		var v140v1, v140v2 map[uint32]int64
		v140v1 = v
		bs140 := testMarshalErr(v140v1, h, t, "enc-map-v140")
		if v == nil {
			v140v2 = nil
		} else {
			v140v2 = make(map[uint32]int64, len(v))
		} // reset map
		testUnmarshalErr(v140v2, bs140, h, t, "dec-map-v140")
		testDeepEqualErr(v140v1, v140v2, t, "equal-map-v140")
		if v == nil {
			v140v2 = nil
		} else {
			v140v2 = make(map[uint32]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v140v2), bs140, h, t, "dec-map-v140-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v140v1, v140v2, t, "equal-map-v140-noaddr")
		if v == nil {
			v140v2 = nil
		} else {
			v140v2 = make(map[uint32]int64, len(v))
		} // reset map
		testUnmarshalErr(&v140v2, bs140, h, t, "dec-map-v140-p-len")
		testDeepEqualErr(v140v1, v140v2, t, "equal-map-v140-p-len")
		bs140 = testMarshalErr(&v140v1, h, t, "enc-map-v140-p")
		v140v2 = nil
		testUnmarshalErr(&v140v2, bs140, h, t, "dec-map-v140-p-nil")
		testDeepEqualErr(v140v1, v140v2, t, "equal-map-v140-p-nil")
		// ...
		if v == nil {
			v140v2 = nil
		} else {
			v140v2 = make(map[uint32]int64, len(v))
		} // reset map
		var v140v3, v140v4 typMapMapUint32Int64
		v140v3 = typMapMapUint32Int64(v140v1)
		v140v4 = typMapMapUint32Int64(v140v2)
		bs140 = testMarshalErr(v140v3, h, t, "enc-map-v140-custom")
		testUnmarshalErr(v140v4, bs140, h, t, "dec-map-v140-p-len")
		testDeepEqualErr(v140v3, v140v4, t, "equal-map-v140-p-len")
	}

	for _, v := range []map[uint32]float32{nil, {}, {44: 0, 33: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v141: %v\n", v)
		var v141v1, v141v2 map[uint32]float32
		v141v1 = v
		bs141 := testMarshalErr(v141v1, h, t, "enc-map-v141")
		if v == nil {
			v141v2 = nil
		} else {
			v141v2 = make(map[uint32]float32, len(v))
		} // reset map
		testUnmarshalErr(v141v2, bs141, h, t, "dec-map-v141")
		testDeepEqualErr(v141v1, v141v2, t, "equal-map-v141")
		if v == nil {
			v141v2 = nil
		} else {
			v141v2 = make(map[uint32]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v141v2), bs141, h, t, "dec-map-v141-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v141v1, v141v2, t, "equal-map-v141-noaddr")
		if v == nil {
			v141v2 = nil
		} else {
			v141v2 = make(map[uint32]float32, len(v))
		} // reset map
		testUnmarshalErr(&v141v2, bs141, h, t, "dec-map-v141-p-len")
		testDeepEqualErr(v141v1, v141v2, t, "equal-map-v141-p-len")
		bs141 = testMarshalErr(&v141v1, h, t, "enc-map-v141-p")
		v141v2 = nil
		testUnmarshalErr(&v141v2, bs141, h, t, "dec-map-v141-p-nil")
		testDeepEqualErr(v141v1, v141v2, t, "equal-map-v141-p-nil")
		// ...
		if v == nil {
			v141v2 = nil
		} else {
			v141v2 = make(map[uint32]float32, len(v))
		} // reset map
		var v141v3, v141v4 typMapMapUint32Float32
		v141v3 = typMapMapUint32Float32(v141v1)
		v141v4 = typMapMapUint32Float32(v141v2)
		bs141 = testMarshalErr(v141v3, h, t, "enc-map-v141-custom")
		testUnmarshalErr(v141v4, bs141, h, t, "dec-map-v141-p-len")
		testDeepEqualErr(v141v3, v141v4, t, "equal-map-v141-p-len")
	}

	for _, v := range []map[uint32]float64{nil, {}, {44: 0, 33: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v142: %v\n", v)
		var v142v1, v142v2 map[uint32]float64
		v142v1 = v
		bs142 := testMarshalErr(v142v1, h, t, "enc-map-v142")
		if v == nil {
			v142v2 = nil
		} else {
			v142v2 = make(map[uint32]float64, len(v))
		} // reset map
		testUnmarshalErr(v142v2, bs142, h, t, "dec-map-v142")
		testDeepEqualErr(v142v1, v142v2, t, "equal-map-v142")
		if v == nil {
			v142v2 = nil
		} else {
			v142v2 = make(map[uint32]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v142v2), bs142, h, t, "dec-map-v142-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v142v1, v142v2, t, "equal-map-v142-noaddr")
		if v == nil {
			v142v2 = nil
		} else {
			v142v2 = make(map[uint32]float64, len(v))
		} // reset map
		testUnmarshalErr(&v142v2, bs142, h, t, "dec-map-v142-p-len")
		testDeepEqualErr(v142v1, v142v2, t, "equal-map-v142-p-len")
		bs142 = testMarshalErr(&v142v1, h, t, "enc-map-v142-p")
		v142v2 = nil
		testUnmarshalErr(&v142v2, bs142, h, t, "dec-map-v142-p-nil")
		testDeepEqualErr(v142v1, v142v2, t, "equal-map-v142-p-nil")
		// ...
		if v == nil {
			v142v2 = nil
		} else {
			v142v2 = make(map[uint32]float64, len(v))
		} // reset map
		var v142v3, v142v4 typMapMapUint32Float64
		v142v3 = typMapMapUint32Float64(v142v1)
		v142v4 = typMapMapUint32Float64(v142v2)
		bs142 = testMarshalErr(v142v3, h, t, "enc-map-v142-custom")
		testUnmarshalErr(v142v4, bs142, h, t, "dec-map-v142-p-len")
		testDeepEqualErr(v142v3, v142v4, t, "equal-map-v142-p-len")
	}

	for _, v := range []map[uint32]bool{nil, {}, {44: false, 33: true}} {
		// fmt.Printf(">>>> running mammoth map v143: %v\n", v)
		var v143v1, v143v2 map[uint32]bool
		v143v1 = v
		bs143 := testMarshalErr(v143v1, h, t, "enc-map-v143")
		if v == nil {
			v143v2 = nil
		} else {
			v143v2 = make(map[uint32]bool, len(v))
		} // reset map
		testUnmarshalErr(v143v2, bs143, h, t, "dec-map-v143")
		testDeepEqualErr(v143v1, v143v2, t, "equal-map-v143")
		if v == nil {
			v143v2 = nil
		} else {
			v143v2 = make(map[uint32]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v143v2), bs143, h, t, "dec-map-v143-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v143v1, v143v2, t, "equal-map-v143-noaddr")
		if v == nil {
			v143v2 = nil
		} else {
			v143v2 = make(map[uint32]bool, len(v))
		} // reset map
		testUnmarshalErr(&v143v2, bs143, h, t, "dec-map-v143-p-len")
		testDeepEqualErr(v143v1, v143v2, t, "equal-map-v143-p-len")
		bs143 = testMarshalErr(&v143v1, h, t, "enc-map-v143-p")
		v143v2 = nil
		testUnmarshalErr(&v143v2, bs143, h, t, "dec-map-v143-p-nil")
		testDeepEqualErr(v143v1, v143v2, t, "equal-map-v143-p-nil")
		// ...
		if v == nil {
			v143v2 = nil
		} else {
			v143v2 = make(map[uint32]bool, len(v))
		} // reset map
		var v143v3, v143v4 typMapMapUint32Bool
		v143v3 = typMapMapUint32Bool(v143v1)
		v143v4 = typMapMapUint32Bool(v143v2)
		bs143 = testMarshalErr(v143v3, h, t, "enc-map-v143-custom")
		testUnmarshalErr(v143v4, bs143, h, t, "dec-map-v143-p-len")
		testDeepEqualErr(v143v3, v143v4, t, "equal-map-v143-p-len")
	}

	for _, v := range []map[uint64]interface{}{nil, {}, {44: nil, 33: "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v146: %v\n", v)
		var v146v1, v146v2 map[uint64]interface{}
		v146v1 = v
		bs146 := testMarshalErr(v146v1, h, t, "enc-map-v146")
		if v == nil {
			v146v2 = nil
		} else {
			v146v2 = make(map[uint64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v146v2, bs146, h, t, "dec-map-v146")
		testDeepEqualErr(v146v1, v146v2, t, "equal-map-v146")
		if v == nil {
			v146v2 = nil
		} else {
			v146v2 = make(map[uint64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v146v2), bs146, h, t, "dec-map-v146-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v146v1, v146v2, t, "equal-map-v146-noaddr")
		if v == nil {
			v146v2 = nil
		} else {
			v146v2 = make(map[uint64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v146v2, bs146, h, t, "dec-map-v146-p-len")
		testDeepEqualErr(v146v1, v146v2, t, "equal-map-v146-p-len")
		bs146 = testMarshalErr(&v146v1, h, t, "enc-map-v146-p")
		v146v2 = nil
		testUnmarshalErr(&v146v2, bs146, h, t, "dec-map-v146-p-nil")
		testDeepEqualErr(v146v1, v146v2, t, "equal-map-v146-p-nil")
		// ...
		if v == nil {
			v146v2 = nil
		} else {
			v146v2 = make(map[uint64]interface{}, len(v))
		} // reset map
		var v146v3, v146v4 typMapMapUint64Intf
		v146v3 = typMapMapUint64Intf(v146v1)
		v146v4 = typMapMapUint64Intf(v146v2)
		bs146 = testMarshalErr(v146v3, h, t, "enc-map-v146-custom")
		testUnmarshalErr(v146v4, bs146, h, t, "dec-map-v146-p-len")
		testDeepEqualErr(v146v3, v146v4, t, "equal-map-v146-p-len")
	}

	for _, v := range []map[uint64]string{nil, {}, {44: "", 33: "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v147: %v\n", v)
		var v147v1, v147v2 map[uint64]string
		v147v1 = v
		bs147 := testMarshalErr(v147v1, h, t, "enc-map-v147")
		if v == nil {
			v147v2 = nil
		} else {
			v147v2 = make(map[uint64]string, len(v))
		} // reset map
		testUnmarshalErr(v147v2, bs147, h, t, "dec-map-v147")
		testDeepEqualErr(v147v1, v147v2, t, "equal-map-v147")
		if v == nil {
			v147v2 = nil
		} else {
			v147v2 = make(map[uint64]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v147v2), bs147, h, t, "dec-map-v147-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v147v1, v147v2, t, "equal-map-v147-noaddr")
		if v == nil {
			v147v2 = nil
		} else {
			v147v2 = make(map[uint64]string, len(v))
		} // reset map
		testUnmarshalErr(&v147v2, bs147, h, t, "dec-map-v147-p-len")
		testDeepEqualErr(v147v1, v147v2, t, "equal-map-v147-p-len")
		bs147 = testMarshalErr(&v147v1, h, t, "enc-map-v147-p")
		v147v2 = nil
		testUnmarshalErr(&v147v2, bs147, h, t, "dec-map-v147-p-nil")
		testDeepEqualErr(v147v1, v147v2, t, "equal-map-v147-p-nil")
		// ...
		if v == nil {
			v147v2 = nil
		} else {
			v147v2 = make(map[uint64]string, len(v))
		} // reset map
		var v147v3, v147v4 typMapMapUint64String
		v147v3 = typMapMapUint64String(v147v1)
		v147v4 = typMapMapUint64String(v147v2)
		bs147 = testMarshalErr(v147v3, h, t, "enc-map-v147-custom")
		testUnmarshalErr(v147v4, bs147, h, t, "dec-map-v147-p-len")
		testDeepEqualErr(v147v3, v147v4, t, "equal-map-v147-p-len")
	}

	for _, v := range []map[uint64]uint{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v148: %v\n", v)
		var v148v1, v148v2 map[uint64]uint
		v148v1 = v
		bs148 := testMarshalErr(v148v1, h, t, "enc-map-v148")
		if v == nil {
			v148v2 = nil
		} else {
			v148v2 = make(map[uint64]uint, len(v))
		} // reset map
		testUnmarshalErr(v148v2, bs148, h, t, "dec-map-v148")
		testDeepEqualErr(v148v1, v148v2, t, "equal-map-v148")
		if v == nil {
			v148v2 = nil
		} else {
			v148v2 = make(map[uint64]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v148v2), bs148, h, t, "dec-map-v148-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v148v1, v148v2, t, "equal-map-v148-noaddr")
		if v == nil {
			v148v2 = nil
		} else {
			v148v2 = make(map[uint64]uint, len(v))
		} // reset map
		testUnmarshalErr(&v148v2, bs148, h, t, "dec-map-v148-p-len")
		testDeepEqualErr(v148v1, v148v2, t, "equal-map-v148-p-len")
		bs148 = testMarshalErr(&v148v1, h, t, "enc-map-v148-p")
		v148v2 = nil
		testUnmarshalErr(&v148v2, bs148, h, t, "dec-map-v148-p-nil")
		testDeepEqualErr(v148v1, v148v2, t, "equal-map-v148-p-nil")
		// ...
		if v == nil {
			v148v2 = nil
		} else {
			v148v2 = make(map[uint64]uint, len(v))
		} // reset map
		var v148v3, v148v4 typMapMapUint64Uint
		v148v3 = typMapMapUint64Uint(v148v1)
		v148v4 = typMapMapUint64Uint(v148v2)
		bs148 = testMarshalErr(v148v3, h, t, "enc-map-v148-custom")
		testUnmarshalErr(v148v4, bs148, h, t, "dec-map-v148-p-len")
		testDeepEqualErr(v148v3, v148v4, t, "equal-map-v148-p-len")
	}

	for _, v := range []map[uint64]uint8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v149: %v\n", v)
		var v149v1, v149v2 map[uint64]uint8
		v149v1 = v
		bs149 := testMarshalErr(v149v1, h, t, "enc-map-v149")
		if v == nil {
			v149v2 = nil
		} else {
			v149v2 = make(map[uint64]uint8, len(v))
		} // reset map
		testUnmarshalErr(v149v2, bs149, h, t, "dec-map-v149")
		testDeepEqualErr(v149v1, v149v2, t, "equal-map-v149")
		if v == nil {
			v149v2 = nil
		} else {
			v149v2 = make(map[uint64]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v149v2), bs149, h, t, "dec-map-v149-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v149v1, v149v2, t, "equal-map-v149-noaddr")
		if v == nil {
			v149v2 = nil
		} else {
			v149v2 = make(map[uint64]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v149v2, bs149, h, t, "dec-map-v149-p-len")
		testDeepEqualErr(v149v1, v149v2, t, "equal-map-v149-p-len")
		bs149 = testMarshalErr(&v149v1, h, t, "enc-map-v149-p")
		v149v2 = nil
		testUnmarshalErr(&v149v2, bs149, h, t, "dec-map-v149-p-nil")
		testDeepEqualErr(v149v1, v149v2, t, "equal-map-v149-p-nil")
		// ...
		if v == nil {
			v149v2 = nil
		} else {
			v149v2 = make(map[uint64]uint8, len(v))
		} // reset map
		var v149v3, v149v4 typMapMapUint64Uint8
		v149v3 = typMapMapUint64Uint8(v149v1)
		v149v4 = typMapMapUint64Uint8(v149v2)
		bs149 = testMarshalErr(v149v3, h, t, "enc-map-v149-custom")
		testUnmarshalErr(v149v4, bs149, h, t, "dec-map-v149-p-len")
		testDeepEqualErr(v149v3, v149v4, t, "equal-map-v149-p-len")
	}

	for _, v := range []map[uint64]uint16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v150: %v\n", v)
		var v150v1, v150v2 map[uint64]uint16
		v150v1 = v
		bs150 := testMarshalErr(v150v1, h, t, "enc-map-v150")
		if v == nil {
			v150v2 = nil
		} else {
			v150v2 = make(map[uint64]uint16, len(v))
		} // reset map
		testUnmarshalErr(v150v2, bs150, h, t, "dec-map-v150")
		testDeepEqualErr(v150v1, v150v2, t, "equal-map-v150")
		if v == nil {
			v150v2 = nil
		} else {
			v150v2 = make(map[uint64]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v150v2), bs150, h, t, "dec-map-v150-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v150v1, v150v2, t, "equal-map-v150-noaddr")
		if v == nil {
			v150v2 = nil
		} else {
			v150v2 = make(map[uint64]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v150v2, bs150, h, t, "dec-map-v150-p-len")
		testDeepEqualErr(v150v1, v150v2, t, "equal-map-v150-p-len")
		bs150 = testMarshalErr(&v150v1, h, t, "enc-map-v150-p")
		v150v2 = nil
		testUnmarshalErr(&v150v2, bs150, h, t, "dec-map-v150-p-nil")
		testDeepEqualErr(v150v1, v150v2, t, "equal-map-v150-p-nil")
		// ...
		if v == nil {
			v150v2 = nil
		} else {
			v150v2 = make(map[uint64]uint16, len(v))
		} // reset map
		var v150v3, v150v4 typMapMapUint64Uint16
		v150v3 = typMapMapUint64Uint16(v150v1)
		v150v4 = typMapMapUint64Uint16(v150v2)
		bs150 = testMarshalErr(v150v3, h, t, "enc-map-v150-custom")
		testUnmarshalErr(v150v4, bs150, h, t, "dec-map-v150-p-len")
		testDeepEqualErr(v150v3, v150v4, t, "equal-map-v150-p-len")
	}

	for _, v := range []map[uint64]uint32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v151: %v\n", v)
		var v151v1, v151v2 map[uint64]uint32
		v151v1 = v
		bs151 := testMarshalErr(v151v1, h, t, "enc-map-v151")
		if v == nil {
			v151v2 = nil
		} else {
			v151v2 = make(map[uint64]uint32, len(v))
		} // reset map
		testUnmarshalErr(v151v2, bs151, h, t, "dec-map-v151")
		testDeepEqualErr(v151v1, v151v2, t, "equal-map-v151")
		if v == nil {
			v151v2 = nil
		} else {
			v151v2 = make(map[uint64]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v151v2), bs151, h, t, "dec-map-v151-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v151v1, v151v2, t, "equal-map-v151-noaddr")
		if v == nil {
			v151v2 = nil
		} else {
			v151v2 = make(map[uint64]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v151v2, bs151, h, t, "dec-map-v151-p-len")
		testDeepEqualErr(v151v1, v151v2, t, "equal-map-v151-p-len")
		bs151 = testMarshalErr(&v151v1, h, t, "enc-map-v151-p")
		v151v2 = nil
		testUnmarshalErr(&v151v2, bs151, h, t, "dec-map-v151-p-nil")
		testDeepEqualErr(v151v1, v151v2, t, "equal-map-v151-p-nil")
		// ...
		if v == nil {
			v151v2 = nil
		} else {
			v151v2 = make(map[uint64]uint32, len(v))
		} // reset map
		var v151v3, v151v4 typMapMapUint64Uint32
		v151v3 = typMapMapUint64Uint32(v151v1)
		v151v4 = typMapMapUint64Uint32(v151v2)
		bs151 = testMarshalErr(v151v3, h, t, "enc-map-v151-custom")
		testUnmarshalErr(v151v4, bs151, h, t, "dec-map-v151-p-len")
		testDeepEqualErr(v151v3, v151v4, t, "equal-map-v151-p-len")
	}

	for _, v := range []map[uint64]uint64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v152: %v\n", v)
		var v152v1, v152v2 map[uint64]uint64
		v152v1 = v
		bs152 := testMarshalErr(v152v1, h, t, "enc-map-v152")
		if v == nil {
			v152v2 = nil
		} else {
			v152v2 = make(map[uint64]uint64, len(v))
		} // reset map
		testUnmarshalErr(v152v2, bs152, h, t, "dec-map-v152")
		testDeepEqualErr(v152v1, v152v2, t, "equal-map-v152")
		if v == nil {
			v152v2 = nil
		} else {
			v152v2 = make(map[uint64]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v152v2), bs152, h, t, "dec-map-v152-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v152v1, v152v2, t, "equal-map-v152-noaddr")
		if v == nil {
			v152v2 = nil
		} else {
			v152v2 = make(map[uint64]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v152v2, bs152, h, t, "dec-map-v152-p-len")
		testDeepEqualErr(v152v1, v152v2, t, "equal-map-v152-p-len")
		bs152 = testMarshalErr(&v152v1, h, t, "enc-map-v152-p")
		v152v2 = nil
		testUnmarshalErr(&v152v2, bs152, h, t, "dec-map-v152-p-nil")
		testDeepEqualErr(v152v1, v152v2, t, "equal-map-v152-p-nil")
		// ...
		if v == nil {
			v152v2 = nil
		} else {
			v152v2 = make(map[uint64]uint64, len(v))
		} // reset map
		var v152v3, v152v4 typMapMapUint64Uint64
		v152v3 = typMapMapUint64Uint64(v152v1)
		v152v4 = typMapMapUint64Uint64(v152v2)
		bs152 = testMarshalErr(v152v3, h, t, "enc-map-v152-custom")
		testUnmarshalErr(v152v4, bs152, h, t, "dec-map-v152-p-len")
		testDeepEqualErr(v152v3, v152v4, t, "equal-map-v152-p-len")
	}

	for _, v := range []map[uint64]uintptr{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v153: %v\n", v)
		var v153v1, v153v2 map[uint64]uintptr
		v153v1 = v
		bs153 := testMarshalErr(v153v1, h, t, "enc-map-v153")
		if v == nil {
			v153v2 = nil
		} else {
			v153v2 = make(map[uint64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v153v2, bs153, h, t, "dec-map-v153")
		testDeepEqualErr(v153v1, v153v2, t, "equal-map-v153")
		if v == nil {
			v153v2 = nil
		} else {
			v153v2 = make(map[uint64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v153v2), bs153, h, t, "dec-map-v153-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v153v1, v153v2, t, "equal-map-v153-noaddr")
		if v == nil {
			v153v2 = nil
		} else {
			v153v2 = make(map[uint64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v153v2, bs153, h, t, "dec-map-v153-p-len")
		testDeepEqualErr(v153v1, v153v2, t, "equal-map-v153-p-len")
		bs153 = testMarshalErr(&v153v1, h, t, "enc-map-v153-p")
		v153v2 = nil
		testUnmarshalErr(&v153v2, bs153, h, t, "dec-map-v153-p-nil")
		testDeepEqualErr(v153v1, v153v2, t, "equal-map-v153-p-nil")
		// ...
		if v == nil {
			v153v2 = nil
		} else {
			v153v2 = make(map[uint64]uintptr, len(v))
		} // reset map
		var v153v3, v153v4 typMapMapUint64Uintptr
		v153v3 = typMapMapUint64Uintptr(v153v1)
		v153v4 = typMapMapUint64Uintptr(v153v2)
		bs153 = testMarshalErr(v153v3, h, t, "enc-map-v153-custom")
		testUnmarshalErr(v153v4, bs153, h, t, "dec-map-v153-p-len")
		testDeepEqualErr(v153v3, v153v4, t, "equal-map-v153-p-len")
	}

	for _, v := range []map[uint64]int{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v154: %v\n", v)
		var v154v1, v154v2 map[uint64]int
		v154v1 = v
		bs154 := testMarshalErr(v154v1, h, t, "enc-map-v154")
		if v == nil {
			v154v2 = nil
		} else {
			v154v2 = make(map[uint64]int, len(v))
		} // reset map
		testUnmarshalErr(v154v2, bs154, h, t, "dec-map-v154")
		testDeepEqualErr(v154v1, v154v2, t, "equal-map-v154")
		if v == nil {
			v154v2 = nil
		} else {
			v154v2 = make(map[uint64]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v154v2), bs154, h, t, "dec-map-v154-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v154v1, v154v2, t, "equal-map-v154-noaddr")
		if v == nil {
			v154v2 = nil
		} else {
			v154v2 = make(map[uint64]int, len(v))
		} // reset map
		testUnmarshalErr(&v154v2, bs154, h, t, "dec-map-v154-p-len")
		testDeepEqualErr(v154v1, v154v2, t, "equal-map-v154-p-len")
		bs154 = testMarshalErr(&v154v1, h, t, "enc-map-v154-p")
		v154v2 = nil
		testUnmarshalErr(&v154v2, bs154, h, t, "dec-map-v154-p-nil")
		testDeepEqualErr(v154v1, v154v2, t, "equal-map-v154-p-nil")
		// ...
		if v == nil {
			v154v2 = nil
		} else {
			v154v2 = make(map[uint64]int, len(v))
		} // reset map
		var v154v3, v154v4 typMapMapUint64Int
		v154v3 = typMapMapUint64Int(v154v1)
		v154v4 = typMapMapUint64Int(v154v2)
		bs154 = testMarshalErr(v154v3, h, t, "enc-map-v154-custom")
		testUnmarshalErr(v154v4, bs154, h, t, "dec-map-v154-p-len")
		testDeepEqualErr(v154v3, v154v4, t, "equal-map-v154-p-len")
	}

	for _, v := range []map[uint64]int8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v155: %v\n", v)
		var v155v1, v155v2 map[uint64]int8
		v155v1 = v
		bs155 := testMarshalErr(v155v1, h, t, "enc-map-v155")
		if v == nil {
			v155v2 = nil
		} else {
			v155v2 = make(map[uint64]int8, len(v))
		} // reset map
		testUnmarshalErr(v155v2, bs155, h, t, "dec-map-v155")
		testDeepEqualErr(v155v1, v155v2, t, "equal-map-v155")
		if v == nil {
			v155v2 = nil
		} else {
			v155v2 = make(map[uint64]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v155v2), bs155, h, t, "dec-map-v155-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v155v1, v155v2, t, "equal-map-v155-noaddr")
		if v == nil {
			v155v2 = nil
		} else {
			v155v2 = make(map[uint64]int8, len(v))
		} // reset map
		testUnmarshalErr(&v155v2, bs155, h, t, "dec-map-v155-p-len")
		testDeepEqualErr(v155v1, v155v2, t, "equal-map-v155-p-len")
		bs155 = testMarshalErr(&v155v1, h, t, "enc-map-v155-p")
		v155v2 = nil
		testUnmarshalErr(&v155v2, bs155, h, t, "dec-map-v155-p-nil")
		testDeepEqualErr(v155v1, v155v2, t, "equal-map-v155-p-nil")
		// ...
		if v == nil {
			v155v2 = nil
		} else {
			v155v2 = make(map[uint64]int8, len(v))
		} // reset map
		var v155v3, v155v4 typMapMapUint64Int8
		v155v3 = typMapMapUint64Int8(v155v1)
		v155v4 = typMapMapUint64Int8(v155v2)
		bs155 = testMarshalErr(v155v3, h, t, "enc-map-v155-custom")
		testUnmarshalErr(v155v4, bs155, h, t, "dec-map-v155-p-len")
		testDeepEqualErr(v155v3, v155v4, t, "equal-map-v155-p-len")
	}

	for _, v := range []map[uint64]int16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v156: %v\n", v)
		var v156v1, v156v2 map[uint64]int16
		v156v1 = v
		bs156 := testMarshalErr(v156v1, h, t, "enc-map-v156")
		if v == nil {
			v156v2 = nil
		} else {
			v156v2 = make(map[uint64]int16, len(v))
		} // reset map
		testUnmarshalErr(v156v2, bs156, h, t, "dec-map-v156")
		testDeepEqualErr(v156v1, v156v2, t, "equal-map-v156")
		if v == nil {
			v156v2 = nil
		} else {
			v156v2 = make(map[uint64]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v156v2), bs156, h, t, "dec-map-v156-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v156v1, v156v2, t, "equal-map-v156-noaddr")
		if v == nil {
			v156v2 = nil
		} else {
			v156v2 = make(map[uint64]int16, len(v))
		} // reset map
		testUnmarshalErr(&v156v2, bs156, h, t, "dec-map-v156-p-len")
		testDeepEqualErr(v156v1, v156v2, t, "equal-map-v156-p-len")
		bs156 = testMarshalErr(&v156v1, h, t, "enc-map-v156-p")
		v156v2 = nil
		testUnmarshalErr(&v156v2, bs156, h, t, "dec-map-v156-p-nil")
		testDeepEqualErr(v156v1, v156v2, t, "equal-map-v156-p-nil")
		// ...
		if v == nil {
			v156v2 = nil
		} else {
			v156v2 = make(map[uint64]int16, len(v))
		} // reset map
		var v156v3, v156v4 typMapMapUint64Int16
		v156v3 = typMapMapUint64Int16(v156v1)
		v156v4 = typMapMapUint64Int16(v156v2)
		bs156 = testMarshalErr(v156v3, h, t, "enc-map-v156-custom")
		testUnmarshalErr(v156v4, bs156, h, t, "dec-map-v156-p-len")
		testDeepEqualErr(v156v3, v156v4, t, "equal-map-v156-p-len")
	}

	for _, v := range []map[uint64]int32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v157: %v\n", v)
		var v157v1, v157v2 map[uint64]int32
		v157v1 = v
		bs157 := testMarshalErr(v157v1, h, t, "enc-map-v157")
		if v == nil {
			v157v2 = nil
		} else {
			v157v2 = make(map[uint64]int32, len(v))
		} // reset map
		testUnmarshalErr(v157v2, bs157, h, t, "dec-map-v157")
		testDeepEqualErr(v157v1, v157v2, t, "equal-map-v157")
		if v == nil {
			v157v2 = nil
		} else {
			v157v2 = make(map[uint64]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v157v2), bs157, h, t, "dec-map-v157-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v157v1, v157v2, t, "equal-map-v157-noaddr")
		if v == nil {
			v157v2 = nil
		} else {
			v157v2 = make(map[uint64]int32, len(v))
		} // reset map
		testUnmarshalErr(&v157v2, bs157, h, t, "dec-map-v157-p-len")
		testDeepEqualErr(v157v1, v157v2, t, "equal-map-v157-p-len")
		bs157 = testMarshalErr(&v157v1, h, t, "enc-map-v157-p")
		v157v2 = nil
		testUnmarshalErr(&v157v2, bs157, h, t, "dec-map-v157-p-nil")
		testDeepEqualErr(v157v1, v157v2, t, "equal-map-v157-p-nil")
		// ...
		if v == nil {
			v157v2 = nil
		} else {
			v157v2 = make(map[uint64]int32, len(v))
		} // reset map
		var v157v3, v157v4 typMapMapUint64Int32
		v157v3 = typMapMapUint64Int32(v157v1)
		v157v4 = typMapMapUint64Int32(v157v2)
		bs157 = testMarshalErr(v157v3, h, t, "enc-map-v157-custom")
		testUnmarshalErr(v157v4, bs157, h, t, "dec-map-v157-p-len")
		testDeepEqualErr(v157v3, v157v4, t, "equal-map-v157-p-len")
	}

	for _, v := range []map[uint64]int64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v158: %v\n", v)
		var v158v1, v158v2 map[uint64]int64
		v158v1 = v
		bs158 := testMarshalErr(v158v1, h, t, "enc-map-v158")
		if v == nil {
			v158v2 = nil
		} else {
			v158v2 = make(map[uint64]int64, len(v))
		} // reset map
		testUnmarshalErr(v158v2, bs158, h, t, "dec-map-v158")
		testDeepEqualErr(v158v1, v158v2, t, "equal-map-v158")
		if v == nil {
			v158v2 = nil
		} else {
			v158v2 = make(map[uint64]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v158v2), bs158, h, t, "dec-map-v158-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v158v1, v158v2, t, "equal-map-v158-noaddr")
		if v == nil {
			v158v2 = nil
		} else {
			v158v2 = make(map[uint64]int64, len(v))
		} // reset map
		testUnmarshalErr(&v158v2, bs158, h, t, "dec-map-v158-p-len")
		testDeepEqualErr(v158v1, v158v2, t, "equal-map-v158-p-len")
		bs158 = testMarshalErr(&v158v1, h, t, "enc-map-v158-p")
		v158v2 = nil
		testUnmarshalErr(&v158v2, bs158, h, t, "dec-map-v158-p-nil")
		testDeepEqualErr(v158v1, v158v2, t, "equal-map-v158-p-nil")
		// ...
		if v == nil {
			v158v2 = nil
		} else {
			v158v2 = make(map[uint64]int64, len(v))
		} // reset map
		var v158v3, v158v4 typMapMapUint64Int64
		v158v3 = typMapMapUint64Int64(v158v1)
		v158v4 = typMapMapUint64Int64(v158v2)
		bs158 = testMarshalErr(v158v3, h, t, "enc-map-v158-custom")
		testUnmarshalErr(v158v4, bs158, h, t, "dec-map-v158-p-len")
		testDeepEqualErr(v158v3, v158v4, t, "equal-map-v158-p-len")
	}

	for _, v := range []map[uint64]float32{nil, {}, {33: 0, 44: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v159: %v\n", v)
		var v159v1, v159v2 map[uint64]float32
		v159v1 = v
		bs159 := testMarshalErr(v159v1, h, t, "enc-map-v159")
		if v == nil {
			v159v2 = nil
		} else {
			v159v2 = make(map[uint64]float32, len(v))
		} // reset map
		testUnmarshalErr(v159v2, bs159, h, t, "dec-map-v159")
		testDeepEqualErr(v159v1, v159v2, t, "equal-map-v159")
		if v == nil {
			v159v2 = nil
		} else {
			v159v2 = make(map[uint64]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v159v2), bs159, h, t, "dec-map-v159-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v159v1, v159v2, t, "equal-map-v159-noaddr")
		if v == nil {
			v159v2 = nil
		} else {
			v159v2 = make(map[uint64]float32, len(v))
		} // reset map
		testUnmarshalErr(&v159v2, bs159, h, t, "dec-map-v159-p-len")
		testDeepEqualErr(v159v1, v159v2, t, "equal-map-v159-p-len")
		bs159 = testMarshalErr(&v159v1, h, t, "enc-map-v159-p")
		v159v2 = nil
		testUnmarshalErr(&v159v2, bs159, h, t, "dec-map-v159-p-nil")
		testDeepEqualErr(v159v1, v159v2, t, "equal-map-v159-p-nil")
		// ...
		if v == nil {
			v159v2 = nil
		} else {
			v159v2 = make(map[uint64]float32, len(v))
		} // reset map
		var v159v3, v159v4 typMapMapUint64Float32
		v159v3 = typMapMapUint64Float32(v159v1)
		v159v4 = typMapMapUint64Float32(v159v2)
		bs159 = testMarshalErr(v159v3, h, t, "enc-map-v159-custom")
		testUnmarshalErr(v159v4, bs159, h, t, "dec-map-v159-p-len")
		testDeepEqualErr(v159v3, v159v4, t, "equal-map-v159-p-len")
	}

	for _, v := range []map[uint64]float64{nil, {}, {33: 0, 44: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v160: %v\n", v)
		var v160v1, v160v2 map[uint64]float64
		v160v1 = v
		bs160 := testMarshalErr(v160v1, h, t, "enc-map-v160")
		if v == nil {
			v160v2 = nil
		} else {
			v160v2 = make(map[uint64]float64, len(v))
		} // reset map
		testUnmarshalErr(v160v2, bs160, h, t, "dec-map-v160")
		testDeepEqualErr(v160v1, v160v2, t, "equal-map-v160")
		if v == nil {
			v160v2 = nil
		} else {
			v160v2 = make(map[uint64]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v160v2), bs160, h, t, "dec-map-v160-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v160v1, v160v2, t, "equal-map-v160-noaddr")
		if v == nil {
			v160v2 = nil
		} else {
			v160v2 = make(map[uint64]float64, len(v))
		} // reset map
		testUnmarshalErr(&v160v2, bs160, h, t, "dec-map-v160-p-len")
		testDeepEqualErr(v160v1, v160v2, t, "equal-map-v160-p-len")
		bs160 = testMarshalErr(&v160v1, h, t, "enc-map-v160-p")
		v160v2 = nil
		testUnmarshalErr(&v160v2, bs160, h, t, "dec-map-v160-p-nil")
		testDeepEqualErr(v160v1, v160v2, t, "equal-map-v160-p-nil")
		// ...
		if v == nil {
			v160v2 = nil
		} else {
			v160v2 = make(map[uint64]float64, len(v))
		} // reset map
		var v160v3, v160v4 typMapMapUint64Float64
		v160v3 = typMapMapUint64Float64(v160v1)
		v160v4 = typMapMapUint64Float64(v160v2)
		bs160 = testMarshalErr(v160v3, h, t, "enc-map-v160-custom")
		testUnmarshalErr(v160v4, bs160, h, t, "dec-map-v160-p-len")
		testDeepEqualErr(v160v3, v160v4, t, "equal-map-v160-p-len")
	}

	for _, v := range []map[uint64]bool{nil, {}, {33: false, 44: true}} {
		// fmt.Printf(">>>> running mammoth map v161: %v\n", v)
		var v161v1, v161v2 map[uint64]bool
		v161v1 = v
		bs161 := testMarshalErr(v161v1, h, t, "enc-map-v161")
		if v == nil {
			v161v2 = nil
		} else {
			v161v2 = make(map[uint64]bool, len(v))
		} // reset map
		testUnmarshalErr(v161v2, bs161, h, t, "dec-map-v161")
		testDeepEqualErr(v161v1, v161v2, t, "equal-map-v161")
		if v == nil {
			v161v2 = nil
		} else {
			v161v2 = make(map[uint64]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v161v2), bs161, h, t, "dec-map-v161-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v161v1, v161v2, t, "equal-map-v161-noaddr")
		if v == nil {
			v161v2 = nil
		} else {
			v161v2 = make(map[uint64]bool, len(v))
		} // reset map
		testUnmarshalErr(&v161v2, bs161, h, t, "dec-map-v161-p-len")
		testDeepEqualErr(v161v1, v161v2, t, "equal-map-v161-p-len")
		bs161 = testMarshalErr(&v161v1, h, t, "enc-map-v161-p")
		v161v2 = nil
		testUnmarshalErr(&v161v2, bs161, h, t, "dec-map-v161-p-nil")
		testDeepEqualErr(v161v1, v161v2, t, "equal-map-v161-p-nil")
		// ...
		if v == nil {
			v161v2 = nil
		} else {
			v161v2 = make(map[uint64]bool, len(v))
		} // reset map
		var v161v3, v161v4 typMapMapUint64Bool
		v161v3 = typMapMapUint64Bool(v161v1)
		v161v4 = typMapMapUint64Bool(v161v2)
		bs161 = testMarshalErr(v161v3, h, t, "enc-map-v161-custom")
		testUnmarshalErr(v161v4, bs161, h, t, "dec-map-v161-p-len")
		testDeepEqualErr(v161v3, v161v4, t, "equal-map-v161-p-len")
	}

	for _, v := range []map[uintptr]interface{}{nil, {}, {33: nil, 44: "string-is-an-interface"}} {
		// fmt.Printf(">>>> running mammoth map v164: %v\n", v)
		var v164v1, v164v2 map[uintptr]interface{}
		v164v1 = v
		bs164 := testMarshalErr(v164v1, h, t, "enc-map-v164")
		if v == nil {
			v164v2 = nil
		} else {
			v164v2 = make(map[uintptr]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v164v2, bs164, h, t, "dec-map-v164")
		testDeepEqualErr(v164v1, v164v2, t, "equal-map-v164")
		if v == nil {
			v164v2 = nil
		} else {
			v164v2 = make(map[uintptr]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v164v2), bs164, h, t, "dec-map-v164-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v164v1, v164v2, t, "equal-map-v164-noaddr")
		if v == nil {
			v164v2 = nil
		} else {
			v164v2 = make(map[uintptr]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v164v2, bs164, h, t, "dec-map-v164-p-len")
		testDeepEqualErr(v164v1, v164v2, t, "equal-map-v164-p-len")
		bs164 = testMarshalErr(&v164v1, h, t, "enc-map-v164-p")
		v164v2 = nil
		testUnmarshalErr(&v164v2, bs164, h, t, "dec-map-v164-p-nil")
		testDeepEqualErr(v164v1, v164v2, t, "equal-map-v164-p-nil")
		// ...
		if v == nil {
			v164v2 = nil
		} else {
			v164v2 = make(map[uintptr]interface{}, len(v))
		} // reset map
		var v164v3, v164v4 typMapMapUintptrIntf
		v164v3 = typMapMapUintptrIntf(v164v1)
		v164v4 = typMapMapUintptrIntf(v164v2)
		bs164 = testMarshalErr(v164v3, h, t, "enc-map-v164-custom")
		testUnmarshalErr(v164v4, bs164, h, t, "dec-map-v164-p-len")
		testDeepEqualErr(v164v3, v164v4, t, "equal-map-v164-p-len")
	}

	for _, v := range []map[uintptr]string{nil, {}, {33: "", 44: "some-string"}} {
		// fmt.Printf(">>>> running mammoth map v165: %v\n", v)
		var v165v1, v165v2 map[uintptr]string
		v165v1 = v
		bs165 := testMarshalErr(v165v1, h, t, "enc-map-v165")
		if v == nil {
			v165v2 = nil
		} else {
			v165v2 = make(map[uintptr]string, len(v))
		} // reset map
		testUnmarshalErr(v165v2, bs165, h, t, "dec-map-v165")
		testDeepEqualErr(v165v1, v165v2, t, "equal-map-v165")
		if v == nil {
			v165v2 = nil
		} else {
			v165v2 = make(map[uintptr]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v165v2), bs165, h, t, "dec-map-v165-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v165v1, v165v2, t, "equal-map-v165-noaddr")
		if v == nil {
			v165v2 = nil
		} else {
			v165v2 = make(map[uintptr]string, len(v))
		} // reset map
		testUnmarshalErr(&v165v2, bs165, h, t, "dec-map-v165-p-len")
		testDeepEqualErr(v165v1, v165v2, t, "equal-map-v165-p-len")
		bs165 = testMarshalErr(&v165v1, h, t, "enc-map-v165-p")
		v165v2 = nil
		testUnmarshalErr(&v165v2, bs165, h, t, "dec-map-v165-p-nil")
		testDeepEqualErr(v165v1, v165v2, t, "equal-map-v165-p-nil")
		// ...
		if v == nil {
			v165v2 = nil
		} else {
			v165v2 = make(map[uintptr]string, len(v))
		} // reset map
		var v165v3, v165v4 typMapMapUintptrString
		v165v3 = typMapMapUintptrString(v165v1)
		v165v4 = typMapMapUintptrString(v165v2)
		bs165 = testMarshalErr(v165v3, h, t, "enc-map-v165-custom")
		testUnmarshalErr(v165v4, bs165, h, t, "dec-map-v165-p-len")
		testDeepEqualErr(v165v3, v165v4, t, "equal-map-v165-p-len")
	}

	for _, v := range []map[uintptr]uint{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v166: %v\n", v)
		var v166v1, v166v2 map[uintptr]uint
		v166v1 = v
		bs166 := testMarshalErr(v166v1, h, t, "enc-map-v166")
		if v == nil {
			v166v2 = nil
		} else {
			v166v2 = make(map[uintptr]uint, len(v))
		} // reset map
		testUnmarshalErr(v166v2, bs166, h, t, "dec-map-v166")
		testDeepEqualErr(v166v1, v166v2, t, "equal-map-v166")
		if v == nil {
			v166v2 = nil
		} else {
			v166v2 = make(map[uintptr]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v166v2), bs166, h, t, "dec-map-v166-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v166v1, v166v2, t, "equal-map-v166-noaddr")
		if v == nil {
			v166v2 = nil
		} else {
			v166v2 = make(map[uintptr]uint, len(v))
		} // reset map
		testUnmarshalErr(&v166v2, bs166, h, t, "dec-map-v166-p-len")
		testDeepEqualErr(v166v1, v166v2, t, "equal-map-v166-p-len")
		bs166 = testMarshalErr(&v166v1, h, t, "enc-map-v166-p")
		v166v2 = nil
		testUnmarshalErr(&v166v2, bs166, h, t, "dec-map-v166-p-nil")
		testDeepEqualErr(v166v1, v166v2, t, "equal-map-v166-p-nil")
		// ...
		if v == nil {
			v166v2 = nil
		} else {
			v166v2 = make(map[uintptr]uint, len(v))
		} // reset map
		var v166v3, v166v4 typMapMapUintptrUint
		v166v3 = typMapMapUintptrUint(v166v1)
		v166v4 = typMapMapUintptrUint(v166v2)
		bs166 = testMarshalErr(v166v3, h, t, "enc-map-v166-custom")
		testUnmarshalErr(v166v4, bs166, h, t, "dec-map-v166-p-len")
		testDeepEqualErr(v166v3, v166v4, t, "equal-map-v166-p-len")
	}

	for _, v := range []map[uintptr]uint8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v167: %v\n", v)
		var v167v1, v167v2 map[uintptr]uint8
		v167v1 = v
		bs167 := testMarshalErr(v167v1, h, t, "enc-map-v167")
		if v == nil {
			v167v2 = nil
		} else {
			v167v2 = make(map[uintptr]uint8, len(v))
		} // reset map
		testUnmarshalErr(v167v2, bs167, h, t, "dec-map-v167")
		testDeepEqualErr(v167v1, v167v2, t, "equal-map-v167")
		if v == nil {
			v167v2 = nil
		} else {
			v167v2 = make(map[uintptr]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v167v2), bs167, h, t, "dec-map-v167-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v167v1, v167v2, t, "equal-map-v167-noaddr")
		if v == nil {
			v167v2 = nil
		} else {
			v167v2 = make(map[uintptr]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v167v2, bs167, h, t, "dec-map-v167-p-len")
		testDeepEqualErr(v167v1, v167v2, t, "equal-map-v167-p-len")
		bs167 = testMarshalErr(&v167v1, h, t, "enc-map-v167-p")
		v167v2 = nil
		testUnmarshalErr(&v167v2, bs167, h, t, "dec-map-v167-p-nil")
		testDeepEqualErr(v167v1, v167v2, t, "equal-map-v167-p-nil")
		// ...
		if v == nil {
			v167v2 = nil
		} else {
			v167v2 = make(map[uintptr]uint8, len(v))
		} // reset map
		var v167v3, v167v4 typMapMapUintptrUint8
		v167v3 = typMapMapUintptrUint8(v167v1)
		v167v4 = typMapMapUintptrUint8(v167v2)
		bs167 = testMarshalErr(v167v3, h, t, "enc-map-v167-custom")
		testUnmarshalErr(v167v4, bs167, h, t, "dec-map-v167-p-len")
		testDeepEqualErr(v167v3, v167v4, t, "equal-map-v167-p-len")
	}

	for _, v := range []map[uintptr]uint16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v168: %v\n", v)
		var v168v1, v168v2 map[uintptr]uint16
		v168v1 = v
		bs168 := testMarshalErr(v168v1, h, t, "enc-map-v168")
		if v == nil {
			v168v2 = nil
		} else {
			v168v2 = make(map[uintptr]uint16, len(v))
		} // reset map
		testUnmarshalErr(v168v2, bs168, h, t, "dec-map-v168")
		testDeepEqualErr(v168v1, v168v2, t, "equal-map-v168")
		if v == nil {
			v168v2 = nil
		} else {
			v168v2 = make(map[uintptr]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v168v2), bs168, h, t, "dec-map-v168-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v168v1, v168v2, t, "equal-map-v168-noaddr")
		if v == nil {
			v168v2 = nil
		} else {
			v168v2 = make(map[uintptr]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v168v2, bs168, h, t, "dec-map-v168-p-len")
		testDeepEqualErr(v168v1, v168v2, t, "equal-map-v168-p-len")
		bs168 = testMarshalErr(&v168v1, h, t, "enc-map-v168-p")
		v168v2 = nil
		testUnmarshalErr(&v168v2, bs168, h, t, "dec-map-v168-p-nil")
		testDeepEqualErr(v168v1, v168v2, t, "equal-map-v168-p-nil")
		// ...
		if v == nil {
			v168v2 = nil
		} else {
			v168v2 = make(map[uintptr]uint16, len(v))
		} // reset map
		var v168v3, v168v4 typMapMapUintptrUint16
		v168v3 = typMapMapUintptrUint16(v168v1)
		v168v4 = typMapMapUintptrUint16(v168v2)
		bs168 = testMarshalErr(v168v3, h, t, "enc-map-v168-custom")
		testUnmarshalErr(v168v4, bs168, h, t, "dec-map-v168-p-len")
		testDeepEqualErr(v168v3, v168v4, t, "equal-map-v168-p-len")
	}

	for _, v := range []map[uintptr]uint32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v169: %v\n", v)
		var v169v1, v169v2 map[uintptr]uint32
		v169v1 = v
		bs169 := testMarshalErr(v169v1, h, t, "enc-map-v169")
		if v == nil {
			v169v2 = nil
		} else {
			v169v2 = make(map[uintptr]uint32, len(v))
		} // reset map
		testUnmarshalErr(v169v2, bs169, h, t, "dec-map-v169")
		testDeepEqualErr(v169v1, v169v2, t, "equal-map-v169")
		if v == nil {
			v169v2 = nil
		} else {
			v169v2 = make(map[uintptr]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v169v2), bs169, h, t, "dec-map-v169-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v169v1, v169v2, t, "equal-map-v169-noaddr")
		if v == nil {
			v169v2 = nil
		} else {
			v169v2 = make(map[uintptr]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v169v2, bs169, h, t, "dec-map-v169-p-len")
		testDeepEqualErr(v169v1, v169v2, t, "equal-map-v169-p-len")
		bs169 = testMarshalErr(&v169v1, h, t, "enc-map-v169-p")
		v169v2 = nil
		testUnmarshalErr(&v169v2, bs169, h, t, "dec-map-v169-p-nil")
		testDeepEqualErr(v169v1, v169v2, t, "equal-map-v169-p-nil")
		// ...
		if v == nil {
			v169v2 = nil
		} else {
			v169v2 = make(map[uintptr]uint32, len(v))
		} // reset map
		var v169v3, v169v4 typMapMapUintptrUint32
		v169v3 = typMapMapUintptrUint32(v169v1)
		v169v4 = typMapMapUintptrUint32(v169v2)
		bs169 = testMarshalErr(v169v3, h, t, "enc-map-v169-custom")
		testUnmarshalErr(v169v4, bs169, h, t, "dec-map-v169-p-len")
		testDeepEqualErr(v169v3, v169v4, t, "equal-map-v169-p-len")
	}

	for _, v := range []map[uintptr]uint64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v170: %v\n", v)
		var v170v1, v170v2 map[uintptr]uint64
		v170v1 = v
		bs170 := testMarshalErr(v170v1, h, t, "enc-map-v170")
		if v == nil {
			v170v2 = nil
		} else {
			v170v2 = make(map[uintptr]uint64, len(v))
		} // reset map
		testUnmarshalErr(v170v2, bs170, h, t, "dec-map-v170")
		testDeepEqualErr(v170v1, v170v2, t, "equal-map-v170")
		if v == nil {
			v170v2 = nil
		} else {
			v170v2 = make(map[uintptr]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v170v2), bs170, h, t, "dec-map-v170-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v170v1, v170v2, t, "equal-map-v170-noaddr")
		if v == nil {
			v170v2 = nil
		} else {
			v170v2 = make(map[uintptr]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v170v2, bs170, h, t, "dec-map-v170-p-len")
		testDeepEqualErr(v170v1, v170v2, t, "equal-map-v170-p-len")
		bs170 = testMarshalErr(&v170v1, h, t, "enc-map-v170-p")
		v170v2 = nil
		testUnmarshalErr(&v170v2, bs170, h, t, "dec-map-v170-p-nil")
		testDeepEqualErr(v170v1, v170v2, t, "equal-map-v170-p-nil")
		// ...
		if v == nil {
			v170v2 = nil
		} else {
			v170v2 = make(map[uintptr]uint64, len(v))
		} // reset map
		var v170v3, v170v4 typMapMapUintptrUint64
		v170v3 = typMapMapUintptrUint64(v170v1)
		v170v4 = typMapMapUintptrUint64(v170v2)
		bs170 = testMarshalErr(v170v3, h, t, "enc-map-v170-custom")
		testUnmarshalErr(v170v4, bs170, h, t, "dec-map-v170-p-len")
		testDeepEqualErr(v170v3, v170v4, t, "equal-map-v170-p-len")
	}

	for _, v := range []map[uintptr]uintptr{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v171: %v\n", v)
		var v171v1, v171v2 map[uintptr]uintptr
		v171v1 = v
		bs171 := testMarshalErr(v171v1, h, t, "enc-map-v171")
		if v == nil {
			v171v2 = nil
		} else {
			v171v2 = make(map[uintptr]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v171v2, bs171, h, t, "dec-map-v171")
		testDeepEqualErr(v171v1, v171v2, t, "equal-map-v171")
		if v == nil {
			v171v2 = nil
		} else {
			v171v2 = make(map[uintptr]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v171v2), bs171, h, t, "dec-map-v171-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v171v1, v171v2, t, "equal-map-v171-noaddr")
		if v == nil {
			v171v2 = nil
		} else {
			v171v2 = make(map[uintptr]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v171v2, bs171, h, t, "dec-map-v171-p-len")
		testDeepEqualErr(v171v1, v171v2, t, "equal-map-v171-p-len")
		bs171 = testMarshalErr(&v171v1, h, t, "enc-map-v171-p")
		v171v2 = nil
		testUnmarshalErr(&v171v2, bs171, h, t, "dec-map-v171-p-nil")
		testDeepEqualErr(v171v1, v171v2, t, "equal-map-v171-p-nil")
		// ...
		if v == nil {
			v171v2 = nil
		} else {
			v171v2 = make(map[uintptr]uintptr, len(v))
		} // reset map
		var v171v3, v171v4 typMapMapUintptrUintptr
		v171v3 = typMapMapUintptrUintptr(v171v1)
		v171v4 = typMapMapUintptrUintptr(v171v2)
		bs171 = testMarshalErr(v171v3, h, t, "enc-map-v171-custom")
		testUnmarshalErr(v171v4, bs171, h, t, "dec-map-v171-p-len")
		testDeepEqualErr(v171v3, v171v4, t, "equal-map-v171-p-len")
	}

	for _, v := range []map[uintptr]int{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v172: %v\n", v)
		var v172v1, v172v2 map[uintptr]int
		v172v1 = v
		bs172 := testMarshalErr(v172v1, h, t, "enc-map-v172")
		if v == nil {
			v172v2 = nil
		} else {
			v172v2 = make(map[uintptr]int, len(v))
		} // reset map
		testUnmarshalErr(v172v2, bs172, h, t, "dec-map-v172")
		testDeepEqualErr(v172v1, v172v2, t, "equal-map-v172")
		if v == nil {
			v172v2 = nil
		} else {
			v172v2 = make(map[uintptr]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v172v2), bs172, h, t, "dec-map-v172-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v172v1, v172v2, t, "equal-map-v172-noaddr")
		if v == nil {
			v172v2 = nil
		} else {
			v172v2 = make(map[uintptr]int, len(v))
		} // reset map
		testUnmarshalErr(&v172v2, bs172, h, t, "dec-map-v172-p-len")
		testDeepEqualErr(v172v1, v172v2, t, "equal-map-v172-p-len")
		bs172 = testMarshalErr(&v172v1, h, t, "enc-map-v172-p")
		v172v2 = nil
		testUnmarshalErr(&v172v2, bs172, h, t, "dec-map-v172-p-nil")
		testDeepEqualErr(v172v1, v172v2, t, "equal-map-v172-p-nil")
		// ...
		if v == nil {
			v172v2 = nil
		} else {
			v172v2 = make(map[uintptr]int, len(v))
		} // reset map
		var v172v3, v172v4 typMapMapUintptrInt
		v172v3 = typMapMapUintptrInt(v172v1)
		v172v4 = typMapMapUintptrInt(v172v2)
		bs172 = testMarshalErr(v172v3, h, t, "enc-map-v172-custom")
		testUnmarshalErr(v172v4, bs172, h, t, "dec-map-v172-p-len")
		testDeepEqualErr(v172v3, v172v4, t, "equal-map-v172-p-len")
	}

	for _, v := range []map[uintptr]int8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v173: %v\n", v)
		var v173v1, v173v2 map[uintptr]int8
		v173v1 = v
		bs173 := testMarshalErr(v173v1, h, t, "enc-map-v173")
		if v == nil {
			v173v2 = nil
		} else {
			v173v2 = make(map[uintptr]int8, len(v))
		} // reset map
		testUnmarshalErr(v173v2, bs173, h, t, "dec-map-v173")
		testDeepEqualErr(v173v1, v173v2, t, "equal-map-v173")
		if v == nil {
			v173v2 = nil
		} else {
			v173v2 = make(map[uintptr]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v173v2), bs173, h, t, "dec-map-v173-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v173v1, v173v2, t, "equal-map-v173-noaddr")
		if v == nil {
			v173v2 = nil
		} else {
			v173v2 = make(map[uintptr]int8, len(v))
		} // reset map
		testUnmarshalErr(&v173v2, bs173, h, t, "dec-map-v173-p-len")
		testDeepEqualErr(v173v1, v173v2, t, "equal-map-v173-p-len")
		bs173 = testMarshalErr(&v173v1, h, t, "enc-map-v173-p")
		v173v2 = nil
		testUnmarshalErr(&v173v2, bs173, h, t, "dec-map-v173-p-nil")
		testDeepEqualErr(v173v1, v173v2, t, "equal-map-v173-p-nil")
		// ...
		if v == nil {
			v173v2 = nil
		} else {
			v173v2 = make(map[uintptr]int8, len(v))
		} // reset map
		var v173v3, v173v4 typMapMapUintptrInt8
		v173v3 = typMapMapUintptrInt8(v173v1)
		v173v4 = typMapMapUintptrInt8(v173v2)
		bs173 = testMarshalErr(v173v3, h, t, "enc-map-v173-custom")
		testUnmarshalErr(v173v4, bs173, h, t, "dec-map-v173-p-len")
		testDeepEqualErr(v173v3, v173v4, t, "equal-map-v173-p-len")
	}

	for _, v := range []map[uintptr]int16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v174: %v\n", v)
		var v174v1, v174v2 map[uintptr]int16
		v174v1 = v
		bs174 := testMarshalErr(v174v1, h, t, "enc-map-v174")
		if v == nil {
			v174v2 = nil
		} else {
			v174v2 = make(map[uintptr]int16, len(v))
		} // reset map
		testUnmarshalErr(v174v2, bs174, h, t, "dec-map-v174")
		testDeepEqualErr(v174v1, v174v2, t, "equal-map-v174")
		if v == nil {
			v174v2 = nil
		} else {
			v174v2 = make(map[uintptr]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v174v2), bs174, h, t, "dec-map-v174-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v174v1, v174v2, t, "equal-map-v174-noaddr")
		if v == nil {
			v174v2 = nil
		} else {
			v174v2 = make(map[uintptr]int16, len(v))
		} // reset map
		testUnmarshalErr(&v174v2, bs174, h, t, "dec-map-v174-p-len")
		testDeepEqualErr(v174v1, v174v2, t, "equal-map-v174-p-len")
		bs174 = testMarshalErr(&v174v1, h, t, "enc-map-v174-p")
		v174v2 = nil
		testUnmarshalErr(&v174v2, bs174, h, t, "dec-map-v174-p-nil")
		testDeepEqualErr(v174v1, v174v2, t, "equal-map-v174-p-nil")
		// ...
		if v == nil {
			v174v2 = nil
		} else {
			v174v2 = make(map[uintptr]int16, len(v))
		} // reset map
		var v174v3, v174v4 typMapMapUintptrInt16
		v174v3 = typMapMapUintptrInt16(v174v1)
		v174v4 = typMapMapUintptrInt16(v174v2)
		bs174 = testMarshalErr(v174v3, h, t, "enc-map-v174-custom")
		testUnmarshalErr(v174v4, bs174, h, t, "dec-map-v174-p-len")
		testDeepEqualErr(v174v3, v174v4, t, "equal-map-v174-p-len")
	}

	for _, v := range []map[uintptr]int32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v175: %v\n", v)
		var v175v1, v175v2 map[uintptr]int32
		v175v1 = v
		bs175 := testMarshalErr(v175v1, h, t, "enc-map-v175")
		if v == nil {
			v175v2 = nil
		} else {
			v175v2 = make(map[uintptr]int32, len(v))
		} // reset map
		testUnmarshalErr(v175v2, bs175, h, t, "dec-map-v175")
		testDeepEqualErr(v175v1, v175v2, t, "equal-map-v175")
		if v == nil {
			v175v2 = nil
		} else {
			v175v2 = make(map[uintptr]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v175v2), bs175, h, t, "dec-map-v175-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v175v1, v175v2, t, "equal-map-v175-noaddr")
		if v == nil {
			v175v2 = nil
		} else {
			v175v2 = make(map[uintptr]int32, len(v))
		} // reset map
		testUnmarshalErr(&v175v2, bs175, h, t, "dec-map-v175-p-len")
		testDeepEqualErr(v175v1, v175v2, t, "equal-map-v175-p-len")
		bs175 = testMarshalErr(&v175v1, h, t, "enc-map-v175-p")
		v175v2 = nil
		testUnmarshalErr(&v175v2, bs175, h, t, "dec-map-v175-p-nil")
		testDeepEqualErr(v175v1, v175v2, t, "equal-map-v175-p-nil")
		// ...
		if v == nil {
			v175v2 = nil
		} else {
			v175v2 = make(map[uintptr]int32, len(v))
		} // reset map
		var v175v3, v175v4 typMapMapUintptrInt32
		v175v3 = typMapMapUintptrInt32(v175v1)
		v175v4 = typMapMapUintptrInt32(v175v2)
		bs175 = testMarshalErr(v175v3, h, t, "enc-map-v175-custom")
		testUnmarshalErr(v175v4, bs175, h, t, "dec-map-v175-p-len")
		testDeepEqualErr(v175v3, v175v4, t, "equal-map-v175-p-len")
	}

	for _, v := range []map[uintptr]int64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v176: %v\n", v)
		var v176v1, v176v2 map[uintptr]int64
		v176v1 = v
		bs176 := testMarshalErr(v176v1, h, t, "enc-map-v176")
		if v == nil {
			v176v2 = nil
		} else {
			v176v2 = make(map[uintptr]int64, len(v))
		} // reset map
		testUnmarshalErr(v176v2, bs176, h, t, "dec-map-v176")
		testDeepEqualErr(v176v1, v176v2, t, "equal-map-v176")
		if v == nil {
			v176v2 = nil
		} else {
			v176v2 = make(map[uintptr]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v176v2), bs176, h, t, "dec-map-v176-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v176v1, v176v2, t, "equal-map-v176-noaddr")
		if v == nil {
			v176v2 = nil
		} else {
			v176v2 = make(map[uintptr]int64, len(v))
		} // reset map
		testUnmarshalErr(&v176v2, bs176, h, t, "dec-map-v176-p-len")
		testDeepEqualErr(v176v1, v176v2, t, "equal-map-v176-p-len")
		bs176 = testMarshalErr(&v176v1, h, t, "enc-map-v176-p")
		v176v2 = nil
		testUnmarshalErr(&v176v2, bs176, h, t, "dec-map-v176-p-nil")
		testDeepEqualErr(v176v1, v176v2, t, "equal-map-v176-p-nil")
		// ...
		if v == nil {
			v176v2 = nil
		} else {
			v176v2 = make(map[uintptr]int64, len(v))
		} // reset map
		var v176v3, v176v4 typMapMapUintptrInt64
		v176v3 = typMapMapUintptrInt64(v176v1)
		v176v4 = typMapMapUintptrInt64(v176v2)
		bs176 = testMarshalErr(v176v3, h, t, "enc-map-v176-custom")
		testUnmarshalErr(v176v4, bs176, h, t, "dec-map-v176-p-len")
		testDeepEqualErr(v176v3, v176v4, t, "equal-map-v176-p-len")
	}

	for _, v := range []map[uintptr]float32{nil, {}, {44: 0, 33: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v177: %v\n", v)
		var v177v1, v177v2 map[uintptr]float32
		v177v1 = v
		bs177 := testMarshalErr(v177v1, h, t, "enc-map-v177")
		if v == nil {
			v177v2 = nil
		} else {
			v177v2 = make(map[uintptr]float32, len(v))
		} // reset map
		testUnmarshalErr(v177v2, bs177, h, t, "dec-map-v177")
		testDeepEqualErr(v177v1, v177v2, t, "equal-map-v177")
		if v == nil {
			v177v2 = nil
		} else {
			v177v2 = make(map[uintptr]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v177v2), bs177, h, t, "dec-map-v177-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v177v1, v177v2, t, "equal-map-v177-noaddr")
		if v == nil {
			v177v2 = nil
		} else {
			v177v2 = make(map[uintptr]float32, len(v))
		} // reset map
		testUnmarshalErr(&v177v2, bs177, h, t, "dec-map-v177-p-len")
		testDeepEqualErr(v177v1, v177v2, t, "equal-map-v177-p-len")
		bs177 = testMarshalErr(&v177v1, h, t, "enc-map-v177-p")
		v177v2 = nil
		testUnmarshalErr(&v177v2, bs177, h, t, "dec-map-v177-p-nil")
		testDeepEqualErr(v177v1, v177v2, t, "equal-map-v177-p-nil")
		// ...
		if v == nil {
			v177v2 = nil
		} else {
			v177v2 = make(map[uintptr]float32, len(v))
		} // reset map
		var v177v3, v177v4 typMapMapUintptrFloat32
		v177v3 = typMapMapUintptrFloat32(v177v1)
		v177v4 = typMapMapUintptrFloat32(v177v2)
		bs177 = testMarshalErr(v177v3, h, t, "enc-map-v177-custom")
		testUnmarshalErr(v177v4, bs177, h, t, "dec-map-v177-p-len")
		testDeepEqualErr(v177v3, v177v4, t, "equal-map-v177-p-len")
	}

	for _, v := range []map[uintptr]float64{nil, {}, {44: 0, 33: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v178: %v\n", v)
		var v178v1, v178v2 map[uintptr]float64
		v178v1 = v
		bs178 := testMarshalErr(v178v1, h, t, "enc-map-v178")
		if v == nil {
			v178v2 = nil
		} else {
			v178v2 = make(map[uintptr]float64, len(v))
		} // reset map
		testUnmarshalErr(v178v2, bs178, h, t, "dec-map-v178")
		testDeepEqualErr(v178v1, v178v2, t, "equal-map-v178")
		if v == nil {
			v178v2 = nil
		} else {
			v178v2 = make(map[uintptr]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v178v2), bs178, h, t, "dec-map-v178-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v178v1, v178v2, t, "equal-map-v178-noaddr")
		if v == nil {
			v178v2 = nil
		} else {
			v178v2 = make(map[uintptr]float64, len(v))
		} // reset map
		testUnmarshalErr(&v178v2, bs178, h, t, "dec-map-v178-p-len")
		testDeepEqualErr(v178v1, v178v2, t, "equal-map-v178-p-len")
		bs178 = testMarshalErr(&v178v1, h, t, "enc-map-v178-p")
		v178v2 = nil
		testUnmarshalErr(&v178v2, bs178, h, t, "dec-map-v178-p-nil")
		testDeepEqualErr(v178v1, v178v2, t, "equal-map-v178-p-nil")
		// ...
		if v == nil {
			v178v2 = nil
		} else {
			v178v2 = make(map[uintptr]float64, len(v))
		} // reset map
		var v178v3, v178v4 typMapMapUintptrFloat64
		v178v3 = typMapMapUintptrFloat64(v178v1)
		v178v4 = typMapMapUintptrFloat64(v178v2)
		bs178 = testMarshalErr(v178v3, h, t, "enc-map-v178-custom")
		testUnmarshalErr(v178v4, bs178, h, t, "dec-map-v178-p-len")
		testDeepEqualErr(v178v3, v178v4, t, "equal-map-v178-p-len")
	}

	for _, v := range []map[uintptr]bool{nil, {}, {44: false, 33: true}} {
		// fmt.Printf(">>>> running mammoth map v179: %v\n", v)
		var v179v1, v179v2 map[uintptr]bool
		v179v1 = v
		bs179 := testMarshalErr(v179v1, h, t, "enc-map-v179")
		if v == nil {
			v179v2 = nil
		} else {
			v179v2 = make(map[uintptr]bool, len(v))
		} // reset map
		testUnmarshalErr(v179v2, bs179, h, t, "dec-map-v179")
		testDeepEqualErr(v179v1, v179v2, t, "equal-map-v179")
		if v == nil {
			v179v2 = nil
		} else {
			v179v2 = make(map[uintptr]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v179v2), bs179, h, t, "dec-map-v179-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v179v1, v179v2, t, "equal-map-v179-noaddr")
		if v == nil {
			v179v2 = nil
		} else {
			v179v2 = make(map[uintptr]bool, len(v))
		} // reset map
		testUnmarshalErr(&v179v2, bs179, h, t, "dec-map-v179-p-len")
		testDeepEqualErr(v179v1, v179v2, t, "equal-map-v179-p-len")
		bs179 = testMarshalErr(&v179v1, h, t, "enc-map-v179-p")
		v179v2 = nil
		testUnmarshalErr(&v179v2, bs179, h, t, "dec-map-v179-p-nil")
		testDeepEqualErr(v179v1, v179v2, t, "equal-map-v179-p-nil")
		// ...
		if v == nil {
			v179v2 = nil
		} else {
			v179v2 = make(map[uintptr]bool, len(v))
		} // reset map
		var v179v3, v179v4 typMapMapUintptrBool
		v179v3 = typMapMapUintptrBool(v179v1)
		v179v4 = typMapMapUintptrBool(v179v2)
		bs179 = testMarshalErr(v179v3, h, t, "enc-map-v179-custom")
		testUnmarshalErr(v179v4, bs179, h, t, "dec-map-v179-p-len")
		testDeepEqualErr(v179v3, v179v4, t, "equal-map-v179-p-len")
	}

	for _, v := range []map[int]interface{}{nil, {}, {44: nil, 33: "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v182: %v\n", v)
		var v182v1, v182v2 map[int]interface{}
		v182v1 = v
		bs182 := testMarshalErr(v182v1, h, t, "enc-map-v182")
		if v == nil {
			v182v2 = nil
		} else {
			v182v2 = make(map[int]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v182v2, bs182, h, t, "dec-map-v182")
		testDeepEqualErr(v182v1, v182v2, t, "equal-map-v182")
		if v == nil {
			v182v2 = nil
		} else {
			v182v2 = make(map[int]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v182v2), bs182, h, t, "dec-map-v182-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v182v1, v182v2, t, "equal-map-v182-noaddr")
		if v == nil {
			v182v2 = nil
		} else {
			v182v2 = make(map[int]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v182v2, bs182, h, t, "dec-map-v182-p-len")
		testDeepEqualErr(v182v1, v182v2, t, "equal-map-v182-p-len")
		bs182 = testMarshalErr(&v182v1, h, t, "enc-map-v182-p")
		v182v2 = nil
		testUnmarshalErr(&v182v2, bs182, h, t, "dec-map-v182-p-nil")
		testDeepEqualErr(v182v1, v182v2, t, "equal-map-v182-p-nil")
		// ...
		if v == nil {
			v182v2 = nil
		} else {
			v182v2 = make(map[int]interface{}, len(v))
		} // reset map
		var v182v3, v182v4 typMapMapIntIntf
		v182v3 = typMapMapIntIntf(v182v1)
		v182v4 = typMapMapIntIntf(v182v2)
		bs182 = testMarshalErr(v182v3, h, t, "enc-map-v182-custom")
		testUnmarshalErr(v182v4, bs182, h, t, "dec-map-v182-p-len")
		testDeepEqualErr(v182v3, v182v4, t, "equal-map-v182-p-len")
	}

	for _, v := range []map[int]string{nil, {}, {44: "", 33: "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v183: %v\n", v)
		var v183v1, v183v2 map[int]string
		v183v1 = v
		bs183 := testMarshalErr(v183v1, h, t, "enc-map-v183")
		if v == nil {
			v183v2 = nil
		} else {
			v183v2 = make(map[int]string, len(v))
		} // reset map
		testUnmarshalErr(v183v2, bs183, h, t, "dec-map-v183")
		testDeepEqualErr(v183v1, v183v2, t, "equal-map-v183")
		if v == nil {
			v183v2 = nil
		} else {
			v183v2 = make(map[int]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v183v2), bs183, h, t, "dec-map-v183-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v183v1, v183v2, t, "equal-map-v183-noaddr")
		if v == nil {
			v183v2 = nil
		} else {
			v183v2 = make(map[int]string, len(v))
		} // reset map
		testUnmarshalErr(&v183v2, bs183, h, t, "dec-map-v183-p-len")
		testDeepEqualErr(v183v1, v183v2, t, "equal-map-v183-p-len")
		bs183 = testMarshalErr(&v183v1, h, t, "enc-map-v183-p")
		v183v2 = nil
		testUnmarshalErr(&v183v2, bs183, h, t, "dec-map-v183-p-nil")
		testDeepEqualErr(v183v1, v183v2, t, "equal-map-v183-p-nil")
		// ...
		if v == nil {
			v183v2 = nil
		} else {
			v183v2 = make(map[int]string, len(v))
		} // reset map
		var v183v3, v183v4 typMapMapIntString
		v183v3 = typMapMapIntString(v183v1)
		v183v4 = typMapMapIntString(v183v2)
		bs183 = testMarshalErr(v183v3, h, t, "enc-map-v183-custom")
		testUnmarshalErr(v183v4, bs183, h, t, "dec-map-v183-p-len")
		testDeepEqualErr(v183v3, v183v4, t, "equal-map-v183-p-len")
	}

	for _, v := range []map[int]uint{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v184: %v\n", v)
		var v184v1, v184v2 map[int]uint
		v184v1 = v
		bs184 := testMarshalErr(v184v1, h, t, "enc-map-v184")
		if v == nil {
			v184v2 = nil
		} else {
			v184v2 = make(map[int]uint, len(v))
		} // reset map
		testUnmarshalErr(v184v2, bs184, h, t, "dec-map-v184")
		testDeepEqualErr(v184v1, v184v2, t, "equal-map-v184")
		if v == nil {
			v184v2 = nil
		} else {
			v184v2 = make(map[int]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v184v2), bs184, h, t, "dec-map-v184-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v184v1, v184v2, t, "equal-map-v184-noaddr")
		if v == nil {
			v184v2 = nil
		} else {
			v184v2 = make(map[int]uint, len(v))
		} // reset map
		testUnmarshalErr(&v184v2, bs184, h, t, "dec-map-v184-p-len")
		testDeepEqualErr(v184v1, v184v2, t, "equal-map-v184-p-len")
		bs184 = testMarshalErr(&v184v1, h, t, "enc-map-v184-p")
		v184v2 = nil
		testUnmarshalErr(&v184v2, bs184, h, t, "dec-map-v184-p-nil")
		testDeepEqualErr(v184v1, v184v2, t, "equal-map-v184-p-nil")
		// ...
		if v == nil {
			v184v2 = nil
		} else {
			v184v2 = make(map[int]uint, len(v))
		} // reset map
		var v184v3, v184v4 typMapMapIntUint
		v184v3 = typMapMapIntUint(v184v1)
		v184v4 = typMapMapIntUint(v184v2)
		bs184 = testMarshalErr(v184v3, h, t, "enc-map-v184-custom")
		testUnmarshalErr(v184v4, bs184, h, t, "dec-map-v184-p-len")
		testDeepEqualErr(v184v3, v184v4, t, "equal-map-v184-p-len")
	}

	for _, v := range []map[int]uint8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v185: %v\n", v)
		var v185v1, v185v2 map[int]uint8
		v185v1 = v
		bs185 := testMarshalErr(v185v1, h, t, "enc-map-v185")
		if v == nil {
			v185v2 = nil
		} else {
			v185v2 = make(map[int]uint8, len(v))
		} // reset map
		testUnmarshalErr(v185v2, bs185, h, t, "dec-map-v185")
		testDeepEqualErr(v185v1, v185v2, t, "equal-map-v185")
		if v == nil {
			v185v2 = nil
		} else {
			v185v2 = make(map[int]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v185v2), bs185, h, t, "dec-map-v185-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v185v1, v185v2, t, "equal-map-v185-noaddr")
		if v == nil {
			v185v2 = nil
		} else {
			v185v2 = make(map[int]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v185v2, bs185, h, t, "dec-map-v185-p-len")
		testDeepEqualErr(v185v1, v185v2, t, "equal-map-v185-p-len")
		bs185 = testMarshalErr(&v185v1, h, t, "enc-map-v185-p")
		v185v2 = nil
		testUnmarshalErr(&v185v2, bs185, h, t, "dec-map-v185-p-nil")
		testDeepEqualErr(v185v1, v185v2, t, "equal-map-v185-p-nil")
		// ...
		if v == nil {
			v185v2 = nil
		} else {
			v185v2 = make(map[int]uint8, len(v))
		} // reset map
		var v185v3, v185v4 typMapMapIntUint8
		v185v3 = typMapMapIntUint8(v185v1)
		v185v4 = typMapMapIntUint8(v185v2)
		bs185 = testMarshalErr(v185v3, h, t, "enc-map-v185-custom")
		testUnmarshalErr(v185v4, bs185, h, t, "dec-map-v185-p-len")
		testDeepEqualErr(v185v3, v185v4, t, "equal-map-v185-p-len")
	}

	for _, v := range []map[int]uint16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v186: %v\n", v)
		var v186v1, v186v2 map[int]uint16
		v186v1 = v
		bs186 := testMarshalErr(v186v1, h, t, "enc-map-v186")
		if v == nil {
			v186v2 = nil
		} else {
			v186v2 = make(map[int]uint16, len(v))
		} // reset map
		testUnmarshalErr(v186v2, bs186, h, t, "dec-map-v186")
		testDeepEqualErr(v186v1, v186v2, t, "equal-map-v186")
		if v == nil {
			v186v2 = nil
		} else {
			v186v2 = make(map[int]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v186v2), bs186, h, t, "dec-map-v186-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v186v1, v186v2, t, "equal-map-v186-noaddr")
		if v == nil {
			v186v2 = nil
		} else {
			v186v2 = make(map[int]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v186v2, bs186, h, t, "dec-map-v186-p-len")
		testDeepEqualErr(v186v1, v186v2, t, "equal-map-v186-p-len")
		bs186 = testMarshalErr(&v186v1, h, t, "enc-map-v186-p")
		v186v2 = nil
		testUnmarshalErr(&v186v2, bs186, h, t, "dec-map-v186-p-nil")
		testDeepEqualErr(v186v1, v186v2, t, "equal-map-v186-p-nil")
		// ...
		if v == nil {
			v186v2 = nil
		} else {
			v186v2 = make(map[int]uint16, len(v))
		} // reset map
		var v186v3, v186v4 typMapMapIntUint16
		v186v3 = typMapMapIntUint16(v186v1)
		v186v4 = typMapMapIntUint16(v186v2)
		bs186 = testMarshalErr(v186v3, h, t, "enc-map-v186-custom")
		testUnmarshalErr(v186v4, bs186, h, t, "dec-map-v186-p-len")
		testDeepEqualErr(v186v3, v186v4, t, "equal-map-v186-p-len")
	}

	for _, v := range []map[int]uint32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v187: %v\n", v)
		var v187v1, v187v2 map[int]uint32
		v187v1 = v
		bs187 := testMarshalErr(v187v1, h, t, "enc-map-v187")
		if v == nil {
			v187v2 = nil
		} else {
			v187v2 = make(map[int]uint32, len(v))
		} // reset map
		testUnmarshalErr(v187v2, bs187, h, t, "dec-map-v187")
		testDeepEqualErr(v187v1, v187v2, t, "equal-map-v187")
		if v == nil {
			v187v2 = nil
		} else {
			v187v2 = make(map[int]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v187v2), bs187, h, t, "dec-map-v187-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v187v1, v187v2, t, "equal-map-v187-noaddr")
		if v == nil {
			v187v2 = nil
		} else {
			v187v2 = make(map[int]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v187v2, bs187, h, t, "dec-map-v187-p-len")
		testDeepEqualErr(v187v1, v187v2, t, "equal-map-v187-p-len")
		bs187 = testMarshalErr(&v187v1, h, t, "enc-map-v187-p")
		v187v2 = nil
		testUnmarshalErr(&v187v2, bs187, h, t, "dec-map-v187-p-nil")
		testDeepEqualErr(v187v1, v187v2, t, "equal-map-v187-p-nil")
		// ...
		if v == nil {
			v187v2 = nil
		} else {
			v187v2 = make(map[int]uint32, len(v))
		} // reset map
		var v187v3, v187v4 typMapMapIntUint32
		v187v3 = typMapMapIntUint32(v187v1)
		v187v4 = typMapMapIntUint32(v187v2)
		bs187 = testMarshalErr(v187v3, h, t, "enc-map-v187-custom")
		testUnmarshalErr(v187v4, bs187, h, t, "dec-map-v187-p-len")
		testDeepEqualErr(v187v3, v187v4, t, "equal-map-v187-p-len")
	}

	for _, v := range []map[int]uint64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v188: %v\n", v)
		var v188v1, v188v2 map[int]uint64
		v188v1 = v
		bs188 := testMarshalErr(v188v1, h, t, "enc-map-v188")
		if v == nil {
			v188v2 = nil
		} else {
			v188v2 = make(map[int]uint64, len(v))
		} // reset map
		testUnmarshalErr(v188v2, bs188, h, t, "dec-map-v188")
		testDeepEqualErr(v188v1, v188v2, t, "equal-map-v188")
		if v == nil {
			v188v2 = nil
		} else {
			v188v2 = make(map[int]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v188v2), bs188, h, t, "dec-map-v188-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v188v1, v188v2, t, "equal-map-v188-noaddr")
		if v == nil {
			v188v2 = nil
		} else {
			v188v2 = make(map[int]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v188v2, bs188, h, t, "dec-map-v188-p-len")
		testDeepEqualErr(v188v1, v188v2, t, "equal-map-v188-p-len")
		bs188 = testMarshalErr(&v188v1, h, t, "enc-map-v188-p")
		v188v2 = nil
		testUnmarshalErr(&v188v2, bs188, h, t, "dec-map-v188-p-nil")
		testDeepEqualErr(v188v1, v188v2, t, "equal-map-v188-p-nil")
		// ...
		if v == nil {
			v188v2 = nil
		} else {
			v188v2 = make(map[int]uint64, len(v))
		} // reset map
		var v188v3, v188v4 typMapMapIntUint64
		v188v3 = typMapMapIntUint64(v188v1)
		v188v4 = typMapMapIntUint64(v188v2)
		bs188 = testMarshalErr(v188v3, h, t, "enc-map-v188-custom")
		testUnmarshalErr(v188v4, bs188, h, t, "dec-map-v188-p-len")
		testDeepEqualErr(v188v3, v188v4, t, "equal-map-v188-p-len")
	}

	for _, v := range []map[int]uintptr{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v189: %v\n", v)
		var v189v1, v189v2 map[int]uintptr
		v189v1 = v
		bs189 := testMarshalErr(v189v1, h, t, "enc-map-v189")
		if v == nil {
			v189v2 = nil
		} else {
			v189v2 = make(map[int]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v189v2, bs189, h, t, "dec-map-v189")
		testDeepEqualErr(v189v1, v189v2, t, "equal-map-v189")
		if v == nil {
			v189v2 = nil
		} else {
			v189v2 = make(map[int]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v189v2), bs189, h, t, "dec-map-v189-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v189v1, v189v2, t, "equal-map-v189-noaddr")
		if v == nil {
			v189v2 = nil
		} else {
			v189v2 = make(map[int]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v189v2, bs189, h, t, "dec-map-v189-p-len")
		testDeepEqualErr(v189v1, v189v2, t, "equal-map-v189-p-len")
		bs189 = testMarshalErr(&v189v1, h, t, "enc-map-v189-p")
		v189v2 = nil
		testUnmarshalErr(&v189v2, bs189, h, t, "dec-map-v189-p-nil")
		testDeepEqualErr(v189v1, v189v2, t, "equal-map-v189-p-nil")
		// ...
		if v == nil {
			v189v2 = nil
		} else {
			v189v2 = make(map[int]uintptr, len(v))
		} // reset map
		var v189v3, v189v4 typMapMapIntUintptr
		v189v3 = typMapMapIntUintptr(v189v1)
		v189v4 = typMapMapIntUintptr(v189v2)
		bs189 = testMarshalErr(v189v3, h, t, "enc-map-v189-custom")
		testUnmarshalErr(v189v4, bs189, h, t, "dec-map-v189-p-len")
		testDeepEqualErr(v189v3, v189v4, t, "equal-map-v189-p-len")
	}

	for _, v := range []map[int]int{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v190: %v\n", v)
		var v190v1, v190v2 map[int]int
		v190v1 = v
		bs190 := testMarshalErr(v190v1, h, t, "enc-map-v190")
		if v == nil {
			v190v2 = nil
		} else {
			v190v2 = make(map[int]int, len(v))
		} // reset map
		testUnmarshalErr(v190v2, bs190, h, t, "dec-map-v190")
		testDeepEqualErr(v190v1, v190v2, t, "equal-map-v190")
		if v == nil {
			v190v2 = nil
		} else {
			v190v2 = make(map[int]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v190v2), bs190, h, t, "dec-map-v190-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v190v1, v190v2, t, "equal-map-v190-noaddr")
		if v == nil {
			v190v2 = nil
		} else {
			v190v2 = make(map[int]int, len(v))
		} // reset map
		testUnmarshalErr(&v190v2, bs190, h, t, "dec-map-v190-p-len")
		testDeepEqualErr(v190v1, v190v2, t, "equal-map-v190-p-len")
		bs190 = testMarshalErr(&v190v1, h, t, "enc-map-v190-p")
		v190v2 = nil
		testUnmarshalErr(&v190v2, bs190, h, t, "dec-map-v190-p-nil")
		testDeepEqualErr(v190v1, v190v2, t, "equal-map-v190-p-nil")
		// ...
		if v == nil {
			v190v2 = nil
		} else {
			v190v2 = make(map[int]int, len(v))
		} // reset map
		var v190v3, v190v4 typMapMapIntInt
		v190v3 = typMapMapIntInt(v190v1)
		v190v4 = typMapMapIntInt(v190v2)
		bs190 = testMarshalErr(v190v3, h, t, "enc-map-v190-custom")
		testUnmarshalErr(v190v4, bs190, h, t, "dec-map-v190-p-len")
		testDeepEqualErr(v190v3, v190v4, t, "equal-map-v190-p-len")
	}

	for _, v := range []map[int]int8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v191: %v\n", v)
		var v191v1, v191v2 map[int]int8
		v191v1 = v
		bs191 := testMarshalErr(v191v1, h, t, "enc-map-v191")
		if v == nil {
			v191v2 = nil
		} else {
			v191v2 = make(map[int]int8, len(v))
		} // reset map
		testUnmarshalErr(v191v2, bs191, h, t, "dec-map-v191")
		testDeepEqualErr(v191v1, v191v2, t, "equal-map-v191")
		if v == nil {
			v191v2 = nil
		} else {
			v191v2 = make(map[int]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v191v2), bs191, h, t, "dec-map-v191-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v191v1, v191v2, t, "equal-map-v191-noaddr")
		if v == nil {
			v191v2 = nil
		} else {
			v191v2 = make(map[int]int8, len(v))
		} // reset map
		testUnmarshalErr(&v191v2, bs191, h, t, "dec-map-v191-p-len")
		testDeepEqualErr(v191v1, v191v2, t, "equal-map-v191-p-len")
		bs191 = testMarshalErr(&v191v1, h, t, "enc-map-v191-p")
		v191v2 = nil
		testUnmarshalErr(&v191v2, bs191, h, t, "dec-map-v191-p-nil")
		testDeepEqualErr(v191v1, v191v2, t, "equal-map-v191-p-nil")
		// ...
		if v == nil {
			v191v2 = nil
		} else {
			v191v2 = make(map[int]int8, len(v))
		} // reset map
		var v191v3, v191v4 typMapMapIntInt8
		v191v3 = typMapMapIntInt8(v191v1)
		v191v4 = typMapMapIntInt8(v191v2)
		bs191 = testMarshalErr(v191v3, h, t, "enc-map-v191-custom")
		testUnmarshalErr(v191v4, bs191, h, t, "dec-map-v191-p-len")
		testDeepEqualErr(v191v3, v191v4, t, "equal-map-v191-p-len")
	}

	for _, v := range []map[int]int16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v192: %v\n", v)
		var v192v1, v192v2 map[int]int16
		v192v1 = v
		bs192 := testMarshalErr(v192v1, h, t, "enc-map-v192")
		if v == nil {
			v192v2 = nil
		} else {
			v192v2 = make(map[int]int16, len(v))
		} // reset map
		testUnmarshalErr(v192v2, bs192, h, t, "dec-map-v192")
		testDeepEqualErr(v192v1, v192v2, t, "equal-map-v192")
		if v == nil {
			v192v2 = nil
		} else {
			v192v2 = make(map[int]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v192v2), bs192, h, t, "dec-map-v192-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v192v1, v192v2, t, "equal-map-v192-noaddr")
		if v == nil {
			v192v2 = nil
		} else {
			v192v2 = make(map[int]int16, len(v))
		} // reset map
		testUnmarshalErr(&v192v2, bs192, h, t, "dec-map-v192-p-len")
		testDeepEqualErr(v192v1, v192v2, t, "equal-map-v192-p-len")
		bs192 = testMarshalErr(&v192v1, h, t, "enc-map-v192-p")
		v192v2 = nil
		testUnmarshalErr(&v192v2, bs192, h, t, "dec-map-v192-p-nil")
		testDeepEqualErr(v192v1, v192v2, t, "equal-map-v192-p-nil")
		// ...
		if v == nil {
			v192v2 = nil
		} else {
			v192v2 = make(map[int]int16, len(v))
		} // reset map
		var v192v3, v192v4 typMapMapIntInt16
		v192v3 = typMapMapIntInt16(v192v1)
		v192v4 = typMapMapIntInt16(v192v2)
		bs192 = testMarshalErr(v192v3, h, t, "enc-map-v192-custom")
		testUnmarshalErr(v192v4, bs192, h, t, "dec-map-v192-p-len")
		testDeepEqualErr(v192v3, v192v4, t, "equal-map-v192-p-len")
	}

	for _, v := range []map[int]int32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v193: %v\n", v)
		var v193v1, v193v2 map[int]int32
		v193v1 = v
		bs193 := testMarshalErr(v193v1, h, t, "enc-map-v193")
		if v == nil {
			v193v2 = nil
		} else {
			v193v2 = make(map[int]int32, len(v))
		} // reset map
		testUnmarshalErr(v193v2, bs193, h, t, "dec-map-v193")
		testDeepEqualErr(v193v1, v193v2, t, "equal-map-v193")
		if v == nil {
			v193v2 = nil
		} else {
			v193v2 = make(map[int]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v193v2), bs193, h, t, "dec-map-v193-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v193v1, v193v2, t, "equal-map-v193-noaddr")
		if v == nil {
			v193v2 = nil
		} else {
			v193v2 = make(map[int]int32, len(v))
		} // reset map
		testUnmarshalErr(&v193v2, bs193, h, t, "dec-map-v193-p-len")
		testDeepEqualErr(v193v1, v193v2, t, "equal-map-v193-p-len")
		bs193 = testMarshalErr(&v193v1, h, t, "enc-map-v193-p")
		v193v2 = nil
		testUnmarshalErr(&v193v2, bs193, h, t, "dec-map-v193-p-nil")
		testDeepEqualErr(v193v1, v193v2, t, "equal-map-v193-p-nil")
		// ...
		if v == nil {
			v193v2 = nil
		} else {
			v193v2 = make(map[int]int32, len(v))
		} // reset map
		var v193v3, v193v4 typMapMapIntInt32
		v193v3 = typMapMapIntInt32(v193v1)
		v193v4 = typMapMapIntInt32(v193v2)
		bs193 = testMarshalErr(v193v3, h, t, "enc-map-v193-custom")
		testUnmarshalErr(v193v4, bs193, h, t, "dec-map-v193-p-len")
		testDeepEqualErr(v193v3, v193v4, t, "equal-map-v193-p-len")
	}

	for _, v := range []map[int]int64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v194: %v\n", v)
		var v194v1, v194v2 map[int]int64
		v194v1 = v
		bs194 := testMarshalErr(v194v1, h, t, "enc-map-v194")
		if v == nil {
			v194v2 = nil
		} else {
			v194v2 = make(map[int]int64, len(v))
		} // reset map
		testUnmarshalErr(v194v2, bs194, h, t, "dec-map-v194")
		testDeepEqualErr(v194v1, v194v2, t, "equal-map-v194")
		if v == nil {
			v194v2 = nil
		} else {
			v194v2 = make(map[int]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v194v2), bs194, h, t, "dec-map-v194-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v194v1, v194v2, t, "equal-map-v194-noaddr")
		if v == nil {
			v194v2 = nil
		} else {
			v194v2 = make(map[int]int64, len(v))
		} // reset map
		testUnmarshalErr(&v194v2, bs194, h, t, "dec-map-v194-p-len")
		testDeepEqualErr(v194v1, v194v2, t, "equal-map-v194-p-len")
		bs194 = testMarshalErr(&v194v1, h, t, "enc-map-v194-p")
		v194v2 = nil
		testUnmarshalErr(&v194v2, bs194, h, t, "dec-map-v194-p-nil")
		testDeepEqualErr(v194v1, v194v2, t, "equal-map-v194-p-nil")
		// ...
		if v == nil {
			v194v2 = nil
		} else {
			v194v2 = make(map[int]int64, len(v))
		} // reset map
		var v194v3, v194v4 typMapMapIntInt64
		v194v3 = typMapMapIntInt64(v194v1)
		v194v4 = typMapMapIntInt64(v194v2)
		bs194 = testMarshalErr(v194v3, h, t, "enc-map-v194-custom")
		testUnmarshalErr(v194v4, bs194, h, t, "dec-map-v194-p-len")
		testDeepEqualErr(v194v3, v194v4, t, "equal-map-v194-p-len")
	}

	for _, v := range []map[int]float32{nil, {}, {33: 0, 44: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v195: %v\n", v)
		var v195v1, v195v2 map[int]float32
		v195v1 = v
		bs195 := testMarshalErr(v195v1, h, t, "enc-map-v195")
		if v == nil {
			v195v2 = nil
		} else {
			v195v2 = make(map[int]float32, len(v))
		} // reset map
		testUnmarshalErr(v195v2, bs195, h, t, "dec-map-v195")
		testDeepEqualErr(v195v1, v195v2, t, "equal-map-v195")
		if v == nil {
			v195v2 = nil
		} else {
			v195v2 = make(map[int]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v195v2), bs195, h, t, "dec-map-v195-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v195v1, v195v2, t, "equal-map-v195-noaddr")
		if v == nil {
			v195v2 = nil
		} else {
			v195v2 = make(map[int]float32, len(v))
		} // reset map
		testUnmarshalErr(&v195v2, bs195, h, t, "dec-map-v195-p-len")
		testDeepEqualErr(v195v1, v195v2, t, "equal-map-v195-p-len")
		bs195 = testMarshalErr(&v195v1, h, t, "enc-map-v195-p")
		v195v2 = nil
		testUnmarshalErr(&v195v2, bs195, h, t, "dec-map-v195-p-nil")
		testDeepEqualErr(v195v1, v195v2, t, "equal-map-v195-p-nil")
		// ...
		if v == nil {
			v195v2 = nil
		} else {
			v195v2 = make(map[int]float32, len(v))
		} // reset map
		var v195v3, v195v4 typMapMapIntFloat32
		v195v3 = typMapMapIntFloat32(v195v1)
		v195v4 = typMapMapIntFloat32(v195v2)
		bs195 = testMarshalErr(v195v3, h, t, "enc-map-v195-custom")
		testUnmarshalErr(v195v4, bs195, h, t, "dec-map-v195-p-len")
		testDeepEqualErr(v195v3, v195v4, t, "equal-map-v195-p-len")
	}

	for _, v := range []map[int]float64{nil, {}, {33: 0, 44: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v196: %v\n", v)
		var v196v1, v196v2 map[int]float64
		v196v1 = v
		bs196 := testMarshalErr(v196v1, h, t, "enc-map-v196")
		if v == nil {
			v196v2 = nil
		} else {
			v196v2 = make(map[int]float64, len(v))
		} // reset map
		testUnmarshalErr(v196v2, bs196, h, t, "dec-map-v196")
		testDeepEqualErr(v196v1, v196v2, t, "equal-map-v196")
		if v == nil {
			v196v2 = nil
		} else {
			v196v2 = make(map[int]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v196v2), bs196, h, t, "dec-map-v196-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v196v1, v196v2, t, "equal-map-v196-noaddr")
		if v == nil {
			v196v2 = nil
		} else {
			v196v2 = make(map[int]float64, len(v))
		} // reset map
		testUnmarshalErr(&v196v2, bs196, h, t, "dec-map-v196-p-len")
		testDeepEqualErr(v196v1, v196v2, t, "equal-map-v196-p-len")
		bs196 = testMarshalErr(&v196v1, h, t, "enc-map-v196-p")
		v196v2 = nil
		testUnmarshalErr(&v196v2, bs196, h, t, "dec-map-v196-p-nil")
		testDeepEqualErr(v196v1, v196v2, t, "equal-map-v196-p-nil")
		// ...
		if v == nil {
			v196v2 = nil
		} else {
			v196v2 = make(map[int]float64, len(v))
		} // reset map
		var v196v3, v196v4 typMapMapIntFloat64
		v196v3 = typMapMapIntFloat64(v196v1)
		v196v4 = typMapMapIntFloat64(v196v2)
		bs196 = testMarshalErr(v196v3, h, t, "enc-map-v196-custom")
		testUnmarshalErr(v196v4, bs196, h, t, "dec-map-v196-p-len")
		testDeepEqualErr(v196v3, v196v4, t, "equal-map-v196-p-len")
	}

	for _, v := range []map[int]bool{nil, {}, {33: false, 44: true}} {
		// fmt.Printf(">>>> running mammoth map v197: %v\n", v)
		var v197v1, v197v2 map[int]bool
		v197v1 = v
		bs197 := testMarshalErr(v197v1, h, t, "enc-map-v197")
		if v == nil {
			v197v2 = nil
		} else {
			v197v2 = make(map[int]bool, len(v))
		} // reset map
		testUnmarshalErr(v197v2, bs197, h, t, "dec-map-v197")
		testDeepEqualErr(v197v1, v197v2, t, "equal-map-v197")
		if v == nil {
			v197v2 = nil
		} else {
			v197v2 = make(map[int]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v197v2), bs197, h, t, "dec-map-v197-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v197v1, v197v2, t, "equal-map-v197-noaddr")
		if v == nil {
			v197v2 = nil
		} else {
			v197v2 = make(map[int]bool, len(v))
		} // reset map
		testUnmarshalErr(&v197v2, bs197, h, t, "dec-map-v197-p-len")
		testDeepEqualErr(v197v1, v197v2, t, "equal-map-v197-p-len")
		bs197 = testMarshalErr(&v197v1, h, t, "enc-map-v197-p")
		v197v2 = nil
		testUnmarshalErr(&v197v2, bs197, h, t, "dec-map-v197-p-nil")
		testDeepEqualErr(v197v1, v197v2, t, "equal-map-v197-p-nil")
		// ...
		if v == nil {
			v197v2 = nil
		} else {
			v197v2 = make(map[int]bool, len(v))
		} // reset map
		var v197v3, v197v4 typMapMapIntBool
		v197v3 = typMapMapIntBool(v197v1)
		v197v4 = typMapMapIntBool(v197v2)
		bs197 = testMarshalErr(v197v3, h, t, "enc-map-v197-custom")
		testUnmarshalErr(v197v4, bs197, h, t, "dec-map-v197-p-len")
		testDeepEqualErr(v197v3, v197v4, t, "equal-map-v197-p-len")
	}

	for _, v := range []map[int8]interface{}{nil, {}, {33: nil, 44: "string-is-an-interface"}} {
		// fmt.Printf(">>>> running mammoth map v200: %v\n", v)
		var v200v1, v200v2 map[int8]interface{}
		v200v1 = v
		bs200 := testMarshalErr(v200v1, h, t, "enc-map-v200")
		if v == nil {
			v200v2 = nil
		} else {
			v200v2 = make(map[int8]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v200v2, bs200, h, t, "dec-map-v200")
		testDeepEqualErr(v200v1, v200v2, t, "equal-map-v200")
		if v == nil {
			v200v2 = nil
		} else {
			v200v2 = make(map[int8]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v200v2), bs200, h, t, "dec-map-v200-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v200v1, v200v2, t, "equal-map-v200-noaddr")
		if v == nil {
			v200v2 = nil
		} else {
			v200v2 = make(map[int8]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v200v2, bs200, h, t, "dec-map-v200-p-len")
		testDeepEqualErr(v200v1, v200v2, t, "equal-map-v200-p-len")
		bs200 = testMarshalErr(&v200v1, h, t, "enc-map-v200-p")
		v200v2 = nil
		testUnmarshalErr(&v200v2, bs200, h, t, "dec-map-v200-p-nil")
		testDeepEqualErr(v200v1, v200v2, t, "equal-map-v200-p-nil")
		// ...
		if v == nil {
			v200v2 = nil
		} else {
			v200v2 = make(map[int8]interface{}, len(v))
		} // reset map
		var v200v3, v200v4 typMapMapInt8Intf
		v200v3 = typMapMapInt8Intf(v200v1)
		v200v4 = typMapMapInt8Intf(v200v2)
		bs200 = testMarshalErr(v200v3, h, t, "enc-map-v200-custom")
		testUnmarshalErr(v200v4, bs200, h, t, "dec-map-v200-p-len")
		testDeepEqualErr(v200v3, v200v4, t, "equal-map-v200-p-len")
	}

	for _, v := range []map[int8]string{nil, {}, {33: "", 44: "some-string"}} {
		// fmt.Printf(">>>> running mammoth map v201: %v\n", v)
		var v201v1, v201v2 map[int8]string
		v201v1 = v
		bs201 := testMarshalErr(v201v1, h, t, "enc-map-v201")
		if v == nil {
			v201v2 = nil
		} else {
			v201v2 = make(map[int8]string, len(v))
		} // reset map
		testUnmarshalErr(v201v2, bs201, h, t, "dec-map-v201")
		testDeepEqualErr(v201v1, v201v2, t, "equal-map-v201")
		if v == nil {
			v201v2 = nil
		} else {
			v201v2 = make(map[int8]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v201v2), bs201, h, t, "dec-map-v201-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v201v1, v201v2, t, "equal-map-v201-noaddr")
		if v == nil {
			v201v2 = nil
		} else {
			v201v2 = make(map[int8]string, len(v))
		} // reset map
		testUnmarshalErr(&v201v2, bs201, h, t, "dec-map-v201-p-len")
		testDeepEqualErr(v201v1, v201v2, t, "equal-map-v201-p-len")
		bs201 = testMarshalErr(&v201v1, h, t, "enc-map-v201-p")
		v201v2 = nil
		testUnmarshalErr(&v201v2, bs201, h, t, "dec-map-v201-p-nil")
		testDeepEqualErr(v201v1, v201v2, t, "equal-map-v201-p-nil")
		// ...
		if v == nil {
			v201v2 = nil
		} else {
			v201v2 = make(map[int8]string, len(v))
		} // reset map
		var v201v3, v201v4 typMapMapInt8String
		v201v3 = typMapMapInt8String(v201v1)
		v201v4 = typMapMapInt8String(v201v2)
		bs201 = testMarshalErr(v201v3, h, t, "enc-map-v201-custom")
		testUnmarshalErr(v201v4, bs201, h, t, "dec-map-v201-p-len")
		testDeepEqualErr(v201v3, v201v4, t, "equal-map-v201-p-len")
	}

	for _, v := range []map[int8]uint{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v202: %v\n", v)
		var v202v1, v202v2 map[int8]uint
		v202v1 = v
		bs202 := testMarshalErr(v202v1, h, t, "enc-map-v202")
		if v == nil {
			v202v2 = nil
		} else {
			v202v2 = make(map[int8]uint, len(v))
		} // reset map
		testUnmarshalErr(v202v2, bs202, h, t, "dec-map-v202")
		testDeepEqualErr(v202v1, v202v2, t, "equal-map-v202")
		if v == nil {
			v202v2 = nil
		} else {
			v202v2 = make(map[int8]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v202v2), bs202, h, t, "dec-map-v202-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v202v1, v202v2, t, "equal-map-v202-noaddr")
		if v == nil {
			v202v2 = nil
		} else {
			v202v2 = make(map[int8]uint, len(v))
		} // reset map
		testUnmarshalErr(&v202v2, bs202, h, t, "dec-map-v202-p-len")
		testDeepEqualErr(v202v1, v202v2, t, "equal-map-v202-p-len")
		bs202 = testMarshalErr(&v202v1, h, t, "enc-map-v202-p")
		v202v2 = nil
		testUnmarshalErr(&v202v2, bs202, h, t, "dec-map-v202-p-nil")
		testDeepEqualErr(v202v1, v202v2, t, "equal-map-v202-p-nil")
		// ...
		if v == nil {
			v202v2 = nil
		} else {
			v202v2 = make(map[int8]uint, len(v))
		} // reset map
		var v202v3, v202v4 typMapMapInt8Uint
		v202v3 = typMapMapInt8Uint(v202v1)
		v202v4 = typMapMapInt8Uint(v202v2)
		bs202 = testMarshalErr(v202v3, h, t, "enc-map-v202-custom")
		testUnmarshalErr(v202v4, bs202, h, t, "dec-map-v202-p-len")
		testDeepEqualErr(v202v3, v202v4, t, "equal-map-v202-p-len")
	}

	for _, v := range []map[int8]uint8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v203: %v\n", v)
		var v203v1, v203v2 map[int8]uint8
		v203v1 = v
		bs203 := testMarshalErr(v203v1, h, t, "enc-map-v203")
		if v == nil {
			v203v2 = nil
		} else {
			v203v2 = make(map[int8]uint8, len(v))
		} // reset map
		testUnmarshalErr(v203v2, bs203, h, t, "dec-map-v203")
		testDeepEqualErr(v203v1, v203v2, t, "equal-map-v203")
		if v == nil {
			v203v2 = nil
		} else {
			v203v2 = make(map[int8]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v203v2), bs203, h, t, "dec-map-v203-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v203v1, v203v2, t, "equal-map-v203-noaddr")
		if v == nil {
			v203v2 = nil
		} else {
			v203v2 = make(map[int8]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v203v2, bs203, h, t, "dec-map-v203-p-len")
		testDeepEqualErr(v203v1, v203v2, t, "equal-map-v203-p-len")
		bs203 = testMarshalErr(&v203v1, h, t, "enc-map-v203-p")
		v203v2 = nil
		testUnmarshalErr(&v203v2, bs203, h, t, "dec-map-v203-p-nil")
		testDeepEqualErr(v203v1, v203v2, t, "equal-map-v203-p-nil")
		// ...
		if v == nil {
			v203v2 = nil
		} else {
			v203v2 = make(map[int8]uint8, len(v))
		} // reset map
		var v203v3, v203v4 typMapMapInt8Uint8
		v203v3 = typMapMapInt8Uint8(v203v1)
		v203v4 = typMapMapInt8Uint8(v203v2)
		bs203 = testMarshalErr(v203v3, h, t, "enc-map-v203-custom")
		testUnmarshalErr(v203v4, bs203, h, t, "dec-map-v203-p-len")
		testDeepEqualErr(v203v3, v203v4, t, "equal-map-v203-p-len")
	}

	for _, v := range []map[int8]uint16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v204: %v\n", v)
		var v204v1, v204v2 map[int8]uint16
		v204v1 = v
		bs204 := testMarshalErr(v204v1, h, t, "enc-map-v204")
		if v == nil {
			v204v2 = nil
		} else {
			v204v2 = make(map[int8]uint16, len(v))
		} // reset map
		testUnmarshalErr(v204v2, bs204, h, t, "dec-map-v204")
		testDeepEqualErr(v204v1, v204v2, t, "equal-map-v204")
		if v == nil {
			v204v2 = nil
		} else {
			v204v2 = make(map[int8]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v204v2), bs204, h, t, "dec-map-v204-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v204v1, v204v2, t, "equal-map-v204-noaddr")
		if v == nil {
			v204v2 = nil
		} else {
			v204v2 = make(map[int8]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v204v2, bs204, h, t, "dec-map-v204-p-len")
		testDeepEqualErr(v204v1, v204v2, t, "equal-map-v204-p-len")
		bs204 = testMarshalErr(&v204v1, h, t, "enc-map-v204-p")
		v204v2 = nil
		testUnmarshalErr(&v204v2, bs204, h, t, "dec-map-v204-p-nil")
		testDeepEqualErr(v204v1, v204v2, t, "equal-map-v204-p-nil")
		// ...
		if v == nil {
			v204v2 = nil
		} else {
			v204v2 = make(map[int8]uint16, len(v))
		} // reset map
		var v204v3, v204v4 typMapMapInt8Uint16
		v204v3 = typMapMapInt8Uint16(v204v1)
		v204v4 = typMapMapInt8Uint16(v204v2)
		bs204 = testMarshalErr(v204v3, h, t, "enc-map-v204-custom")
		testUnmarshalErr(v204v4, bs204, h, t, "dec-map-v204-p-len")
		testDeepEqualErr(v204v3, v204v4, t, "equal-map-v204-p-len")
	}

	for _, v := range []map[int8]uint32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v205: %v\n", v)
		var v205v1, v205v2 map[int8]uint32
		v205v1 = v
		bs205 := testMarshalErr(v205v1, h, t, "enc-map-v205")
		if v == nil {
			v205v2 = nil
		} else {
			v205v2 = make(map[int8]uint32, len(v))
		} // reset map
		testUnmarshalErr(v205v2, bs205, h, t, "dec-map-v205")
		testDeepEqualErr(v205v1, v205v2, t, "equal-map-v205")
		if v == nil {
			v205v2 = nil
		} else {
			v205v2 = make(map[int8]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v205v2), bs205, h, t, "dec-map-v205-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v205v1, v205v2, t, "equal-map-v205-noaddr")
		if v == nil {
			v205v2 = nil
		} else {
			v205v2 = make(map[int8]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v205v2, bs205, h, t, "dec-map-v205-p-len")
		testDeepEqualErr(v205v1, v205v2, t, "equal-map-v205-p-len")
		bs205 = testMarshalErr(&v205v1, h, t, "enc-map-v205-p")
		v205v2 = nil
		testUnmarshalErr(&v205v2, bs205, h, t, "dec-map-v205-p-nil")
		testDeepEqualErr(v205v1, v205v2, t, "equal-map-v205-p-nil")
		// ...
		if v == nil {
			v205v2 = nil
		} else {
			v205v2 = make(map[int8]uint32, len(v))
		} // reset map
		var v205v3, v205v4 typMapMapInt8Uint32
		v205v3 = typMapMapInt8Uint32(v205v1)
		v205v4 = typMapMapInt8Uint32(v205v2)
		bs205 = testMarshalErr(v205v3, h, t, "enc-map-v205-custom")
		testUnmarshalErr(v205v4, bs205, h, t, "dec-map-v205-p-len")
		testDeepEqualErr(v205v3, v205v4, t, "equal-map-v205-p-len")
	}

	for _, v := range []map[int8]uint64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v206: %v\n", v)
		var v206v1, v206v2 map[int8]uint64
		v206v1 = v
		bs206 := testMarshalErr(v206v1, h, t, "enc-map-v206")
		if v == nil {
			v206v2 = nil
		} else {
			v206v2 = make(map[int8]uint64, len(v))
		} // reset map
		testUnmarshalErr(v206v2, bs206, h, t, "dec-map-v206")
		testDeepEqualErr(v206v1, v206v2, t, "equal-map-v206")
		if v == nil {
			v206v2 = nil
		} else {
			v206v2 = make(map[int8]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v206v2), bs206, h, t, "dec-map-v206-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v206v1, v206v2, t, "equal-map-v206-noaddr")
		if v == nil {
			v206v2 = nil
		} else {
			v206v2 = make(map[int8]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v206v2, bs206, h, t, "dec-map-v206-p-len")
		testDeepEqualErr(v206v1, v206v2, t, "equal-map-v206-p-len")
		bs206 = testMarshalErr(&v206v1, h, t, "enc-map-v206-p")
		v206v2 = nil
		testUnmarshalErr(&v206v2, bs206, h, t, "dec-map-v206-p-nil")
		testDeepEqualErr(v206v1, v206v2, t, "equal-map-v206-p-nil")
		// ...
		if v == nil {
			v206v2 = nil
		} else {
			v206v2 = make(map[int8]uint64, len(v))
		} // reset map
		var v206v3, v206v4 typMapMapInt8Uint64
		v206v3 = typMapMapInt8Uint64(v206v1)
		v206v4 = typMapMapInt8Uint64(v206v2)
		bs206 = testMarshalErr(v206v3, h, t, "enc-map-v206-custom")
		testUnmarshalErr(v206v4, bs206, h, t, "dec-map-v206-p-len")
		testDeepEqualErr(v206v3, v206v4, t, "equal-map-v206-p-len")
	}

	for _, v := range []map[int8]uintptr{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v207: %v\n", v)
		var v207v1, v207v2 map[int8]uintptr
		v207v1 = v
		bs207 := testMarshalErr(v207v1, h, t, "enc-map-v207")
		if v == nil {
			v207v2 = nil
		} else {
			v207v2 = make(map[int8]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v207v2, bs207, h, t, "dec-map-v207")
		testDeepEqualErr(v207v1, v207v2, t, "equal-map-v207")
		if v == nil {
			v207v2 = nil
		} else {
			v207v2 = make(map[int8]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v207v2), bs207, h, t, "dec-map-v207-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v207v1, v207v2, t, "equal-map-v207-noaddr")
		if v == nil {
			v207v2 = nil
		} else {
			v207v2 = make(map[int8]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v207v2, bs207, h, t, "dec-map-v207-p-len")
		testDeepEqualErr(v207v1, v207v2, t, "equal-map-v207-p-len")
		bs207 = testMarshalErr(&v207v1, h, t, "enc-map-v207-p")
		v207v2 = nil
		testUnmarshalErr(&v207v2, bs207, h, t, "dec-map-v207-p-nil")
		testDeepEqualErr(v207v1, v207v2, t, "equal-map-v207-p-nil")
		// ...
		if v == nil {
			v207v2 = nil
		} else {
			v207v2 = make(map[int8]uintptr, len(v))
		} // reset map
		var v207v3, v207v4 typMapMapInt8Uintptr
		v207v3 = typMapMapInt8Uintptr(v207v1)
		v207v4 = typMapMapInt8Uintptr(v207v2)
		bs207 = testMarshalErr(v207v3, h, t, "enc-map-v207-custom")
		testUnmarshalErr(v207v4, bs207, h, t, "dec-map-v207-p-len")
		testDeepEqualErr(v207v3, v207v4, t, "equal-map-v207-p-len")
	}

	for _, v := range []map[int8]int{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v208: %v\n", v)
		var v208v1, v208v2 map[int8]int
		v208v1 = v
		bs208 := testMarshalErr(v208v1, h, t, "enc-map-v208")
		if v == nil {
			v208v2 = nil
		} else {
			v208v2 = make(map[int8]int, len(v))
		} // reset map
		testUnmarshalErr(v208v2, bs208, h, t, "dec-map-v208")
		testDeepEqualErr(v208v1, v208v2, t, "equal-map-v208")
		if v == nil {
			v208v2 = nil
		} else {
			v208v2 = make(map[int8]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v208v2), bs208, h, t, "dec-map-v208-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v208v1, v208v2, t, "equal-map-v208-noaddr")
		if v == nil {
			v208v2 = nil
		} else {
			v208v2 = make(map[int8]int, len(v))
		} // reset map
		testUnmarshalErr(&v208v2, bs208, h, t, "dec-map-v208-p-len")
		testDeepEqualErr(v208v1, v208v2, t, "equal-map-v208-p-len")
		bs208 = testMarshalErr(&v208v1, h, t, "enc-map-v208-p")
		v208v2 = nil
		testUnmarshalErr(&v208v2, bs208, h, t, "dec-map-v208-p-nil")
		testDeepEqualErr(v208v1, v208v2, t, "equal-map-v208-p-nil")
		// ...
		if v == nil {
			v208v2 = nil
		} else {
			v208v2 = make(map[int8]int, len(v))
		} // reset map
		var v208v3, v208v4 typMapMapInt8Int
		v208v3 = typMapMapInt8Int(v208v1)
		v208v4 = typMapMapInt8Int(v208v2)
		bs208 = testMarshalErr(v208v3, h, t, "enc-map-v208-custom")
		testUnmarshalErr(v208v4, bs208, h, t, "dec-map-v208-p-len")
		testDeepEqualErr(v208v3, v208v4, t, "equal-map-v208-p-len")
	}

	for _, v := range []map[int8]int8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v209: %v\n", v)
		var v209v1, v209v2 map[int8]int8
		v209v1 = v
		bs209 := testMarshalErr(v209v1, h, t, "enc-map-v209")
		if v == nil {
			v209v2 = nil
		} else {
			v209v2 = make(map[int8]int8, len(v))
		} // reset map
		testUnmarshalErr(v209v2, bs209, h, t, "dec-map-v209")
		testDeepEqualErr(v209v1, v209v2, t, "equal-map-v209")
		if v == nil {
			v209v2 = nil
		} else {
			v209v2 = make(map[int8]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v209v2), bs209, h, t, "dec-map-v209-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v209v1, v209v2, t, "equal-map-v209-noaddr")
		if v == nil {
			v209v2 = nil
		} else {
			v209v2 = make(map[int8]int8, len(v))
		} // reset map
		testUnmarshalErr(&v209v2, bs209, h, t, "dec-map-v209-p-len")
		testDeepEqualErr(v209v1, v209v2, t, "equal-map-v209-p-len")
		bs209 = testMarshalErr(&v209v1, h, t, "enc-map-v209-p")
		v209v2 = nil
		testUnmarshalErr(&v209v2, bs209, h, t, "dec-map-v209-p-nil")
		testDeepEqualErr(v209v1, v209v2, t, "equal-map-v209-p-nil")
		// ...
		if v == nil {
			v209v2 = nil
		} else {
			v209v2 = make(map[int8]int8, len(v))
		} // reset map
		var v209v3, v209v4 typMapMapInt8Int8
		v209v3 = typMapMapInt8Int8(v209v1)
		v209v4 = typMapMapInt8Int8(v209v2)
		bs209 = testMarshalErr(v209v3, h, t, "enc-map-v209-custom")
		testUnmarshalErr(v209v4, bs209, h, t, "dec-map-v209-p-len")
		testDeepEqualErr(v209v3, v209v4, t, "equal-map-v209-p-len")
	}

	for _, v := range []map[int8]int16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v210: %v\n", v)
		var v210v1, v210v2 map[int8]int16
		v210v1 = v
		bs210 := testMarshalErr(v210v1, h, t, "enc-map-v210")
		if v == nil {
			v210v2 = nil
		} else {
			v210v2 = make(map[int8]int16, len(v))
		} // reset map
		testUnmarshalErr(v210v2, bs210, h, t, "dec-map-v210")
		testDeepEqualErr(v210v1, v210v2, t, "equal-map-v210")
		if v == nil {
			v210v2 = nil
		} else {
			v210v2 = make(map[int8]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v210v2), bs210, h, t, "dec-map-v210-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v210v1, v210v2, t, "equal-map-v210-noaddr")
		if v == nil {
			v210v2 = nil
		} else {
			v210v2 = make(map[int8]int16, len(v))
		} // reset map
		testUnmarshalErr(&v210v2, bs210, h, t, "dec-map-v210-p-len")
		testDeepEqualErr(v210v1, v210v2, t, "equal-map-v210-p-len")
		bs210 = testMarshalErr(&v210v1, h, t, "enc-map-v210-p")
		v210v2 = nil
		testUnmarshalErr(&v210v2, bs210, h, t, "dec-map-v210-p-nil")
		testDeepEqualErr(v210v1, v210v2, t, "equal-map-v210-p-nil")
		// ...
		if v == nil {
			v210v2 = nil
		} else {
			v210v2 = make(map[int8]int16, len(v))
		} // reset map
		var v210v3, v210v4 typMapMapInt8Int16
		v210v3 = typMapMapInt8Int16(v210v1)
		v210v4 = typMapMapInt8Int16(v210v2)
		bs210 = testMarshalErr(v210v3, h, t, "enc-map-v210-custom")
		testUnmarshalErr(v210v4, bs210, h, t, "dec-map-v210-p-len")
		testDeepEqualErr(v210v3, v210v4, t, "equal-map-v210-p-len")
	}

	for _, v := range []map[int8]int32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v211: %v\n", v)
		var v211v1, v211v2 map[int8]int32
		v211v1 = v
		bs211 := testMarshalErr(v211v1, h, t, "enc-map-v211")
		if v == nil {
			v211v2 = nil
		} else {
			v211v2 = make(map[int8]int32, len(v))
		} // reset map
		testUnmarshalErr(v211v2, bs211, h, t, "dec-map-v211")
		testDeepEqualErr(v211v1, v211v2, t, "equal-map-v211")
		if v == nil {
			v211v2 = nil
		} else {
			v211v2 = make(map[int8]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v211v2), bs211, h, t, "dec-map-v211-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v211v1, v211v2, t, "equal-map-v211-noaddr")
		if v == nil {
			v211v2 = nil
		} else {
			v211v2 = make(map[int8]int32, len(v))
		} // reset map
		testUnmarshalErr(&v211v2, bs211, h, t, "dec-map-v211-p-len")
		testDeepEqualErr(v211v1, v211v2, t, "equal-map-v211-p-len")
		bs211 = testMarshalErr(&v211v1, h, t, "enc-map-v211-p")
		v211v2 = nil
		testUnmarshalErr(&v211v2, bs211, h, t, "dec-map-v211-p-nil")
		testDeepEqualErr(v211v1, v211v2, t, "equal-map-v211-p-nil")
		// ...
		if v == nil {
			v211v2 = nil
		} else {
			v211v2 = make(map[int8]int32, len(v))
		} // reset map
		var v211v3, v211v4 typMapMapInt8Int32
		v211v3 = typMapMapInt8Int32(v211v1)
		v211v4 = typMapMapInt8Int32(v211v2)
		bs211 = testMarshalErr(v211v3, h, t, "enc-map-v211-custom")
		testUnmarshalErr(v211v4, bs211, h, t, "dec-map-v211-p-len")
		testDeepEqualErr(v211v3, v211v4, t, "equal-map-v211-p-len")
	}

	for _, v := range []map[int8]int64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v212: %v\n", v)
		var v212v1, v212v2 map[int8]int64
		v212v1 = v
		bs212 := testMarshalErr(v212v1, h, t, "enc-map-v212")
		if v == nil {
			v212v2 = nil
		} else {
			v212v2 = make(map[int8]int64, len(v))
		} // reset map
		testUnmarshalErr(v212v2, bs212, h, t, "dec-map-v212")
		testDeepEqualErr(v212v1, v212v2, t, "equal-map-v212")
		if v == nil {
			v212v2 = nil
		} else {
			v212v2 = make(map[int8]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v212v2), bs212, h, t, "dec-map-v212-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v212v1, v212v2, t, "equal-map-v212-noaddr")
		if v == nil {
			v212v2 = nil
		} else {
			v212v2 = make(map[int8]int64, len(v))
		} // reset map
		testUnmarshalErr(&v212v2, bs212, h, t, "dec-map-v212-p-len")
		testDeepEqualErr(v212v1, v212v2, t, "equal-map-v212-p-len")
		bs212 = testMarshalErr(&v212v1, h, t, "enc-map-v212-p")
		v212v2 = nil
		testUnmarshalErr(&v212v2, bs212, h, t, "dec-map-v212-p-nil")
		testDeepEqualErr(v212v1, v212v2, t, "equal-map-v212-p-nil")
		// ...
		if v == nil {
			v212v2 = nil
		} else {
			v212v2 = make(map[int8]int64, len(v))
		} // reset map
		var v212v3, v212v4 typMapMapInt8Int64
		v212v3 = typMapMapInt8Int64(v212v1)
		v212v4 = typMapMapInt8Int64(v212v2)
		bs212 = testMarshalErr(v212v3, h, t, "enc-map-v212-custom")
		testUnmarshalErr(v212v4, bs212, h, t, "dec-map-v212-p-len")
		testDeepEqualErr(v212v3, v212v4, t, "equal-map-v212-p-len")
	}

	for _, v := range []map[int8]float32{nil, {}, {44: 0, 33: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v213: %v\n", v)
		var v213v1, v213v2 map[int8]float32
		v213v1 = v
		bs213 := testMarshalErr(v213v1, h, t, "enc-map-v213")
		if v == nil {
			v213v2 = nil
		} else {
			v213v2 = make(map[int8]float32, len(v))
		} // reset map
		testUnmarshalErr(v213v2, bs213, h, t, "dec-map-v213")
		testDeepEqualErr(v213v1, v213v2, t, "equal-map-v213")
		if v == nil {
			v213v2 = nil
		} else {
			v213v2 = make(map[int8]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v213v2), bs213, h, t, "dec-map-v213-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v213v1, v213v2, t, "equal-map-v213-noaddr")
		if v == nil {
			v213v2 = nil
		} else {
			v213v2 = make(map[int8]float32, len(v))
		} // reset map
		testUnmarshalErr(&v213v2, bs213, h, t, "dec-map-v213-p-len")
		testDeepEqualErr(v213v1, v213v2, t, "equal-map-v213-p-len")
		bs213 = testMarshalErr(&v213v1, h, t, "enc-map-v213-p")
		v213v2 = nil
		testUnmarshalErr(&v213v2, bs213, h, t, "dec-map-v213-p-nil")
		testDeepEqualErr(v213v1, v213v2, t, "equal-map-v213-p-nil")
		// ...
		if v == nil {
			v213v2 = nil
		} else {
			v213v2 = make(map[int8]float32, len(v))
		} // reset map
		var v213v3, v213v4 typMapMapInt8Float32
		v213v3 = typMapMapInt8Float32(v213v1)
		v213v4 = typMapMapInt8Float32(v213v2)
		bs213 = testMarshalErr(v213v3, h, t, "enc-map-v213-custom")
		testUnmarshalErr(v213v4, bs213, h, t, "dec-map-v213-p-len")
		testDeepEqualErr(v213v3, v213v4, t, "equal-map-v213-p-len")
	}

	for _, v := range []map[int8]float64{nil, {}, {44: 0, 33: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v214: %v\n", v)
		var v214v1, v214v2 map[int8]float64
		v214v1 = v
		bs214 := testMarshalErr(v214v1, h, t, "enc-map-v214")
		if v == nil {
			v214v2 = nil
		} else {
			v214v2 = make(map[int8]float64, len(v))
		} // reset map
		testUnmarshalErr(v214v2, bs214, h, t, "dec-map-v214")
		testDeepEqualErr(v214v1, v214v2, t, "equal-map-v214")
		if v == nil {
			v214v2 = nil
		} else {
			v214v2 = make(map[int8]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v214v2), bs214, h, t, "dec-map-v214-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v214v1, v214v2, t, "equal-map-v214-noaddr")
		if v == nil {
			v214v2 = nil
		} else {
			v214v2 = make(map[int8]float64, len(v))
		} // reset map
		testUnmarshalErr(&v214v2, bs214, h, t, "dec-map-v214-p-len")
		testDeepEqualErr(v214v1, v214v2, t, "equal-map-v214-p-len")
		bs214 = testMarshalErr(&v214v1, h, t, "enc-map-v214-p")
		v214v2 = nil
		testUnmarshalErr(&v214v2, bs214, h, t, "dec-map-v214-p-nil")
		testDeepEqualErr(v214v1, v214v2, t, "equal-map-v214-p-nil")
		// ...
		if v == nil {
			v214v2 = nil
		} else {
			v214v2 = make(map[int8]float64, len(v))
		} // reset map
		var v214v3, v214v4 typMapMapInt8Float64
		v214v3 = typMapMapInt8Float64(v214v1)
		v214v4 = typMapMapInt8Float64(v214v2)
		bs214 = testMarshalErr(v214v3, h, t, "enc-map-v214-custom")
		testUnmarshalErr(v214v4, bs214, h, t, "dec-map-v214-p-len")
		testDeepEqualErr(v214v3, v214v4, t, "equal-map-v214-p-len")
	}

	for _, v := range []map[int8]bool{nil, {}, {44: false, 33: true}} {
		// fmt.Printf(">>>> running mammoth map v215: %v\n", v)
		var v215v1, v215v2 map[int8]bool
		v215v1 = v
		bs215 := testMarshalErr(v215v1, h, t, "enc-map-v215")
		if v == nil {
			v215v2 = nil
		} else {
			v215v2 = make(map[int8]bool, len(v))
		} // reset map
		testUnmarshalErr(v215v2, bs215, h, t, "dec-map-v215")
		testDeepEqualErr(v215v1, v215v2, t, "equal-map-v215")
		if v == nil {
			v215v2 = nil
		} else {
			v215v2 = make(map[int8]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v215v2), bs215, h, t, "dec-map-v215-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v215v1, v215v2, t, "equal-map-v215-noaddr")
		if v == nil {
			v215v2 = nil
		} else {
			v215v2 = make(map[int8]bool, len(v))
		} // reset map
		testUnmarshalErr(&v215v2, bs215, h, t, "dec-map-v215-p-len")
		testDeepEqualErr(v215v1, v215v2, t, "equal-map-v215-p-len")
		bs215 = testMarshalErr(&v215v1, h, t, "enc-map-v215-p")
		v215v2 = nil
		testUnmarshalErr(&v215v2, bs215, h, t, "dec-map-v215-p-nil")
		testDeepEqualErr(v215v1, v215v2, t, "equal-map-v215-p-nil")
		// ...
		if v == nil {
			v215v2 = nil
		} else {
			v215v2 = make(map[int8]bool, len(v))
		} // reset map
		var v215v3, v215v4 typMapMapInt8Bool
		v215v3 = typMapMapInt8Bool(v215v1)
		v215v4 = typMapMapInt8Bool(v215v2)
		bs215 = testMarshalErr(v215v3, h, t, "enc-map-v215-custom")
		testUnmarshalErr(v215v4, bs215, h, t, "dec-map-v215-p-len")
		testDeepEqualErr(v215v3, v215v4, t, "equal-map-v215-p-len")
	}

	for _, v := range []map[int16]interface{}{nil, {}, {44: nil, 33: "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v218: %v\n", v)
		var v218v1, v218v2 map[int16]interface{}
		v218v1 = v
		bs218 := testMarshalErr(v218v1, h, t, "enc-map-v218")
		if v == nil {
			v218v2 = nil
		} else {
			v218v2 = make(map[int16]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v218v2, bs218, h, t, "dec-map-v218")
		testDeepEqualErr(v218v1, v218v2, t, "equal-map-v218")
		if v == nil {
			v218v2 = nil
		} else {
			v218v2 = make(map[int16]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v218v2), bs218, h, t, "dec-map-v218-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v218v1, v218v2, t, "equal-map-v218-noaddr")
		if v == nil {
			v218v2 = nil
		} else {
			v218v2 = make(map[int16]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v218v2, bs218, h, t, "dec-map-v218-p-len")
		testDeepEqualErr(v218v1, v218v2, t, "equal-map-v218-p-len")
		bs218 = testMarshalErr(&v218v1, h, t, "enc-map-v218-p")
		v218v2 = nil
		testUnmarshalErr(&v218v2, bs218, h, t, "dec-map-v218-p-nil")
		testDeepEqualErr(v218v1, v218v2, t, "equal-map-v218-p-nil")
		// ...
		if v == nil {
			v218v2 = nil
		} else {
			v218v2 = make(map[int16]interface{}, len(v))
		} // reset map
		var v218v3, v218v4 typMapMapInt16Intf
		v218v3 = typMapMapInt16Intf(v218v1)
		v218v4 = typMapMapInt16Intf(v218v2)
		bs218 = testMarshalErr(v218v3, h, t, "enc-map-v218-custom")
		testUnmarshalErr(v218v4, bs218, h, t, "dec-map-v218-p-len")
		testDeepEqualErr(v218v3, v218v4, t, "equal-map-v218-p-len")
	}

	for _, v := range []map[int16]string{nil, {}, {44: "", 33: "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v219: %v\n", v)
		var v219v1, v219v2 map[int16]string
		v219v1 = v
		bs219 := testMarshalErr(v219v1, h, t, "enc-map-v219")
		if v == nil {
			v219v2 = nil
		} else {
			v219v2 = make(map[int16]string, len(v))
		} // reset map
		testUnmarshalErr(v219v2, bs219, h, t, "dec-map-v219")
		testDeepEqualErr(v219v1, v219v2, t, "equal-map-v219")
		if v == nil {
			v219v2 = nil
		} else {
			v219v2 = make(map[int16]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v219v2), bs219, h, t, "dec-map-v219-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v219v1, v219v2, t, "equal-map-v219-noaddr")
		if v == nil {
			v219v2 = nil
		} else {
			v219v2 = make(map[int16]string, len(v))
		} // reset map
		testUnmarshalErr(&v219v2, bs219, h, t, "dec-map-v219-p-len")
		testDeepEqualErr(v219v1, v219v2, t, "equal-map-v219-p-len")
		bs219 = testMarshalErr(&v219v1, h, t, "enc-map-v219-p")
		v219v2 = nil
		testUnmarshalErr(&v219v2, bs219, h, t, "dec-map-v219-p-nil")
		testDeepEqualErr(v219v1, v219v2, t, "equal-map-v219-p-nil")
		// ...
		if v == nil {
			v219v2 = nil
		} else {
			v219v2 = make(map[int16]string, len(v))
		} // reset map
		var v219v3, v219v4 typMapMapInt16String
		v219v3 = typMapMapInt16String(v219v1)
		v219v4 = typMapMapInt16String(v219v2)
		bs219 = testMarshalErr(v219v3, h, t, "enc-map-v219-custom")
		testUnmarshalErr(v219v4, bs219, h, t, "dec-map-v219-p-len")
		testDeepEqualErr(v219v3, v219v4, t, "equal-map-v219-p-len")
	}

	for _, v := range []map[int16]uint{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v220: %v\n", v)
		var v220v1, v220v2 map[int16]uint
		v220v1 = v
		bs220 := testMarshalErr(v220v1, h, t, "enc-map-v220")
		if v == nil {
			v220v2 = nil
		} else {
			v220v2 = make(map[int16]uint, len(v))
		} // reset map
		testUnmarshalErr(v220v2, bs220, h, t, "dec-map-v220")
		testDeepEqualErr(v220v1, v220v2, t, "equal-map-v220")
		if v == nil {
			v220v2 = nil
		} else {
			v220v2 = make(map[int16]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v220v2), bs220, h, t, "dec-map-v220-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v220v1, v220v2, t, "equal-map-v220-noaddr")
		if v == nil {
			v220v2 = nil
		} else {
			v220v2 = make(map[int16]uint, len(v))
		} // reset map
		testUnmarshalErr(&v220v2, bs220, h, t, "dec-map-v220-p-len")
		testDeepEqualErr(v220v1, v220v2, t, "equal-map-v220-p-len")
		bs220 = testMarshalErr(&v220v1, h, t, "enc-map-v220-p")
		v220v2 = nil
		testUnmarshalErr(&v220v2, bs220, h, t, "dec-map-v220-p-nil")
		testDeepEqualErr(v220v1, v220v2, t, "equal-map-v220-p-nil")
		// ...
		if v == nil {
			v220v2 = nil
		} else {
			v220v2 = make(map[int16]uint, len(v))
		} // reset map
		var v220v3, v220v4 typMapMapInt16Uint
		v220v3 = typMapMapInt16Uint(v220v1)
		v220v4 = typMapMapInt16Uint(v220v2)
		bs220 = testMarshalErr(v220v3, h, t, "enc-map-v220-custom")
		testUnmarshalErr(v220v4, bs220, h, t, "dec-map-v220-p-len")
		testDeepEqualErr(v220v3, v220v4, t, "equal-map-v220-p-len")
	}

	for _, v := range []map[int16]uint8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v221: %v\n", v)
		var v221v1, v221v2 map[int16]uint8
		v221v1 = v
		bs221 := testMarshalErr(v221v1, h, t, "enc-map-v221")
		if v == nil {
			v221v2 = nil
		} else {
			v221v2 = make(map[int16]uint8, len(v))
		} // reset map
		testUnmarshalErr(v221v2, bs221, h, t, "dec-map-v221")
		testDeepEqualErr(v221v1, v221v2, t, "equal-map-v221")
		if v == nil {
			v221v2 = nil
		} else {
			v221v2 = make(map[int16]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v221v2), bs221, h, t, "dec-map-v221-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v221v1, v221v2, t, "equal-map-v221-noaddr")
		if v == nil {
			v221v2 = nil
		} else {
			v221v2 = make(map[int16]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v221v2, bs221, h, t, "dec-map-v221-p-len")
		testDeepEqualErr(v221v1, v221v2, t, "equal-map-v221-p-len")
		bs221 = testMarshalErr(&v221v1, h, t, "enc-map-v221-p")
		v221v2 = nil
		testUnmarshalErr(&v221v2, bs221, h, t, "dec-map-v221-p-nil")
		testDeepEqualErr(v221v1, v221v2, t, "equal-map-v221-p-nil")
		// ...
		if v == nil {
			v221v2 = nil
		} else {
			v221v2 = make(map[int16]uint8, len(v))
		} // reset map
		var v221v3, v221v4 typMapMapInt16Uint8
		v221v3 = typMapMapInt16Uint8(v221v1)
		v221v4 = typMapMapInt16Uint8(v221v2)
		bs221 = testMarshalErr(v221v3, h, t, "enc-map-v221-custom")
		testUnmarshalErr(v221v4, bs221, h, t, "dec-map-v221-p-len")
		testDeepEqualErr(v221v3, v221v4, t, "equal-map-v221-p-len")
	}

	for _, v := range []map[int16]uint16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v222: %v\n", v)
		var v222v1, v222v2 map[int16]uint16
		v222v1 = v
		bs222 := testMarshalErr(v222v1, h, t, "enc-map-v222")
		if v == nil {
			v222v2 = nil
		} else {
			v222v2 = make(map[int16]uint16, len(v))
		} // reset map
		testUnmarshalErr(v222v2, bs222, h, t, "dec-map-v222")
		testDeepEqualErr(v222v1, v222v2, t, "equal-map-v222")
		if v == nil {
			v222v2 = nil
		} else {
			v222v2 = make(map[int16]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v222v2), bs222, h, t, "dec-map-v222-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v222v1, v222v2, t, "equal-map-v222-noaddr")
		if v == nil {
			v222v2 = nil
		} else {
			v222v2 = make(map[int16]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v222v2, bs222, h, t, "dec-map-v222-p-len")
		testDeepEqualErr(v222v1, v222v2, t, "equal-map-v222-p-len")
		bs222 = testMarshalErr(&v222v1, h, t, "enc-map-v222-p")
		v222v2 = nil
		testUnmarshalErr(&v222v2, bs222, h, t, "dec-map-v222-p-nil")
		testDeepEqualErr(v222v1, v222v2, t, "equal-map-v222-p-nil")
		// ...
		if v == nil {
			v222v2 = nil
		} else {
			v222v2 = make(map[int16]uint16, len(v))
		} // reset map
		var v222v3, v222v4 typMapMapInt16Uint16
		v222v3 = typMapMapInt16Uint16(v222v1)
		v222v4 = typMapMapInt16Uint16(v222v2)
		bs222 = testMarshalErr(v222v3, h, t, "enc-map-v222-custom")
		testUnmarshalErr(v222v4, bs222, h, t, "dec-map-v222-p-len")
		testDeepEqualErr(v222v3, v222v4, t, "equal-map-v222-p-len")
	}

	for _, v := range []map[int16]uint32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v223: %v\n", v)
		var v223v1, v223v2 map[int16]uint32
		v223v1 = v
		bs223 := testMarshalErr(v223v1, h, t, "enc-map-v223")
		if v == nil {
			v223v2 = nil
		} else {
			v223v2 = make(map[int16]uint32, len(v))
		} // reset map
		testUnmarshalErr(v223v2, bs223, h, t, "dec-map-v223")
		testDeepEqualErr(v223v1, v223v2, t, "equal-map-v223")
		if v == nil {
			v223v2 = nil
		} else {
			v223v2 = make(map[int16]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v223v2), bs223, h, t, "dec-map-v223-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v223v1, v223v2, t, "equal-map-v223-noaddr")
		if v == nil {
			v223v2 = nil
		} else {
			v223v2 = make(map[int16]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v223v2, bs223, h, t, "dec-map-v223-p-len")
		testDeepEqualErr(v223v1, v223v2, t, "equal-map-v223-p-len")
		bs223 = testMarshalErr(&v223v1, h, t, "enc-map-v223-p")
		v223v2 = nil
		testUnmarshalErr(&v223v2, bs223, h, t, "dec-map-v223-p-nil")
		testDeepEqualErr(v223v1, v223v2, t, "equal-map-v223-p-nil")
		// ...
		if v == nil {
			v223v2 = nil
		} else {
			v223v2 = make(map[int16]uint32, len(v))
		} // reset map
		var v223v3, v223v4 typMapMapInt16Uint32
		v223v3 = typMapMapInt16Uint32(v223v1)
		v223v4 = typMapMapInt16Uint32(v223v2)
		bs223 = testMarshalErr(v223v3, h, t, "enc-map-v223-custom")
		testUnmarshalErr(v223v4, bs223, h, t, "dec-map-v223-p-len")
		testDeepEqualErr(v223v3, v223v4, t, "equal-map-v223-p-len")
	}

	for _, v := range []map[int16]uint64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v224: %v\n", v)
		var v224v1, v224v2 map[int16]uint64
		v224v1 = v
		bs224 := testMarshalErr(v224v1, h, t, "enc-map-v224")
		if v == nil {
			v224v2 = nil
		} else {
			v224v2 = make(map[int16]uint64, len(v))
		} // reset map
		testUnmarshalErr(v224v2, bs224, h, t, "dec-map-v224")
		testDeepEqualErr(v224v1, v224v2, t, "equal-map-v224")
		if v == nil {
			v224v2 = nil
		} else {
			v224v2 = make(map[int16]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v224v2), bs224, h, t, "dec-map-v224-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v224v1, v224v2, t, "equal-map-v224-noaddr")
		if v == nil {
			v224v2 = nil
		} else {
			v224v2 = make(map[int16]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v224v2, bs224, h, t, "dec-map-v224-p-len")
		testDeepEqualErr(v224v1, v224v2, t, "equal-map-v224-p-len")
		bs224 = testMarshalErr(&v224v1, h, t, "enc-map-v224-p")
		v224v2 = nil
		testUnmarshalErr(&v224v2, bs224, h, t, "dec-map-v224-p-nil")
		testDeepEqualErr(v224v1, v224v2, t, "equal-map-v224-p-nil")
		// ...
		if v == nil {
			v224v2 = nil
		} else {
			v224v2 = make(map[int16]uint64, len(v))
		} // reset map
		var v224v3, v224v4 typMapMapInt16Uint64
		v224v3 = typMapMapInt16Uint64(v224v1)
		v224v4 = typMapMapInt16Uint64(v224v2)
		bs224 = testMarshalErr(v224v3, h, t, "enc-map-v224-custom")
		testUnmarshalErr(v224v4, bs224, h, t, "dec-map-v224-p-len")
		testDeepEqualErr(v224v3, v224v4, t, "equal-map-v224-p-len")
	}

	for _, v := range []map[int16]uintptr{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v225: %v\n", v)
		var v225v1, v225v2 map[int16]uintptr
		v225v1 = v
		bs225 := testMarshalErr(v225v1, h, t, "enc-map-v225")
		if v == nil {
			v225v2 = nil
		} else {
			v225v2 = make(map[int16]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v225v2, bs225, h, t, "dec-map-v225")
		testDeepEqualErr(v225v1, v225v2, t, "equal-map-v225")
		if v == nil {
			v225v2 = nil
		} else {
			v225v2 = make(map[int16]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v225v2), bs225, h, t, "dec-map-v225-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v225v1, v225v2, t, "equal-map-v225-noaddr")
		if v == nil {
			v225v2 = nil
		} else {
			v225v2 = make(map[int16]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v225v2, bs225, h, t, "dec-map-v225-p-len")
		testDeepEqualErr(v225v1, v225v2, t, "equal-map-v225-p-len")
		bs225 = testMarshalErr(&v225v1, h, t, "enc-map-v225-p")
		v225v2 = nil
		testUnmarshalErr(&v225v2, bs225, h, t, "dec-map-v225-p-nil")
		testDeepEqualErr(v225v1, v225v2, t, "equal-map-v225-p-nil")
		// ...
		if v == nil {
			v225v2 = nil
		} else {
			v225v2 = make(map[int16]uintptr, len(v))
		} // reset map
		var v225v3, v225v4 typMapMapInt16Uintptr
		v225v3 = typMapMapInt16Uintptr(v225v1)
		v225v4 = typMapMapInt16Uintptr(v225v2)
		bs225 = testMarshalErr(v225v3, h, t, "enc-map-v225-custom")
		testUnmarshalErr(v225v4, bs225, h, t, "dec-map-v225-p-len")
		testDeepEqualErr(v225v3, v225v4, t, "equal-map-v225-p-len")
	}

	for _, v := range []map[int16]int{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v226: %v\n", v)
		var v226v1, v226v2 map[int16]int
		v226v1 = v
		bs226 := testMarshalErr(v226v1, h, t, "enc-map-v226")
		if v == nil {
			v226v2 = nil
		} else {
			v226v2 = make(map[int16]int, len(v))
		} // reset map
		testUnmarshalErr(v226v2, bs226, h, t, "dec-map-v226")
		testDeepEqualErr(v226v1, v226v2, t, "equal-map-v226")
		if v == nil {
			v226v2 = nil
		} else {
			v226v2 = make(map[int16]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v226v2), bs226, h, t, "dec-map-v226-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v226v1, v226v2, t, "equal-map-v226-noaddr")
		if v == nil {
			v226v2 = nil
		} else {
			v226v2 = make(map[int16]int, len(v))
		} // reset map
		testUnmarshalErr(&v226v2, bs226, h, t, "dec-map-v226-p-len")
		testDeepEqualErr(v226v1, v226v2, t, "equal-map-v226-p-len")
		bs226 = testMarshalErr(&v226v1, h, t, "enc-map-v226-p")
		v226v2 = nil
		testUnmarshalErr(&v226v2, bs226, h, t, "dec-map-v226-p-nil")
		testDeepEqualErr(v226v1, v226v2, t, "equal-map-v226-p-nil")
		// ...
		if v == nil {
			v226v2 = nil
		} else {
			v226v2 = make(map[int16]int, len(v))
		} // reset map
		var v226v3, v226v4 typMapMapInt16Int
		v226v3 = typMapMapInt16Int(v226v1)
		v226v4 = typMapMapInt16Int(v226v2)
		bs226 = testMarshalErr(v226v3, h, t, "enc-map-v226-custom")
		testUnmarshalErr(v226v4, bs226, h, t, "dec-map-v226-p-len")
		testDeepEqualErr(v226v3, v226v4, t, "equal-map-v226-p-len")
	}

	for _, v := range []map[int16]int8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v227: %v\n", v)
		var v227v1, v227v2 map[int16]int8
		v227v1 = v
		bs227 := testMarshalErr(v227v1, h, t, "enc-map-v227")
		if v == nil {
			v227v2 = nil
		} else {
			v227v2 = make(map[int16]int8, len(v))
		} // reset map
		testUnmarshalErr(v227v2, bs227, h, t, "dec-map-v227")
		testDeepEqualErr(v227v1, v227v2, t, "equal-map-v227")
		if v == nil {
			v227v2 = nil
		} else {
			v227v2 = make(map[int16]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v227v2), bs227, h, t, "dec-map-v227-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v227v1, v227v2, t, "equal-map-v227-noaddr")
		if v == nil {
			v227v2 = nil
		} else {
			v227v2 = make(map[int16]int8, len(v))
		} // reset map
		testUnmarshalErr(&v227v2, bs227, h, t, "dec-map-v227-p-len")
		testDeepEqualErr(v227v1, v227v2, t, "equal-map-v227-p-len")
		bs227 = testMarshalErr(&v227v1, h, t, "enc-map-v227-p")
		v227v2 = nil
		testUnmarshalErr(&v227v2, bs227, h, t, "dec-map-v227-p-nil")
		testDeepEqualErr(v227v1, v227v2, t, "equal-map-v227-p-nil")
		// ...
		if v == nil {
			v227v2 = nil
		} else {
			v227v2 = make(map[int16]int8, len(v))
		} // reset map
		var v227v3, v227v4 typMapMapInt16Int8
		v227v3 = typMapMapInt16Int8(v227v1)
		v227v4 = typMapMapInt16Int8(v227v2)
		bs227 = testMarshalErr(v227v3, h, t, "enc-map-v227-custom")
		testUnmarshalErr(v227v4, bs227, h, t, "dec-map-v227-p-len")
		testDeepEqualErr(v227v3, v227v4, t, "equal-map-v227-p-len")
	}

	for _, v := range []map[int16]int16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v228: %v\n", v)
		var v228v1, v228v2 map[int16]int16
		v228v1 = v
		bs228 := testMarshalErr(v228v1, h, t, "enc-map-v228")
		if v == nil {
			v228v2 = nil
		} else {
			v228v2 = make(map[int16]int16, len(v))
		} // reset map
		testUnmarshalErr(v228v2, bs228, h, t, "dec-map-v228")
		testDeepEqualErr(v228v1, v228v2, t, "equal-map-v228")
		if v == nil {
			v228v2 = nil
		} else {
			v228v2 = make(map[int16]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v228v2), bs228, h, t, "dec-map-v228-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v228v1, v228v2, t, "equal-map-v228-noaddr")
		if v == nil {
			v228v2 = nil
		} else {
			v228v2 = make(map[int16]int16, len(v))
		} // reset map
		testUnmarshalErr(&v228v2, bs228, h, t, "dec-map-v228-p-len")
		testDeepEqualErr(v228v1, v228v2, t, "equal-map-v228-p-len")
		bs228 = testMarshalErr(&v228v1, h, t, "enc-map-v228-p")
		v228v2 = nil
		testUnmarshalErr(&v228v2, bs228, h, t, "dec-map-v228-p-nil")
		testDeepEqualErr(v228v1, v228v2, t, "equal-map-v228-p-nil")
		// ...
		if v == nil {
			v228v2 = nil
		} else {
			v228v2 = make(map[int16]int16, len(v))
		} // reset map
		var v228v3, v228v4 typMapMapInt16Int16
		v228v3 = typMapMapInt16Int16(v228v1)
		v228v4 = typMapMapInt16Int16(v228v2)
		bs228 = testMarshalErr(v228v3, h, t, "enc-map-v228-custom")
		testUnmarshalErr(v228v4, bs228, h, t, "dec-map-v228-p-len")
		testDeepEqualErr(v228v3, v228v4, t, "equal-map-v228-p-len")
	}

	for _, v := range []map[int16]int32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v229: %v\n", v)
		var v229v1, v229v2 map[int16]int32
		v229v1 = v
		bs229 := testMarshalErr(v229v1, h, t, "enc-map-v229")
		if v == nil {
			v229v2 = nil
		} else {
			v229v2 = make(map[int16]int32, len(v))
		} // reset map
		testUnmarshalErr(v229v2, bs229, h, t, "dec-map-v229")
		testDeepEqualErr(v229v1, v229v2, t, "equal-map-v229")
		if v == nil {
			v229v2 = nil
		} else {
			v229v2 = make(map[int16]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v229v2), bs229, h, t, "dec-map-v229-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v229v1, v229v2, t, "equal-map-v229-noaddr")
		if v == nil {
			v229v2 = nil
		} else {
			v229v2 = make(map[int16]int32, len(v))
		} // reset map
		testUnmarshalErr(&v229v2, bs229, h, t, "dec-map-v229-p-len")
		testDeepEqualErr(v229v1, v229v2, t, "equal-map-v229-p-len")
		bs229 = testMarshalErr(&v229v1, h, t, "enc-map-v229-p")
		v229v2 = nil
		testUnmarshalErr(&v229v2, bs229, h, t, "dec-map-v229-p-nil")
		testDeepEqualErr(v229v1, v229v2, t, "equal-map-v229-p-nil")
		// ...
		if v == nil {
			v229v2 = nil
		} else {
			v229v2 = make(map[int16]int32, len(v))
		} // reset map
		var v229v3, v229v4 typMapMapInt16Int32
		v229v3 = typMapMapInt16Int32(v229v1)
		v229v4 = typMapMapInt16Int32(v229v2)
		bs229 = testMarshalErr(v229v3, h, t, "enc-map-v229-custom")
		testUnmarshalErr(v229v4, bs229, h, t, "dec-map-v229-p-len")
		testDeepEqualErr(v229v3, v229v4, t, "equal-map-v229-p-len")
	}

	for _, v := range []map[int16]int64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v230: %v\n", v)
		var v230v1, v230v2 map[int16]int64
		v230v1 = v
		bs230 := testMarshalErr(v230v1, h, t, "enc-map-v230")
		if v == nil {
			v230v2 = nil
		} else {
			v230v2 = make(map[int16]int64, len(v))
		} // reset map
		testUnmarshalErr(v230v2, bs230, h, t, "dec-map-v230")
		testDeepEqualErr(v230v1, v230v2, t, "equal-map-v230")
		if v == nil {
			v230v2 = nil
		} else {
			v230v2 = make(map[int16]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v230v2), bs230, h, t, "dec-map-v230-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v230v1, v230v2, t, "equal-map-v230-noaddr")
		if v == nil {
			v230v2 = nil
		} else {
			v230v2 = make(map[int16]int64, len(v))
		} // reset map
		testUnmarshalErr(&v230v2, bs230, h, t, "dec-map-v230-p-len")
		testDeepEqualErr(v230v1, v230v2, t, "equal-map-v230-p-len")
		bs230 = testMarshalErr(&v230v1, h, t, "enc-map-v230-p")
		v230v2 = nil
		testUnmarshalErr(&v230v2, bs230, h, t, "dec-map-v230-p-nil")
		testDeepEqualErr(v230v1, v230v2, t, "equal-map-v230-p-nil")
		// ...
		if v == nil {
			v230v2 = nil
		} else {
			v230v2 = make(map[int16]int64, len(v))
		} // reset map
		var v230v3, v230v4 typMapMapInt16Int64
		v230v3 = typMapMapInt16Int64(v230v1)
		v230v4 = typMapMapInt16Int64(v230v2)
		bs230 = testMarshalErr(v230v3, h, t, "enc-map-v230-custom")
		testUnmarshalErr(v230v4, bs230, h, t, "dec-map-v230-p-len")
		testDeepEqualErr(v230v3, v230v4, t, "equal-map-v230-p-len")
	}

	for _, v := range []map[int16]float32{nil, {}, {33: 0, 44: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v231: %v\n", v)
		var v231v1, v231v2 map[int16]float32
		v231v1 = v
		bs231 := testMarshalErr(v231v1, h, t, "enc-map-v231")
		if v == nil {
			v231v2 = nil
		} else {
			v231v2 = make(map[int16]float32, len(v))
		} // reset map
		testUnmarshalErr(v231v2, bs231, h, t, "dec-map-v231")
		testDeepEqualErr(v231v1, v231v2, t, "equal-map-v231")
		if v == nil {
			v231v2 = nil
		} else {
			v231v2 = make(map[int16]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v231v2), bs231, h, t, "dec-map-v231-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v231v1, v231v2, t, "equal-map-v231-noaddr")
		if v == nil {
			v231v2 = nil
		} else {
			v231v2 = make(map[int16]float32, len(v))
		} // reset map
		testUnmarshalErr(&v231v2, bs231, h, t, "dec-map-v231-p-len")
		testDeepEqualErr(v231v1, v231v2, t, "equal-map-v231-p-len")
		bs231 = testMarshalErr(&v231v1, h, t, "enc-map-v231-p")
		v231v2 = nil
		testUnmarshalErr(&v231v2, bs231, h, t, "dec-map-v231-p-nil")
		testDeepEqualErr(v231v1, v231v2, t, "equal-map-v231-p-nil")
		// ...
		if v == nil {
			v231v2 = nil
		} else {
			v231v2 = make(map[int16]float32, len(v))
		} // reset map
		var v231v3, v231v4 typMapMapInt16Float32
		v231v3 = typMapMapInt16Float32(v231v1)
		v231v4 = typMapMapInt16Float32(v231v2)
		bs231 = testMarshalErr(v231v3, h, t, "enc-map-v231-custom")
		testUnmarshalErr(v231v4, bs231, h, t, "dec-map-v231-p-len")
		testDeepEqualErr(v231v3, v231v4, t, "equal-map-v231-p-len")
	}

	for _, v := range []map[int16]float64{nil, {}, {33: 0, 44: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v232: %v\n", v)
		var v232v1, v232v2 map[int16]float64
		v232v1 = v
		bs232 := testMarshalErr(v232v1, h, t, "enc-map-v232")
		if v == nil {
			v232v2 = nil
		} else {
			v232v2 = make(map[int16]float64, len(v))
		} // reset map
		testUnmarshalErr(v232v2, bs232, h, t, "dec-map-v232")
		testDeepEqualErr(v232v1, v232v2, t, "equal-map-v232")
		if v == nil {
			v232v2 = nil
		} else {
			v232v2 = make(map[int16]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v232v2), bs232, h, t, "dec-map-v232-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v232v1, v232v2, t, "equal-map-v232-noaddr")
		if v == nil {
			v232v2 = nil
		} else {
			v232v2 = make(map[int16]float64, len(v))
		} // reset map
		testUnmarshalErr(&v232v2, bs232, h, t, "dec-map-v232-p-len")
		testDeepEqualErr(v232v1, v232v2, t, "equal-map-v232-p-len")
		bs232 = testMarshalErr(&v232v1, h, t, "enc-map-v232-p")
		v232v2 = nil
		testUnmarshalErr(&v232v2, bs232, h, t, "dec-map-v232-p-nil")
		testDeepEqualErr(v232v1, v232v2, t, "equal-map-v232-p-nil")
		// ...
		if v == nil {
			v232v2 = nil
		} else {
			v232v2 = make(map[int16]float64, len(v))
		} // reset map
		var v232v3, v232v4 typMapMapInt16Float64
		v232v3 = typMapMapInt16Float64(v232v1)
		v232v4 = typMapMapInt16Float64(v232v2)
		bs232 = testMarshalErr(v232v3, h, t, "enc-map-v232-custom")
		testUnmarshalErr(v232v4, bs232, h, t, "dec-map-v232-p-len")
		testDeepEqualErr(v232v3, v232v4, t, "equal-map-v232-p-len")
	}

	for _, v := range []map[int16]bool{nil, {}, {33: false, 44: true}} {
		// fmt.Printf(">>>> running mammoth map v233: %v\n", v)
		var v233v1, v233v2 map[int16]bool
		v233v1 = v
		bs233 := testMarshalErr(v233v1, h, t, "enc-map-v233")
		if v == nil {
			v233v2 = nil
		} else {
			v233v2 = make(map[int16]bool, len(v))
		} // reset map
		testUnmarshalErr(v233v2, bs233, h, t, "dec-map-v233")
		testDeepEqualErr(v233v1, v233v2, t, "equal-map-v233")
		if v == nil {
			v233v2 = nil
		} else {
			v233v2 = make(map[int16]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v233v2), bs233, h, t, "dec-map-v233-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v233v1, v233v2, t, "equal-map-v233-noaddr")
		if v == nil {
			v233v2 = nil
		} else {
			v233v2 = make(map[int16]bool, len(v))
		} // reset map
		testUnmarshalErr(&v233v2, bs233, h, t, "dec-map-v233-p-len")
		testDeepEqualErr(v233v1, v233v2, t, "equal-map-v233-p-len")
		bs233 = testMarshalErr(&v233v1, h, t, "enc-map-v233-p")
		v233v2 = nil
		testUnmarshalErr(&v233v2, bs233, h, t, "dec-map-v233-p-nil")
		testDeepEqualErr(v233v1, v233v2, t, "equal-map-v233-p-nil")
		// ...
		if v == nil {
			v233v2 = nil
		} else {
			v233v2 = make(map[int16]bool, len(v))
		} // reset map
		var v233v3, v233v4 typMapMapInt16Bool
		v233v3 = typMapMapInt16Bool(v233v1)
		v233v4 = typMapMapInt16Bool(v233v2)
		bs233 = testMarshalErr(v233v3, h, t, "enc-map-v233-custom")
		testUnmarshalErr(v233v4, bs233, h, t, "dec-map-v233-p-len")
		testDeepEqualErr(v233v3, v233v4, t, "equal-map-v233-p-len")
	}

	for _, v := range []map[int32]interface{}{nil, {}, {33: nil, 44: "string-is-an-interface"}} {
		// fmt.Printf(">>>> running mammoth map v236: %v\n", v)
		var v236v1, v236v2 map[int32]interface{}
		v236v1 = v
		bs236 := testMarshalErr(v236v1, h, t, "enc-map-v236")
		if v == nil {
			v236v2 = nil
		} else {
			v236v2 = make(map[int32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v236v2, bs236, h, t, "dec-map-v236")
		testDeepEqualErr(v236v1, v236v2, t, "equal-map-v236")
		if v == nil {
			v236v2 = nil
		} else {
			v236v2 = make(map[int32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v236v2), bs236, h, t, "dec-map-v236-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v236v1, v236v2, t, "equal-map-v236-noaddr")
		if v == nil {
			v236v2 = nil
		} else {
			v236v2 = make(map[int32]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v236v2, bs236, h, t, "dec-map-v236-p-len")
		testDeepEqualErr(v236v1, v236v2, t, "equal-map-v236-p-len")
		bs236 = testMarshalErr(&v236v1, h, t, "enc-map-v236-p")
		v236v2 = nil
		testUnmarshalErr(&v236v2, bs236, h, t, "dec-map-v236-p-nil")
		testDeepEqualErr(v236v1, v236v2, t, "equal-map-v236-p-nil")
		// ...
		if v == nil {
			v236v2 = nil
		} else {
			v236v2 = make(map[int32]interface{}, len(v))
		} // reset map
		var v236v3, v236v4 typMapMapInt32Intf
		v236v3 = typMapMapInt32Intf(v236v1)
		v236v4 = typMapMapInt32Intf(v236v2)
		bs236 = testMarshalErr(v236v3, h, t, "enc-map-v236-custom")
		testUnmarshalErr(v236v4, bs236, h, t, "dec-map-v236-p-len")
		testDeepEqualErr(v236v3, v236v4, t, "equal-map-v236-p-len")
	}

	for _, v := range []map[int32]string{nil, {}, {33: "", 44: "some-string"}} {
		// fmt.Printf(">>>> running mammoth map v237: %v\n", v)
		var v237v1, v237v2 map[int32]string
		v237v1 = v
		bs237 := testMarshalErr(v237v1, h, t, "enc-map-v237")
		if v == nil {
			v237v2 = nil
		} else {
			v237v2 = make(map[int32]string, len(v))
		} // reset map
		testUnmarshalErr(v237v2, bs237, h, t, "dec-map-v237")
		testDeepEqualErr(v237v1, v237v2, t, "equal-map-v237")
		if v == nil {
			v237v2 = nil
		} else {
			v237v2 = make(map[int32]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v237v2), bs237, h, t, "dec-map-v237-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v237v1, v237v2, t, "equal-map-v237-noaddr")
		if v == nil {
			v237v2 = nil
		} else {
			v237v2 = make(map[int32]string, len(v))
		} // reset map
		testUnmarshalErr(&v237v2, bs237, h, t, "dec-map-v237-p-len")
		testDeepEqualErr(v237v1, v237v2, t, "equal-map-v237-p-len")
		bs237 = testMarshalErr(&v237v1, h, t, "enc-map-v237-p")
		v237v2 = nil
		testUnmarshalErr(&v237v2, bs237, h, t, "dec-map-v237-p-nil")
		testDeepEqualErr(v237v1, v237v2, t, "equal-map-v237-p-nil")
		// ...
		if v == nil {
			v237v2 = nil
		} else {
			v237v2 = make(map[int32]string, len(v))
		} // reset map
		var v237v3, v237v4 typMapMapInt32String
		v237v3 = typMapMapInt32String(v237v1)
		v237v4 = typMapMapInt32String(v237v2)
		bs237 = testMarshalErr(v237v3, h, t, "enc-map-v237-custom")
		testUnmarshalErr(v237v4, bs237, h, t, "dec-map-v237-p-len")
		testDeepEqualErr(v237v3, v237v4, t, "equal-map-v237-p-len")
	}

	for _, v := range []map[int32]uint{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v238: %v\n", v)
		var v238v1, v238v2 map[int32]uint
		v238v1 = v
		bs238 := testMarshalErr(v238v1, h, t, "enc-map-v238")
		if v == nil {
			v238v2 = nil
		} else {
			v238v2 = make(map[int32]uint, len(v))
		} // reset map
		testUnmarshalErr(v238v2, bs238, h, t, "dec-map-v238")
		testDeepEqualErr(v238v1, v238v2, t, "equal-map-v238")
		if v == nil {
			v238v2 = nil
		} else {
			v238v2 = make(map[int32]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v238v2), bs238, h, t, "dec-map-v238-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v238v1, v238v2, t, "equal-map-v238-noaddr")
		if v == nil {
			v238v2 = nil
		} else {
			v238v2 = make(map[int32]uint, len(v))
		} // reset map
		testUnmarshalErr(&v238v2, bs238, h, t, "dec-map-v238-p-len")
		testDeepEqualErr(v238v1, v238v2, t, "equal-map-v238-p-len")
		bs238 = testMarshalErr(&v238v1, h, t, "enc-map-v238-p")
		v238v2 = nil
		testUnmarshalErr(&v238v2, bs238, h, t, "dec-map-v238-p-nil")
		testDeepEqualErr(v238v1, v238v2, t, "equal-map-v238-p-nil")
		// ...
		if v == nil {
			v238v2 = nil
		} else {
			v238v2 = make(map[int32]uint, len(v))
		} // reset map
		var v238v3, v238v4 typMapMapInt32Uint
		v238v3 = typMapMapInt32Uint(v238v1)
		v238v4 = typMapMapInt32Uint(v238v2)
		bs238 = testMarshalErr(v238v3, h, t, "enc-map-v238-custom")
		testUnmarshalErr(v238v4, bs238, h, t, "dec-map-v238-p-len")
		testDeepEqualErr(v238v3, v238v4, t, "equal-map-v238-p-len")
	}

	for _, v := range []map[int32]uint8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v239: %v\n", v)
		var v239v1, v239v2 map[int32]uint8
		v239v1 = v
		bs239 := testMarshalErr(v239v1, h, t, "enc-map-v239")
		if v == nil {
			v239v2 = nil
		} else {
			v239v2 = make(map[int32]uint8, len(v))
		} // reset map
		testUnmarshalErr(v239v2, bs239, h, t, "dec-map-v239")
		testDeepEqualErr(v239v1, v239v2, t, "equal-map-v239")
		if v == nil {
			v239v2 = nil
		} else {
			v239v2 = make(map[int32]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v239v2), bs239, h, t, "dec-map-v239-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v239v1, v239v2, t, "equal-map-v239-noaddr")
		if v == nil {
			v239v2 = nil
		} else {
			v239v2 = make(map[int32]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v239v2, bs239, h, t, "dec-map-v239-p-len")
		testDeepEqualErr(v239v1, v239v2, t, "equal-map-v239-p-len")
		bs239 = testMarshalErr(&v239v1, h, t, "enc-map-v239-p")
		v239v2 = nil
		testUnmarshalErr(&v239v2, bs239, h, t, "dec-map-v239-p-nil")
		testDeepEqualErr(v239v1, v239v2, t, "equal-map-v239-p-nil")
		// ...
		if v == nil {
			v239v2 = nil
		} else {
			v239v2 = make(map[int32]uint8, len(v))
		} // reset map
		var v239v3, v239v4 typMapMapInt32Uint8
		v239v3 = typMapMapInt32Uint8(v239v1)
		v239v4 = typMapMapInt32Uint8(v239v2)
		bs239 = testMarshalErr(v239v3, h, t, "enc-map-v239-custom")
		testUnmarshalErr(v239v4, bs239, h, t, "dec-map-v239-p-len")
		testDeepEqualErr(v239v3, v239v4, t, "equal-map-v239-p-len")
	}

	for _, v := range []map[int32]uint16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v240: %v\n", v)
		var v240v1, v240v2 map[int32]uint16
		v240v1 = v
		bs240 := testMarshalErr(v240v1, h, t, "enc-map-v240")
		if v == nil {
			v240v2 = nil
		} else {
			v240v2 = make(map[int32]uint16, len(v))
		} // reset map
		testUnmarshalErr(v240v2, bs240, h, t, "dec-map-v240")
		testDeepEqualErr(v240v1, v240v2, t, "equal-map-v240")
		if v == nil {
			v240v2 = nil
		} else {
			v240v2 = make(map[int32]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v240v2), bs240, h, t, "dec-map-v240-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v240v1, v240v2, t, "equal-map-v240-noaddr")
		if v == nil {
			v240v2 = nil
		} else {
			v240v2 = make(map[int32]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v240v2, bs240, h, t, "dec-map-v240-p-len")
		testDeepEqualErr(v240v1, v240v2, t, "equal-map-v240-p-len")
		bs240 = testMarshalErr(&v240v1, h, t, "enc-map-v240-p")
		v240v2 = nil
		testUnmarshalErr(&v240v2, bs240, h, t, "dec-map-v240-p-nil")
		testDeepEqualErr(v240v1, v240v2, t, "equal-map-v240-p-nil")
		// ...
		if v == nil {
			v240v2 = nil
		} else {
			v240v2 = make(map[int32]uint16, len(v))
		} // reset map
		var v240v3, v240v4 typMapMapInt32Uint16
		v240v3 = typMapMapInt32Uint16(v240v1)
		v240v4 = typMapMapInt32Uint16(v240v2)
		bs240 = testMarshalErr(v240v3, h, t, "enc-map-v240-custom")
		testUnmarshalErr(v240v4, bs240, h, t, "dec-map-v240-p-len")
		testDeepEqualErr(v240v3, v240v4, t, "equal-map-v240-p-len")
	}

	for _, v := range []map[int32]uint32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v241: %v\n", v)
		var v241v1, v241v2 map[int32]uint32
		v241v1 = v
		bs241 := testMarshalErr(v241v1, h, t, "enc-map-v241")
		if v == nil {
			v241v2 = nil
		} else {
			v241v2 = make(map[int32]uint32, len(v))
		} // reset map
		testUnmarshalErr(v241v2, bs241, h, t, "dec-map-v241")
		testDeepEqualErr(v241v1, v241v2, t, "equal-map-v241")
		if v == nil {
			v241v2 = nil
		} else {
			v241v2 = make(map[int32]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v241v2), bs241, h, t, "dec-map-v241-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v241v1, v241v2, t, "equal-map-v241-noaddr")
		if v == nil {
			v241v2 = nil
		} else {
			v241v2 = make(map[int32]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v241v2, bs241, h, t, "dec-map-v241-p-len")
		testDeepEqualErr(v241v1, v241v2, t, "equal-map-v241-p-len")
		bs241 = testMarshalErr(&v241v1, h, t, "enc-map-v241-p")
		v241v2 = nil
		testUnmarshalErr(&v241v2, bs241, h, t, "dec-map-v241-p-nil")
		testDeepEqualErr(v241v1, v241v2, t, "equal-map-v241-p-nil")
		// ...
		if v == nil {
			v241v2 = nil
		} else {
			v241v2 = make(map[int32]uint32, len(v))
		} // reset map
		var v241v3, v241v4 typMapMapInt32Uint32
		v241v3 = typMapMapInt32Uint32(v241v1)
		v241v4 = typMapMapInt32Uint32(v241v2)
		bs241 = testMarshalErr(v241v3, h, t, "enc-map-v241-custom")
		testUnmarshalErr(v241v4, bs241, h, t, "dec-map-v241-p-len")
		testDeepEqualErr(v241v3, v241v4, t, "equal-map-v241-p-len")
	}

	for _, v := range []map[int32]uint64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v242: %v\n", v)
		var v242v1, v242v2 map[int32]uint64
		v242v1 = v
		bs242 := testMarshalErr(v242v1, h, t, "enc-map-v242")
		if v == nil {
			v242v2 = nil
		} else {
			v242v2 = make(map[int32]uint64, len(v))
		} // reset map
		testUnmarshalErr(v242v2, bs242, h, t, "dec-map-v242")
		testDeepEqualErr(v242v1, v242v2, t, "equal-map-v242")
		if v == nil {
			v242v2 = nil
		} else {
			v242v2 = make(map[int32]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v242v2), bs242, h, t, "dec-map-v242-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v242v1, v242v2, t, "equal-map-v242-noaddr")
		if v == nil {
			v242v2 = nil
		} else {
			v242v2 = make(map[int32]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v242v2, bs242, h, t, "dec-map-v242-p-len")
		testDeepEqualErr(v242v1, v242v2, t, "equal-map-v242-p-len")
		bs242 = testMarshalErr(&v242v1, h, t, "enc-map-v242-p")
		v242v2 = nil
		testUnmarshalErr(&v242v2, bs242, h, t, "dec-map-v242-p-nil")
		testDeepEqualErr(v242v1, v242v2, t, "equal-map-v242-p-nil")
		// ...
		if v == nil {
			v242v2 = nil
		} else {
			v242v2 = make(map[int32]uint64, len(v))
		} // reset map
		var v242v3, v242v4 typMapMapInt32Uint64
		v242v3 = typMapMapInt32Uint64(v242v1)
		v242v4 = typMapMapInt32Uint64(v242v2)
		bs242 = testMarshalErr(v242v3, h, t, "enc-map-v242-custom")
		testUnmarshalErr(v242v4, bs242, h, t, "dec-map-v242-p-len")
		testDeepEqualErr(v242v3, v242v4, t, "equal-map-v242-p-len")
	}

	for _, v := range []map[int32]uintptr{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v243: %v\n", v)
		var v243v1, v243v2 map[int32]uintptr
		v243v1 = v
		bs243 := testMarshalErr(v243v1, h, t, "enc-map-v243")
		if v == nil {
			v243v2 = nil
		} else {
			v243v2 = make(map[int32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v243v2, bs243, h, t, "dec-map-v243")
		testDeepEqualErr(v243v1, v243v2, t, "equal-map-v243")
		if v == nil {
			v243v2 = nil
		} else {
			v243v2 = make(map[int32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v243v2), bs243, h, t, "dec-map-v243-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v243v1, v243v2, t, "equal-map-v243-noaddr")
		if v == nil {
			v243v2 = nil
		} else {
			v243v2 = make(map[int32]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v243v2, bs243, h, t, "dec-map-v243-p-len")
		testDeepEqualErr(v243v1, v243v2, t, "equal-map-v243-p-len")
		bs243 = testMarshalErr(&v243v1, h, t, "enc-map-v243-p")
		v243v2 = nil
		testUnmarshalErr(&v243v2, bs243, h, t, "dec-map-v243-p-nil")
		testDeepEqualErr(v243v1, v243v2, t, "equal-map-v243-p-nil")
		// ...
		if v == nil {
			v243v2 = nil
		} else {
			v243v2 = make(map[int32]uintptr, len(v))
		} // reset map
		var v243v3, v243v4 typMapMapInt32Uintptr
		v243v3 = typMapMapInt32Uintptr(v243v1)
		v243v4 = typMapMapInt32Uintptr(v243v2)
		bs243 = testMarshalErr(v243v3, h, t, "enc-map-v243-custom")
		testUnmarshalErr(v243v4, bs243, h, t, "dec-map-v243-p-len")
		testDeepEqualErr(v243v3, v243v4, t, "equal-map-v243-p-len")
	}

	for _, v := range []map[int32]int{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v244: %v\n", v)
		var v244v1, v244v2 map[int32]int
		v244v1 = v
		bs244 := testMarshalErr(v244v1, h, t, "enc-map-v244")
		if v == nil {
			v244v2 = nil
		} else {
			v244v2 = make(map[int32]int, len(v))
		} // reset map
		testUnmarshalErr(v244v2, bs244, h, t, "dec-map-v244")
		testDeepEqualErr(v244v1, v244v2, t, "equal-map-v244")
		if v == nil {
			v244v2 = nil
		} else {
			v244v2 = make(map[int32]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v244v2), bs244, h, t, "dec-map-v244-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v244v1, v244v2, t, "equal-map-v244-noaddr")
		if v == nil {
			v244v2 = nil
		} else {
			v244v2 = make(map[int32]int, len(v))
		} // reset map
		testUnmarshalErr(&v244v2, bs244, h, t, "dec-map-v244-p-len")
		testDeepEqualErr(v244v1, v244v2, t, "equal-map-v244-p-len")
		bs244 = testMarshalErr(&v244v1, h, t, "enc-map-v244-p")
		v244v2 = nil
		testUnmarshalErr(&v244v2, bs244, h, t, "dec-map-v244-p-nil")
		testDeepEqualErr(v244v1, v244v2, t, "equal-map-v244-p-nil")
		// ...
		if v == nil {
			v244v2 = nil
		} else {
			v244v2 = make(map[int32]int, len(v))
		} // reset map
		var v244v3, v244v4 typMapMapInt32Int
		v244v3 = typMapMapInt32Int(v244v1)
		v244v4 = typMapMapInt32Int(v244v2)
		bs244 = testMarshalErr(v244v3, h, t, "enc-map-v244-custom")
		testUnmarshalErr(v244v4, bs244, h, t, "dec-map-v244-p-len")
		testDeepEqualErr(v244v3, v244v4, t, "equal-map-v244-p-len")
	}

	for _, v := range []map[int32]int8{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v245: %v\n", v)
		var v245v1, v245v2 map[int32]int8
		v245v1 = v
		bs245 := testMarshalErr(v245v1, h, t, "enc-map-v245")
		if v == nil {
			v245v2 = nil
		} else {
			v245v2 = make(map[int32]int8, len(v))
		} // reset map
		testUnmarshalErr(v245v2, bs245, h, t, "dec-map-v245")
		testDeepEqualErr(v245v1, v245v2, t, "equal-map-v245")
		if v == nil {
			v245v2 = nil
		} else {
			v245v2 = make(map[int32]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v245v2), bs245, h, t, "dec-map-v245-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v245v1, v245v2, t, "equal-map-v245-noaddr")
		if v == nil {
			v245v2 = nil
		} else {
			v245v2 = make(map[int32]int8, len(v))
		} // reset map
		testUnmarshalErr(&v245v2, bs245, h, t, "dec-map-v245-p-len")
		testDeepEqualErr(v245v1, v245v2, t, "equal-map-v245-p-len")
		bs245 = testMarshalErr(&v245v1, h, t, "enc-map-v245-p")
		v245v2 = nil
		testUnmarshalErr(&v245v2, bs245, h, t, "dec-map-v245-p-nil")
		testDeepEqualErr(v245v1, v245v2, t, "equal-map-v245-p-nil")
		// ...
		if v == nil {
			v245v2 = nil
		} else {
			v245v2 = make(map[int32]int8, len(v))
		} // reset map
		var v245v3, v245v4 typMapMapInt32Int8
		v245v3 = typMapMapInt32Int8(v245v1)
		v245v4 = typMapMapInt32Int8(v245v2)
		bs245 = testMarshalErr(v245v3, h, t, "enc-map-v245-custom")
		testUnmarshalErr(v245v4, bs245, h, t, "dec-map-v245-p-len")
		testDeepEqualErr(v245v3, v245v4, t, "equal-map-v245-p-len")
	}

	for _, v := range []map[int32]int16{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v246: %v\n", v)
		var v246v1, v246v2 map[int32]int16
		v246v1 = v
		bs246 := testMarshalErr(v246v1, h, t, "enc-map-v246")
		if v == nil {
			v246v2 = nil
		} else {
			v246v2 = make(map[int32]int16, len(v))
		} // reset map
		testUnmarshalErr(v246v2, bs246, h, t, "dec-map-v246")
		testDeepEqualErr(v246v1, v246v2, t, "equal-map-v246")
		if v == nil {
			v246v2 = nil
		} else {
			v246v2 = make(map[int32]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v246v2), bs246, h, t, "dec-map-v246-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v246v1, v246v2, t, "equal-map-v246-noaddr")
		if v == nil {
			v246v2 = nil
		} else {
			v246v2 = make(map[int32]int16, len(v))
		} // reset map
		testUnmarshalErr(&v246v2, bs246, h, t, "dec-map-v246-p-len")
		testDeepEqualErr(v246v1, v246v2, t, "equal-map-v246-p-len")
		bs246 = testMarshalErr(&v246v1, h, t, "enc-map-v246-p")
		v246v2 = nil
		testUnmarshalErr(&v246v2, bs246, h, t, "dec-map-v246-p-nil")
		testDeepEqualErr(v246v1, v246v2, t, "equal-map-v246-p-nil")
		// ...
		if v == nil {
			v246v2 = nil
		} else {
			v246v2 = make(map[int32]int16, len(v))
		} // reset map
		var v246v3, v246v4 typMapMapInt32Int16
		v246v3 = typMapMapInt32Int16(v246v1)
		v246v4 = typMapMapInt32Int16(v246v2)
		bs246 = testMarshalErr(v246v3, h, t, "enc-map-v246-custom")
		testUnmarshalErr(v246v4, bs246, h, t, "dec-map-v246-p-len")
		testDeepEqualErr(v246v3, v246v4, t, "equal-map-v246-p-len")
	}

	for _, v := range []map[int32]int32{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v247: %v\n", v)
		var v247v1, v247v2 map[int32]int32
		v247v1 = v
		bs247 := testMarshalErr(v247v1, h, t, "enc-map-v247")
		if v == nil {
			v247v2 = nil
		} else {
			v247v2 = make(map[int32]int32, len(v))
		} // reset map
		testUnmarshalErr(v247v2, bs247, h, t, "dec-map-v247")
		testDeepEqualErr(v247v1, v247v2, t, "equal-map-v247")
		if v == nil {
			v247v2 = nil
		} else {
			v247v2 = make(map[int32]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v247v2), bs247, h, t, "dec-map-v247-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v247v1, v247v2, t, "equal-map-v247-noaddr")
		if v == nil {
			v247v2 = nil
		} else {
			v247v2 = make(map[int32]int32, len(v))
		} // reset map
		testUnmarshalErr(&v247v2, bs247, h, t, "dec-map-v247-p-len")
		testDeepEqualErr(v247v1, v247v2, t, "equal-map-v247-p-len")
		bs247 = testMarshalErr(&v247v1, h, t, "enc-map-v247-p")
		v247v2 = nil
		testUnmarshalErr(&v247v2, bs247, h, t, "dec-map-v247-p-nil")
		testDeepEqualErr(v247v1, v247v2, t, "equal-map-v247-p-nil")
		// ...
		if v == nil {
			v247v2 = nil
		} else {
			v247v2 = make(map[int32]int32, len(v))
		} // reset map
		var v247v3, v247v4 typMapMapInt32Int32
		v247v3 = typMapMapInt32Int32(v247v1)
		v247v4 = typMapMapInt32Int32(v247v2)
		bs247 = testMarshalErr(v247v3, h, t, "enc-map-v247-custom")
		testUnmarshalErr(v247v4, bs247, h, t, "dec-map-v247-p-len")
		testDeepEqualErr(v247v3, v247v4, t, "equal-map-v247-p-len")
	}

	for _, v := range []map[int32]int64{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v248: %v\n", v)
		var v248v1, v248v2 map[int32]int64
		v248v1 = v
		bs248 := testMarshalErr(v248v1, h, t, "enc-map-v248")
		if v == nil {
			v248v2 = nil
		} else {
			v248v2 = make(map[int32]int64, len(v))
		} // reset map
		testUnmarshalErr(v248v2, bs248, h, t, "dec-map-v248")
		testDeepEqualErr(v248v1, v248v2, t, "equal-map-v248")
		if v == nil {
			v248v2 = nil
		} else {
			v248v2 = make(map[int32]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v248v2), bs248, h, t, "dec-map-v248-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v248v1, v248v2, t, "equal-map-v248-noaddr")
		if v == nil {
			v248v2 = nil
		} else {
			v248v2 = make(map[int32]int64, len(v))
		} // reset map
		testUnmarshalErr(&v248v2, bs248, h, t, "dec-map-v248-p-len")
		testDeepEqualErr(v248v1, v248v2, t, "equal-map-v248-p-len")
		bs248 = testMarshalErr(&v248v1, h, t, "enc-map-v248-p")
		v248v2 = nil
		testUnmarshalErr(&v248v2, bs248, h, t, "dec-map-v248-p-nil")
		testDeepEqualErr(v248v1, v248v2, t, "equal-map-v248-p-nil")
		// ...
		if v == nil {
			v248v2 = nil
		} else {
			v248v2 = make(map[int32]int64, len(v))
		} // reset map
		var v248v3, v248v4 typMapMapInt32Int64
		v248v3 = typMapMapInt32Int64(v248v1)
		v248v4 = typMapMapInt32Int64(v248v2)
		bs248 = testMarshalErr(v248v3, h, t, "enc-map-v248-custom")
		testUnmarshalErr(v248v4, bs248, h, t, "dec-map-v248-p-len")
		testDeepEqualErr(v248v3, v248v4, t, "equal-map-v248-p-len")
	}

	for _, v := range []map[int32]float32{nil, {}, {44: 0, 33: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v249: %v\n", v)
		var v249v1, v249v2 map[int32]float32
		v249v1 = v
		bs249 := testMarshalErr(v249v1, h, t, "enc-map-v249")
		if v == nil {
			v249v2 = nil
		} else {
			v249v2 = make(map[int32]float32, len(v))
		} // reset map
		testUnmarshalErr(v249v2, bs249, h, t, "dec-map-v249")
		testDeepEqualErr(v249v1, v249v2, t, "equal-map-v249")
		if v == nil {
			v249v2 = nil
		} else {
			v249v2 = make(map[int32]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v249v2), bs249, h, t, "dec-map-v249-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v249v1, v249v2, t, "equal-map-v249-noaddr")
		if v == nil {
			v249v2 = nil
		} else {
			v249v2 = make(map[int32]float32, len(v))
		} // reset map
		testUnmarshalErr(&v249v2, bs249, h, t, "dec-map-v249-p-len")
		testDeepEqualErr(v249v1, v249v2, t, "equal-map-v249-p-len")
		bs249 = testMarshalErr(&v249v1, h, t, "enc-map-v249-p")
		v249v2 = nil
		testUnmarshalErr(&v249v2, bs249, h, t, "dec-map-v249-p-nil")
		testDeepEqualErr(v249v1, v249v2, t, "equal-map-v249-p-nil")
		// ...
		if v == nil {
			v249v2 = nil
		} else {
			v249v2 = make(map[int32]float32, len(v))
		} // reset map
		var v249v3, v249v4 typMapMapInt32Float32
		v249v3 = typMapMapInt32Float32(v249v1)
		v249v4 = typMapMapInt32Float32(v249v2)
		bs249 = testMarshalErr(v249v3, h, t, "enc-map-v249-custom")
		testUnmarshalErr(v249v4, bs249, h, t, "dec-map-v249-p-len")
		testDeepEqualErr(v249v3, v249v4, t, "equal-map-v249-p-len")
	}

	for _, v := range []map[int32]float64{nil, {}, {44: 0, 33: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v250: %v\n", v)
		var v250v1, v250v2 map[int32]float64
		v250v1 = v
		bs250 := testMarshalErr(v250v1, h, t, "enc-map-v250")
		if v == nil {
			v250v2 = nil
		} else {
			v250v2 = make(map[int32]float64, len(v))
		} // reset map
		testUnmarshalErr(v250v2, bs250, h, t, "dec-map-v250")
		testDeepEqualErr(v250v1, v250v2, t, "equal-map-v250")
		if v == nil {
			v250v2 = nil
		} else {
			v250v2 = make(map[int32]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v250v2), bs250, h, t, "dec-map-v250-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v250v1, v250v2, t, "equal-map-v250-noaddr")
		if v == nil {
			v250v2 = nil
		} else {
			v250v2 = make(map[int32]float64, len(v))
		} // reset map
		testUnmarshalErr(&v250v2, bs250, h, t, "dec-map-v250-p-len")
		testDeepEqualErr(v250v1, v250v2, t, "equal-map-v250-p-len")
		bs250 = testMarshalErr(&v250v1, h, t, "enc-map-v250-p")
		v250v2 = nil
		testUnmarshalErr(&v250v2, bs250, h, t, "dec-map-v250-p-nil")
		testDeepEqualErr(v250v1, v250v2, t, "equal-map-v250-p-nil")
		// ...
		if v == nil {
			v250v2 = nil
		} else {
			v250v2 = make(map[int32]float64, len(v))
		} // reset map
		var v250v3, v250v4 typMapMapInt32Float64
		v250v3 = typMapMapInt32Float64(v250v1)
		v250v4 = typMapMapInt32Float64(v250v2)
		bs250 = testMarshalErr(v250v3, h, t, "enc-map-v250-custom")
		testUnmarshalErr(v250v4, bs250, h, t, "dec-map-v250-p-len")
		testDeepEqualErr(v250v3, v250v4, t, "equal-map-v250-p-len")
	}

	for _, v := range []map[int32]bool{nil, {}, {44: false, 33: true}} {
		// fmt.Printf(">>>> running mammoth map v251: %v\n", v)
		var v251v1, v251v2 map[int32]bool
		v251v1 = v
		bs251 := testMarshalErr(v251v1, h, t, "enc-map-v251")
		if v == nil {
			v251v2 = nil
		} else {
			v251v2 = make(map[int32]bool, len(v))
		} // reset map
		testUnmarshalErr(v251v2, bs251, h, t, "dec-map-v251")
		testDeepEqualErr(v251v1, v251v2, t, "equal-map-v251")
		if v == nil {
			v251v2 = nil
		} else {
			v251v2 = make(map[int32]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v251v2), bs251, h, t, "dec-map-v251-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v251v1, v251v2, t, "equal-map-v251-noaddr")
		if v == nil {
			v251v2 = nil
		} else {
			v251v2 = make(map[int32]bool, len(v))
		} // reset map
		testUnmarshalErr(&v251v2, bs251, h, t, "dec-map-v251-p-len")
		testDeepEqualErr(v251v1, v251v2, t, "equal-map-v251-p-len")
		bs251 = testMarshalErr(&v251v1, h, t, "enc-map-v251-p")
		v251v2 = nil
		testUnmarshalErr(&v251v2, bs251, h, t, "dec-map-v251-p-nil")
		testDeepEqualErr(v251v1, v251v2, t, "equal-map-v251-p-nil")
		// ...
		if v == nil {
			v251v2 = nil
		} else {
			v251v2 = make(map[int32]bool, len(v))
		} // reset map
		var v251v3, v251v4 typMapMapInt32Bool
		v251v3 = typMapMapInt32Bool(v251v1)
		v251v4 = typMapMapInt32Bool(v251v2)
		bs251 = testMarshalErr(v251v3, h, t, "enc-map-v251-custom")
		testUnmarshalErr(v251v4, bs251, h, t, "dec-map-v251-p-len")
		testDeepEqualErr(v251v3, v251v4, t, "equal-map-v251-p-len")
	}

	for _, v := range []map[int64]interface{}{nil, {}, {44: nil, 33: "string-is-an-interface-2"}} {
		// fmt.Printf(">>>> running mammoth map v254: %v\n", v)
		var v254v1, v254v2 map[int64]interface{}
		v254v1 = v
		bs254 := testMarshalErr(v254v1, h, t, "enc-map-v254")
		if v == nil {
			v254v2 = nil
		} else {
			v254v2 = make(map[int64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v254v2, bs254, h, t, "dec-map-v254")
		testDeepEqualErr(v254v1, v254v2, t, "equal-map-v254")
		if v == nil {
			v254v2 = nil
		} else {
			v254v2 = make(map[int64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v254v2), bs254, h, t, "dec-map-v254-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v254v1, v254v2, t, "equal-map-v254-noaddr")
		if v == nil {
			v254v2 = nil
		} else {
			v254v2 = make(map[int64]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v254v2, bs254, h, t, "dec-map-v254-p-len")
		testDeepEqualErr(v254v1, v254v2, t, "equal-map-v254-p-len")
		bs254 = testMarshalErr(&v254v1, h, t, "enc-map-v254-p")
		v254v2 = nil
		testUnmarshalErr(&v254v2, bs254, h, t, "dec-map-v254-p-nil")
		testDeepEqualErr(v254v1, v254v2, t, "equal-map-v254-p-nil")
		// ...
		if v == nil {
			v254v2 = nil
		} else {
			v254v2 = make(map[int64]interface{}, len(v))
		} // reset map
		var v254v3, v254v4 typMapMapInt64Intf
		v254v3 = typMapMapInt64Intf(v254v1)
		v254v4 = typMapMapInt64Intf(v254v2)
		bs254 = testMarshalErr(v254v3, h, t, "enc-map-v254-custom")
		testUnmarshalErr(v254v4, bs254, h, t, "dec-map-v254-p-len")
		testDeepEqualErr(v254v3, v254v4, t, "equal-map-v254-p-len")
	}

	for _, v := range []map[int64]string{nil, {}, {44: "", 33: "some-string-2"}} {
		// fmt.Printf(">>>> running mammoth map v255: %v\n", v)
		var v255v1, v255v2 map[int64]string
		v255v1 = v
		bs255 := testMarshalErr(v255v1, h, t, "enc-map-v255")
		if v == nil {
			v255v2 = nil
		} else {
			v255v2 = make(map[int64]string, len(v))
		} // reset map
		testUnmarshalErr(v255v2, bs255, h, t, "dec-map-v255")
		testDeepEqualErr(v255v1, v255v2, t, "equal-map-v255")
		if v == nil {
			v255v2 = nil
		} else {
			v255v2 = make(map[int64]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v255v2), bs255, h, t, "dec-map-v255-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v255v1, v255v2, t, "equal-map-v255-noaddr")
		if v == nil {
			v255v2 = nil
		} else {
			v255v2 = make(map[int64]string, len(v))
		} // reset map
		testUnmarshalErr(&v255v2, bs255, h, t, "dec-map-v255-p-len")
		testDeepEqualErr(v255v1, v255v2, t, "equal-map-v255-p-len")
		bs255 = testMarshalErr(&v255v1, h, t, "enc-map-v255-p")
		v255v2 = nil
		testUnmarshalErr(&v255v2, bs255, h, t, "dec-map-v255-p-nil")
		testDeepEqualErr(v255v1, v255v2, t, "equal-map-v255-p-nil")
		// ...
		if v == nil {
			v255v2 = nil
		} else {
			v255v2 = make(map[int64]string, len(v))
		} // reset map
		var v255v3, v255v4 typMapMapInt64String
		v255v3 = typMapMapInt64String(v255v1)
		v255v4 = typMapMapInt64String(v255v2)
		bs255 = testMarshalErr(v255v3, h, t, "enc-map-v255-custom")
		testUnmarshalErr(v255v4, bs255, h, t, "dec-map-v255-p-len")
		testDeepEqualErr(v255v3, v255v4, t, "equal-map-v255-p-len")
	}

	for _, v := range []map[int64]uint{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v256: %v\n", v)
		var v256v1, v256v2 map[int64]uint
		v256v1 = v
		bs256 := testMarshalErr(v256v1, h, t, "enc-map-v256")
		if v == nil {
			v256v2 = nil
		} else {
			v256v2 = make(map[int64]uint, len(v))
		} // reset map
		testUnmarshalErr(v256v2, bs256, h, t, "dec-map-v256")
		testDeepEqualErr(v256v1, v256v2, t, "equal-map-v256")
		if v == nil {
			v256v2 = nil
		} else {
			v256v2 = make(map[int64]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v256v2), bs256, h, t, "dec-map-v256-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v256v1, v256v2, t, "equal-map-v256-noaddr")
		if v == nil {
			v256v2 = nil
		} else {
			v256v2 = make(map[int64]uint, len(v))
		} // reset map
		testUnmarshalErr(&v256v2, bs256, h, t, "dec-map-v256-p-len")
		testDeepEqualErr(v256v1, v256v2, t, "equal-map-v256-p-len")
		bs256 = testMarshalErr(&v256v1, h, t, "enc-map-v256-p")
		v256v2 = nil
		testUnmarshalErr(&v256v2, bs256, h, t, "dec-map-v256-p-nil")
		testDeepEqualErr(v256v1, v256v2, t, "equal-map-v256-p-nil")
		// ...
		if v == nil {
			v256v2 = nil
		} else {
			v256v2 = make(map[int64]uint, len(v))
		} // reset map
		var v256v3, v256v4 typMapMapInt64Uint
		v256v3 = typMapMapInt64Uint(v256v1)
		v256v4 = typMapMapInt64Uint(v256v2)
		bs256 = testMarshalErr(v256v3, h, t, "enc-map-v256-custom")
		testUnmarshalErr(v256v4, bs256, h, t, "dec-map-v256-p-len")
		testDeepEqualErr(v256v3, v256v4, t, "equal-map-v256-p-len")
	}

	for _, v := range []map[int64]uint8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v257: %v\n", v)
		var v257v1, v257v2 map[int64]uint8
		v257v1 = v
		bs257 := testMarshalErr(v257v1, h, t, "enc-map-v257")
		if v == nil {
			v257v2 = nil
		} else {
			v257v2 = make(map[int64]uint8, len(v))
		} // reset map
		testUnmarshalErr(v257v2, bs257, h, t, "dec-map-v257")
		testDeepEqualErr(v257v1, v257v2, t, "equal-map-v257")
		if v == nil {
			v257v2 = nil
		} else {
			v257v2 = make(map[int64]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v257v2), bs257, h, t, "dec-map-v257-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v257v1, v257v2, t, "equal-map-v257-noaddr")
		if v == nil {
			v257v2 = nil
		} else {
			v257v2 = make(map[int64]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v257v2, bs257, h, t, "dec-map-v257-p-len")
		testDeepEqualErr(v257v1, v257v2, t, "equal-map-v257-p-len")
		bs257 = testMarshalErr(&v257v1, h, t, "enc-map-v257-p")
		v257v2 = nil
		testUnmarshalErr(&v257v2, bs257, h, t, "dec-map-v257-p-nil")
		testDeepEqualErr(v257v1, v257v2, t, "equal-map-v257-p-nil")
		// ...
		if v == nil {
			v257v2 = nil
		} else {
			v257v2 = make(map[int64]uint8, len(v))
		} // reset map
		var v257v3, v257v4 typMapMapInt64Uint8
		v257v3 = typMapMapInt64Uint8(v257v1)
		v257v4 = typMapMapInt64Uint8(v257v2)
		bs257 = testMarshalErr(v257v3, h, t, "enc-map-v257-custom")
		testUnmarshalErr(v257v4, bs257, h, t, "dec-map-v257-p-len")
		testDeepEqualErr(v257v3, v257v4, t, "equal-map-v257-p-len")
	}

	for _, v := range []map[int64]uint16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v258: %v\n", v)
		var v258v1, v258v2 map[int64]uint16
		v258v1 = v
		bs258 := testMarshalErr(v258v1, h, t, "enc-map-v258")
		if v == nil {
			v258v2 = nil
		} else {
			v258v2 = make(map[int64]uint16, len(v))
		} // reset map
		testUnmarshalErr(v258v2, bs258, h, t, "dec-map-v258")
		testDeepEqualErr(v258v1, v258v2, t, "equal-map-v258")
		if v == nil {
			v258v2 = nil
		} else {
			v258v2 = make(map[int64]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v258v2), bs258, h, t, "dec-map-v258-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v258v1, v258v2, t, "equal-map-v258-noaddr")
		if v == nil {
			v258v2 = nil
		} else {
			v258v2 = make(map[int64]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v258v2, bs258, h, t, "dec-map-v258-p-len")
		testDeepEqualErr(v258v1, v258v2, t, "equal-map-v258-p-len")
		bs258 = testMarshalErr(&v258v1, h, t, "enc-map-v258-p")
		v258v2 = nil
		testUnmarshalErr(&v258v2, bs258, h, t, "dec-map-v258-p-nil")
		testDeepEqualErr(v258v1, v258v2, t, "equal-map-v258-p-nil")
		// ...
		if v == nil {
			v258v2 = nil
		} else {
			v258v2 = make(map[int64]uint16, len(v))
		} // reset map
		var v258v3, v258v4 typMapMapInt64Uint16
		v258v3 = typMapMapInt64Uint16(v258v1)
		v258v4 = typMapMapInt64Uint16(v258v2)
		bs258 = testMarshalErr(v258v3, h, t, "enc-map-v258-custom")
		testUnmarshalErr(v258v4, bs258, h, t, "dec-map-v258-p-len")
		testDeepEqualErr(v258v3, v258v4, t, "equal-map-v258-p-len")
	}

	for _, v := range []map[int64]uint32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v259: %v\n", v)
		var v259v1, v259v2 map[int64]uint32
		v259v1 = v
		bs259 := testMarshalErr(v259v1, h, t, "enc-map-v259")
		if v == nil {
			v259v2 = nil
		} else {
			v259v2 = make(map[int64]uint32, len(v))
		} // reset map
		testUnmarshalErr(v259v2, bs259, h, t, "dec-map-v259")
		testDeepEqualErr(v259v1, v259v2, t, "equal-map-v259")
		if v == nil {
			v259v2 = nil
		} else {
			v259v2 = make(map[int64]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v259v2), bs259, h, t, "dec-map-v259-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v259v1, v259v2, t, "equal-map-v259-noaddr")
		if v == nil {
			v259v2 = nil
		} else {
			v259v2 = make(map[int64]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v259v2, bs259, h, t, "dec-map-v259-p-len")
		testDeepEqualErr(v259v1, v259v2, t, "equal-map-v259-p-len")
		bs259 = testMarshalErr(&v259v1, h, t, "enc-map-v259-p")
		v259v2 = nil
		testUnmarshalErr(&v259v2, bs259, h, t, "dec-map-v259-p-nil")
		testDeepEqualErr(v259v1, v259v2, t, "equal-map-v259-p-nil")
		// ...
		if v == nil {
			v259v2 = nil
		} else {
			v259v2 = make(map[int64]uint32, len(v))
		} // reset map
		var v259v3, v259v4 typMapMapInt64Uint32
		v259v3 = typMapMapInt64Uint32(v259v1)
		v259v4 = typMapMapInt64Uint32(v259v2)
		bs259 = testMarshalErr(v259v3, h, t, "enc-map-v259-custom")
		testUnmarshalErr(v259v4, bs259, h, t, "dec-map-v259-p-len")
		testDeepEqualErr(v259v3, v259v4, t, "equal-map-v259-p-len")
	}

	for _, v := range []map[int64]uint64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v260: %v\n", v)
		var v260v1, v260v2 map[int64]uint64
		v260v1 = v
		bs260 := testMarshalErr(v260v1, h, t, "enc-map-v260")
		if v == nil {
			v260v2 = nil
		} else {
			v260v2 = make(map[int64]uint64, len(v))
		} // reset map
		testUnmarshalErr(v260v2, bs260, h, t, "dec-map-v260")
		testDeepEqualErr(v260v1, v260v2, t, "equal-map-v260")
		if v == nil {
			v260v2 = nil
		} else {
			v260v2 = make(map[int64]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v260v2), bs260, h, t, "dec-map-v260-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v260v1, v260v2, t, "equal-map-v260-noaddr")
		if v == nil {
			v260v2 = nil
		} else {
			v260v2 = make(map[int64]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v260v2, bs260, h, t, "dec-map-v260-p-len")
		testDeepEqualErr(v260v1, v260v2, t, "equal-map-v260-p-len")
		bs260 = testMarshalErr(&v260v1, h, t, "enc-map-v260-p")
		v260v2 = nil
		testUnmarshalErr(&v260v2, bs260, h, t, "dec-map-v260-p-nil")
		testDeepEqualErr(v260v1, v260v2, t, "equal-map-v260-p-nil")
		// ...
		if v == nil {
			v260v2 = nil
		} else {
			v260v2 = make(map[int64]uint64, len(v))
		} // reset map
		var v260v3, v260v4 typMapMapInt64Uint64
		v260v3 = typMapMapInt64Uint64(v260v1)
		v260v4 = typMapMapInt64Uint64(v260v2)
		bs260 = testMarshalErr(v260v3, h, t, "enc-map-v260-custom")
		testUnmarshalErr(v260v4, bs260, h, t, "dec-map-v260-p-len")
		testDeepEqualErr(v260v3, v260v4, t, "equal-map-v260-p-len")
	}

	for _, v := range []map[int64]uintptr{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v261: %v\n", v)
		var v261v1, v261v2 map[int64]uintptr
		v261v1 = v
		bs261 := testMarshalErr(v261v1, h, t, "enc-map-v261")
		if v == nil {
			v261v2 = nil
		} else {
			v261v2 = make(map[int64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v261v2, bs261, h, t, "dec-map-v261")
		testDeepEqualErr(v261v1, v261v2, t, "equal-map-v261")
		if v == nil {
			v261v2 = nil
		} else {
			v261v2 = make(map[int64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v261v2), bs261, h, t, "dec-map-v261-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v261v1, v261v2, t, "equal-map-v261-noaddr")
		if v == nil {
			v261v2 = nil
		} else {
			v261v2 = make(map[int64]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v261v2, bs261, h, t, "dec-map-v261-p-len")
		testDeepEqualErr(v261v1, v261v2, t, "equal-map-v261-p-len")
		bs261 = testMarshalErr(&v261v1, h, t, "enc-map-v261-p")
		v261v2 = nil
		testUnmarshalErr(&v261v2, bs261, h, t, "dec-map-v261-p-nil")
		testDeepEqualErr(v261v1, v261v2, t, "equal-map-v261-p-nil")
		// ...
		if v == nil {
			v261v2 = nil
		} else {
			v261v2 = make(map[int64]uintptr, len(v))
		} // reset map
		var v261v3, v261v4 typMapMapInt64Uintptr
		v261v3 = typMapMapInt64Uintptr(v261v1)
		v261v4 = typMapMapInt64Uintptr(v261v2)
		bs261 = testMarshalErr(v261v3, h, t, "enc-map-v261-custom")
		testUnmarshalErr(v261v4, bs261, h, t, "dec-map-v261-p-len")
		testDeepEqualErr(v261v3, v261v4, t, "equal-map-v261-p-len")
	}

	for _, v := range []map[int64]int{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v262: %v\n", v)
		var v262v1, v262v2 map[int64]int
		v262v1 = v
		bs262 := testMarshalErr(v262v1, h, t, "enc-map-v262")
		if v == nil {
			v262v2 = nil
		} else {
			v262v2 = make(map[int64]int, len(v))
		} // reset map
		testUnmarshalErr(v262v2, bs262, h, t, "dec-map-v262")
		testDeepEqualErr(v262v1, v262v2, t, "equal-map-v262")
		if v == nil {
			v262v2 = nil
		} else {
			v262v2 = make(map[int64]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v262v2), bs262, h, t, "dec-map-v262-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v262v1, v262v2, t, "equal-map-v262-noaddr")
		if v == nil {
			v262v2 = nil
		} else {
			v262v2 = make(map[int64]int, len(v))
		} // reset map
		testUnmarshalErr(&v262v2, bs262, h, t, "dec-map-v262-p-len")
		testDeepEqualErr(v262v1, v262v2, t, "equal-map-v262-p-len")
		bs262 = testMarshalErr(&v262v1, h, t, "enc-map-v262-p")
		v262v2 = nil
		testUnmarshalErr(&v262v2, bs262, h, t, "dec-map-v262-p-nil")
		testDeepEqualErr(v262v1, v262v2, t, "equal-map-v262-p-nil")
		// ...
		if v == nil {
			v262v2 = nil
		} else {
			v262v2 = make(map[int64]int, len(v))
		} // reset map
		var v262v3, v262v4 typMapMapInt64Int
		v262v3 = typMapMapInt64Int(v262v1)
		v262v4 = typMapMapInt64Int(v262v2)
		bs262 = testMarshalErr(v262v3, h, t, "enc-map-v262-custom")
		testUnmarshalErr(v262v4, bs262, h, t, "dec-map-v262-p-len")
		testDeepEqualErr(v262v3, v262v4, t, "equal-map-v262-p-len")
	}

	for _, v := range []map[int64]int8{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v263: %v\n", v)
		var v263v1, v263v2 map[int64]int8
		v263v1 = v
		bs263 := testMarshalErr(v263v1, h, t, "enc-map-v263")
		if v == nil {
			v263v2 = nil
		} else {
			v263v2 = make(map[int64]int8, len(v))
		} // reset map
		testUnmarshalErr(v263v2, bs263, h, t, "dec-map-v263")
		testDeepEqualErr(v263v1, v263v2, t, "equal-map-v263")
		if v == nil {
			v263v2 = nil
		} else {
			v263v2 = make(map[int64]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v263v2), bs263, h, t, "dec-map-v263-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v263v1, v263v2, t, "equal-map-v263-noaddr")
		if v == nil {
			v263v2 = nil
		} else {
			v263v2 = make(map[int64]int8, len(v))
		} // reset map
		testUnmarshalErr(&v263v2, bs263, h, t, "dec-map-v263-p-len")
		testDeepEqualErr(v263v1, v263v2, t, "equal-map-v263-p-len")
		bs263 = testMarshalErr(&v263v1, h, t, "enc-map-v263-p")
		v263v2 = nil
		testUnmarshalErr(&v263v2, bs263, h, t, "dec-map-v263-p-nil")
		testDeepEqualErr(v263v1, v263v2, t, "equal-map-v263-p-nil")
		// ...
		if v == nil {
			v263v2 = nil
		} else {
			v263v2 = make(map[int64]int8, len(v))
		} // reset map
		var v263v3, v263v4 typMapMapInt64Int8
		v263v3 = typMapMapInt64Int8(v263v1)
		v263v4 = typMapMapInt64Int8(v263v2)
		bs263 = testMarshalErr(v263v3, h, t, "enc-map-v263-custom")
		testUnmarshalErr(v263v4, bs263, h, t, "dec-map-v263-p-len")
		testDeepEqualErr(v263v3, v263v4, t, "equal-map-v263-p-len")
	}

	for _, v := range []map[int64]int16{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v264: %v\n", v)
		var v264v1, v264v2 map[int64]int16
		v264v1 = v
		bs264 := testMarshalErr(v264v1, h, t, "enc-map-v264")
		if v == nil {
			v264v2 = nil
		} else {
			v264v2 = make(map[int64]int16, len(v))
		} // reset map
		testUnmarshalErr(v264v2, bs264, h, t, "dec-map-v264")
		testDeepEqualErr(v264v1, v264v2, t, "equal-map-v264")
		if v == nil {
			v264v2 = nil
		} else {
			v264v2 = make(map[int64]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v264v2), bs264, h, t, "dec-map-v264-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v264v1, v264v2, t, "equal-map-v264-noaddr")
		if v == nil {
			v264v2 = nil
		} else {
			v264v2 = make(map[int64]int16, len(v))
		} // reset map
		testUnmarshalErr(&v264v2, bs264, h, t, "dec-map-v264-p-len")
		testDeepEqualErr(v264v1, v264v2, t, "equal-map-v264-p-len")
		bs264 = testMarshalErr(&v264v1, h, t, "enc-map-v264-p")
		v264v2 = nil
		testUnmarshalErr(&v264v2, bs264, h, t, "dec-map-v264-p-nil")
		testDeepEqualErr(v264v1, v264v2, t, "equal-map-v264-p-nil")
		// ...
		if v == nil {
			v264v2 = nil
		} else {
			v264v2 = make(map[int64]int16, len(v))
		} // reset map
		var v264v3, v264v4 typMapMapInt64Int16
		v264v3 = typMapMapInt64Int16(v264v1)
		v264v4 = typMapMapInt64Int16(v264v2)
		bs264 = testMarshalErr(v264v3, h, t, "enc-map-v264-custom")
		testUnmarshalErr(v264v4, bs264, h, t, "dec-map-v264-p-len")
		testDeepEqualErr(v264v3, v264v4, t, "equal-map-v264-p-len")
	}

	for _, v := range []map[int64]int32{nil, {}, {33: 0, 44: 33}} {
		// fmt.Printf(">>>> running mammoth map v265: %v\n", v)
		var v265v1, v265v2 map[int64]int32
		v265v1 = v
		bs265 := testMarshalErr(v265v1, h, t, "enc-map-v265")
		if v == nil {
			v265v2 = nil
		} else {
			v265v2 = make(map[int64]int32, len(v))
		} // reset map
		testUnmarshalErr(v265v2, bs265, h, t, "dec-map-v265")
		testDeepEqualErr(v265v1, v265v2, t, "equal-map-v265")
		if v == nil {
			v265v2 = nil
		} else {
			v265v2 = make(map[int64]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v265v2), bs265, h, t, "dec-map-v265-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v265v1, v265v2, t, "equal-map-v265-noaddr")
		if v == nil {
			v265v2 = nil
		} else {
			v265v2 = make(map[int64]int32, len(v))
		} // reset map
		testUnmarshalErr(&v265v2, bs265, h, t, "dec-map-v265-p-len")
		testDeepEqualErr(v265v1, v265v2, t, "equal-map-v265-p-len")
		bs265 = testMarshalErr(&v265v1, h, t, "enc-map-v265-p")
		v265v2 = nil
		testUnmarshalErr(&v265v2, bs265, h, t, "dec-map-v265-p-nil")
		testDeepEqualErr(v265v1, v265v2, t, "equal-map-v265-p-nil")
		// ...
		if v == nil {
			v265v2 = nil
		} else {
			v265v2 = make(map[int64]int32, len(v))
		} // reset map
		var v265v3, v265v4 typMapMapInt64Int32
		v265v3 = typMapMapInt64Int32(v265v1)
		v265v4 = typMapMapInt64Int32(v265v2)
		bs265 = testMarshalErr(v265v3, h, t, "enc-map-v265-custom")
		testUnmarshalErr(v265v4, bs265, h, t, "dec-map-v265-p-len")
		testDeepEqualErr(v265v3, v265v4, t, "equal-map-v265-p-len")
	}

	for _, v := range []map[int64]int64{nil, {}, {44: 0, 33: 44}} {
		// fmt.Printf(">>>> running mammoth map v266: %v\n", v)
		var v266v1, v266v2 map[int64]int64
		v266v1 = v
		bs266 := testMarshalErr(v266v1, h, t, "enc-map-v266")
		if v == nil {
			v266v2 = nil
		} else {
			v266v2 = make(map[int64]int64, len(v))
		} // reset map
		testUnmarshalErr(v266v2, bs266, h, t, "dec-map-v266")
		testDeepEqualErr(v266v1, v266v2, t, "equal-map-v266")
		if v == nil {
			v266v2 = nil
		} else {
			v266v2 = make(map[int64]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v266v2), bs266, h, t, "dec-map-v266-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v266v1, v266v2, t, "equal-map-v266-noaddr")
		if v == nil {
			v266v2 = nil
		} else {
			v266v2 = make(map[int64]int64, len(v))
		} // reset map
		testUnmarshalErr(&v266v2, bs266, h, t, "dec-map-v266-p-len")
		testDeepEqualErr(v266v1, v266v2, t, "equal-map-v266-p-len")
		bs266 = testMarshalErr(&v266v1, h, t, "enc-map-v266-p")
		v266v2 = nil
		testUnmarshalErr(&v266v2, bs266, h, t, "dec-map-v266-p-nil")
		testDeepEqualErr(v266v1, v266v2, t, "equal-map-v266-p-nil")
		// ...
		if v == nil {
			v266v2 = nil
		} else {
			v266v2 = make(map[int64]int64, len(v))
		} // reset map
		var v266v3, v266v4 typMapMapInt64Int64
		v266v3 = typMapMapInt64Int64(v266v1)
		v266v4 = typMapMapInt64Int64(v266v2)
		bs266 = testMarshalErr(v266v3, h, t, "enc-map-v266-custom")
		testUnmarshalErr(v266v4, bs266, h, t, "dec-map-v266-p-len")
		testDeepEqualErr(v266v3, v266v4, t, "equal-map-v266-p-len")
	}

	for _, v := range []map[int64]float32{nil, {}, {33: 0, 44: 22.2}} {
		// fmt.Printf(">>>> running mammoth map v267: %v\n", v)
		var v267v1, v267v2 map[int64]float32
		v267v1 = v
		bs267 := testMarshalErr(v267v1, h, t, "enc-map-v267")
		if v == nil {
			v267v2 = nil
		} else {
			v267v2 = make(map[int64]float32, len(v))
		} // reset map
		testUnmarshalErr(v267v2, bs267, h, t, "dec-map-v267")
		testDeepEqualErr(v267v1, v267v2, t, "equal-map-v267")
		if v == nil {
			v267v2 = nil
		} else {
			v267v2 = make(map[int64]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v267v2), bs267, h, t, "dec-map-v267-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v267v1, v267v2, t, "equal-map-v267-noaddr")
		if v == nil {
			v267v2 = nil
		} else {
			v267v2 = make(map[int64]float32, len(v))
		} // reset map
		testUnmarshalErr(&v267v2, bs267, h, t, "dec-map-v267-p-len")
		testDeepEqualErr(v267v1, v267v2, t, "equal-map-v267-p-len")
		bs267 = testMarshalErr(&v267v1, h, t, "enc-map-v267-p")
		v267v2 = nil
		testUnmarshalErr(&v267v2, bs267, h, t, "dec-map-v267-p-nil")
		testDeepEqualErr(v267v1, v267v2, t, "equal-map-v267-p-nil")
		// ...
		if v == nil {
			v267v2 = nil
		} else {
			v267v2 = make(map[int64]float32, len(v))
		} // reset map
		var v267v3, v267v4 typMapMapInt64Float32
		v267v3 = typMapMapInt64Float32(v267v1)
		v267v4 = typMapMapInt64Float32(v267v2)
		bs267 = testMarshalErr(v267v3, h, t, "enc-map-v267-custom")
		testUnmarshalErr(v267v4, bs267, h, t, "dec-map-v267-p-len")
		testDeepEqualErr(v267v3, v267v4, t, "equal-map-v267-p-len")
	}

	for _, v := range []map[int64]float64{nil, {}, {33: 0, 44: 11.1}} {
		// fmt.Printf(">>>> running mammoth map v268: %v\n", v)
		var v268v1, v268v2 map[int64]float64
		v268v1 = v
		bs268 := testMarshalErr(v268v1, h, t, "enc-map-v268")
		if v == nil {
			v268v2 = nil
		} else {
			v268v2 = make(map[int64]float64, len(v))
		} // reset map
		testUnmarshalErr(v268v2, bs268, h, t, "dec-map-v268")
		testDeepEqualErr(v268v1, v268v2, t, "equal-map-v268")
		if v == nil {
			v268v2 = nil
		} else {
			v268v2 = make(map[int64]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v268v2), bs268, h, t, "dec-map-v268-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v268v1, v268v2, t, "equal-map-v268-noaddr")
		if v == nil {
			v268v2 = nil
		} else {
			v268v2 = make(map[int64]float64, len(v))
		} // reset map
		testUnmarshalErr(&v268v2, bs268, h, t, "dec-map-v268-p-len")
		testDeepEqualErr(v268v1, v268v2, t, "equal-map-v268-p-len")
		bs268 = testMarshalErr(&v268v1, h, t, "enc-map-v268-p")
		v268v2 = nil
		testUnmarshalErr(&v268v2, bs268, h, t, "dec-map-v268-p-nil")
		testDeepEqualErr(v268v1, v268v2, t, "equal-map-v268-p-nil")
		// ...
		if v == nil {
			v268v2 = nil
		} else {
			v268v2 = make(map[int64]float64, len(v))
		} // reset map
		var v268v3, v268v4 typMapMapInt64Float64
		v268v3 = typMapMapInt64Float64(v268v1)
		v268v4 = typMapMapInt64Float64(v268v2)
		bs268 = testMarshalErr(v268v3, h, t, "enc-map-v268-custom")
		testUnmarshalErr(v268v4, bs268, h, t, "dec-map-v268-p-len")
		testDeepEqualErr(v268v3, v268v4, t, "equal-map-v268-p-len")
	}

	for _, v := range []map[int64]bool{nil, {}, {33: false, 44: true}} {
		// fmt.Printf(">>>> running mammoth map v269: %v\n", v)
		var v269v1, v269v2 map[int64]bool
		v269v1 = v
		bs269 := testMarshalErr(v269v1, h, t, "enc-map-v269")
		if v == nil {
			v269v2 = nil
		} else {
			v269v2 = make(map[int64]bool, len(v))
		} // reset map
		testUnmarshalErr(v269v2, bs269, h, t, "dec-map-v269")
		testDeepEqualErr(v269v1, v269v2, t, "equal-map-v269")
		if v == nil {
			v269v2 = nil
		} else {
			v269v2 = make(map[int64]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v269v2), bs269, h, t, "dec-map-v269-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v269v1, v269v2, t, "equal-map-v269-noaddr")
		if v == nil {
			v269v2 = nil
		} else {
			v269v2 = make(map[int64]bool, len(v))
		} // reset map
		testUnmarshalErr(&v269v2, bs269, h, t, "dec-map-v269-p-len")
		testDeepEqualErr(v269v1, v269v2, t, "equal-map-v269-p-len")
		bs269 = testMarshalErr(&v269v1, h, t, "enc-map-v269-p")
		v269v2 = nil
		testUnmarshalErr(&v269v2, bs269, h, t, "dec-map-v269-p-nil")
		testDeepEqualErr(v269v1, v269v2, t, "equal-map-v269-p-nil")
		// ...
		if v == nil {
			v269v2 = nil
		} else {
			v269v2 = make(map[int64]bool, len(v))
		} // reset map
		var v269v3, v269v4 typMapMapInt64Bool
		v269v3 = typMapMapInt64Bool(v269v1)
		v269v4 = typMapMapInt64Bool(v269v2)
		bs269 = testMarshalErr(v269v3, h, t, "enc-map-v269-custom")
		testUnmarshalErr(v269v4, bs269, h, t, "dec-map-v269-p-len")
		testDeepEqualErr(v269v3, v269v4, t, "equal-map-v269-p-len")
	}

	for _, v := range []map[bool]interface{}{nil, {}, {true: nil}} {
		// fmt.Printf(">>>> running mammoth map v272: %v\n", v)
		var v272v1, v272v2 map[bool]interface{}
		v272v1 = v
		bs272 := testMarshalErr(v272v1, h, t, "enc-map-v272")
		if v == nil {
			v272v2 = nil
		} else {
			v272v2 = make(map[bool]interface{}, len(v))
		} // reset map
		testUnmarshalErr(v272v2, bs272, h, t, "dec-map-v272")
		testDeepEqualErr(v272v1, v272v2, t, "equal-map-v272")
		if v == nil {
			v272v2 = nil
		} else {
			v272v2 = make(map[bool]interface{}, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v272v2), bs272, h, t, "dec-map-v272-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v272v1, v272v2, t, "equal-map-v272-noaddr")
		if v == nil {
			v272v2 = nil
		} else {
			v272v2 = make(map[bool]interface{}, len(v))
		} // reset map
		testUnmarshalErr(&v272v2, bs272, h, t, "dec-map-v272-p-len")
		testDeepEqualErr(v272v1, v272v2, t, "equal-map-v272-p-len")
		bs272 = testMarshalErr(&v272v1, h, t, "enc-map-v272-p")
		v272v2 = nil
		testUnmarshalErr(&v272v2, bs272, h, t, "dec-map-v272-p-nil")
		testDeepEqualErr(v272v1, v272v2, t, "equal-map-v272-p-nil")
		// ...
		if v == nil {
			v272v2 = nil
		} else {
			v272v2 = make(map[bool]interface{}, len(v))
		} // reset map
		var v272v3, v272v4 typMapMapBoolIntf
		v272v3 = typMapMapBoolIntf(v272v1)
		v272v4 = typMapMapBoolIntf(v272v2)
		bs272 = testMarshalErr(v272v3, h, t, "enc-map-v272-custom")
		testUnmarshalErr(v272v4, bs272, h, t, "dec-map-v272-p-len")
		testDeepEqualErr(v272v3, v272v4, t, "equal-map-v272-p-len")
	}

	for _, v := range []map[bool]string{nil, {}, {true: ""}} {
		// fmt.Printf(">>>> running mammoth map v273: %v\n", v)
		var v273v1, v273v2 map[bool]string
		v273v1 = v
		bs273 := testMarshalErr(v273v1, h, t, "enc-map-v273")
		if v == nil {
			v273v2 = nil
		} else {
			v273v2 = make(map[bool]string, len(v))
		} // reset map
		testUnmarshalErr(v273v2, bs273, h, t, "dec-map-v273")
		testDeepEqualErr(v273v1, v273v2, t, "equal-map-v273")
		if v == nil {
			v273v2 = nil
		} else {
			v273v2 = make(map[bool]string, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v273v2), bs273, h, t, "dec-map-v273-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v273v1, v273v2, t, "equal-map-v273-noaddr")
		if v == nil {
			v273v2 = nil
		} else {
			v273v2 = make(map[bool]string, len(v))
		} // reset map
		testUnmarshalErr(&v273v2, bs273, h, t, "dec-map-v273-p-len")
		testDeepEqualErr(v273v1, v273v2, t, "equal-map-v273-p-len")
		bs273 = testMarshalErr(&v273v1, h, t, "enc-map-v273-p")
		v273v2 = nil
		testUnmarshalErr(&v273v2, bs273, h, t, "dec-map-v273-p-nil")
		testDeepEqualErr(v273v1, v273v2, t, "equal-map-v273-p-nil")
		// ...
		if v == nil {
			v273v2 = nil
		} else {
			v273v2 = make(map[bool]string, len(v))
		} // reset map
		var v273v3, v273v4 typMapMapBoolString
		v273v3 = typMapMapBoolString(v273v1)
		v273v4 = typMapMapBoolString(v273v2)
		bs273 = testMarshalErr(v273v3, h, t, "enc-map-v273-custom")
		testUnmarshalErr(v273v4, bs273, h, t, "dec-map-v273-p-len")
		testDeepEqualErr(v273v3, v273v4, t, "equal-map-v273-p-len")
	}

	for _, v := range []map[bool]uint{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v274: %v\n", v)
		var v274v1, v274v2 map[bool]uint
		v274v1 = v
		bs274 := testMarshalErr(v274v1, h, t, "enc-map-v274")
		if v == nil {
			v274v2 = nil
		} else {
			v274v2 = make(map[bool]uint, len(v))
		} // reset map
		testUnmarshalErr(v274v2, bs274, h, t, "dec-map-v274")
		testDeepEqualErr(v274v1, v274v2, t, "equal-map-v274")
		if v == nil {
			v274v2 = nil
		} else {
			v274v2 = make(map[bool]uint, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v274v2), bs274, h, t, "dec-map-v274-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v274v1, v274v2, t, "equal-map-v274-noaddr")
		if v == nil {
			v274v2 = nil
		} else {
			v274v2 = make(map[bool]uint, len(v))
		} // reset map
		testUnmarshalErr(&v274v2, bs274, h, t, "dec-map-v274-p-len")
		testDeepEqualErr(v274v1, v274v2, t, "equal-map-v274-p-len")
		bs274 = testMarshalErr(&v274v1, h, t, "enc-map-v274-p")
		v274v2 = nil
		testUnmarshalErr(&v274v2, bs274, h, t, "dec-map-v274-p-nil")
		testDeepEqualErr(v274v1, v274v2, t, "equal-map-v274-p-nil")
		// ...
		if v == nil {
			v274v2 = nil
		} else {
			v274v2 = make(map[bool]uint, len(v))
		} // reset map
		var v274v3, v274v4 typMapMapBoolUint
		v274v3 = typMapMapBoolUint(v274v1)
		v274v4 = typMapMapBoolUint(v274v2)
		bs274 = testMarshalErr(v274v3, h, t, "enc-map-v274-custom")
		testUnmarshalErr(v274v4, bs274, h, t, "dec-map-v274-p-len")
		testDeepEqualErr(v274v3, v274v4, t, "equal-map-v274-p-len")
	}

	for _, v := range []map[bool]uint8{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v275: %v\n", v)
		var v275v1, v275v2 map[bool]uint8
		v275v1 = v
		bs275 := testMarshalErr(v275v1, h, t, "enc-map-v275")
		if v == nil {
			v275v2 = nil
		} else {
			v275v2 = make(map[bool]uint8, len(v))
		} // reset map
		testUnmarshalErr(v275v2, bs275, h, t, "dec-map-v275")
		testDeepEqualErr(v275v1, v275v2, t, "equal-map-v275")
		if v == nil {
			v275v2 = nil
		} else {
			v275v2 = make(map[bool]uint8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v275v2), bs275, h, t, "dec-map-v275-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v275v1, v275v2, t, "equal-map-v275-noaddr")
		if v == nil {
			v275v2 = nil
		} else {
			v275v2 = make(map[bool]uint8, len(v))
		} // reset map
		testUnmarshalErr(&v275v2, bs275, h, t, "dec-map-v275-p-len")
		testDeepEqualErr(v275v1, v275v2, t, "equal-map-v275-p-len")
		bs275 = testMarshalErr(&v275v1, h, t, "enc-map-v275-p")
		v275v2 = nil
		testUnmarshalErr(&v275v2, bs275, h, t, "dec-map-v275-p-nil")
		testDeepEqualErr(v275v1, v275v2, t, "equal-map-v275-p-nil")
		// ...
		if v == nil {
			v275v2 = nil
		} else {
			v275v2 = make(map[bool]uint8, len(v))
		} // reset map
		var v275v3, v275v4 typMapMapBoolUint8
		v275v3 = typMapMapBoolUint8(v275v1)
		v275v4 = typMapMapBoolUint8(v275v2)
		bs275 = testMarshalErr(v275v3, h, t, "enc-map-v275-custom")
		testUnmarshalErr(v275v4, bs275, h, t, "dec-map-v275-p-len")
		testDeepEqualErr(v275v3, v275v4, t, "equal-map-v275-p-len")
	}

	for _, v := range []map[bool]uint16{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v276: %v\n", v)
		var v276v1, v276v2 map[bool]uint16
		v276v1 = v
		bs276 := testMarshalErr(v276v1, h, t, "enc-map-v276")
		if v == nil {
			v276v2 = nil
		} else {
			v276v2 = make(map[bool]uint16, len(v))
		} // reset map
		testUnmarshalErr(v276v2, bs276, h, t, "dec-map-v276")
		testDeepEqualErr(v276v1, v276v2, t, "equal-map-v276")
		if v == nil {
			v276v2 = nil
		} else {
			v276v2 = make(map[bool]uint16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v276v2), bs276, h, t, "dec-map-v276-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v276v1, v276v2, t, "equal-map-v276-noaddr")
		if v == nil {
			v276v2 = nil
		} else {
			v276v2 = make(map[bool]uint16, len(v))
		} // reset map
		testUnmarshalErr(&v276v2, bs276, h, t, "dec-map-v276-p-len")
		testDeepEqualErr(v276v1, v276v2, t, "equal-map-v276-p-len")
		bs276 = testMarshalErr(&v276v1, h, t, "enc-map-v276-p")
		v276v2 = nil
		testUnmarshalErr(&v276v2, bs276, h, t, "dec-map-v276-p-nil")
		testDeepEqualErr(v276v1, v276v2, t, "equal-map-v276-p-nil")
		// ...
		if v == nil {
			v276v2 = nil
		} else {
			v276v2 = make(map[bool]uint16, len(v))
		} // reset map
		var v276v3, v276v4 typMapMapBoolUint16
		v276v3 = typMapMapBoolUint16(v276v1)
		v276v4 = typMapMapBoolUint16(v276v2)
		bs276 = testMarshalErr(v276v3, h, t, "enc-map-v276-custom")
		testUnmarshalErr(v276v4, bs276, h, t, "dec-map-v276-p-len")
		testDeepEqualErr(v276v3, v276v4, t, "equal-map-v276-p-len")
	}

	for _, v := range []map[bool]uint32{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v277: %v\n", v)
		var v277v1, v277v2 map[bool]uint32
		v277v1 = v
		bs277 := testMarshalErr(v277v1, h, t, "enc-map-v277")
		if v == nil {
			v277v2 = nil
		} else {
			v277v2 = make(map[bool]uint32, len(v))
		} // reset map
		testUnmarshalErr(v277v2, bs277, h, t, "dec-map-v277")
		testDeepEqualErr(v277v1, v277v2, t, "equal-map-v277")
		if v == nil {
			v277v2 = nil
		} else {
			v277v2 = make(map[bool]uint32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v277v2), bs277, h, t, "dec-map-v277-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v277v1, v277v2, t, "equal-map-v277-noaddr")
		if v == nil {
			v277v2 = nil
		} else {
			v277v2 = make(map[bool]uint32, len(v))
		} // reset map
		testUnmarshalErr(&v277v2, bs277, h, t, "dec-map-v277-p-len")
		testDeepEqualErr(v277v1, v277v2, t, "equal-map-v277-p-len")
		bs277 = testMarshalErr(&v277v1, h, t, "enc-map-v277-p")
		v277v2 = nil
		testUnmarshalErr(&v277v2, bs277, h, t, "dec-map-v277-p-nil")
		testDeepEqualErr(v277v1, v277v2, t, "equal-map-v277-p-nil")
		// ...
		if v == nil {
			v277v2 = nil
		} else {
			v277v2 = make(map[bool]uint32, len(v))
		} // reset map
		var v277v3, v277v4 typMapMapBoolUint32
		v277v3 = typMapMapBoolUint32(v277v1)
		v277v4 = typMapMapBoolUint32(v277v2)
		bs277 = testMarshalErr(v277v3, h, t, "enc-map-v277-custom")
		testUnmarshalErr(v277v4, bs277, h, t, "dec-map-v277-p-len")
		testDeepEqualErr(v277v3, v277v4, t, "equal-map-v277-p-len")
	}

	for _, v := range []map[bool]uint64{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v278: %v\n", v)
		var v278v1, v278v2 map[bool]uint64
		v278v1 = v
		bs278 := testMarshalErr(v278v1, h, t, "enc-map-v278")
		if v == nil {
			v278v2 = nil
		} else {
			v278v2 = make(map[bool]uint64, len(v))
		} // reset map
		testUnmarshalErr(v278v2, bs278, h, t, "dec-map-v278")
		testDeepEqualErr(v278v1, v278v2, t, "equal-map-v278")
		if v == nil {
			v278v2 = nil
		} else {
			v278v2 = make(map[bool]uint64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v278v2), bs278, h, t, "dec-map-v278-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v278v1, v278v2, t, "equal-map-v278-noaddr")
		if v == nil {
			v278v2 = nil
		} else {
			v278v2 = make(map[bool]uint64, len(v))
		} // reset map
		testUnmarshalErr(&v278v2, bs278, h, t, "dec-map-v278-p-len")
		testDeepEqualErr(v278v1, v278v2, t, "equal-map-v278-p-len")
		bs278 = testMarshalErr(&v278v1, h, t, "enc-map-v278-p")
		v278v2 = nil
		testUnmarshalErr(&v278v2, bs278, h, t, "dec-map-v278-p-nil")
		testDeepEqualErr(v278v1, v278v2, t, "equal-map-v278-p-nil")
		// ...
		if v == nil {
			v278v2 = nil
		} else {
			v278v2 = make(map[bool]uint64, len(v))
		} // reset map
		var v278v3, v278v4 typMapMapBoolUint64
		v278v3 = typMapMapBoolUint64(v278v1)
		v278v4 = typMapMapBoolUint64(v278v2)
		bs278 = testMarshalErr(v278v3, h, t, "enc-map-v278-custom")
		testUnmarshalErr(v278v4, bs278, h, t, "dec-map-v278-p-len")
		testDeepEqualErr(v278v3, v278v4, t, "equal-map-v278-p-len")
	}

	for _, v := range []map[bool]uintptr{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v279: %v\n", v)
		var v279v1, v279v2 map[bool]uintptr
		v279v1 = v
		bs279 := testMarshalErr(v279v1, h, t, "enc-map-v279")
		if v == nil {
			v279v2 = nil
		} else {
			v279v2 = make(map[bool]uintptr, len(v))
		} // reset map
		testUnmarshalErr(v279v2, bs279, h, t, "dec-map-v279")
		testDeepEqualErr(v279v1, v279v2, t, "equal-map-v279")
		if v == nil {
			v279v2 = nil
		} else {
			v279v2 = make(map[bool]uintptr, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v279v2), bs279, h, t, "dec-map-v279-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v279v1, v279v2, t, "equal-map-v279-noaddr")
		if v == nil {
			v279v2 = nil
		} else {
			v279v2 = make(map[bool]uintptr, len(v))
		} // reset map
		testUnmarshalErr(&v279v2, bs279, h, t, "dec-map-v279-p-len")
		testDeepEqualErr(v279v1, v279v2, t, "equal-map-v279-p-len")
		bs279 = testMarshalErr(&v279v1, h, t, "enc-map-v279-p")
		v279v2 = nil
		testUnmarshalErr(&v279v2, bs279, h, t, "dec-map-v279-p-nil")
		testDeepEqualErr(v279v1, v279v2, t, "equal-map-v279-p-nil")
		// ...
		if v == nil {
			v279v2 = nil
		} else {
			v279v2 = make(map[bool]uintptr, len(v))
		} // reset map
		var v279v3, v279v4 typMapMapBoolUintptr
		v279v3 = typMapMapBoolUintptr(v279v1)
		v279v4 = typMapMapBoolUintptr(v279v2)
		bs279 = testMarshalErr(v279v3, h, t, "enc-map-v279-custom")
		testUnmarshalErr(v279v4, bs279, h, t, "dec-map-v279-p-len")
		testDeepEqualErr(v279v3, v279v4, t, "equal-map-v279-p-len")
	}

	for _, v := range []map[bool]int{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v280: %v\n", v)
		var v280v1, v280v2 map[bool]int
		v280v1 = v
		bs280 := testMarshalErr(v280v1, h, t, "enc-map-v280")
		if v == nil {
			v280v2 = nil
		} else {
			v280v2 = make(map[bool]int, len(v))
		} // reset map
		testUnmarshalErr(v280v2, bs280, h, t, "dec-map-v280")
		testDeepEqualErr(v280v1, v280v2, t, "equal-map-v280")
		if v == nil {
			v280v2 = nil
		} else {
			v280v2 = make(map[bool]int, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v280v2), bs280, h, t, "dec-map-v280-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v280v1, v280v2, t, "equal-map-v280-noaddr")
		if v == nil {
			v280v2 = nil
		} else {
			v280v2 = make(map[bool]int, len(v))
		} // reset map
		testUnmarshalErr(&v280v2, bs280, h, t, "dec-map-v280-p-len")
		testDeepEqualErr(v280v1, v280v2, t, "equal-map-v280-p-len")
		bs280 = testMarshalErr(&v280v1, h, t, "enc-map-v280-p")
		v280v2 = nil
		testUnmarshalErr(&v280v2, bs280, h, t, "dec-map-v280-p-nil")
		testDeepEqualErr(v280v1, v280v2, t, "equal-map-v280-p-nil")
		// ...
		if v == nil {
			v280v2 = nil
		} else {
			v280v2 = make(map[bool]int, len(v))
		} // reset map
		var v280v3, v280v4 typMapMapBoolInt
		v280v3 = typMapMapBoolInt(v280v1)
		v280v4 = typMapMapBoolInt(v280v2)
		bs280 = testMarshalErr(v280v3, h, t, "enc-map-v280-custom")
		testUnmarshalErr(v280v4, bs280, h, t, "dec-map-v280-p-len")
		testDeepEqualErr(v280v3, v280v4, t, "equal-map-v280-p-len")
	}

	for _, v := range []map[bool]int8{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v281: %v\n", v)
		var v281v1, v281v2 map[bool]int8
		v281v1 = v
		bs281 := testMarshalErr(v281v1, h, t, "enc-map-v281")
		if v == nil {
			v281v2 = nil
		} else {
			v281v2 = make(map[bool]int8, len(v))
		} // reset map
		testUnmarshalErr(v281v2, bs281, h, t, "dec-map-v281")
		testDeepEqualErr(v281v1, v281v2, t, "equal-map-v281")
		if v == nil {
			v281v2 = nil
		} else {
			v281v2 = make(map[bool]int8, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v281v2), bs281, h, t, "dec-map-v281-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v281v1, v281v2, t, "equal-map-v281-noaddr")
		if v == nil {
			v281v2 = nil
		} else {
			v281v2 = make(map[bool]int8, len(v))
		} // reset map
		testUnmarshalErr(&v281v2, bs281, h, t, "dec-map-v281-p-len")
		testDeepEqualErr(v281v1, v281v2, t, "equal-map-v281-p-len")
		bs281 = testMarshalErr(&v281v1, h, t, "enc-map-v281-p")
		v281v2 = nil
		testUnmarshalErr(&v281v2, bs281, h, t, "dec-map-v281-p-nil")
		testDeepEqualErr(v281v1, v281v2, t, "equal-map-v281-p-nil")
		// ...
		if v == nil {
			v281v2 = nil
		} else {
			v281v2 = make(map[bool]int8, len(v))
		} // reset map
		var v281v3, v281v4 typMapMapBoolInt8
		v281v3 = typMapMapBoolInt8(v281v1)
		v281v4 = typMapMapBoolInt8(v281v2)
		bs281 = testMarshalErr(v281v3, h, t, "enc-map-v281-custom")
		testUnmarshalErr(v281v4, bs281, h, t, "dec-map-v281-p-len")
		testDeepEqualErr(v281v3, v281v4, t, "equal-map-v281-p-len")
	}

	for _, v := range []map[bool]int16{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v282: %v\n", v)
		var v282v1, v282v2 map[bool]int16
		v282v1 = v
		bs282 := testMarshalErr(v282v1, h, t, "enc-map-v282")
		if v == nil {
			v282v2 = nil
		} else {
			v282v2 = make(map[bool]int16, len(v))
		} // reset map
		testUnmarshalErr(v282v2, bs282, h, t, "dec-map-v282")
		testDeepEqualErr(v282v1, v282v2, t, "equal-map-v282")
		if v == nil {
			v282v2 = nil
		} else {
			v282v2 = make(map[bool]int16, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v282v2), bs282, h, t, "dec-map-v282-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v282v1, v282v2, t, "equal-map-v282-noaddr")
		if v == nil {
			v282v2 = nil
		} else {
			v282v2 = make(map[bool]int16, len(v))
		} // reset map
		testUnmarshalErr(&v282v2, bs282, h, t, "dec-map-v282-p-len")
		testDeepEqualErr(v282v1, v282v2, t, "equal-map-v282-p-len")
		bs282 = testMarshalErr(&v282v1, h, t, "enc-map-v282-p")
		v282v2 = nil
		testUnmarshalErr(&v282v2, bs282, h, t, "dec-map-v282-p-nil")
		testDeepEqualErr(v282v1, v282v2, t, "equal-map-v282-p-nil")
		// ...
		if v == nil {
			v282v2 = nil
		} else {
			v282v2 = make(map[bool]int16, len(v))
		} // reset map
		var v282v3, v282v4 typMapMapBoolInt16
		v282v3 = typMapMapBoolInt16(v282v1)
		v282v4 = typMapMapBoolInt16(v282v2)
		bs282 = testMarshalErr(v282v3, h, t, "enc-map-v282-custom")
		testUnmarshalErr(v282v4, bs282, h, t, "dec-map-v282-p-len")
		testDeepEqualErr(v282v3, v282v4, t, "equal-map-v282-p-len")
	}

	for _, v := range []map[bool]int32{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v283: %v\n", v)
		var v283v1, v283v2 map[bool]int32
		v283v1 = v
		bs283 := testMarshalErr(v283v1, h, t, "enc-map-v283")
		if v == nil {
			v283v2 = nil
		} else {
			v283v2 = make(map[bool]int32, len(v))
		} // reset map
		testUnmarshalErr(v283v2, bs283, h, t, "dec-map-v283")
		testDeepEqualErr(v283v1, v283v2, t, "equal-map-v283")
		if v == nil {
			v283v2 = nil
		} else {
			v283v2 = make(map[bool]int32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v283v2), bs283, h, t, "dec-map-v283-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v283v1, v283v2, t, "equal-map-v283-noaddr")
		if v == nil {
			v283v2 = nil
		} else {
			v283v2 = make(map[bool]int32, len(v))
		} // reset map
		testUnmarshalErr(&v283v2, bs283, h, t, "dec-map-v283-p-len")
		testDeepEqualErr(v283v1, v283v2, t, "equal-map-v283-p-len")
		bs283 = testMarshalErr(&v283v1, h, t, "enc-map-v283-p")
		v283v2 = nil
		testUnmarshalErr(&v283v2, bs283, h, t, "dec-map-v283-p-nil")
		testDeepEqualErr(v283v1, v283v2, t, "equal-map-v283-p-nil")
		// ...
		if v == nil {
			v283v2 = nil
		} else {
			v283v2 = make(map[bool]int32, len(v))
		} // reset map
		var v283v3, v283v4 typMapMapBoolInt32
		v283v3 = typMapMapBoolInt32(v283v1)
		v283v4 = typMapMapBoolInt32(v283v2)
		bs283 = testMarshalErr(v283v3, h, t, "enc-map-v283-custom")
		testUnmarshalErr(v283v4, bs283, h, t, "dec-map-v283-p-len")
		testDeepEqualErr(v283v3, v283v4, t, "equal-map-v283-p-len")
	}

	for _, v := range []map[bool]int64{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v284: %v\n", v)
		var v284v1, v284v2 map[bool]int64
		v284v1 = v
		bs284 := testMarshalErr(v284v1, h, t, "enc-map-v284")
		if v == nil {
			v284v2 = nil
		} else {
			v284v2 = make(map[bool]int64, len(v))
		} // reset map
		testUnmarshalErr(v284v2, bs284, h, t, "dec-map-v284")
		testDeepEqualErr(v284v1, v284v2, t, "equal-map-v284")
		if v == nil {
			v284v2 = nil
		} else {
			v284v2 = make(map[bool]int64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v284v2), bs284, h, t, "dec-map-v284-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v284v1, v284v2, t, "equal-map-v284-noaddr")
		if v == nil {
			v284v2 = nil
		} else {
			v284v2 = make(map[bool]int64, len(v))
		} // reset map
		testUnmarshalErr(&v284v2, bs284, h, t, "dec-map-v284-p-len")
		testDeepEqualErr(v284v1, v284v2, t, "equal-map-v284-p-len")
		bs284 = testMarshalErr(&v284v1, h, t, "enc-map-v284-p")
		v284v2 = nil
		testUnmarshalErr(&v284v2, bs284, h, t, "dec-map-v284-p-nil")
		testDeepEqualErr(v284v1, v284v2, t, "equal-map-v284-p-nil")
		// ...
		if v == nil {
			v284v2 = nil
		} else {
			v284v2 = make(map[bool]int64, len(v))
		} // reset map
		var v284v3, v284v4 typMapMapBoolInt64
		v284v3 = typMapMapBoolInt64(v284v1)
		v284v4 = typMapMapBoolInt64(v284v2)
		bs284 = testMarshalErr(v284v3, h, t, "enc-map-v284-custom")
		testUnmarshalErr(v284v4, bs284, h, t, "dec-map-v284-p-len")
		testDeepEqualErr(v284v3, v284v4, t, "equal-map-v284-p-len")
	}

	for _, v := range []map[bool]float32{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v285: %v\n", v)
		var v285v1, v285v2 map[bool]float32
		v285v1 = v
		bs285 := testMarshalErr(v285v1, h, t, "enc-map-v285")
		if v == nil {
			v285v2 = nil
		} else {
			v285v2 = make(map[bool]float32, len(v))
		} // reset map
		testUnmarshalErr(v285v2, bs285, h, t, "dec-map-v285")
		testDeepEqualErr(v285v1, v285v2, t, "equal-map-v285")
		if v == nil {
			v285v2 = nil
		} else {
			v285v2 = make(map[bool]float32, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v285v2), bs285, h, t, "dec-map-v285-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v285v1, v285v2, t, "equal-map-v285-noaddr")
		if v == nil {
			v285v2 = nil
		} else {
			v285v2 = make(map[bool]float32, len(v))
		} // reset map
		testUnmarshalErr(&v285v2, bs285, h, t, "dec-map-v285-p-len")
		testDeepEqualErr(v285v1, v285v2, t, "equal-map-v285-p-len")
		bs285 = testMarshalErr(&v285v1, h, t, "enc-map-v285-p")
		v285v2 = nil
		testUnmarshalErr(&v285v2, bs285, h, t, "dec-map-v285-p-nil")
		testDeepEqualErr(v285v1, v285v2, t, "equal-map-v285-p-nil")
		// ...
		if v == nil {
			v285v2 = nil
		} else {
			v285v2 = make(map[bool]float32, len(v))
		} // reset map
		var v285v3, v285v4 typMapMapBoolFloat32
		v285v3 = typMapMapBoolFloat32(v285v1)
		v285v4 = typMapMapBoolFloat32(v285v2)
		bs285 = testMarshalErr(v285v3, h, t, "enc-map-v285-custom")
		testUnmarshalErr(v285v4, bs285, h, t, "dec-map-v285-p-len")
		testDeepEqualErr(v285v3, v285v4, t, "equal-map-v285-p-len")
	}

	for _, v := range []map[bool]float64{nil, {}, {true: 0}} {
		// fmt.Printf(">>>> running mammoth map v286: %v\n", v)
		var v286v1, v286v2 map[bool]float64
		v286v1 = v
		bs286 := testMarshalErr(v286v1, h, t, "enc-map-v286")
		if v == nil {
			v286v2 = nil
		} else {
			v286v2 = make(map[bool]float64, len(v))
		} // reset map
		testUnmarshalErr(v286v2, bs286, h, t, "dec-map-v286")
		testDeepEqualErr(v286v1, v286v2, t, "equal-map-v286")
		if v == nil {
			v286v2 = nil
		} else {
			v286v2 = make(map[bool]float64, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v286v2), bs286, h, t, "dec-map-v286-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v286v1, v286v2, t, "equal-map-v286-noaddr")
		if v == nil {
			v286v2 = nil
		} else {
			v286v2 = make(map[bool]float64, len(v))
		} // reset map
		testUnmarshalErr(&v286v2, bs286, h, t, "dec-map-v286-p-len")
		testDeepEqualErr(v286v1, v286v2, t, "equal-map-v286-p-len")
		bs286 = testMarshalErr(&v286v1, h, t, "enc-map-v286-p")
		v286v2 = nil
		testUnmarshalErr(&v286v2, bs286, h, t, "dec-map-v286-p-nil")
		testDeepEqualErr(v286v1, v286v2, t, "equal-map-v286-p-nil")
		// ...
		if v == nil {
			v286v2 = nil
		} else {
			v286v2 = make(map[bool]float64, len(v))
		} // reset map
		var v286v3, v286v4 typMapMapBoolFloat64
		v286v3 = typMapMapBoolFloat64(v286v1)
		v286v4 = typMapMapBoolFloat64(v286v2)
		bs286 = testMarshalErr(v286v3, h, t, "enc-map-v286-custom")
		testUnmarshalErr(v286v4, bs286, h, t, "dec-map-v286-p-len")
		testDeepEqualErr(v286v3, v286v4, t, "equal-map-v286-p-len")
	}

	for _, v := range []map[bool]bool{nil, {}, {true: false}} {
		// fmt.Printf(">>>> running mammoth map v287: %v\n", v)
		var v287v1, v287v2 map[bool]bool
		v287v1 = v
		bs287 := testMarshalErr(v287v1, h, t, "enc-map-v287")
		if v == nil {
			v287v2 = nil
		} else {
			v287v2 = make(map[bool]bool, len(v))
		} // reset map
		testUnmarshalErr(v287v2, bs287, h, t, "dec-map-v287")
		testDeepEqualErr(v287v1, v287v2, t, "equal-map-v287")
		if v == nil {
			v287v2 = nil
		} else {
			v287v2 = make(map[bool]bool, len(v))
		} // reset map
		testUnmarshalErr(reflect.ValueOf(v287v2), bs287, h, t, "dec-map-v287-noaddr") // decode into non-addressable map value
		testDeepEqualErr(v287v1, v287v2, t, "equal-map-v287-noaddr")
		if v == nil {
			v287v2 = nil
		} else {
			v287v2 = make(map[bool]bool, len(v))
		} // reset map
		testUnmarshalErr(&v287v2, bs287, h, t, "dec-map-v287-p-len")
		testDeepEqualErr(v287v1, v287v2, t, "equal-map-v287-p-len")
		bs287 = testMarshalErr(&v287v1, h, t, "enc-map-v287-p")
		v287v2 = nil
		testUnmarshalErr(&v287v2, bs287, h, t, "dec-map-v287-p-nil")
		testDeepEqualErr(v287v1, v287v2, t, "equal-map-v287-p-nil")
		// ...
		if v == nil {
			v287v2 = nil
		} else {
			v287v2 = make(map[bool]bool, len(v))
		} // reset map
		var v287v3, v287v4 typMapMapBoolBool
		v287v3 = typMapMapBoolBool(v287v1)
		v287v4 = typMapMapBoolBool(v287v2)
		bs287 = testMarshalErr(v287v3, h, t, "enc-map-v287-custom")
		testUnmarshalErr(v287v4, bs287, h, t, "dec-map-v287-p-len")
		testDeepEqualErr(v287v3, v287v4, t, "equal-map-v287-p-len")
	}

}

func doTestMammothMapsAndSlices(t *testing.T, h Handle) {
	doTestMammothSlices(t, h)
	doTestMammothMaps(t, h)
}
