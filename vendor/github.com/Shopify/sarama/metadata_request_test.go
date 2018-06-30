package sarama

import "testing"

var (
	metadataRequestNoTopicsV0 = []byte{
		0x00, 0x00, 0x00, 0x00}

	metadataRequestOneTopicV0 = []byte{
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x06, 't', 'o', 'p', 'i', 'c', '1'}

	metadataRequestThreeTopicsV0 = []byte{
		0x00, 0x00, 0x00, 0x03,
		0x00, 0x03, 'f', 'o', 'o',
		0x00, 0x03, 'b', 'a', 'r',
		0x00, 0x03, 'b', 'a', 'z'}

	metadataRequestNoTopicsV1 = []byte{
		0xff, 0xff, 0xff, 0xff}

	metadataRequestAutoCreateV4   = append(metadataRequestOneTopicV0, byte(1))
	metadataRequestNoAutoCreateV4 = append(metadataRequestOneTopicV0, byte(0))
)

func TestMetadataRequestV0(t *testing.T) {
	request := new(MetadataRequest)
	testRequest(t, "no topics", request, metadataRequestNoTopicsV0)

	request.Topics = []string{"topic1"}
	testRequest(t, "one topic", request, metadataRequestOneTopicV0)

	request.Topics = []string{"foo", "bar", "baz"}
	testRequest(t, "three topics", request, metadataRequestThreeTopicsV0)
}

func TestMetadataRequestV1(t *testing.T) {
	request := new(MetadataRequest)
	request.Version = 1
	testRequest(t, "no topics", request, metadataRequestNoTopicsV1)

	request.Topics = []string{"topic1"}
	testRequest(t, "one topic", request, metadataRequestOneTopicV0)

	request.Topics = []string{"foo", "bar", "baz"}
	testRequest(t, "three topics", request, metadataRequestThreeTopicsV0)
}

func TestMetadataRequestV2(t *testing.T) {
	request := new(MetadataRequest)
	request.Version = 2
	testRequest(t, "no topics", request, metadataRequestNoTopicsV1)

	request.Topics = []string{"topic1"}
	testRequest(t, "one topic", request, metadataRequestOneTopicV0)
}

func TestMetadataRequestV3(t *testing.T) {
	request := new(MetadataRequest)
	request.Version = 3
	testRequest(t, "no topics", request, metadataRequestNoTopicsV1)

	request.Topics = []string{"topic1"}
	testRequest(t, "one topic", request, metadataRequestOneTopicV0)
}

func TestMetadataRequestV4(t *testing.T) {
	request := new(MetadataRequest)
	request.Version = 4
	request.Topics = []string{"topic1"}
	request.AllowAutoTopicCreation = true
	testRequest(t, "one topic", request, metadataRequestAutoCreateV4)

	request.AllowAutoTopicCreation = false
	testRequest(t, "one topic", request, metadataRequestNoAutoCreateV4)
}
