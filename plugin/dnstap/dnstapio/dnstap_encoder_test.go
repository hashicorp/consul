package dnstapio

import (
	"bytes"
	"testing"

	tap "github.com/dnstap/golang-dnstap"
	fs "github.com/farsightsec/golang-framestream"
	"github.com/golang/protobuf/proto"
)

func dnstapMsg() *tap.Dnstap {
	t := tap.Dnstap_MESSAGE
	mt := tap.Message_CLIENT_RESPONSE
	msg := &tap.Message{Type: &mt}
	return &tap.Dnstap{Type: &t, Message: msg}
}

func TestEncoderCompatibility(t *testing.T) {
	opts := &fs.EncoderOptions{
		ContentType:   []byte("protobuf:dnstap.DnstapTest"),
		Bidirectional: false,
	}
	msg := dnstapMsg()

	//framestream encoder
	fsW := new(bytes.Buffer)
	fsEnc, err := fs.NewEncoder(fsW, opts)
	if err != nil {
		t.Fatal(err)
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	fsEnc.Write(data)
	fsEnc.Close()

	//dnstap encoder
	dnstapW := new(bytes.Buffer)
	dnstapEnc := newDnstapEncoder(opts)
	if err := dnstapEnc.resetWriter(dnstapW); err != nil {
		t.Fatal(err)
	}
	dnstapEnc.writeMsg(msg)
	dnstapEnc.flushBuffer()
	dnstapEnc.close()

	//compare results
	if !bytes.Equal(fsW.Bytes(), dnstapW.Bytes()) {
		t.Fatal("dnstapEncoder is not compatible with framestream Encoder")
	}
}
