package proxy

import (
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

// Runner manages the lifecycle of one Proxier.
type Runner struct {
	name string
	p    Proxier

	// Stopping is if a flag that is updated and read atomically
	stopping int32
	stopCh   chan struct{}
	// wg is used to signal back to Stop when all goroutines have stopped
	wg sync.WaitGroup

	logger *log.Logger
}

// NewRunner returns a Runner ready to Listen.
func NewRunner(name string, p Proxier) *Runner {
	return NewRunnerWithLogger(name, p, log.New(os.Stdout, "", log.LstdFlags))
}

// NewRunnerWithLogger returns a Runner ready to Listen using the specified
// log.Logger.
func NewRunnerWithLogger(name string, p Proxier, logger *log.Logger) *Runner {
	return &Runner{
		name:   name,
		p:      p,
		stopCh: make(chan struct{}),
		logger: logger,
	}
}

// Listen starts the proxier instance. It blocks until a fatal error occurs or
// Stop() is called.
func (r *Runner) Listen() error {
	if atomic.LoadInt32(&r.stopping) == 1 {
		return ErrStopped
	}

	l, err := r.p.Listener()
	if err != nil {
		return err
	}
	r.logger.Printf("[INFO] proxy: %s listening on %s", r.name, l.Addr().String())

	// Run goroutine that will close listener on stop
	go func() {
		<-r.stopCh
		l.Close()
		r.logger.Printf("[INFO] proxy: %s shutdown", r.name)
	}()

	// Add one for the accept loop
	r.wg.Add(1)
	defer r.wg.Done()

	for {
		conn, err := l.Accept()
		if err != nil {
			if atomic.LoadInt32(&r.stopping) == 1 {
				return nil
			}
			return err
		}

		go r.handle(conn)
	}

	return nil
}

func (r *Runner) handle(conn net.Conn) {
	r.wg.Add(1)
	defer r.wg.Done()

	// Start a goroutine that will watch for the Runner stopping and close the
	// conn, or watch for the Proxier closing (e.g. because other end hung up) and
	// stop the goroutine to avoid leaks
	doneCh := make(chan struct{})
	defer close(doneCh)

	go func() {
		select {
		case <-r.stopCh:
			r.logger.Printf("[DEBUG] proxy: %s: terminating conn", r.name)
			conn.Close()
			return
		case <-doneCh:
			// Connection is already closed, this goroutine not needed any more
			return
		}
	}()

	err := r.p.HandleConn(conn)
	if err != nil {
		r.logger.Printf("[DEBUG] proxy: %s: connection terminated: %s", r.name, err)
	} else {
		r.logger.Printf("[DEBUG] proxy: %s: connection terminated", r.name)
	}
}

// Stop stops the Listener and closes any active connections immediately.
func (r *Runner) Stop() error {
	old := atomic.SwapInt32(&r.stopping, 1)
	if old == 0 {
		close(r.stopCh)
	}
	r.wg.Wait()
	return nil
}
