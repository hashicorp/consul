package dnstapio

import (
	"log"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
)

// DnstapIO wraps the dnstap I/O routine.
type DnstapIO struct {
	protocol Protocol
	queue    chan tap.Dnstap
	stop     chan bool
}

// Protocol is either `out.TCP` or `out.Socket`.
type Protocol interface {
	// Write takes a single frame at once.
	Write([]byte) (int, error)

	Close() error
}

// New dnstap I/O routine from Protocol.
func New(w Protocol) *DnstapIO {
	dio := DnstapIO{}
	dio.protocol = w
	dio.queue = make(chan tap.Dnstap, 10)
	dio.stop = make(chan bool)
	go dio.serve()
	return &dio
}

// Dnstap enqueues the payload for log.
func (dio *DnstapIO) Dnstap(payload tap.Dnstap) {
	select {
	case dio.queue <- payload:
	default:
		log.Println("[WARN] Dnstap payload dropped.")
	}
}

func (dio *DnstapIO) serve() {
	for {
		select {
		case payload := <-dio.queue:
			frame, err := proto.Marshal(&payload)
			if err == nil {
				dio.protocol.Write(frame)
			} else {
				log.Printf("[ERROR] Invalid dnstap payload dropped: %s\n", err)
			}
		case <-dio.stop:
			close(dio.queue)
			dio.stop <- true
			return
		}
	}
}

// Close waits until the I/O routine is finished to return.
func (dio DnstapIO) Close() error {
	dio.stop <- true
	<-dio.stop
	close(dio.stop)
	return dio.protocol.Close()
}
