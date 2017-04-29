package sarama

import "testing"

var (
	heartbeatResponseNoError = []byte{
		0x00, 0x00}
)

func TestHeartbeatResponse(t *testing.T) {
	var response *HeartbeatResponse

	response = new(HeartbeatResponse)
	testVersionDecodable(t, "no error", response, heartbeatResponseNoError, 0)
	if response.Err != ErrNoError {
		t.Error("Decoding error failed: no error expected but found", response.Err)
	}
}
