package sarama

import (
	"testing"
)

var (
	emptyDeleteGroupsResponse = []byte{
		0, 0, 0, 0, // does not violate any quota
		0, 0, 0, 0, // no groups
	}

	errorDeleteGroupsResponse = []byte{
		0, 0, 0, 0, // does not violate any quota
		0, 0, 0, 1, // 1 group
		0, 3, 'f', 'o', 'o', // group name
		0, 31, // error ErrClusterAuthorizationFailed
	}

	noErrorDeleteGroupsResponse = []byte{
		0, 0, 0, 0, // does not violate any quota
		0, 0, 0, 1, // 1 group
		0, 3, 'f', 'o', 'o', // group name
		0, 0, // no error
	}
)

func TestDeleteGroupsResponse(t *testing.T) {
	var response *DeleteGroupsResponse

	response = new(DeleteGroupsResponse)
	testVersionDecodable(t, "empty", response, emptyDeleteGroupsResponse, 0)
	if response.ThrottleTime != 0 {
		t.Error("Expected no violation")
	}
	if len(response.GroupErrorCodes) != 0 {
		t.Error("Expected no groups")
	}

	response = new(DeleteGroupsResponse)
	testVersionDecodable(t, "error", response, errorDeleteGroupsResponse, 0)
	if response.ThrottleTime != 0 {
		t.Error("Expected no violation")
	}
	if response.GroupErrorCodes["foo"] != ErrClusterAuthorizationFailed {
		t.Error("Expected error ErrClusterAuthorizationFailed, found:", response.GroupErrorCodes["foo"])
	}

	response = new(DeleteGroupsResponse)
	testVersionDecodable(t, "no error", response, noErrorDeleteGroupsResponse, 0)
	if response.ThrottleTime != 0 {
		t.Error("Expected no violation")
	}
	if response.GroupErrorCodes["foo"] != ErrNoError {
		t.Error("Expected error ErrClusterAuthorizationFailed, found:", response.GroupErrorCodes["foo"])
	}
}
