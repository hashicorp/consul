package witch

import (
	"fmt"
	"github.com/v2pro/plz/countlog"
	"net/http"
	"os"
	"sync/atomic"
	"time"
	"github.com/json-iterator/go"
)

var theEventQueue = newEventQueue()

type eventQueue struct {
	msgChan            chan []byte
	droppedEventsCount uint64
}

func newEventQueue() *eventQueue {
	return &eventQueue{
		msgChan: make(chan []byte, 10240),
	}
}

func (q *eventQueue) Write(buf []byte) (int, error) {
	select {
	case q.msgChan <- buf:
	default:
		dropped := atomic.AddUint64(&q.droppedEventsCount, 1)
		if dropped%10000 == 1 {
			os.Stderr.Write([]byte(fmt.Sprintf(
				"witch event queue overflow, dropped %v events since start\n", dropped)))
			os.Stderr.Sync()
		}
	}
	return len(buf), nil
}

func (q *eventQueue) consume() [][]byte {
	events := make([][]byte, 0, 4)
	timer := time.NewTimer(10 * time.Second)
	select {
	case event := <-theEventQueue.msgChan:
		events = append(events, event)
	case <-timer.C:
		// timeout
	}
	time.Sleep(time.Millisecond * 10)
	for {
		select {
		case event := <-theEventQueue.msgChan:
			events = append(events, event)
			if len(events) > 1000 {
				return events
			}
		default:
			return events
		}
	}
}

func moreEvents(respWriter http.ResponseWriter, req *http.Request) {
	setCurrentGoRoutineIsKoala()
	defer func() {
		recovered := recover()
		countlog.LogPanic(recovered)
	}()
	respWriter.Header().Add("Access-Control-Allow-Origin", "*")
	respWriter.Write([]byte("["))
	stream := jsoniter.ConfigDefault.BorrowStream(respWriter)
	defer jsoniter.ConfigDefault.ReturnStream(stream)
	events := theEventQueue.consume()
	stream.WriteArrayStart()
	for i, event := range events {
		if i != 0 {
			stream.WriteMore()
		}
		stream.Write(event)
	}
	stream.WriteArrayEnd()
	stream.Flush()
}
