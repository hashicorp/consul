package sarama

import (
	"testing"
	"time"
)

var deleteTopicsRequest = []byte{
	0, 0, 0, 2,
	0, 5, 't', 'o', 'p', 'i', 'c',
	0, 5, 'o', 't', 'h', 'e', 'r',
	0, 0, 0, 100,
}

func TestDeleteTopicsRequest(t *testing.T) {
	req := &DeleteTopicsRequest{
		Topics:  []string{"topic", "other"},
		Timeout: 100 * time.Millisecond,
	}

	testRequest(t, "", req, deleteTopicsRequest)
}
