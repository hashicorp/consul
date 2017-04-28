package sarama

import (
	"reflect"
	"testing"
)

var (
	describeGroupsResponseEmpty = []byte{
		0, 0, 0, 0, // no groups
	}

	describeGroupsResponsePopulated = []byte{
		0, 0, 0, 2, // 2 groups

		0, 0, // no error
		0, 3, 'f', 'o', 'o', // Group ID
		0, 3, 'b', 'a', 'r', // State
		0, 8, 'c', 'o', 'n', 's', 'u', 'm', 'e', 'r', // ConsumerProtocol type
		0, 3, 'b', 'a', 'z', // Protocol name
		0, 0, 0, 1, // 1 member
		0, 2, 'i', 'd', // Member ID
		0, 6, 's', 'a', 'r', 'a', 'm', 'a', // Client ID
		0, 9, 'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't', // Client Host
		0, 0, 0, 3, 0x01, 0x02, 0x03, // MemberMetadata
		0, 0, 0, 3, 0x04, 0x05, 0x06, // MemberAssignment

		0, 30, // ErrGroupAuthorizationFailed
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0, 0, 0,
	}
)

func TestDescribeGroupsResponse(t *testing.T) {
	var response *DescribeGroupsResponse

	response = new(DescribeGroupsResponse)
	testVersionDecodable(t, "empty", response, describeGroupsResponseEmpty, 0)
	if len(response.Groups) != 0 {
		t.Error("Expected no groups")
	}

	response = new(DescribeGroupsResponse)
	testVersionDecodable(t, "populated", response, describeGroupsResponsePopulated, 0)
	if len(response.Groups) != 2 {
		t.Error("Expected two groups")
	}

	group0 := response.Groups[0]
	if group0.Err != ErrNoError {
		t.Error("Unxpected groups[0].Err, found", group0.Err)
	}
	if group0.GroupId != "foo" {
		t.Error("Unxpected groups[0].GroupId, found", group0.GroupId)
	}
	if group0.State != "bar" {
		t.Error("Unxpected groups[0].State, found", group0.State)
	}
	if group0.ProtocolType != "consumer" {
		t.Error("Unxpected groups[0].ProtocolType, found", group0.ProtocolType)
	}
	if group0.Protocol != "baz" {
		t.Error("Unxpected groups[0].Protocol, found", group0.Protocol)
	}
	if len(group0.Members) != 1 {
		t.Error("Unxpected groups[0].Members, found", group0.Members)
	}
	if group0.Members["id"].ClientId != "sarama" {
		t.Error("Unxpected groups[0].Members[id].ClientId, found", group0.Members["id"].ClientId)
	}
	if group0.Members["id"].ClientHost != "localhost" {
		t.Error("Unxpected groups[0].Members[id].ClientHost, found", group0.Members["id"].ClientHost)
	}
	if !reflect.DeepEqual(group0.Members["id"].MemberMetadata, []byte{0x01, 0x02, 0x03}) {
		t.Error("Unxpected groups[0].Members[id].MemberMetadata, found", group0.Members["id"].MemberMetadata)
	}
	if !reflect.DeepEqual(group0.Members["id"].MemberAssignment, []byte{0x04, 0x05, 0x06}) {
		t.Error("Unxpected groups[0].Members[id].MemberAssignment, found", group0.Members["id"].MemberAssignment)
	}

	group1 := response.Groups[1]
	if group1.Err != ErrGroupAuthorizationFailed {
		t.Error("Unxpected groups[1].Err, found", group0.Err)
	}
	if len(group1.Members) != 0 {
		t.Error("Unxpected groups[1].Members, found", group0.Members)
	}
}
