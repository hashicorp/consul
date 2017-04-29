package sarama

import "testing"

var (
	apiVersionResponse = []byte{
		0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x03,
		0x00, 0x02,
		0x00, 0x01,
	}
)

func TestApiVersionsResponse(t *testing.T) {
	var response *ApiVersionsResponse

	response = new(ApiVersionsResponse)
	testVersionDecodable(t, "no error", response, apiVersionResponse, 0)
	if response.Err != ErrNoError {
		t.Error("Decoding error failed: no error expected but found", response.Err)
	}
	if response.ApiVersions[0].ApiKey != 0x03 {
		t.Error("Decoding error: expected 0x03 but got", response.ApiVersions[0].ApiKey)
	}
	if response.ApiVersions[0].MinVersion != 0x02 {
		t.Error("Decoding error: expected 0x02 but got", response.ApiVersions[0].MinVersion)
	}
	if response.ApiVersions[0].MaxVersion != 0x01 {
		t.Error("Decoding error: expected 0x01 but got", response.ApiVersions[0].MaxVersion)
	}
}
