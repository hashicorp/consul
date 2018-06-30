package sarama

import (
	"bytes"
	"testing"
)

var (
	emptyFetchResponse = []byte{
		0x00, 0x00, 0x00, 0x00}

	oneMessageFetchResponse = []byte{
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x05, 't', 'o', 'p', 'i', 'c',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x05,
		0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x10, 0x10, 0x10, 0x10,
		0x00, 0x00, 0x00, 0x1C,
		// messageSet
		0x00, 0x00, 0x00, 0x00, 0x00, 0x55, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x10,
		// message
		0x23, 0x96, 0x4a, 0xf7, // CRC
		0x00,
		0x00,
		0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x00, 0x00, 0x02, 0x00, 0xEE}

	oneRecordFetchResponse = []byte{
		0x00, 0x00, 0x00, 0x00, // ThrottleTime
		0x00, 0x00, 0x00, 0x01, // Number of Topics
		0x00, 0x05, 't', 'o', 'p', 'i', 'c', // Topic
		0x00, 0x00, 0x00, 0x01, // Number of Partitions
		0x00, 0x00, 0x00, 0x05, // Partition
		0x00, 0x01, // Error
		0x00, 0x00, 0x00, 0x00, 0x10, 0x10, 0x10, 0x10, // High Watermark Offset
		0x00, 0x00, 0x00, 0x00, 0x10, 0x10, 0x10, 0x10, // Last Stable Offset
		0x00, 0x00, 0x00, 0x00, // Number of Aborted Transactions
		0x00, 0x00, 0x00, 0x52, // Records length
		// recordBatch
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x46,
		0x00, 0x00, 0x00, 0x00,
		0x02,
		0xDB, 0x47, 0x14, 0xC9,
		0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0A,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		// record
		0x28,
		0x00,
		0x0A,
		0x00,
		0x08, 0x01, 0x02, 0x03, 0x04,
		0x06, 0x05, 0x06, 0x07,
		0x02,
		0x06, 0x08, 0x09, 0x0A,
		0x04, 0x0B, 0x0C}

	oneMessageFetchResponseV4 = []byte{
		0x00, 0x00, 0x00, 0x00, // ThrottleTime
		0x00, 0x00, 0x00, 0x01, // Number of Topics
		0x00, 0x05, 't', 'o', 'p', 'i', 'c', // Topic
		0x00, 0x00, 0x00, 0x01, // Number of Partitions
		0x00, 0x00, 0x00, 0x05, // Partition
		0x00, 0x01, // Error
		0x00, 0x00, 0x00, 0x00, 0x10, 0x10, 0x10, 0x10, // High Watermark Offset
		0x00, 0x00, 0x00, 0x00, 0x10, 0x10, 0x10, 0x10, // Last Stable Offset
		0x00, 0x00, 0x00, 0x00, // Number of Aborted Transactions
		0x00, 0x00, 0x00, 0x1C,
		// messageSet
		0x00, 0x00, 0x00, 0x00, 0x00, 0x55, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x10,
		// message
		0x23, 0x96, 0x4a, 0xf7, // CRC
		0x00,
		0x00,
		0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x00, 0x00, 0x02, 0x00, 0xEE}
)

func TestEmptyFetchResponse(t *testing.T) {
	response := FetchResponse{}
	testVersionDecodable(t, "empty", &response, emptyFetchResponse, 0)

	if len(response.Blocks) != 0 {
		t.Error("Decoding produced topic blocks where there were none.")
	}

}

