package dnstapio

import (
	"log"
	"net"
	"time"

	tap "github.com/dnstap/golang-dnstap"
	fs "github.com/farsightsec/golang-framestream"
	"github.com/golang/protobuf/proto"
)

const (
	tcpTimeout   = 4 * time.Second
	flushTimeout = 1 * time.Second
	queueSize    = 1000
)

type dnstapIO struct {
	enc   *fs.Encoder
	conn  net.Conn
	queue chan tap.Dnstap
}

// New returns a new and initialized DnstapIO.
func New() DnstapIO {
	return &dnstapIO{queue: make(chan tap.Dnstap, queueSize)}
}

// DnstapIO interface
type DnstapIO interface {
	Connect(endpoint string, socket bool) error
	Dnstap(payload tap.Dnstap)
	Close()
}

// Connect connects to the dnstop endpoint.
func (dio *dnstapIO) Connect(endpoint string, socket bool) error {
	var err error
	if socket {
		dio.conn, err = net.Dial("unix", endpoint)
	} else {
		dio.conn, err = net.DialTimeout("tcp", endpoint, tcpTimeout)
	}
	if err != nil {
		return err
	}
	dio.enc, err = fs.NewEncoder(dio.conn, &fs.EncoderOptions{
		ContentType:   []byte("protobuf:dnstap.Dnstap"),
		Bidirectional: true,
	})
	if err != nil {
		return err
	}
	go dio.serve()
	return nil
}

// Dnstap enqueues the payload for log.
func (dio *dnstapIO) Dnstap(payload tap.Dnstap) {
	select {
	case dio.queue <- payload:
	default:
		log.Printf("[ERROR] Dnstap payload dropped")
	}
}

// Close waits until the I/O routine is finished to return.
func (dio *dnstapIO) Close() {
	close(dio.queue)
}

func (dio *dnstapIO) serve() {
	timeout := time.After(flushTimeout)
	for {
		select {
		case payload, ok := <-dio.queue:
			if !ok {
				dio.enc.Close()
				dio.conn.Close()
				return
			}
			frame, err := proto.Marshal(&payload)
			if err != nil {
				log.Printf("[ERROR] Invalid dnstap payload dropped: %s", err)
				continue
			}
			_, err = dio.enc.Write(frame)
			if err != nil {
				log.Printf("[ERROR] Cannot write dnstap payload: %s", err)
				continue
			}
		case <-timeout:
			err := dio.enc.Flush()
			if err != nil {
				log.Printf("[ERROR] Cannot flush dnstap payloads: %s", err)
			}
			timeout = time.After(flushTimeout)
		}
	}
}
