package sarama

import (
	"testing"
	"time"
)

var (
	endTxnResponse = []byte{
		0, 0, 0, 100,
		0, 49,
	}
)

func TestEndTxnResponse(t *testing.T) {
	resp := &EndTxnResponse{
		ThrottleTime: 100 * time.Millisecond,
		Err:          ErrInvalidProducerIDMapping,
	}

	testResponse(t, "", resp, endTxnResponse)
}
