package sarama

import (
	"reflect"
	"testing"
)

var (
	syncGroupResponseNoError = []byte{
		0x00, 0x00, // No error
		0, 0, 0, 3, 0x01, 0x02, 0x03, // Member assignment data
	}

	syncGroupResponseWithError = []byte{
		0, 27, // ErrRebalanceInProgress
		0, 0, 0, 0, // No member assignment data
	}
)

func TestSyncGroupResponse(t *testing.T) {
	var response *SyncGroupResponse

	response = new(SyncGroupResponse)
	testVersionDecodable(t, "no error", response, syncGroupResponseNoError, 0)
	if response.Err != ErrNoError {
		t.Error("Decoding Err failed: no error expected but found", response.Err)
	}
	if !reflect.DeepEqual(response.MemberAssignment, []byte{0x01, 0x02, 0x03}) {
		t.Error("Decoding MemberAssignment failed, found:", response.MemberAssignment)
	}

	response = new(SyncGroupResponse)
	testVersionDecodable(t, "no error", response, syncGroupResponseWithError, 0)
	if response.Err != ErrRebalanceInProgress {
		t.Error("Decoding Err failed: ErrRebalanceInProgress expected but found", response.Err)
	}
	if !reflect.DeepEqual(response.MemberAssignment, []byte{}) {
		t.Error("Decoding MemberAssignment failed, found:", response.MemberAssignment)
	}
}
