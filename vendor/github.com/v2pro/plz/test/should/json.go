package should

import (
	"encoding/json"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/test/testify/assert"
	"runtime"
	"reflect"
)

func JsonEqual(expected string, actual interface{}) {
	t := test.CurrentT()
	test.Helper()
	var expectedObj interface{}
	err := json.Unmarshal([]byte(expected), &expectedObj)
	if err != nil {
		t.Error("expected json is invalid: " + err.Error())
		return
	}
	var actualJson []byte
	switch actualVal := actual.(type) {
	case string:
		actualJson = []byte(actualVal)
	case []byte:
		actualJson = actualVal
	default:
		actualJson, err = json.Marshal(actual)
		t.Error("actual can not marshal to json: " + err.Error())
		return
	}
	var actualObj interface{}
	err = json.Unmarshal(actualJson, &actualObj)
	if err != nil {
		t.Log(string(actualJson))
		t.Error("actual json is invalid: " + err.Error())
		return
	}
	maskAnything(expectedObj, actualObj)
	if assert.Equal(t, expectedObj, actualObj) {
		return
	}
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Error("check failed")
		return
	}
	t.Log(string(actualJson))
	t.Error(test.ExtractFailedLines(file, line))
}

func maskAnything(expected interface{}, actual interface{}) {
	switch reflect.TypeOf(expected).Kind() {
	case reflect.Map:
		if reflect.ValueOf(actual).Kind() != reflect.Map {
			return
		}
		expectedVal := reflect.ValueOf(expected)
		actualVal := reflect.ValueOf(actual)
		keys := expectedVal.MapKeys()
		for _, key := range keys {
			elem := expectedVal.MapIndex(key).Interface()
			if elem == "ANYTHING" {
				actualVal.SetMapIndex(key, reflect.ValueOf("ANYTHING"))
				continue
			}
			maskAnything(elem, actualVal.MapIndex(key).Interface())
		}
	}
}
