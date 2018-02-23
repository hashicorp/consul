package sarama

import (
	"testing"
	"time"
)

var (
	addOffsetsToTxnResponse = []byte{
		0, 0, 0, 100,
		0, 47,
	}
)

func TestAddOffsetsToTxnResponse(t *testing.T) {
	resp := &AddOffsetsToTxnResponse{
		ThrottleTime: 100 * time.Millisecond,
		Err:          ErrInvalidProducerEpoch,
	}

	testResponse(t, "", resp, addOffsetsToTxnResponse)
}
