package msg

import (
	"fmt"

	lib "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
)

func wrap(m *lib.Message) *lib.Dnstap {
	t := lib.Dnstap_MESSAGE
	return &lib.Dnstap{
		Type:    &t,
		Message: m,
	}
}

func Marshal(m *lib.Message) (data []byte, err error) {
	data, err = proto.Marshal(wrap(m))
	if err != nil {
		err = fmt.Errorf("proto: %s", err)
		return
	}
	return
}
