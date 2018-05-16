package output

import (
	"context"
	"io"
	"github.com/v2pro/plz/countlog/spi"
	"fmt"
	"time"
	"github.com/v2pro/plz/concurrent"
)

type blockingQueueWriter struct {
	queue  chan []byte
	writer io.Writer
	executor *concurrent.UnboundedExecutor
}

type nonBlockingQueueWriter struct {
	blockingQueueWriter
	onMessageDropped func(message []byte)
}

type AsyncWriterConfig struct {
	QueueLength      int
	Writer           io.Writer
	IsQueueBlocking  bool
	OnMessageDropped func(msg []byte)
}

type ClosableWriter interface {
	io.Closer
	io.Writer
}

func NewAsyncWriter(cfg AsyncWriterConfig) ClosableWriter {
	queueLength := cfg.QueueLength
	if queueLength == 0 {
		queueLength = 1024
	}
	onMessageDropped := cfg.OnMessageDropped
	if onMessageDropped == nil {
		droppedCount := 0
		onMessageDropped = func(msg []byte) {
			droppedCount++
			if droppedCount%1000 == 1 {
				spi.OnError(fmt.Errorf("countlog async writer congestion, dropped %v messages so far", droppedCount))
			}
		}
	}
	executor := concurrent.NewUnboundedExecutor()
	if cfg.IsQueueBlocking {
		asyncWriter := &blockingQueueWriter{
			queue:  make(chan []byte, queueLength),
			writer: cfg.Writer,
			executor: executor,
		}
		executor.Go(asyncWriter.asyncWrite)
		return asyncWriter
	}
	asyncWriter := &nonBlockingQueueWriter{
		blockingQueueWriter: blockingQueueWriter{
			queue:  make(chan []byte, queueLength),
			writer: cfg.Writer,
			executor: executor,
		},
		onMessageDropped: onMessageDropped,
	}
	executor.Go(asyncWriter.asyncWrite)
	return asyncWriter
}

func (writer *blockingQueueWriter) asyncWrite(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// give 1 second to write remaining logs
			timer := time.NewTimer(time.Second)
			for {
				select {
				case <-timer.C:
					// time is up
					return
				case buf := <-writer.queue:
					_, err := writer.writer.Write(buf)
					if err != nil {
						spi.OnError(err)
					}
				default:
					// all written out
					return
				}
			}
			return
		case buf := <-writer.queue:
			_, err := writer.writer.Write(buf)
			if err != nil {
				spi.OnError(err)
			}
		}
	}
}

func (writer *blockingQueueWriter) Write(buf []byte) (int, error) {
	writer.queue <- append([]byte(nil), buf...)
	return len(buf), nil
}

func (writer *nonBlockingQueueWriter) Write(buf []byte) (int, error) {
	select {
	case writer.queue <- append([]byte(nil), buf...):
	default:
		writer.onMessageDropped(buf)
	}
	return len(buf), nil
}

func (writer *blockingQueueWriter) Close() error {
	writer.executor.StopAndWaitForever()
	closer, _ := writer.writer.(io.Closer)
	if closer == nil {
		return nil
	}
	return closer.Close()
}
