package sarama

import "testing"

var (
	consumerMetadataResponseError = []byte{
		0x00, 0x0E,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00, 0x00, 0x00}

	consumerMetadataResponseSuccess = []byte{
		0x00, 0x00,
		0x00, 0x00, 0x00, 0xAB,
		0x00, 0x03, 'f', 'o', 'o',
		0x00, 0x00, 0xCC, 0xDD}
)

func TestConsumerMetadataResponseError(t *testing.T) {
	response := &ConsumerMetadataResponse{Err: ErrOffsetsLoadInProgress}
	testEncodable(t, "", response, consumerMetadataResponseError)

	decodedResp := &ConsumerMetadataResponse{}
	if err := versionedDecode(consumerMetadataResponseError, decodedResp, 0); err != nil {
		t.Error("could not decode: ", err)
	}

	if decodedResp.Err != ErrOffsetsLoadInProgress {
		t.Errorf("got %s, want %s", decodedResp.Err, ErrOffsetsLoadInProgress)
	}
}

func TestConsumerMetadataResponseSuccess(t *testing.T) {
	broker := NewBroker("foo:52445")
	broker.id = 0xAB
	response := ConsumerMetadataResponse{
		Coordinator:     broker,
		CoordinatorID:   0xAB,
		CoordinatorHost: "foo",
		CoordinatorPort: 0xCCDD,
		Err:             ErrNoError,
	}
	testResponse(t, "success", &response, consumerMetadataResponseSuccess)
}
