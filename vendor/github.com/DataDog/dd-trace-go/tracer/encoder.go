package tracer

import (
	"bytes"
	"encoding/json"

	"github.com/ugorji/go/codec"
)

const (
	jsonContentType    = "application/json"
	msgpackContentType = "application/msgpack"
)

// Encoder is a generic interface that expects encoding methods for traces and
// services, and a Read() method that will be used by the http handler
type Encoder interface {
	EncodeTraces(traces [][]*Span) error
	EncodeServices(services map[string]Service) error
	Read(p []byte) (int, error)
	ContentType() string
}

var mh codec.MsgpackHandle

// msgpackEncoder encodes a list of traces in Msgpack format
type msgpackEncoder struct {
	buffer      *bytes.Buffer
	encoder     *codec.Encoder
	contentType string
}

func newMsgpackEncoder() *msgpackEncoder {
	buffer := &bytes.Buffer{}
	encoder := codec.NewEncoder(buffer, &mh)

	return &msgpackEncoder{
		buffer:      buffer,
		encoder:     encoder,
		contentType: msgpackContentType,
	}
}

// EncodeTraces serializes the given trace list into the internal buffer,
// returning the error if any.
func (e *msgpackEncoder) EncodeTraces(traces [][]*Span) error {
	return e.encoder.Encode(traces)
}

// EncodeServices serializes a service map into the internal buffer.
func (e *msgpackEncoder) EncodeServices(services map[string]Service) error {
	return e.encoder.Encode(services)
}

// Read values from the internal buffer
func (e *msgpackEncoder) Read(p []byte) (int, error) {
	return e.buffer.Read(p)
}

// ContentType return the msgpackEncoder content-type
func (e *msgpackEncoder) ContentType() string {
	return e.contentType
}

// jsonEncoder encodes a list of traces in JSON format
type jsonEncoder struct {
	buffer      *bytes.Buffer
	encoder     *json.Encoder
	contentType string
}

// newJSONEncoder returns a new encoder for the JSON format.
func newJSONEncoder() *jsonEncoder {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)

	return &jsonEncoder{
		buffer:      buffer,
		encoder:     encoder,
		contentType: jsonContentType,
	}
}

// EncodeTraces serializes the given trace list into the internal buffer,
// returning the error if any.
func (e *jsonEncoder) EncodeTraces(traces [][]*Span) error {
	return e.encoder.Encode(traces)
}

// EncodeServices serializes a service map into the internal buffer.
func (e *jsonEncoder) EncodeServices(services map[string]Service) error {
	return e.encoder.Encode(services)
}

// Read values from the internal buffer
func (e *jsonEncoder) Read(p []byte) (int, error) {
	return e.buffer.Read(p)
}

// ContentType return the jsonEncoder content-type
func (e *jsonEncoder) ContentType() string {
	return e.contentType
}

// encoderFactory will provide a new encoder each time we want to flush traces or services.
type encoderFactory func() Encoder

func jsonEncoderFactory() Encoder {
	return newJSONEncoder()
}

func msgpackEncoderFactory() Encoder {
	return newMsgpackEncoder()
}
