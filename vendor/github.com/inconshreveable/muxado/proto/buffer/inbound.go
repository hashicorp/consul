package buffer

import (
	"errors"
	"io"
	"io/ioutil"
	"sync"
	"time"
)

var (
	AlreadyClosed = errors.New("Buffer already closed")
)

// A specialized concurrent circular buffer intended to buffer a stream's inbound data with the following properties:
// - Minimizes copies by skipping the buffer if a write occurs while a reader is waiting
// - Provides a mechnaism to time out reads after a deadline
// - Provides a mechanism to set an 'error' that will fail reads when the buffer is empty
type waitingReader struct {
	buf []byte
	n   int
}

type Inbound struct {
	*Circular
	*sync.Cond
	err error
	waitingReader
	deadline time.Time
	timer    *time.Timer
}

func NewInbound(size int) *Inbound {
	return &Inbound{
		Circular: NewCircular(size),
		Cond:     sync.NewCond(new(sync.Mutex)),
	}
}

func (b *Inbound) SetDeadline(t time.Time) {
	b.L.Lock()

	// set the deadline
	b.deadline = t

	// how long until the deadline
	delay := t.Sub(time.Now())

	if b.timer != nil {
		b.timer.Stop()
	}

	// after the delay, wake up waiters
	b.timer = time.AfterFunc(delay, func() {
		b.Broadcast()
	})

	b.L.Unlock()
}

func (b *Inbound) SetError(err error) {
	b.L.Lock()
	b.err = err
	b.Broadcast()
	b.L.Unlock()
}

func (b *Inbound) GetError() (err error) {
	b.L.Lock()
	err = b.err
	b.L.Unlock()
	return
}

func (b *Inbound) ReadFrom(rd io.Reader) (n int, err error) {
	b.L.Lock()

	if b.err != nil {
		b.L.Unlock()
		if _, err = ioutil.ReadAll(rd); err != nil {
			return
		}
		return 0, AlreadyClosed
	}

	// write directly to a reader's buffer, if possible
	if b.waitingReader.buf != nil {
		b.waitingReader.n, err = readInto(rd, b.waitingReader.buf)
		n += b.waitingReader.n
		b.waitingReader.buf = nil
		if err != nil {
			if err == io.EOF {
				// EOF is not an error
				err = nil
			}

			b.Broadcast()
			b.L.Unlock()
			return
		}
	}

	// write the rest to buffer
	var writeN int
	writeN, err = b.Circular.ReadFrom(rd)
	n += writeN

	b.Broadcast()
	b.L.Unlock()
	return
}

func (b *Inbound) Read(p []byte) (n int, err error) {
	b.L.Lock()

	var wait *waitingReader

	for {
		// we got a direct write to our buffer
		if wait != nil && wait.n != 0 {
			n = wait.n
			break
		}

		// check for timeout
		if !b.deadline.IsZero() {
			if time.Now().After(b.deadline) {
				err = errors.New("Read timeout")
				break
			}
		}

		// try to read from the buffer
		n, _ = b.Circular.Read(p)

		// successfully read some data
		if n != 0 {
			break
		}

		// there's an error
		if b.err != nil {
			err = b.err
			break
		}

		// register for a direct write
		if b.waitingReader.buf == nil {
			wait = &b.waitingReader
			wait.buf = p
			wait.n = 0
		}

		// no data, wait
		b.Wait()
	}

	b.L.Unlock()
	return
}
