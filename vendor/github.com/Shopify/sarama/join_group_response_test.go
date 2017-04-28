package sarama

import (
	"reflect"
	"testing"
)

var (
	joinGroupResponseNoError = []byte{
		0x00, 0x00, // No error
		0x00, 0x01, 0x02, 0x03, // Generation ID
		0, 8, 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l', // Protocol name chosen
		0, 3, 'f', 'o', 'o', // Leader ID
		0, 3, 'b', 'a', 'r', // Member ID
		0, 0, 0, 0, // No member info
	}

	joinGroupResponseWithError = []byte{
		0, 23, // Error: inconsistent group protocol
		0x00, 0x00, 0x00, 0x00, // Generation ID
		0, 0, // Protocol name chosen
		0, 0, // Leader ID
		0, 0, // Member ID
		0, 0, 0, 0, // No member info
	}

	joinGroupResponseLeader = []byte{
		0x00, 0x00, // No error
		0x00, 0x01, 0x02, 0x03, // Generation ID
		0, 8, 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l', // Protocol name chosen
		0, 3, 'f', 'o', 'o', // Leader ID
		0, 3, 'f', 'o', 'o', // Member ID == Leader ID
		0, 0, 0, 1, // 1 member
		0, 3, 'f', 'o', 'o', // Member ID
		0, 0, 0, 3, 0x01, 0x02, 0x03, // Member metadata
	}
)

func TestJoinGroupResponse(t *testing.T) {
	var response *JoinGroupResponse

	response = new(JoinGroupResponse)
	testVersionDecodable(t, "no error", response, joinGroupResponseNoError, 0)
	if response.Err != ErrNoError {
		t.Error("Decoding Err failed: no error expected but found", response.Err)
	}
	if response.GenerationId != 66051 {
		t.Error("Decoding GenerationId failed, found:", response.GenerationId)
	}
	if response.LeaderId != "foo" {
		t.Error("Decoding LeaderId failed, found:", response.LeaderId)
	}
	if response.MemberId != "bar" {
		t.Error("Decoding MemberId failed, found:", response.MemberId)
	}
	if len(response.Members) != 0 {
		t.Error("Decoding Members failed, found:", response.Members)
	}

	response = new(JoinGroupResponse)
	testVersionDecodable(t, "with error", response, joinGroupResponseWithError, 0)
	if response.Err != ErrInconsistentGroupProtocol {
		t.Error("Decoding Err failed: ErrInconsistentGroupProtocol expected but found", response.Err)
	}
	if response.GenerationId != 0 {
		t.Error("Decoding GenerationId failed, found:", response.GenerationId)
	}
	if response.LeaderId != "" {
		t.Error("Decoding LeaderId failed, found:", response.LeaderId)
	}
	if response.MemberId != "" {
		t.Error("Decoding MemberId failed, found:", response.MemberId)
	}
	if len(response.Members) != 0 {
		t.Error("Decoding Members failed, found:", response.Members)
	}

	response = new(JoinGroupResponse)
	testVersionDecodable(t, "with error", response, joinGroupResponseLeader, 0)
	if response.Err != ErrNoError {
		t.Error("Decoding Err failed: ErrNoError expected but found", response.Err)
	}
	if response.GenerationId != 66051 {
		t.Error("Decoding GenerationId failed, found:", response.GenerationId)
	}
	if response.LeaderId != "foo" {
		t.Error("Decoding LeaderId failed, found:", response.LeaderId)
	}
	if response.MemberId != "foo" {
		t.Error("Decoding MemberId failed, found:", response.MemberId)
	}
	if len(response.Members) != 1 {
		t.Error("Decoding Members failed, found:", response.Members)
	}
	if !reflect.DeepEqual(response.Members["foo"], []byte{0x01, 0x02, 0x03}) {
		t.Error("Decoding foo member failed, found:", response.Members["foo"])
	}
}
