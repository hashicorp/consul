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
	queueSize    = 10000
)

type dnstapIO struct {
	endpoint string
	socket   bool
	conn     net.Conn
	enc      *fs.Encoder
	queue    chan tap.Dnstap
}

// New returns a new and initialized DnstapIO.
func New(endpoint string, socket bool) DnstapIO {
	return &dnstapIO{
		endpoint: endpoint,
		socket:   socket,
		queue:    make(chan tap.Dnstap, queueSize),
	}
}

// DnstapIO interface
type DnstapIO interface {
	Connect()
	Dnstap(payload tap.Dnstap)
	Close()
}

func (dio *dnstapIO) newConnect() error {
	var err error
	if dio.socket {
		dio.conn, err = net.Dial("unix", dio.endpoint)
	} else {
		dio.conn, err = net.DialTimeout("tcp", dio.endpoint, tcpTimeout)
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
	return nil
}

// Connect connects to the dnstop endpoint.
func (dio *dnstapIO) Connect() {
	if err := dio.newConnect(); err != nil {
		log.Printf("[ERROR] No connection to dnstap endpoint")
	}
	go dio.serve()
}

// Dnstap enqueues the payload for log.
func (dio *dnstapIO) Dnstap(payload tap.Dnstap) {
	select {
	case dio.queue <- payload:
	default:
		log.Printf("[ERROR] Dnstap payload dropped")
	}
}

func (dio *dnstapIO) closeConnection() {
	dio.enc.Close()
	dio.conn.Close()
	dio.enc = nil
	dio.conn = nil
}

// Close waits until the I/O routine is finished to return.
func (dio *dnstapIO) Close() {
	close(dio.queue)
}

func (dio *dnstapIO) write(payload *tap.Dnstap) {
	if dio.enc == nil {
		if err := dio.newConnect(); err != nil {
			return
		}
	}
	var err error
	if payload != nil {
		frame, e := proto.Marshal(payload)
		if err != nil {
			log.Printf("[ERROR] Invalid dnstap payload dropped: %s", e)
			return
		}
		_, err = dio.enc.Write(frame)
	} else {
		err = dio.enc.Flush()
	}
	if err == nil {
		return
	}
	log.Printf("[WARN] Connection lost: %s", err)
	dio.closeConnection()
	if err := dio.newConnect(); err != nil {
		log.Printf("[ERROR] Cannot write dnstap payload: %s", err)
	} else {
		log.Printf("[INFO] Reconnect to dnstap done")
	}
}

func (dio *dnstapIO) serve() {
	timeout := time.After(flushTimeout)
	for {
		select {
		case payload, ok := <-dio.queue:
			if !ok {
				dio.closeConnection()
				return
			}
			dio.write(&payload)
		case <-timeout:
			dio.write(nil)
			timeout = time.After(flushTimeout)
		}
	}
}
