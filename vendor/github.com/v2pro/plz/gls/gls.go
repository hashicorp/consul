package gls

import "sync"

type globalMapType map[int64]map[interface{}]interface{}

const shardsCount = 16

var globalLocks []*sync.RWMutex
var globalMaps []globalMapType

type copiable interface {
	Copy() interface{}
}

func init() {
	globalMaps = make([]globalMapType, shardsCount)
	globalLocks = make([]*sync.RWMutex, shardsCount)
	for i := 0; i < shardsCount; i++ {
		globalMaps[i] = make(globalMapType)
		globalLocks[i] = &sync.RWMutex{}
	}
}

func ResetGls(goid int64, initialValue map[interface{}]interface{}) {
	shardIndex := goid % shardsCount
	lock := globalLocks[shardIndex]
	lock.Lock()
	globalMaps[shardIndex][goid] = initialValue
	lock.Unlock()
}

func DeleteGls(goid int64) {
	shardIndex := goid % shardsCount
	lock := globalLocks[shardIndex]
	lock.Lock()
	delete(globalMaps[shardIndex], goid)
	lock.Unlock()
}

func GetGls(goid int64) map[interface{}]interface{} {
	shardIndex := goid % shardsCount
	lock := globalLocks[shardIndex]
	lock.RLock()
	gls, found := globalMaps[shardIndex][goid]
	lock.RUnlock()
	if found {
		return gls
	} else {
		return nil
	}
}

func WithGls(f func()) func() {
	parentGls := GetGls(GoID())
	// parentGls can not be used in other goroutine, otherwise not thread safe
	// make a deep for child goroutine
	childGls := map[interface{}]interface{}{}
	for k, v := range parentGls {
		asCopiable, ok := v.(copiable)
		if ok {
			childGls[k] = asCopiable.Copy()
		} else {
			childGls[k] = v
		}
	}
	return func() {
		goid := GoID()
		ResetGls(goid, childGls)
		defer DeleteGls(goid)
		f()
	}
}

func WithEmptyGls(f func()) func() {
	// do not inherit from parent gls
	return func() {
		goid := GoID()
		ResetGls(goid, make(map[interface{}]interface{}))
		defer DeleteGls(goid)
		f()
	}
}

func Get(key interface{}) interface{} {
	glsMap := GetGls(GoID())
	if glsMap == nil {
		return nil
	}
	return glsMap[key]
}

func Set(key interface{}, value interface{}) {
	glsMap := GetGls(GoID())
	if glsMap == nil {
		panic("gls not enabled for this goroutine")
	}
	glsMap[key] = value
}

func IsGlsEnabled(goid int64) bool {
	return GetGls(goid) != nil
}

