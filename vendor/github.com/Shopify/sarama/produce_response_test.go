package sarama

import (
	"fmt"
	"testing"
	"time"
)

var (
	produceResponseNoBlocksV0 = []byte{
		0x00, 0x00, 0x00, 0x00}

	produceResponseManyBlocksVersions = [][]byte{
		{
			0x00, 0x00, 0x00, 0x01,

			0x00, 0x03, 'f', 'o', 'o',
			0x00, 0x00, 0x00, 0x01,

			0x00, 0x00, 0x00, 0x01, // Partition 1
			0x00, 0x02, // ErrInvalidMessage
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, // Offset 255
		}, {
			0x00, 0x00, 0x00, 0x01,

			0x00, 0x03, 'f', 'o', 'o',
			0x00, 0x00, 0x00, 0x01,

			0x00, 0x00, 0x00, 0x01, // Partition 1
			0x00, 0x02, // ErrInvalidMessage
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, // Offset 255

			0x00, 0x00, 0x00, 0x64, // 100 ms throttle time
		}, {
			0x00, 0x00, 0x00, 0x01,

			0x00, 0x03, 'f', 'o', 'o',
			0x00, 0x00, 0x00, 0x01,

			0x00, 0x00, 0x00, 0x01, // Partition 1
			0x00, 0x02, // ErrInvalidMessage
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, // Offset 255
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0xE8, // Timestamp January 1st 0001 at 00:00:01,000 UTC (LogAppendTime was used)

			0x00, 0x00, 0x00, 0x64, // 100 ms throttle time
		},
	}
)

func TestProduceResponseDecode(t *testing.T) {
	response := ProduceResponse{}

	testVersionDecodable(t, "no blocks", &response, produceResponseNoBlocksV0, 0)
	if len(response.Blocks) != 0 {
		t.Error("Decoding produced", len(response.Blocks), "topics where there were none")
	}

	for v, produceResponseManyBlocks := range produceResponseManyBlocksVersions {
		t.Logf("Decoding produceResponseManyBlocks version %d", v)
		testVersionDecodable(t, "many blocks", &response, produceResponseManyBlocks, int16(v))
		if len(response.Blocks) != 1 {
			t.Error("Decoding produced", len(response.Blocks), "topics where there was 1")
		}
		if len(response.Blocks["foo"]) != 1 {
			t.Error("Decoding produced", len(response.Blocks["foo"]), "partitions for 'foo' where there was one")
		}
		block := response.GetBlock("foo", 1)
		if block == nil {
			t.Error("Decoding did not produce a block for foo/1")
		} else {
			if block.Err != ErrInvalidMessage {
				t.Error("Decoding failed for foo/2/Err, got:", int16(block.Err))
			}
			if block.Offset != 255 {
				t.Error("Decoding failed for foo/1/Offset, got:", block.Offset)
			}
			if v >= 2 {
				if block.Timestamp != time.Unix(1, 0) {
					t.Error("Decoding failed for foo/2/Timestamp, got:", block.Timestamp)
				}
			}
		}
		if v >= 1 {
			if expected := 100 * time.Millisecond; response.ThrottleTime != expected {
				t.Error("Failed decoding produced throttle time, expected:", expected, ", got:", response.ThrottleTime)
			}
		}
	}
}

func TestProduceResponseEncode(t *testing.T) {
	response := ProduceResponse{}
	response.Blocks = make(map[string]map[int32]*ProduceResponseBlock)
	testEncodable(t, "empty", &response, produceResponseNoBlocksV0)

	response.Blocks["foo"] = make(map[int32]*ProduceResponseBlock)
	response.Blocks["foo"][1] = &ProduceResponseBlock{
		Err:       ErrInvalidMessage,
		Offset:    255,
		Timestamp: time.Unix(1, 0),
	}
	response.ThrottleTime = 100 * time.Millisecond
	for v, produceResponseManyBlocks := range produceResponseManyBlocksVersions {
		response.Version = int16(v)
		testEncodable(t, fmt.Sprintf("many blocks version %d", v), &response, produceResponseManyBlocks)
	}
}

func TestProduceResponseEncodeInvalidTimestamp(t *testing.T) {
	response := ProduceResponse{}
	response.Version = 2
	response.Blocks = make(map[string]map[int32]*ProduceResponseBlock)
	response.Blocks["t"] = make(map[int32]*ProduceResponseBlock)
	response.Blocks["t"][0] = &ProduceResponseBlock{
		Err:    ErrNoError,
		Offset: 0,
		// Use a timestamp before Unix time
		Timestamp: time.Unix(0, 0).Add(-1 * time.Millisecond),
	}
	response.ThrottleTime = 100 * time.Millisecond
	_, err := encode(&response, nil)
	if err == nil {
		t.Error("Expecting error, got nil")
	}
	if _, ok := err.(PacketEncodingError); !ok {
		t.Error("Expecting PacketEncodingError, got:", err)
	}
}
