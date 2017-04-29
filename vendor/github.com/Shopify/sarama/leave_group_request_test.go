package sarama

import "testing"

var (
	basicLeaveGroupRequest = []byte{
		0, 3, 'f', 'o', 'o',
		0, 3, 'b', 'a', 'r',
	}
)

func TestLeaveGroupRequest(t *testing.T) {
	var request *LeaveGroupRequest

	request = new(LeaveGroupRequest)
	request.GroupId = "foo"
	request.MemberId = "bar"
	testRequest(t, "basic", request, basicLeaveGroupRequest)
}
