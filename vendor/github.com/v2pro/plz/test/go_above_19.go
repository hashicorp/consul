//+build go1.9

package test

import (
	"github.com/v2pro/plz/reflect2"
	"runtime"
	"testing"
	"sync"
)

var helpersField reflect2.StructField
var muField reflect2.StructField

func init() {
	testingType := reflect2.TypeOfPtr((*testing.T)(nil)).Elem()
	structType := testingType.(reflect2.StructType)
	helpersField = structType.FieldByName("helpers")
	muField = structType.FieldByName("mu")
}

func Helper() {
	t := CurrentT()
	t.Helper()
	mu := muField.Get(t).(*sync.RWMutex)
	mu.Lock()
	defer mu.Unlock()
	helpers := *helpersField.Get(t).(*map[string]struct{})
	helpers[callerName(1)] = struct{}{}
}

// callerName gives the function name (qualified with a package path)
// for the caller after skip frames (where 0 means the current function).
func callerName(skip int) string {
	// Make room for the skip PC.
	var pc [2]uintptr
	n := runtime.Callers(skip+2, pc[:]) // skip + runtime.Callers + callerName
	if n == 0 {
		panic("testing: zero callers found")
	}
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame.Function
}
