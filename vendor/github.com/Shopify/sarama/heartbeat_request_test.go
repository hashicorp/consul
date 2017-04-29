package sarama

import "testing"

var (
	basicHeartbeatRequest = []byte{
		0, 3, 'f', 'o', 'o', // Group ID
		0x00, 0x01, 0x02, 0x03, // Generatiuon ID
		0, 3, 'b', 'a', 'z', // Member ID
	}
)

func TestHeartbeatRequest(t *testing.T) {
	var request *HeartbeatRequest

	request = new(HeartbeatRequest)
	request.GroupId = "foo"
	request.GenerationId = 66051
	request.MemberId = "baz"
	testRequest(t, "basic", request, basicHeartbeatRequest)
}
