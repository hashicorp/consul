package out

import (
	"net"
	"testing"

	fs "github.com/farsightsec/golang-framestream"
)

func acceptOne(t *testing.T, l net.Listener) {
	server, err := l.Accept()
	if err != nil {
		t.Fatalf("server accept: %s", err)
		return
	}

	dec, err := fs.NewDecoder(server, &fs.DecoderOptions{
		ContentType:   []byte("protobuf:dnstap.Dnstap"),
		Bidirectional: true,
	})
	if err != nil {
		t.Fatalf("server decoder: %s", err)
		return
	}

	if _, err := dec.Decode(); err != nil {
		t.Errorf("server decode: %s", err)
	}

	if err := server.Close(); err != nil {
		t.Error(err)
	}
}
func sendOne(socket *Socket) error {
	if _, err := socket.Write([]byte("frame")); err != nil {
		return err
	}
	if err := socket.enc.Flush(); err != nil {
		// Would happen during Write in real life.
		socket.conn.Close()
		socket.err = err
		return err
	}
	return nil
}
func TestSocket(t *testing.T) {
	socket, err := NewSocket("dnstap.sock")
	if err == nil {
		t.Fatal("new socket: not listening but no error")
		return
	}

	if err := sendOne(socket); err == nil {
		t.Fatal("not listening but no error")
		return
	}

	l, err := net.Listen("unix", "dnstap.sock")
	if err != nil {
		t.Fatal(err)
		return
	}

	wait := make(chan bool)
	go func() {
		acceptOne(t, l)
		wait <- true
	}()

	if err := sendOne(socket); err != nil {
		t.Fatalf("send one: %s", err)
		return
	}

	<-wait
	if err := sendOne(socket); err == nil {
		panic("must fail")
	}

	go func() {
		acceptOne(t, l)
		wait <- true
	}()

	if err := sendOne(socket); err != nil {
		t.Fatalf("send one: %s", err)
		return
	}

	<-wait
	if err := l.Close(); err != nil {
		t.Error(err)
	}
}
