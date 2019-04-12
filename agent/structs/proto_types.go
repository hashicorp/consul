package structs

import (
	"encoding/json"

	proto "github.com/gogo/protobuf/proto"
)

type UntypedMap map[string]interface{}

func (m UntypedMap) Marshal() ([]byte, error) {
	return json.Marshal(map[string]interface{}(m))
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
	return json.Unmarshal(data, (*map[string]interface{})(m))
}

func (m UntypedMap) Size() int {
	b, _ := m.Marshal()
	return len(b)
}

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
