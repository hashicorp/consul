package sarama

import (
	"reflect"
	"testing"
	"time"
)

var (
	createPartitionResponseSuccess = []byte{
		0, 0, 0, 100, // throttleTimeMs
		0, 0, 0, 1,
		0, 5, 't', 'o', 'p', 'i', 'c',
		0, 0, // no error
		255, 255, // no error message
	}

	createPartitionResponseFail = []byte{
		0, 0, 0, 100, // throttleTimeMs
		0, 0, 0, 1,
		0, 5, 't', 'o', 'p', 'i', 'c',
		0, 37, // partition error
		0, 5, 'e', 'r', 'r', 'o', 'r',
	}
)

func TestCreatePartitionsResponse(t *testing.T) {
	resp := &CreatePartitionsResponse{
		ThrottleTime: 100 * time.Millisecond,
		TopicPartitionErrors: map[string]*TopicPartitionError{
			"topic": &TopicPartitionError{},
		},
	}

	testResponse(t, "success", resp, createPartitionResponseSuccess)
	decodedresp := new(CreatePartitionsResponse)
	testVersionDecodable(t, "success", decodedresp, createPartitionResponseSuccess, 0)
	if !reflect.DeepEqual(decodedresp, resp) {
		t.Errorf("Decoding error: expected %v but got %v", decodedresp, resp)
	}

	errMsg := "error"
	resp.TopicPartitionErrors["topic"].Err = ErrInvalidPartitions
	resp.TopicPartitionErrors["topic"].ErrMsg = &errMsg

	testResponse(t, "with errors", resp, createPartitionResponseFail)
	decodedresp = new(CreatePartitionsResponse)
	testVersionDecodable(t, "with errors", decodedresp, createPartitionResponseFail, 0)
	if !reflect.DeepEqual(decodedresp, resp) {
		t.Errorf("Decoding error: expected %v but got %v", decodedresp, resp)
	}
}
