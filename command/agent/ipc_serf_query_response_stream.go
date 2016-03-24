package agent

import (
	"github.com/hashicorp/serf/serf"
	"log"
	"time"
)

// serfQueryResponseStream is used to stream the serf query results back to a client
type serfQueryResponseStream struct {
	client streamClient
	logger *log.Logger
	seq    uint64
}

func newSerfQueryResponseStream(client streamClient, seq uint64, logger *log.Logger) *serfQueryResponseStream {
	qs := &serfQueryResponseStream{
		client: client,
		logger: logger,
		seq:    seq,
	}
	return qs
}

// Stream is a long running routine used to stream the results of a serf query back to a client
func (qs *serfQueryResponseStream) Stream(resp *serf.QueryResponse) {
	// Setup a timer for the serf query ending
	remaining := resp.Deadline().Sub(time.Now())
	done := time.After(remaining)

	ackCh := resp.AckCh()
	respCh := resp.ResponseCh()
	for {
		select {
		case a := <-ackCh:
			if err := qs.sendAck(a); err != nil {
				qs.logger.Printf("[ERR] agent.ipc: Failed to stream serf query ack to %v: %v", qs.client, err)
				return
			}
		case r := <-respCh:
			if err := qs.sendResponse(r.From, r.Payload); err != nil {
				qs.logger.Printf("[ERR] agent.ipc: Failed to stream serf query response to %v: %v", qs.client, err)
				return
			}
		case <-done:
			if err := qs.sendDone(); err != nil {
				qs.logger.Printf("[ERR] agent.ipc: Failed to stream serf query end to %v: %v", qs.client, err)
			}
			return
		}
	}
}

// sendAck is used to send a single ack
func (qs *serfQueryResponseStream) sendAck(from string) error {
	header := responseHeader{
		Seq:   qs.seq,
		Error: "",
	}
	rec := serfQueryRecord{
		Type: serfQueryRecordAck,
		From: from,
	}
	return qs.client.Send(&header, &rec)
}

// sendResponse is used to send a single response
func (qs *serfQueryResponseStream) sendResponse(from string, payload []byte) error {
	header := responseHeader{
		Seq:   qs.seq,
		Error: "",
	}
	rec := serfQueryRecord{
		Type:    serfQueryRecordResponse,
		From:    from,
		Payload: payload,
	}
	return qs.client.Send(&header, &rec)
}

// sendDone is used to signal the end
func (qs *serfQueryResponseStream) sendDone() error {
	header := responseHeader{
		Seq:   qs.seq,
		Error: "",
	}
	rec := serfQueryRecord{
		Type: serfQueryRecordDone,
	}
	return qs.client.Send(&header, &rec)
}
