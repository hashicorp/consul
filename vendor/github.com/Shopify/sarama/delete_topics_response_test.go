package sarama

import (
	"testing"
	"time"
)

var (
	deleteTopicsResponseV0 = []byte{
		0, 0, 0, 1,
		0, 5, 't', 'o', 'p', 'i', 'c',
		0, 0,
	}

	deleteTopicsResponseV1 = []byte{
		0, 0, 0, 100,
		0, 0, 0, 1,
		0, 5, 't', 'o', 'p', 'i', 'c',
		0, 0,
	}
)

func TestDeleteTopicsResponse(t *testing.T) {
	resp := &DeleteTopicsResponse{
		TopicErrorCodes: map[string]KError{
			"topic": ErrNoError,
		},
	}

	testResponse(t, "version 0", resp, deleteTopicsResponseV0)

	resp.Version = 1
	resp.ThrottleTime = 100 * time.Millisecond

	testResponse(t, "version 1", resp, deleteTopicsResponseV1)
}
