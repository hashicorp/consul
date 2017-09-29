package dnstapio

import (
	"bytes"
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	tap "github.com/dnstap/golang-dnstap"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

type buf struct {
	*bytes.Buffer
	cost time.Duration
}

func (b buf) Write(frame []byte) (int, error) {
	time.Sleep(b.cost)
	return b.Buffer.Write(frame)
}

func (b buf) Close() error {
	return nil
}

func TestRace(t *testing.T) {
	b := buf{&bytes.Buffer{}, 100 * time.Millisecond}
	dio := New(b)
	wg := &sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		timeout := time.After(time.Second)
		go func() {
			for {
				select {
				case <-timeout:
					wg.Done()
					return
				default:
					time.Sleep(50 * time.Millisecond)
					t := tap.Dnstap_MESSAGE
					dio.Dnstap(tap.Dnstap{Type: &t})
				}
			}
		}()
	}
	wg.Wait()
}

func TestClose(t *testing.T) {
	done := make(chan bool)
	var dio *DnstapIO
	go func() {
		b := buf{&bytes.Buffer{}, 0}
		dio = New(b)
		dio.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Not closing.")
	}
	func() {
		defer func() {
			if err := recover(); err == nil {
				t.Fatal("Send on closed channel.")
			}
		}()
		dio.Dnstap(tap.Dnstap{})
	}()
}
