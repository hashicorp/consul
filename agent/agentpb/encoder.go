package agentpb

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
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
	return nil, fmt.Errorf("message does not implement the ProtoMarshaller interface: %T", message)
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

func MarshalJSON(msg proto.Message) ([]byte, error) {
	m := jsonpb.Marshaler{
		EmitDefaults: false,
	}

	var buf bytes.Buffer
	if err := m.Marshal(&buf, msg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UnmarshalJSON(r io.Reader, msg proto.Message) error {
	u := jsonpb.Unmarshaler{
		AllowUnknownFields: true,
	}

	return u.Unmarshal(r, msg)
}

func UnmarshalJSONBytes(b []byte, msg proto.Message) error {
	return UnmarshalJSON(bytes.NewReader(b), msg)
}
