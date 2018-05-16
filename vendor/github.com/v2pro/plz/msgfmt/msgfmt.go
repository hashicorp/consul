package msgfmt

import (
	"io"
	"os"
	"sync"
	"unsafe"
	"fmt"
	"github.com/v2pro/plz/concurrent"
	"github.com/v2pro/plz/reflect2"
)

var bufPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 128)
	},
}

func Sprintf(format string, kvObj ...interface{}) string {
	ptr := reflect2.NoEscape(unsafe.Pointer(&kvObj))
	kv := *(*[]interface{})(ptr)
	buf := FormatterOf(format, kv).Format(nil, kv)
	return string(buf)
}

func Println(valuesObj ...interface{}) (int, error) {
	ptr := reflect2.NoEscape(unsafe.Pointer(&valuesObj))
	values := *(*[]interface{})(ptr)
	return fprintln(os.Stdout, values)
}

func Fprintln(writer io.Writer, valuesObj ...interface{}) (int, error) {
	ptr := reflect2.NoEscape(unsafe.Pointer(&valuesObj))
	values := *(*[]interface{})(ptr)
	return fprintln(writer, values)
}

func fprintln(writer io.Writer, values []interface{}) (int, error) {
	switch len(values) {
	case 0:
		return fmt.Println()
	case 1:
		return Fprintf(writer,"{single_value}\n", "single_value", values[0])
	default:
		return Fprintf(writer, "{multiple_values}\n", "multiple_values", values)
	}
}

func Printf(format string, kvObj ...interface{}) (int, error) {
	ptr := reflect2.NoEscape(unsafe.Pointer(&kvObj))
	kv := *(*[]interface{})(ptr)
	return fprintf(os.Stdout, format, kv)
}

func Fprintf(writer io.Writer, format string, kvObj ...interface{}) (int, error) {
	ptr := reflect2.NoEscape(unsafe.Pointer(&kvObj))
	kv := *(*[]interface{})(ptr)
	return fprintf(writer, format, kv)
}

func fprintf(writer io.Writer, format string, kv []interface{}) (int, error) {
	buf := bufPool.Get().([]byte)[:0]
	formatter := FormatterOf(format, kv)
	formatted := formatter.Format(buf, kv)
	n, err := writer.Write(formatted)
	bufPool.Put(formatted)
	return n, err
}

var formatterCache = concurrent.NewMap()

func FormatterOf(format string, sample []interface{}) Formatter {
	formatterObj, found := formatterCache.Load(format)
	if found {
		return formatterObj.(Formatter)
	}
	formatter := compile(format, sample)
	formatterCache.Store(format, formatter)
	return formatter
}