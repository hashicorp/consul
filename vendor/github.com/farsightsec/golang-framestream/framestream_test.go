package framestream_test

import (
	"bytes"
	"net"
	"testing"

	framestream "github.com/farsightsec/golang-framestream"
)

func testDecoder(t *testing.T, dec *framestream.Decoder, nframes int) {
	i := 1
	for {
		tf, err := dec.Decode()
		if err != nil {
			if i < nframes+1 {
				t.Fatalf("testDecoder(%d): %v", i, err)
			}
			if err != framestream.EOF {
				t.Fatalf("unexpected error: %v != EOF", err)
			}
			return
		}
		if i > nframes {
			t.Errorf("extra frame received: %d", i)
		}
		f := make([]byte, i)
		if bytes.Compare(tf, f) != 0 {
			t.Errorf("testDecoder: received %v != %v", tf, f)
		}
		i++
	}
}

func TestUnidirectional(t *testing.T) {
	buf := new(bytes.Buffer)
	enc, err := framestream.NewEncoder(buf, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i < 10; i++ {
		b := make([]byte, i)
		if _, err = enc.Write(b); err != nil {
			t.Error(err)
		}
	}
	enc.Close()

	dec, err := framestream.NewDecoder(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	testDecoder(t, dec, 9)
}

func TestBidirectional(t *testing.T) {
	client, server := net.Pipe()

	go func() {
		dec, err := framestream.NewDecoder(server,
			&framestream.DecoderOptions{
				Bidirectional: true,
			})

		if err != nil {
			t.Fatal(err)
		}
		testDecoder(t, dec, 9)
	}()

	enc, err := framestream.NewEncoder(client,
		&framestream.EncoderOptions{
			Bidirectional: true,
		})
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i < 10; i++ {
		b := make([]byte, i)
		if _, err := enc.Write(b); err != nil {
			t.Error(err)
		}
	}
	enc.Close()
}

func TestContentTypeMismatch(t *testing.T) {
	buf := new(bytes.Buffer)

	enc, err := framestream.NewEncoder(buf,
		&framestream.EncoderOptions{
			ContentType: []byte("test"),
		})
	if err != nil {
		t.Fatal(err)
	}
	enc.Write([]byte("hello, world"))
	enc.Close()

	_, err = framestream.NewDecoder(buf,
		&framestream.DecoderOptions{
			ContentType: []byte("wrong"),
		})
	if err != framestream.ErrContentTypeMismatch {
		t.Error("expected %v, received %v",
			framestream.ErrContentTypeMismatch,
			err)
	}
}

func TestOversizeFrame(t *testing.T) {
	buf := new(bytes.Buffer)
	enc, err := framestream.NewEncoder(buf, nil)
	if err != nil {
		t.Fatal(err)
	}

	enc.Write(make([]byte, 15))
	enc.Close()

	dec, err := framestream.NewDecoder(buf,
		&framestream.DecoderOptions{
			MaxPayloadSize: 10,
		})
	if err != nil {
		t.Fatal(err)
	}
	_, err = dec.Decode()
	if err != framestream.ErrDataFrameTooLarge {
		t.Error("data frame too large, received %v", err)
	}
}
