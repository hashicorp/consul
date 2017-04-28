package sarama

import "testing"

func TestListGroupsRequest(t *testing.T) {
	testRequest(t, "ListGroupsRequest", &ListGroupsRequest{}, []byte{})
}
