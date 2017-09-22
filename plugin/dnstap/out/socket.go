package out

import (
	"fmt"
	"net"

	fs "github.com/farsightsec/golang-framestream"
)

// Socket is a Frame Streams encoder over a UNIX socket.
type Socket struct {
	path string
	enc  *fs.Encoder
	conn net.Conn
	err  error
}

func openSocket(s *Socket) error {
	conn, err := net.Dial("unix", s.path)
	if err != nil {
		return err
	}
	s.conn = conn

	enc, err := fs.NewEncoder(conn, &fs.EncoderOptions{
		ContentType:   []byte("protobuf:dnstap.Dnstap"),
		Bidirectional: true,
	})
	if err != nil {
		return err
	}
	s.enc = enc

	s.err = nil
	return nil
}

// NewSocket will always return a new Socket.
// err if nothing is listening to it, it will attempt to reconnect on the next Write.
func NewSocket(path string) (s *Socket, err error) {
	s = &Socket{path: path}
	if err = openSocket(s); err != nil {
		err = fmt.Errorf("open socket: %s", err)
		s.err = err
		return
	}
	return
}

// Write a single Frame Streams frame.
func (s *Socket) Write(frame []byte) (int, error) {
	if s.err != nil {
		// is the dnstap tool listening?
		if err := openSocket(s); err != nil {
			return 0, fmt.Errorf("open socket: %s", err)
		}
	}
	n, err := s.enc.Write(frame)
	if err != nil {
		// the dnstap command line tool is down
		s.conn.Close()
		s.err = err
		return 0, err
	}
	return n, nil

}

// Close the socket and flush the remaining frames.
func (s *Socket) Close() error {
	if s.err != nil {
		// nothing to close
		return nil
	}

	defer s.conn.Close()

	if err := s.enc.Flush(); err != nil {
		return fmt.Errorf("flush: %s", err)
	}
	return s.enc.Close()
}
