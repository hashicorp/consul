package sarama

import (
	"testing"
	"time"
)

var (
	createPartitionRequestNoAssignment = []byte{
		0, 0, 0, 1, // one topic
		0, 5, 't', 'o', 'p', 'i', 'c',
		0, 0, 0, 3, // 3 partitions
		255, 255, 255, 255, // no assignments
		0, 0, 0, 100, // timeout
		0, // validate only = false
	}

	createPartitionRequestAssignment = []byte{
		0, 0, 0, 1,
		0, 5, 't', 'o', 'p', 'i', 'c',
		0, 0, 0, 3, // 3 partitions
		0, 0, 0, 2,
		0, 0, 0, 2,
		0, 0, 0, 2, 0, 0, 0, 3,
		0, 0, 0, 2,
		0, 0, 0, 3, 0, 0, 0, 1,
		0, 0, 0, 100,
		1, // validate only = true
	}
)

func TestCreatePartitionsRequest(t *testing.T) {
	req := &CreatePartitionsRequest{
		TopicPartitions: map[string]*TopicPartition{
			"topic": &TopicPartition{
				Count: 3,
			},
		},
		Timeout: 100 * time.Millisecond,
	}

	buf := testRequestEncode(t, "no assignment", req, createPartitionRequestNoAssignment)
	testRequestDecode(t, "no assignment", req, buf)

	req.ValidateOnly = true
	req.TopicPartitions["topic"].Assignment = [][]int32{{2, 3}, {3, 1}}

	buf = testRequestEncode(t, "assignment", req, createPartitionRequestAssignment)
	testRequestDecode(t, "assignment", req, buf)
}
