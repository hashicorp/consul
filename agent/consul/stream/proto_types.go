package stream

import (
	"bytes"
	"encoding/gob"
	"fmt"

	proto "github.com/golang/protobuf/proto"
)

// Protobuf doesn't natively support a map[string]interface type, so we have
// to create a stand-in here.
type UntypedMap map[string]interface{}

func (m UntypedMap) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("encode: %v", err)
	}
	return buf.Bytes(), nil
}

func (m UntypedMap) MarshalTo(data []byte) (n int, err error) {
	bytes, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(data, bytes)
	return len(bytes), nil
}

func (m *UntypedMap) Unmarshal(data []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(data))
	if err := dec.Decode(m); err != nil {
		return err
	}
	return nil
}

func (m UntypedMap) Size() int {
	b, _ := m.Marshal()
	return len(b)
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
