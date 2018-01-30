package dnstapio

import (
	"log"
	"net"
	"sync/atomic"
	"time"

	tap "github.com/dnstap/golang-dnstap"
	fs "github.com/farsightsec/golang-framestream"
)

const (
	tcpWriteBufSize = 1024 * 1024
	tcpTimeout      = 4 * time.Second
	flushTimeout    = 1 * time.Second
	queueSize       = 10000
)

type dnstapIO struct {
	endpoint string
	socket   bool
	conn     net.Conn
	enc      *dnstapEncoder
	queue    chan tap.Dnstap
	dropped  uint32
}

// New returns a new and initialized DnstapIO.
func New(endpoint string, socket bool) DnstapIO {
	return &dnstapIO{
		endpoint: endpoint,
		socket:   socket,
		enc: newDnstapEncoder(&fs.EncoderOptions{
			ContentType:   []byte("protobuf:dnstap.Dnstap"),
			Bidirectional: true,
		}),
		queue: make(chan tap.Dnstap, queueSize),
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
		if dio.conn, err = net.Dial("unix", dio.endpoint); err != nil {
			return err
		}
	} else {
		if dio.conn, err = net.DialTimeout("tcp", dio.endpoint, tcpTimeout); err != nil {
			return err
		}
		if tcpConn, ok := dio.conn.(*net.TCPConn); ok {
			tcpConn.SetWriteBuffer(tcpWriteBufSize)
			tcpConn.SetNoDelay(false)
		}
	}

	return dio.enc.resetWriter(dio.conn)
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
		atomic.AddUint32(&dio.dropped, 1)
	}
}

func (dio *dnstapIO) closeConnection() {
	dio.enc.close()
	if dio.conn != nil {
		dio.conn.Close()
		dio.conn = nil
	}
}

// Close waits until the I/O routine is finished to return.
func (dio *dnstapIO) Close() {
	close(dio.queue)
}

func (dio *dnstapIO) flushBuffer() {
	if dio.conn == nil {
		if err := dio.newConnect(); err != nil {
			return
		}
		log.Printf("[INFO] Reconnected to dnstap")
	}

	if err := dio.enc.flushBuffer(); err != nil {
		log.Printf("[WARN] Connection lost: %s", err)
		dio.closeConnection()
		if err := dio.newConnect(); err != nil {
			log.Printf("[ERROR] Cannot connect to dnstap: %s", err)
		} else {
			log.Printf("[INFO] Reconnected to dnstap")
		}
	}
}

func (dio *dnstapIO) write(payload *tap.Dnstap) {
	if err := dio.enc.writeMsg(payload); err != nil {
		atomic.AddUint32(&dio.dropped, 1)
	}
}

func (dio *dnstapIO) serve() {
	timeout := time.After(flushTimeout)
	for {
		select {
		case payload, ok := <-dio.queue:
			if !ok {
				dio.flushBuffer()
				dio.closeConnection()
				return
			}
			dio.write(&payload)
		case <-timeout:
			if dropped := atomic.SwapUint32(&dio.dropped, 0); dropped > 0 {
				log.Printf("[WARN] Dropped dnstap messages: %d", dropped)
			}
			dio.flushBuffer()
			timeout = time.After(flushTimeout)
		}
	}
}
