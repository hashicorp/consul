package sarama

import "testing"

var (
	joinGroupRequestNoProtocols = []byte{
		0, 9, 'T', 'e', 's', 't', 'G', 'r', 'o', 'u', 'p', // Group ID
		0, 0, 0, 100, // Session timeout
		0, 0, // Member ID
		0, 8, 'c', 'o', 'n', 's', 'u', 'm', 'e', 'r', // Protocol Type
		0, 0, 0, 0, // 0 protocol groups
	}

	joinGroupRequestOneProtocol = []byte{
		0, 9, 'T', 'e', 's', 't', 'G', 'r', 'o', 'u', 'p', // Group ID
		0, 0, 0, 100, // Session timeout
		0, 11, 'O', 'n', 'e', 'P', 'r', 'o', 't', 'o', 'c', 'o', 'l', // Member ID
		0, 8, 'c', 'o', 'n', 's', 'u', 'm', 'e', 'r', // Protocol Type
		0, 0, 0, 1, // 1 group protocol
		0, 3, 'o', 'n', 'e', // Protocol name
		0, 0, 0, 3, 0x01, 0x02, 0x03, // protocol metadata
	}
)

func TestJoinGroupRequest(t *testing.T) {
	var request *JoinGroupRequest

	request = new(JoinGroupRequest)
	request.GroupId = "TestGroup"
	request.SessionTimeout = 100
	request.ProtocolType = "consumer"
	testRequest(t, "no protocols", request, joinGroupRequestNoProtocols)

	request = new(JoinGroupRequest)
	request.GroupId = "TestGroup"
	request.SessionTimeout = 100
	request.MemberId = "OneProtocol"
	request.ProtocolType = "consumer"
	request.AddGroupProtocol("one", []byte{0x01, 0x02, 0x03})
	testRequest(t, "one protocol", request, joinGroupRequestOneProtocol)
}
