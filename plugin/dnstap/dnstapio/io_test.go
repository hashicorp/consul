package dnstapio

import (
	"net"
	"sync"
	"testing"
	"time"

	tap "github.com/dnstap/golang-dnstap"
	fs "github.com/farsightsec/golang-framestream"
)

func accept(t *testing.T, l net.Listener, count int) {
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

	for i := 0; i < count; i++ {
		if _, err := dec.Decode(); err != nil {
			t.Errorf("server decode: %s", err)
		}
	}

	if err := server.Close(); err != nil {
		t.Error(err)
	}
}

const endpointTCP = "localhost:0"

func TestTCP(t *testing.T) {
	dio := New()

	err := dio.Connect(endpointTCP, false)
	if err == nil {
		t.Fatal("Not listening but no error")
	}

	// Start TCP listener
	l, err := net.Listen("tcp", endpointTCP)
	if err != nil {
		t.Fatalf("Cannot start listener: %s", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		accept(t, l, 1)
		wg.Done()
	}()

	err = dio.Connect(l.Addr().String(), false)
	if err != nil {
		t.Fatalf("Cannot connect to listener: %s", err)
	}

	msg := tap.Dnstap_MESSAGE
	dio.Dnstap(tap.Dnstap{Type: &msg})

	wg.Wait()

	dio.Close()
}

const endpointSocket = "dnstap.sock"

func TestSocket(t *testing.T) {
	dio := New()

	err := dio.Connect(endpointSocket, true)
	if err == nil {
		t.Fatal("Not listening but no error")
	}

	// Start Socket listener
	l, err := net.Listen("unix", endpointSocket)
	if err != nil {
		t.Fatalf("Cannot start listener: %s", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		accept(t, l, 1)
		wg.Done()
	}()

	err = dio.Connect(endpointSocket, true)
	if err != nil {
		t.Fatalf("Cannot connect to listener: %s", err)
	}

	msg := tap.Dnstap_MESSAGE
	dio.Dnstap(tap.Dnstap{Type: &msg})

	wg.Wait()

	dio.Close()
}

func TestRace(t *testing.T) {
	count := 10
	dio := New()

	err := dio.Connect(endpointTCP, false)
	if err == nil {
		t.Fatal("Not listening but no error")
	}

	// Start TCP listener
	l, err := net.Listen("tcp", endpointTCP)
	if err != nil {
		t.Fatalf("Cannot start listener: %s", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		accept(t, l, count)
		wg.Done()
	}()

	err = dio.Connect(l.Addr().String(), false)
	if err != nil {
		t.Fatalf("Cannot connect to listener: %s", err)
	}

	msg := tap.Dnstap_MESSAGE
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(i byte) {
			time.Sleep(50 * time.Millisecond)
			dio.Dnstap(tap.Dnstap{Type: &msg, Extra: []byte{i}})
			wg.Done()
		}(byte(i))
	}

	wg.Wait()

	dio.Close()
}
