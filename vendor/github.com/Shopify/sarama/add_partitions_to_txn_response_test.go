package sarama

import (
	"testing"
	"time"
)

var (
	addPartitionsToTxnResponse = []byte{
		0, 0, 0, 100,
		0, 0, 0, 1,
		0, 5, 't', 'o', 'p', 'i', 'c',
		0, 0, 0, 1, // 1 partition error
		0, 0, 0, 2, // partition 2
		0, 48, // error
	}
)

func TestAddPartitionsToTxnResponse(t *testing.T) {
	resp := &AddPartitionsToTxnResponse{
		ThrottleTime: 100 * time.Millisecond,
		Errors: map[string][]*PartitionError{
			"topic": []*PartitionError{&PartitionError{
				Err:       ErrInvalidTxnState,
				Partition: 2,
			}},
		},
	}

	testResponse(t, "", resp, addPartitionsToTxnResponse)
}
