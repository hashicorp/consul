package stats

import (
	"github.com/v2pro/plz/countlog/spi"
	"unsafe"
	"strings"
	"reflect"
	"sync"
	"github.com/v2pro/plz/gls"
	"time"
	"context"
)

type Window struct {
	collector          Collector
	event              string
	dimensionElemCount int
	shards             [16]windowShard
}

type windowShard struct {
	MapMonoid
	lock *sync.Mutex
}

func newWindow(executor Executor, collector Collector, dimensionElemCount int) *Window {
	window := &Window{
		collector:          collector,
		dimensionElemCount: dimensionElemCount,
	}
	for i := 0; i < 16; i++ {
		window.shards[i] = windowShard{
			MapMonoid: MapMonoid{},
			lock:      &sync.Mutex{},
		}
	}
	executor(window.exportEverySecond)
	return window
}

func (window *Window) exportEverySecond(ctx context.Context) {
	timer := time.NewTimer(time.Second)
	for {
		select {
		case <-timer.C:
			window.Export(time.Now())
		case <-ctx.Done():
			return
		}
	}
}

func (window *Window) Mutate() (*sync.Mutex, MapMonoid) {
	shardId := gls.GoID() % 16
	shard := window.shards[shardId]
	return shard.lock, shard.MapMonoid
}

func (window *Window) Export(now time.Time) {
	for i := 0; i < 16; i++ {
		window.exportShard(now, window.shards[i])
	}
}

func (window *Window) exportShard(now time.Time, shard windowShard) {
	shard.lock.Lock()
	defer shard.lock.Unlock()
	// batch allocate memory to hold dimensions
	space := make([]string, len(shard.MapMonoid)*window.dimensionElemCount)
	for dimensionObj, monoid := range shard.MapMonoid {
		dimension := space[:window.dimensionElemCount]
		space = space[window.dimensionElemCount:]
		slice := &sliceHeader{
			Cap:  window.dimensionElemCount,
			Len:  window.dimensionElemCount,
			Data: (*emptyInterface)(unsafe.Pointer(&dimensionObj)).word,
		}
		copy(dimension, *(*[]string)(unsafe.Pointer(slice)))
		window.collector.Collect(&Point{
			Event:     window.event,
			Timestamp: now,
			Dimension: dimension,
			Value:     monoid.Export(),
		})
	}
}

type propIdx int

type dimensionExtractor interface {
	Extract(event *spi.Event, monoid MapMonoid, createElem func() Monoid) Monoid
}

func newDimensionExtractor(site *spi.LogSite) (dimensionExtractor, int) {
	var dimensionElems []string
	for i := 0; i < len(site.Sample); i += 2 {
		key := site.Sample[i].(string)
		if key == "dim" {
			dimensionElems = strings.Split(site.Sample[i+1].(string), ",")
		}
	}
	indices := make([]propIdx, 0, len(dimensionElems))
	for i := 0; i < len(site.Sample); i += 2 {
		key := site.Sample[i].(string)
		for _, dimension := range dimensionElems {
			if key == dimension {
				indices = append(indices, propIdx(i))
				indices = append(indices, propIdx(i+1))
			}
		}
	}
	arrayType := reflect.ArrayOf(len(indices), reflect.TypeOf(""))
	arrayObj := reflect.New(arrayType).Elem().Interface()
	sampleInterface := *(*emptyInterface)(unsafe.Pointer(&arrayObj))
	if len(indices) == 0 {
		return &dimensionExtractor0{}, 0
	}
	if len(indices) <= 2 {
		return &dimensionExtractor2{
			sampleInterface: sampleInterface,
			indices:         indices,
		}, len(indices)
	}
	if len(indices) <= 4 {
		return &dimensionExtractor4{
			sampleInterface: sampleInterface,
			indices:         indices,
		}, len(indices)
	}
	if len(indices) <= 8 {
		return &dimensionExtractor8{
			sampleInterface: sampleInterface,
			indices:         indices,
		}, len(indices)
	}
	return &dimensionExtractorAny{
		sampleInterface: sampleInterface,
		indices:         indices,
	}, len(indices)
}

type dimensionExtractor0 struct {
}

func (extractor *dimensionExtractor0) Extract(event *spi.Event, monoid MapMonoid, createElem func() Monoid) Monoid {
	elem := monoid[0]
	if elem == nil {
		elem = createElem()
		monoid[0] = elem
	}
	return elem
}

type dimensionExtractor2 struct {
	sampleInterface emptyInterface
	indices         []propIdx
}

func (extractor *dimensionExtractor2) Extract(event *spi.Event, monoid MapMonoid, createElem func() Monoid) Monoid {
	dimensionArr := [2]string{}
	dimension := dimensionArr[:len(extractor.indices)]
	return extractDimension(extractor.sampleInterface, dimension,
		extractor.indices, event, monoid, createElem)
}

type dimensionExtractor4 struct {
	sampleInterface emptyInterface
	indices         []propIdx
}

func (extractor *dimensionExtractor4) Extract(event *spi.Event, monoid MapMonoid, createElem func() Monoid) Monoid {
	dimensionArr := [4]string{}
	dimension := dimensionArr[:len(extractor.indices)]
	return extractDimension(extractor.sampleInterface, dimension,
		extractor.indices, event, monoid, createElem)
}

type dimensionExtractor8 struct {
	sampleInterface emptyInterface
	indices         []propIdx
}

func (extractor *dimensionExtractor8) Extract(event *spi.Event, monoid MapMonoid, createElem func() Monoid) Monoid {
	dimensionArr := [8]string{}
	dimension := dimensionArr[:len(extractor.indices)]
	return extractDimension(extractor.sampleInterface, dimension,
		extractor.indices, event, monoid, createElem)
}

type dimensionExtractorAny struct {
	sampleInterface emptyInterface
	indices         []propIdx
}

func (extractor *dimensionExtractorAny) Extract(event *spi.Event, monoid MapMonoid, createElem func() Monoid) Monoid {
	dimension := make([]string, len(extractor.indices))
	return extractDimension(extractor.sampleInterface, dimension,
		extractor.indices, event, monoid, createElem)
}

func extractDimension(
	sampleInterface emptyInterface, dimension []string, indices []propIdx,
	event *spi.Event, monoid MapMonoid, createElem func() Monoid) Monoid {
	for i, idx := range indices {
		dimension[i] = event.Properties[idx].(string)
	}
	dimensionInterface := sampleInterface
	dimensionInterface.word = unsafe.Pointer(&dimension[0])
	dimensionObj := castEmptyInterface(uintptr(unsafe.Pointer(&dimensionInterface)))
	elem := monoid[dimensionObj]
	if elem == nil {
		elem = createElem()
		monoid[copyDimension(dimensionInterface, dimension)] = elem
	}
	return elem
}

func copyDimension(dimensionInterface emptyInterface, dimension []string) interface{} {
	copied := make([]string, len(dimension))
	for i, elem := range dimension {
		copyOfElem := string(append([]byte(nil), elem...))
		copied[i] = copyOfElem
	}
	dimensionInterface.word = unsafe.Pointer(&copied[0])
	return *(*interface{})(unsafe.Pointer(&dimensionInterface))
}

func castEmptyInterface(ptr uintptr) interface{} {
	return *(*interface{})(unsafe.Pointer(ptr))
}

// emptyInterface is the header for an interface{} value.
type emptyInterface struct {
	typ  unsafe.Pointer
	word unsafe.Pointer
}

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}
