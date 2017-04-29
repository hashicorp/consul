package sarama

import "testing"

var (
	emptyOffsetResponse = []byte{
		0x00, 0x00, 0x00, 0x00}

	normalOffsetResponse = []byte{
		0x00, 0x00, 0x00, 0x02,

		0x00, 0x01, 'a',
		0x00, 0x00, 0x00, 0x00,

		0x00, 0x01, 'z',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
		0x00, 0x00,
		0x00, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06}

	normalOffsetResponseV1 = []byte{
		0x00, 0x00, 0x00, 0x02,

		0x00, 0x01, 'a',
		0x00, 0x00, 0x00, 0x00,

		0x00, 0x01, 'z',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
		0x00, 0x00,
		0x00, 0x00, 0x01, 0x58, 0x1A, 0xE6, 0x48, 0x86,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06}
)

func TestEmptyOffsetResponse(t *testing.T) {
	response := OffsetResponse{}

	testVersionDecodable(t, "empty", &response, emptyOffsetResponse, 0)
	if len(response.Blocks) != 0 {
		t.Error("Decoding produced", len(response.Blocks), "topics where there were none.")
	}

	response = OffsetResponse{}

	testVersionDecodable(t, "empty", &response, emptyOffsetResponse, 1)
	if len(response.Blocks) != 0 {
		t.Error("Decoding produced", len(response.Blocks), "topics where there were none.")
	}
}

func TestNormalOffsetResponse(t *testing.T) {
	response := OffsetResponse{}

	testVersionDecodable(t, "normal", &response, normalOffsetResponse, 0)

	if len(response.Blocks) != 2 {
		t.Fatal("Decoding produced", len(response.Blocks), "topics where there were two.")
	}

	if len(response.Blocks["a"]) != 0 {
		t.Fatal("Decoding produced", len(response.Blocks["a"]), "partitions for topic 'a' where there were none.")
	}

	if len(response.Blocks["z"]) != 1 {
		t.Fatal("Decoding produced", len(response.Blocks["z"]), "partitions for topic 'z' where there was one.")
	}

	if response.Blocks["z"][2].Err != ErrNoError {
		t.Fatal("Decoding produced invalid error for topic z partition 2.")
	}

	if len(response.Blocks["z"][2].Offsets) != 2 {
		t.Fatal("Decoding produced invalid number of offsets for topic z partition 2.")
	}

	if response.Blocks["z"][2].Offsets[0] != 5 || response.Blocks["z"][2].Offsets[1] != 6 {
		t.Fatal("Decoding produced invalid offsets for topic z partition 2.")
	}
}

func TestNormalOffsetResponseV1(t *testing.T) {
	response := OffsetResponse{}

	testVersionDecodable(t, "normal", &response, normalOffsetResponseV1, 1)

	if len(response.Blocks) != 2 {
		t.Fatal("Decoding produced", len(response.Blocks), "topics where there were two.")
	}

	if len(response.Blocks["a"]) != 0 {
		t.Fatal("Decoding produced", len(response.Blocks["a"]), "partitions for topic 'a' where there were none.")
	}

	if len(response.Blocks["z"]) != 1 {
		t.Fatal("Decoding produced", len(response.Blocks["z"]), "partitions for topic 'z' where there was one.")
	}

	if response.Blocks["z"][2].Err != ErrNoError {
		t.Fatal("Decoding produced invalid error for topic z partition 2.")
	}

	if response.Blocks["z"][2].Timestamp != 1477920049286 {
		t.Fatal("Decoding produced invalid timestamp for topic z partition 2.", response.Blocks["z"][2].Timestamp)
	}

	if response.Blocks["z"][2].Offset != 6 {
		t.Fatal("Decoding produced invalid offsets for topic z partition 2.")
	}
}
