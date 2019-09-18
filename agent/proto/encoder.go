package proto

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

type ProtoMarshaller interface {
	Size() int
	MarshalTo([]byte) (int, error)
	Unmarshal([]byte) error
	ProtoMessage()
}

func EncodeInterface(t structs.MessageType, message interface{}) ([]byte, error) {
	if marshaller, ok := message.(ProtoMarshaller); ok {
		return Encode(t, marshaller)
	}
	return nil, fmt.Errorf("message does not implement the ProtoMarshaller interface")
}

func Encode(t structs.MessageType, message ProtoMarshaller) ([]byte, error) {
	data := make([]byte, message.Size()+1)
	data[0] = uint8(t)
	if _, err := message.MarshalTo(data[1:]); err != nil {
		return nil, err
	}
	return data, nil
}

func Decode(buf []byte, out ProtoMarshaller) error {
	// Note that this assumes the leading byte indicating the type has already been stripped off
	return out.Unmarshal(buf)
}
