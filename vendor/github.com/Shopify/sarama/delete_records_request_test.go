package sarama

import (
	"testing"
	"time"
)

var deleteRecordsRequest = []byte{
	0, 0, 0, 2,
	0, 5, 'o', 't', 'h', 'e', 'r',
	0, 0, 0, 0,
	0, 5, 't', 'o', 'p', 'i', 'c',
	0, 0, 0, 2,
	0, 0, 0, 19,
	0, 0, 0, 0, 0, 0, 0, 200,
	0, 0, 0, 20,
	0, 0, 0, 0, 0, 0, 0, 190,
	0, 0, 0, 100,
}

func TestDeleteRecordsRequest(t *testing.T) {
	req := &DeleteRecordsRequest{
		Topics: map[string]*DeleteRecordsRequestTopic{
			"topic": {
				PartitionOffsets: map[int32]int64{
					19: 200,
					20: 190,
				},
			},
			"other": {},
		},
		Timeout: 100 * time.Millisecond,
	}

	testRequest(t, "", req, deleteRecordsRequest)
}
