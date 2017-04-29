package zipkintracer

import (
	"errors"
	"testing"

	"github.com/opentracing/opentracing-go/log"
)

type obj struct {
	a int
	b string
}

func getLogFields() []log.Field {
	lazy := func(fv log.Encoder) {
		fv.EmitString("lazy", "logger")
	}
	return []log.Field{
		log.Bool("bool", true),
		log.String("string", "value"),
		log.Error(errors.New("an error")),
		log.Float32("float32", 32.123),
		log.Float64("float64", 64.123),
		log.Int("int", 42),
		log.Int32("int32", 32),
		log.Int64("int64", 64),
		log.Uint32("uint32", 32),
		log.Uint64("uint64", 64),
		log.Object("object", obj{a: 42, b: "string"}),
		log.Lazy(lazy),
		log.String("event", "EventValue"),
	}
}

func TestMaterializeWithJSON(t *testing.T) {
	logFields := getLogFields()
	want := `{"bool":"true","error":"an error","event":"EventValue","float32":"32.123001","float64":"64.123000","int":"42","int32":"32","int64":"64","lazy":"logger","object":"{a:42 b:string}","string":"value","uint32":"32","uint64":"64"}`
	have, err := MaterializeWithJSON(logFields)
	if err != nil {
		t.Fatalf("expected json string, got error %+v", err)
	}
	if want != string(have) {
		t.Errorf("want:\n%s\nhave\n%s", want, have)
	}
}

func TestMaterializeWithLogFmt(t *testing.T) {
	logFields := getLogFields()
	want := `bool=true string=value error="an error" float32=32.123 float64=64.123 int=42 int32=32 int64=64 uint32=32 uint64=64 object="unsupported value type" event=EventValue`
	have, err := MaterializeWithLogFmt(logFields)
	if err != nil {
		t.Fatalf("expected logfmt string, got error %+v", err)
	}
	if want != string(have) {
		t.Errorf("want:\n%s\nhave\n%s", want, have)
	}
}

func TestStrictZipkinMaterializer(t *testing.T) {
	logFields := getLogFields()
	want := `EventValue`
	have, err := StrictZipkinMaterializer(logFields)
	if err != nil {
		t.Fatalf("expected string got error %+v", err)
	}
	if want != string(have) {
		t.Errorf("want:\n%s\nhave\n%s", want, have)
	}
	logFields = []log.Field{log.String("SomeKey", "SomeValue")}
	if _, err = StrictZipkinMaterializer(logFields); err == nil {
		t.Errorf("expected error: %s, got nil", errEventLogNotFound)
	}
}
