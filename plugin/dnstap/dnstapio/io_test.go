package dnstapio

import (
	"net"
	"sync"
	"testing"
	"time"

	tap "github.com/dnstap/golang-dnstap"
	fs "github.com/farsightsec/golang-framestream"
)

const (
	endpointTCP    = "localhost:0"
	endpointSocket = "dnstap.sock"
)

var (
	msgType = tap.Dnstap_MESSAGE
	msg     = tap.Dnstap{Type: &msgType}
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

func TestTransport(t *testing.T) {
	transport := [2][3]string{
		{"tcp", endpointTCP, "false"},
		{"unix", endpointSocket, "true"},
	}

	for _, param := range transport {
		// Start TCP listener
		l, err := net.Listen(param[0], param[1])
		if err != nil {
			t.Fatalf("Cannot start listener: %s", err)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			accept(t, l, 1)
			wg.Done()
		}()

		dio := New(l.Addr().String(), param[2] == "true")
		dio.Connect()

		dio.Dnstap(msg)

		wg.Wait()
		l.Close()
		dio.Close()
	}
}

func TestRace(t *testing.T) {
	count := 10

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

	dio := New(l.Addr().String(), false)
	dio.Connect()
	defer dio.Close()

	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			time.Sleep(50 * time.Millisecond)
			dio.Dnstap(msg)
			wg.Done()
		}()
	}

	wg.Wait()
}

func TestReconnect(t *testing.T) {
	count := 5

	// Start TCP listener
	l, err := net.Listen("tcp", endpointTCP)
	if err != nil {
		t.Fatalf("Cannot start listener: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		accept(t, l, 1)
		wg.Done()
	}()

	addr := l.Addr().String()
	dio := New(addr, false)
	dio.Connect()
	defer dio.Close()

	msg := tap.Dnstap_MESSAGE
	dio.Dnstap(tap.Dnstap{Type: &msg})

	wg.Wait()

	// Close listener
	l.Close()

	// And start TCP listener again on the same port
	l, err = net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Cannot start listener: %s", err)
	}
	defer l.Close()

	wg.Add(1)
	go func() {
		accept(t, l, 1)
		wg.Done()
	}()

	for i := 0; i < count; i++ {
		time.Sleep(time.Second)
		dio.Dnstap(tap.Dnstap{Type: &msg})
	}

	wg.Wait()
}
