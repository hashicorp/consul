package frame

import "fmt"
import "io"

type DebugTransport struct {
	prefix string
	*BasicTransport
}

func (t *DebugTransport) Write(buf []byte) (int, error) {
	fmt.Printf("%v writes %d bytes: %x\n", t.prefix, len(buf), buf)
	return t.BasicTransport.Write(buf)
}

func (t *DebugTransport) WriteFrame(frame WFrame) (err error) {
	// each frame knows how to write iteself to the framer
	return frame.writeTo(t)
}

func (t *DebugTransport) ReadFrame() (f RFrame, err error) {
	f, err = t.BasicTransport.ReadFrame()

	fmt.Printf("%v reads Header length: %v\n", t.prefix, t.Header.Length())
	fmt.Printf("%v reads Header type: %v\n", t.prefix, t.Header.Type())
	fmt.Printf("%v reads Header stream id: %v\n", t.prefix, t.Header.StreamId())
	fmt.Printf("%v reads Header fin: %v\n", t.prefix, t.Header.Fin())
	return
}

func NewDebugTransport(rwc io.ReadWriteCloser, prefix string) *DebugTransport {
	trans := &DebugTransport{
		prefix:         prefix,
		BasicTransport: &BasicTransport{ReadWriteCloser: rwc, Header: make([]byte, headerSize)},
	}
	return trans
}
