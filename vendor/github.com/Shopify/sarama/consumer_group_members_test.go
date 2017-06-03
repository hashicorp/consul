package sarama

import (
	"bytes"
	"reflect"
	"testing"
)

var (
	groupMemberMetadata = []byte{
		0, 1, // Version
		0, 0, 0, 2, // Topic array length
		0, 3, 'o', 'n', 'e', // Topic one
		0, 3, 't', 'w', 'o', // Topic two
		0, 0, 0, 3, 0x01, 0x02, 0x03, // Userdata
	}
	groupMemberAssignment = []byte{
		0, 1, // Version
		0, 0, 0, 1, // Topic array length
		0, 3, 'o', 'n', 'e', // Topic one
		0, 0, 0, 3, // Topic one, partition array length
		0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 4, // 0, 2, 4
		0, 0, 0, 3, 0x01, 0x02, 0x03, // Userdata
	}
)

func TestConsumerGroupMemberMetadata(t *testing.T) {
	meta := &ConsumerGroupMemberMetadata{
		Version:  1,
		Topics:   []string{"one", "two"},
		UserData: []byte{0x01, 0x02, 0x03},
	}

	buf, err := encode(meta, nil)
	if err != nil {
		t.Error("Failed to encode data", err)
	} else if !bytes.Equal(groupMemberMetadata, buf) {
		t.Errorf("Encoded data does not match expectation\nexpected: %v\nactual: %v", groupMemberMetadata, buf)
	}

	meta2 := new(ConsumerGroupMemberMetadata)
	err = decode(buf, meta2)
	if err != nil {
		t.Error("Failed to decode data", err)
	} else if !reflect.DeepEqual(meta, meta2) {
		t.Errorf("Encoded data does not match expectation\nexpected: %v\nactual: %v", meta, meta2)
	}
}

func TestConsumerGroupMemberAssignment(t *testing.T) {
	amt := &ConsumerGroupMemberAssignment{
		Version: 1,
		Topics: map[string][]int32{
			"one": {0, 2, 4},
		},
		UserData: []byte{0x01, 0x02, 0x03},
	}

	buf, err := encode(amt, nil)
	if err != nil {
		t.Error("Failed to encode data", err)
	} else if !bytes.Equal(groupMemberAssignment, buf) {
		t.Errorf("Encoded data does not match expectation\nexpected: %v\nactual: %v", groupMemberAssignment, buf)
	}

	amt2 := new(ConsumerGroupMemberAssignment)
	err = decode(buf, amt2)
	if err != nil {
		t.Error("Failed to decode data", err)
	} else if !reflect.DeepEqual(amt, amt2) {
		t.Errorf("Encoded data does not match expectation\nexpected: %v\nactual: %v", amt, amt2)
	}
}
