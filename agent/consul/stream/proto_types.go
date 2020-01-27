package stream

import (
	proto "github.com/golang/protobuf/proto"
)

//go:generate msgp

// Protobuf doesn't natively support a map[string]interface type, so we have to
// create a stand-in here. We use tinylib/msgp to generate custom message pack
// marshalling instead of using our regular msgpack codec for a few reasons:
//   1. One of the main reasons to switch to protobuf was for encoding
//      performance on servers however runtime reflection in gob of other
//      msgpack codecs makes even a nil UntypedMap (present in most service
//      events) dominate the encoding cost on servers. Generating msgpack
//      encoding is nicer.
//   2. This will become a wire format we have to commit to as changing it will
//      break compatibility between server and client versions etc. and protobuf
//      can't help as it's all opaque encoding to it.
//   3. Using msgpack is better and more universal/well supported than Gob, and
//      more performant than JSON. It's also not a whole new serialization
//      format since we already use msgpack in Serf and old RPCs etc.
type UntypedMap map[string]interface{}

func (m UntypedMap) Marshal() ([]byte, error) {
	return m.MarshalMsg(nil)
}

func (m UntypedMap) MarshalTo(data []byte) (n int, err error) {
	outData, err := m.MarshalMsg(data)
	if err != nil {
		return 0, err
	}
	return len(outData) - len(data), nil
}

func (m *UntypedMap) Unmarshal(data []byte) error {
	_, err := m.UnmarshalMsg(data)
	return err
}

func (m UntypedMap) Size() int {
	return m.Msgsize()
}

// As with UntypedMap above, Headers exists for converting map[string][]string
// to/from protobuf types.
type Headers map[string][]string

func (m Headers) Marshal() ([]byte, error) {
	ph := &ProtoHeaders{
		Headers: make(map[string]*StringList),
	}
	for k, v := range m {
		ph.Headers[k] = &StringList{Values: v}
	}
	return proto.Marshal(ph)
}

func (m Headers) MarshalTo(data []byte) (n int, err error) {
	bytes, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(data, bytes)
	return len(bytes), nil
}

func (m *Headers) Unmarshal(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	*m = make(map[string][]string)
	p := &ProtoHeaders{}

	err := proto.Unmarshal(data, p)
	if err != nil {
		return err
	}

	for k, v := range p.Headers {
		(*m)[k] = v.Values
	}

	return nil
}

func (m Headers) Size() int {
	b, _ := m.Marshal()
	return len(b)
}
