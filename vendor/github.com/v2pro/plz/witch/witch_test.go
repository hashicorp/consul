package witch

import (
	"github.com/v2pro/plz/countlog"
	"math/rand"
	"testing"
	"time"
	"github.com/v2pro/plz/dump"
	"expvar"
	"unsafe"
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
	overflow    *[]unsafe.Pointer
	oldoverflow *[]unsafe.Pointer

	// nextOverflow holds a pointer to a free overflow bucket.
	nextOverflow unsafe.Pointer
}

func init() {
	m := map[int]int{}
	m[1] = 1
	m[2] = 4
	m[3] = 9
	expvar.Publish("map before delete", dump.Snapshot(m))
	delete(m, 2)
	expvar.Publish("map after delete", dump.Snapshot(m))
	//hm := (*hmap)(reflect2.PtrOf(m))
	//for i := 1; i < 30; i++ {
	//	m[i] = i * i
	//	expvar.Publish(fmt.Sprintf("map%v", i), dump.Snapshot(m))
	//	if hm.oldbuckets != nil {
	//		fmt.Println("!!!!", i)
	//		break
	//	}
	//}
}

func Test_witch(t *testing.T) {
	fakeValues := []string{"tom", "jerry", "william", "lily"}
	Start("192.168.3.33:8318")
	go func() {
		defer func() {
			recovered := recover()
			countlog.LogPanic(recovered)
		}()
		for {
			response := []byte{}
			for i := int32(0); i < rand.Int31n(1024*256); i++ {
				response = append(response, fakeValues[rand.Int31n(4)]...)
			}
			//countlog.Debug("event!hello", "user", fakeValues[rand.Int31n(4)],
			//	"response", string(response))
			time.Sleep(time.Millisecond * 500)
		}
	}()
	time.Sleep(time.Hour)
}
