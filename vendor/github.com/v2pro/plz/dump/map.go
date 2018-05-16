package dump

import (
	"context"
	"unsafe"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"math"
	"reflect"
	"encoding/json"
)

// A header for a Go map.
type hmap struct {
	count     int // # live cells == size of map.  Must be first (used by len() builtin)
	flags     uint8
	B         uint8  // log_2 of # of buckets (can hold up to loadFactor * 2^B items)
	noverflow uint16 // approximate number of overflow buckets; see incrnoverflow for details
	hash0     uint32 // hash seed

	buckets    unsafe.Pointer // array of 2^B Buckets. may be nil if count==0.
	oldbuckets unsafe.Pointer // previous bucket array of half the size, non-nil only when growing
	nevacuate  uintptr        // progress counter for evacuation (buckets less than this have been evacuated)

	extra *mapextra // optional fields
}

const bucketCntBits = 3
const bucketCnt = 1 << bucketCntBits

// A bucket for a Go map.
type bmap struct {
	// tophash generally contains the top byte of the hash value
	// for each key in this bucket. If tophash[0] < minTopHash,
	// tophash[0] is a bucket evacuation state instead.
	tophash [bucketCnt]uint8
	// Followed by bucketCnt keys and then bucketCnt values.
	// NOTE: packing all the keys together and then all the values together makes the
	// code a bit more complicated than alternating key/value/key/value/... but it allows
	// us to eliminate padding which would be needed for, e.g., map[int64]int8.
	// Followed by an overflow pointer.
}

// mapextra holds fields that are not present on all maps.
type mapextra struct {
	// If both key and value do not contain pointers and are inline, then we mark bucket
	// type as containing no pointers. This avoids scanning such maps.
	// However, bmap.overflow is a pointer. In order to keep overflow buckets
	// alive, we store pointers to all overflow buckets in hmap.overflow and h.map.oldoverflow.
	// overflow and oldoverflow are only used if key and value do not contain pointers.
	// overflow contains overflow buckets for hmap.buckets.
	// oldoverflow contains overflow buckets for hmap.oldbuckets.
	// The indirection allows to store a pointer to the slice in hiter.
	overflow    *[]*bmap
	oldoverflow *[]*bmap

	// nextOverflow holds a pointer to a free overflow bucket.
	nextOverflow *bmap
}

var topHashEncoder = jsonfmt.EncoderOf(reflect.ArrayOf(bucketCnt, reflect.TypeOf(uint8(0))))

type mapEncoder struct {
	bucketSize   uintptr
	keysSize     uintptr
	keysEncoder  jsonfmt.Encoder
	elemsEncoder jsonfmt.Encoder
}

func newMapEncoder(api jsonfmt.API, valType reflect.Type) *mapEncoder {
	keysEncoder := api.EncoderOf(reflect.ArrayOf(bucketCnt, valType.Key()))
	elemsEncoder := api.EncoderOf(reflect.ArrayOf(bucketCnt, valType.Elem()))
	keysSize := valType.Key().Size() * bucketCnt
	elemsSize := valType.Elem().Size() * bucketCnt
	return &mapEncoder{
		bucketSize:   bucketCnt + keysSize + elemsSize + 8,
		keysSize:     keysSize,
		keysEncoder:  keysEncoder,
		elemsEncoder: elemsEncoder,
	}
}

func (encoder *mapEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	hmap := (*hmap)(ptr)
	space = append(space, `{"count":`...)
	space = jsonfmt.WriteInt64(space, int64(hmap.count))
	space = append(space, `,"flags":`...)
	space = jsonfmt.WriteUint8(space, hmap.flags)
	space = append(space, `,"B":`...)
	space = jsonfmt.WriteUint8(space, hmap.B)
	space = append(space, `,"noverflow":`...)
	space = jsonfmt.WriteUint16(space, hmap.noverflow)
	space = append(space, `,"hash0":`...)
	space = jsonfmt.WriteUint32(space, hmap.hash0)
	space = append(space, `,"buckets":{"__ptr__":"`...)
	bucketsPtr := ptrToStr(uintptr(hmap.buckets))
	space = append(space, bucketsPtr...)
	space = append(space, `"},"oldbuckets":{"__ptr__":"`...)
	oldbucketsPtr := ptrToStr(uintptr(hmap.oldbuckets))
	space = append(space, oldbucketsPtr...)
	space = append(space, `"},"nevacuate":`...)
	space = jsonfmt.WriteUint64(space, uint64(hmap.nevacuate))
	space = append(space, `,"extra":{"__ptr__":"`...)
	extraPtr := ptrToStr(uintptr(unsafe.Pointer(hmap.extra)))
	space = append(space, extraPtr...)
	space = append(space, `"}}`...)
	bucketsCount := int(math.Pow(2, float64(hmap.B)))
	if hmap.buckets != nil {
		bucketsData := encoder.encodeBuckets(ctx, nil, bucketsCount, hmap.buckets)
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		addrMap[bucketsPtr] = json.RawMessage(bucketsData)
	}
	if hmap.oldbuckets != nil {
		oldbucketsData := encoder.encodeBuckets(ctx, nil, bucketsCount / 2, hmap.oldbuckets)
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		addrMap[oldbucketsPtr] = json.RawMessage(oldbucketsData)
	}
	if hmap.extra != nil {
		extraData := encoder.encodeExtra(ctx, nil, unsafe.Pointer(hmap.extra))
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		addrMap[extraPtr] = json.RawMessage(extraData)
	}
	return space
}

