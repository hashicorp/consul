package out

import (
	"net"
	"testing"
)

func sendOneTCP(tcp *TCP) error {
	if _, err := tcp.Write([]byte("frame")); err != nil {
		return err
	}
	if err := tcp.Flush(); err != nil {
		return err
	}
	return nil
}
func TestTCP(t *testing.T) {
	tcp := NewTCP("localhost:14000")

	if err := sendOneTCP(tcp); err == nil {
		t.Fatal("Not listening but no error.")
		return
	}

	l, err := net.Listen("tcp", "localhost:14000")
	if err != nil {
		t.Fatal(err)
		return
	}

	wait := make(chan bool)
	go func() {
		acceptOne(t, l)
		wait <- true
	}()

	if err := sendOneTCP(tcp); err != nil {
		t.Fatalf("send one: %s", err)
		return
	}

	<-wait

	// TODO: When the server isn't responding according to the framestream protocol
	// the thread is blocked.
	/*
		if err := sendOneTCP(tcp); err == nil {
			panic("must fail")
		}
	*/

	go func() {
		acceptOne(t, l)
		wait <- true
	}()

	if err := sendOneTCP(tcp); err != nil {
		t.Fatalf("send one: %s", err)
		return
	}

	<-wait
	if err := l.Close(); err != nil {
		t.Error(err)
	}
}
