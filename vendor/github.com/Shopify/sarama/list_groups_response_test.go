package sarama

import (
	"testing"
)

var (
	listGroupsResponseEmpty = []byte{
		0, 0, // no error
		0, 0, 0, 0, // no groups
	}

	listGroupsResponseError = []byte{
		0, 31, // no error
		0, 0, 0, 0, // ErrClusterAuthorizationFailed
	}

	listGroupsResponseWithConsumer = []byte{
		0, 0, // no error
		0, 0, 0, 1, // 1 group
		0, 3, 'f', 'o', 'o', // group name
		0, 8, 'c', 'o', 'n', 's', 'u', 'm', 'e', 'r', // protocol type
	}
)

func TestListGroupsResponse(t *testing.T) {
	var response *ListGroupsResponse

	response = new(ListGroupsResponse)
	testVersionDecodable(t, "no error", response, listGroupsResponseEmpty, 0)
	if response.Err != ErrNoError {
		t.Error("Expected no gerror, found:", response.Err)
	}
	if len(response.Groups) != 0 {
		t.Error("Expected no groups")
	}

	response = new(ListGroupsResponse)
	testVersionDecodable(t, "no error", response, listGroupsResponseError, 0)
	if response.Err != ErrClusterAuthorizationFailed {
		t.Error("Expected no gerror, found:", response.Err)
	}
	if len(response.Groups) != 0 {
		t.Error("Expected no groups")
	}

	response = new(ListGroupsResponse)
	testVersionDecodable(t, "no error", response, listGroupsResponseWithConsumer, 0)
	if response.Err != ErrNoError {
		t.Error("Expected no gerror, found:", response.Err)
	}
	if len(response.Groups) != 1 {
		t.Error("Expected one group")
	}
	if response.Groups["foo"] != "consumer" {
		t.Error("Expected foo group to use consumer protocol")
	}
}