func (encoder *mapEncoder) encodeExtra(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	extra := (*mapextra)(ptr)
	space = append(space, `{"overflow":{"__ptr__":"`...)
	overflowPtr := ptrToStr(uintptr(unsafe.Pointer(extra.overflow)))
	space = append(space, overflowPtr...)
	space = append(space, `"},"oldoverflow":{"__ptr__":"`...)
	oldoverflowPtr := ptrToStr(uintptr(unsafe.Pointer(extra.oldoverflow)))
	space = append(space, oldoverflowPtr...)
	space = append(space, `"},"nextOverflow":{"__ptr__":"`...)
	nextOverflowPtr := ptrToStr(uintptr(unsafe.Pointer(extra.nextOverflow)))
	space = append(space, nextOverflowPtr...)
	space = append(space, `"}}`...)
	if extra.overflow != nil {
		overflowData := encoder.encodeBucketPtrSlice(ctx, nil, unsafe.Pointer(extra.overflow))
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		addrMap[overflowPtr] = overflowData
	}
	if extra.oldoverflow != nil {
		oldoverflowData := encoder.encodeBucketPtrSlice(ctx, nil, unsafe.Pointer(extra.oldoverflow))
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		addrMap[oldoverflowPtr] = oldoverflowData
	}
	if extra.nextOverflow != nil {
		nextOverflowData := encoder.encodeBucket(ctx, nil, unsafe.Pointer(extra.nextOverflow))
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		addrMap[nextOverflowPtr] = nextOverflowData
	}
	return space
}

// []*bmap
func (encoder *mapEncoder) encodeBucketPtrSlice(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	header := (*sliceHeader)(ptr)
	space = append(space, `{"data":{"__ptr__":"`...)
	ptrStr := ptrToStr(uintptr(header.data))
	space = append(space, ptrStr...)
	space = append(space, `"},"len":`...)
	space = jsonfmt.WriteInt64(space, int64(header.len))
	space = append(space, `,"cap":`...)
	space = jsonfmt.WriteInt64(space, int64(header.cap))
	space = append(space, `}`...)
	data := encoder.encodeBucketPtrArray(ctx, nil, header.len, header.data)
	if uintptr(header.data) != 0 {
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		addrMap[ptrStr] = json.RawMessage(data)
	}
	return space
}

// [N]*bmap
func (encoder *mapEncoder) encodeBucketPtrArray(ctx context.Context, space []byte, count int, ptr unsafe.Pointer) []byte {
	cursor := uintptr(ptr)
	addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
	space = append(space, '[')
	for i := 0; i < count; i++ {
		if i != 0 {
			space = append(space, ',', ' ')
		}
		elemPtr := *(*unsafe.Pointer)(unsafe.Pointer(cursor + 8 * uintptr(i)))
		space = append(space, `{"__ptr__":"`...)
		elemPtrStr := ptrToStr(uintptr(elemPtr))
		space = append(space, elemPtrStr...)
		space = append(space, `"}`...)
		if elemPtr != nil {
			elemData := encoder.encodeBucket(ctx, nil, elemPtr)
			addrMap[elemPtrStr] = json.RawMessage(elemData)
		}
	}
	space = append(space, ']')
	return space
}

func (encoder *mapEncoder) encodeBuckets(ctx context.Context, space []byte, count int, ptr unsafe.Pointer) []byte {
	space = append(space, '[')
	cursor := uintptr(ptr)
	for i := 0; i < count; i++ {
		if i != 0 {
			space = append(space, ',', ' ')
		}
		space = encoder.encodeBucket(ctx, space, unsafe.Pointer(cursor))
		cursor += encoder.bucketSize
	}
	space = append(space, ']')
	return space
}

func (encoder *mapEncoder) encodeBucket(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	bmap := (*bmap)(ptr)
	space = append(space, `{"tophash":`...)
	space = topHashEncoder.Encode(ctx, space, reflect2.PtrOf(bmap.tophash))
	space = append(space, `,"keys":`...)
	keysPtr := uintptr(ptr) + bucketCnt
	space = encoder.keysEncoder.Encode(ctx, space, unsafe.Pointer(keysPtr))
	space = append(space, `,"elems":`...)
	space = encoder.elemsEncoder.Encode(ctx, space, unsafe.Pointer(keysPtr+encoder.keysSize))
	space = append(space, `,"overflow":{"__ptr__":"`...)
	overflowPtr := *(*uintptr)(unsafe.Pointer(uintptr(ptr) + encoder.bucketSize - 8))
	overflowPtrStr := ptrToStr(overflowPtr)
	space = append(space, overflowPtrStr...)
	space = append(space, `"}}`...)
	if overflowPtr != 0 {
		addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
		overflow := encoder.encodeBucket(ctx, nil, unsafe.Pointer(overflowPtr))
		addrMap[overflowPtrStr] = json.RawMessage(overflow)
	}
	return space
}
