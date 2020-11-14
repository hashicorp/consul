package proto

import (
	"reflect"
	"time"

	"github.com/gogo/protobuf/types"
)

var (
	tsType      = reflect.TypeOf((*types.Timestamp)(nil))
	timePtrType = reflect.TypeOf((*time.Time)(nil))
	timeType    = timePtrType.Elem()
	mapStrInf   = reflect.TypeOf((map[string]interface{})(nil))

	epoch1970 = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
)

// HookPBTimestampToTime is a mapstructure decode hook to translate a protobuf timestamp
// to a time.Time value
func HookPBTimestampToTime(from, to reflect.Type, data interface{}) (interface{}, error) {
	if to == timeType && from == tsType {
		ts := data.(*types.Timestamp)
		if ts.Seconds == 0 && ts.Nanos == 0 {
			return time.Time{}, nil
		}
		return time.Unix(ts.Seconds, int64(ts.Nanos)).UTC(), nil
	}

	return data, nil
}

// HookTimeToPBtimestamp is a mapstructure decode hook to translate a time.Time value to
// a protobuf Timestamp value.
func HookTimeToPBTimestamp(from, to reflect.Type, data interface{}) (interface{}, error) {
	// Note that mapstructure doesn't do direct struct to struct conversion in this case. I
	// still don't completely understand why converting the PB TS to time.Time does but
	// I suspect it has something to do with the struct containing a concrete time.Time
	// as opposed to a pointer to a time.Time. Regardless this path through mapstructure
	// first will decode the concrete time.Time into a map[string]interface{} before
	// eventually decoding that map[string]interface{} into the *types.Timestamp. One
	// other note is that mapstructure ends up creating a new Value and sets it it to
	// the time.Time value and thats what gets passed to us. That is why we end up
	// seeing a *time.Time instead of a time.Time.
	if from == timePtrType && to == mapStrInf {
		ts := data.(*time.Time)

		// protobuf only supports times from Jan 1 1970 onward but the time.Time type
		// can represent values back to year 1. Basically
		if ts.Before(epoch1970) {
			return map[string]interface{}{}, nil
		}

		nanos := ts.UnixNano()
		if nanos < 0 {
			return map[string]interface{}{}, nil
		}

		seconds := nanos / 1000000000
		nanos = nanos % 1000000000

		return map[string]interface{}{
			"Seconds": seconds,
			"Nanos":   int32(nanos),
		}, nil
	}
	return data, nil
}