func TestOneMessageFetchResponse(t *testing.T) {
	response := FetchResponse{}
	testVersionDecodable(t, "one message", &response, oneMessageFetchResponse, 0)

	if len(response.Blocks) != 1 {
		t.Fatal("Decoding produced incorrect number of topic blocks.")
	}

	if len(response.Blocks["topic"]) != 1 {
		t.Fatal("Decoding produced incorrect number of partition blocks for topic.")
	}

	block := response.GetBlock("topic", 5)
	if block == nil {
		t.Fatal("GetBlock didn't return block.")
	}
	if block.Err != ErrOffsetOutOfRange {
		t.Error("Decoding didn't produce correct error code.")
	}
	if block.HighWaterMarkOffset != 0x10101010 {
		t.Error("Decoding didn't produce correct high water mark offset.")
	}
	partial, err := block.isPartial()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if partial {
		t.Error("Decoding detected a partial trailing message where there wasn't one.")
	}

	n, err := block.numRecords()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatal("Decoding produced incorrect number of messages.")
	}
	msgBlock := block.RecordsSet[0].MsgSet.Messages[0]
	if msgBlock.Offset != 0x550000 {
		t.Error("Decoding produced incorrect message offset.")
	}
	msg := msgBlock.Msg
	if msg.Codec != CompressionNone {
		t.Error("Decoding produced incorrect message compression.")
	}
	if msg.Key != nil {
		t.Error("Decoding produced message key where there was none.")
	}
	if !bytes.Equal(msg.Value, []byte{0x00, 0xEE}) {
		t.Error("Decoding produced incorrect message value.")
	}
}

func TestOneRecordFetchResponse(t *testing.T) {
	response := FetchResponse{}
	testVersionDecodable(t, "one record", &response, oneRecordFetchResponse, 4)

	if len(response.Blocks) != 1 {
		t.Fatal("Decoding produced incorrect number of topic blocks.")
	}

	if len(response.Blocks["topic"]) != 1 {
		t.Fatal("Decoding produced incorrect number of partition blocks for topic.")
	}

	block := response.GetBlock("topic", 5)
	if block == nil {
		t.Fatal("GetBlock didn't return block.")
	}
	if block.Err != ErrOffsetOutOfRange {
		t.Error("Decoding didn't produce correct error code.")
	}
	if block.HighWaterMarkOffset != 0x10101010 {
		t.Error("Decoding didn't produce correct high water mark offset.")
	}
	partial, err := block.isPartial()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if partial {
		t.Error("Decoding detected a partial trailing record where there wasn't one.")
	}

	n, err := block.numRecords()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatal("Decoding produced incorrect number of records.")
	}
	rec := block.RecordsSet[0].RecordBatch.Records[0]
	if !bytes.Equal(rec.Key, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Error("Decoding produced incorrect record key.")
	}
	if !bytes.Equal(rec.Value, []byte{0x05, 0x06, 0x07}) {
		t.Error("Decoding produced incorrect record value.")
	}
}

func TestOneMessageFetchResponseV4(t *testing.T) {
	response := FetchResponse{}
	testVersionDecodable(t, "one message v4", &response, oneMessageFetchResponseV4, 4)

	if len(response.Blocks) != 1 {
		t.Fatal("Decoding produced incorrect number of topic blocks.")
	}

	if len(response.Blocks["topic"]) != 1 {
		t.Fatal("Decoding produced incorrect number of partition blocks for topic.")
	}

	block := response.GetBlock("topic", 5)
	if block == nil {
		t.Fatal("GetBlock didn't return block.")
	}
	if block.Err != ErrOffsetOutOfRange {
		t.Error("Decoding didn't produce correct error code.")
	}
	if block.HighWaterMarkOffset != 0x10101010 {
		t.Error("Decoding didn't produce correct high water mark offset.")
	}
	partial, err := block.isPartial()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if partial {
		t.Error("Decoding detected a partial trailing record where there wasn't one.")
	}

	n, err := block.numRecords()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatal("Decoding produced incorrect number of records.")
	}
	msgBlock := block.RecordsSet[0].MsgSet.Messages[0]
	if msgBlock.Offset != 0x550000 {
		t.Error("Decoding produced incorrect message offset.")
	}
	msg := msgBlock.Msg
	if msg.Codec != CompressionNone {
		t.Error("Decoding produced incorrect message compression.")
	}
	if msg.Key != nil {
		t.Error("Decoding produced message key where there was none.")
	}
	if !bytes.Equal(msg.Value, []byte{0x00, 0xEE}) {
		t.Error("Decoding produced incorrect message value.")
	}
}
