package msg

import (
	"fmt"

	lib "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
)

// Wrap a dnstap message in the top-level dnstap type.
func Wrap(m *lib.Message) lib.Dnstap {
	t := lib.Dnstap_MESSAGE
	return lib.Dnstap{
		Type:    &t,
		Message: m,
	}
}

// Marshal encodes the message to a binary dnstap payload.
func Marshal(m *lib.Message) (data []byte, err error) {
	payload := Wrap(m)
	data, err = proto.Marshal(&payload)
	if err != nil {
		err = fmt.Errorf("proto: %s", err)
		return
	}
	return
}
