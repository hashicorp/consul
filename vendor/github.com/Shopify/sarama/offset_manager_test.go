package sarama

import (
	"testing"
	"time"
)

func initOffsetManager(t *testing.T) (om OffsetManager,
	testClient Client, broker, coordinator *MockBroker) {

	config := NewConfig()
	config.Metadata.Retry.Max = 1
	config.Consumer.Offsets.CommitInterval = 1 * time.Millisecond
	config.Version = V0_9_0_0

	broker = NewMockBroker(t, 1)
	coordinator = NewMockBroker(t, 2)

	seedMeta := new(MetadataResponse)
	seedMeta.AddBroker(coordinator.Addr(), coordinator.BrokerID())
	seedMeta.AddTopicPartition("my_topic", 0, 1, []int32{}, []int32{}, ErrNoError)
	seedMeta.AddTopicPartition("my_topic", 1, 1, []int32{}, []int32{}, ErrNoError)
	broker.Returns(seedMeta)

	var err error
	testClient, err = NewClient([]string{broker.Addr()}, config)
	if err != nil {
		t.Fatal(err)
	}

	broker.Returns(&ConsumerMetadataResponse{
		CoordinatorID:   coordinator.BrokerID(),
		CoordinatorHost: "127.0.0.1",
		CoordinatorPort: coordinator.Port(),
	})

	om, err = NewOffsetManagerFromClient("group", testClient)
	if err != nil {
		t.Fatal(err)
	}

	return om, testClient, broker, coordinator
}

func initPartitionOffsetManager(t *testing.T, om OffsetManager,
	coordinator *MockBroker, initialOffset int64, metadata string) PartitionOffsetManager {

	fetchResponse := new(OffsetFetchResponse)
	fetchResponse.AddBlock("my_topic", 0, &OffsetFetchResponseBlock{
		Err:      ErrNoError,
		Offset:   initialOffset,
		Metadata: metadata,
	})
	coordinator.Returns(fetchResponse)

	pom, err := om.ManagePartition("my_topic", 0)
	if err != nil {
		t.Fatal(err)
	}

	return pom
}

func TestNewOffsetManager(t *testing.T) {
	seedBroker := NewMockBroker(t, 1)
	seedBroker.Returns(new(MetadataResponse))

	testClient, err := NewClient([]string{seedBroker.Addr()}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewOffsetManagerFromClient("group", testClient)
	if err != nil {
		t.Error(err)
	}

	safeClose(t, testClient)

	_, err = NewOffsetManagerFromClient("group", testClient)
	if err != ErrClosedClient {
		t.Errorf("Error expected for closed client; actual value: %v", err)
	}

	seedBroker.Close()
}

// Test recovery from ErrNotCoordinatorForConsumer
// on first fetchInitialOffset call
func TestOffsetManagerFetchInitialFail(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)

	// Error on first fetchInitialOffset call
	responseBlock := OffsetFetchResponseBlock{
		Err:      ErrNotCoordinatorForConsumer,
		Offset:   5,
		Metadata: "test_meta",
	}

	fetchResponse := new(OffsetFetchResponse)
	fetchResponse.AddBlock("my_topic", 0, &responseBlock)
	coordinator.Returns(fetchResponse)

	// Refresh coordinator
	newCoordinator := NewMockBroker(t, 3)
	broker.Returns(&ConsumerMetadataResponse{
		CoordinatorID:   newCoordinator.BrokerID(),
		CoordinatorHost: "127.0.0.1",
		CoordinatorPort: newCoordinator.Port(),
	})

	// Second fetchInitialOffset call is fine
	fetchResponse2 := new(OffsetFetchResponse)
	responseBlock2 := responseBlock
	responseBlock2.Err = ErrNoError
	fetchResponse2.AddBlock("my_topic", 0, &responseBlock2)
	newCoordinator.Returns(fetchResponse2)

	pom, err := om.ManagePartition("my_topic", 0)
	if err != nil {
		t.Error(err)
	}

	broker.Close()
	coordinator.Close()
	newCoordinator.Close()
	safeClose(t, pom)
	safeClose(t, om)
	safeClose(t, testClient)
}

// Test fetchInitialOffset retry on ErrOffsetsLoadInProgress
func TestOffsetManagerFetchInitialLoadInProgress(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)

	// Error on first fetchInitialOffset call
	responseBlock := OffsetFetchResponseBlock{
		Err:      ErrOffsetsLoadInProgress,
		Offset:   5,
		Metadata: "test_meta",
	}

	fetchResponse := new(OffsetFetchResponse)
	fetchResponse.AddBlock("my_topic", 0, &responseBlock)
	coordinator.Returns(fetchResponse)

	// Second fetchInitialOffset call is fine
	fetchResponse2 := new(OffsetFetchResponse)
	responseBlock2 := responseBlock
	responseBlock2.Err = ErrNoError
	fetchResponse2.AddBlock("my_topic", 0, &responseBlock2)
	coordinator.Returns(fetchResponse2)

	pom, err := om.ManagePartition("my_topic", 0)
	if err != nil {
		t.Error(err)
	}

	broker.Close()
	coordinator.Close()
	safeClose(t, pom)
	safeClose(t, om)
	safeClose(t, testClient)
}

func TestPartitionOffsetManagerInitialOffset(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	testClient.Config().Consumer.Offsets.Initial = OffsetOldest

	// Kafka returns -1 if no offset has been stored for this partition yet.
	pom := initPartitionOffsetManager(t, om, coordinator, -1, "")

	offset, meta := pom.NextOffset()
	if offset != OffsetOldest {
		t.Errorf("Expected offset 5. Actual: %v", offset)
	}
	if meta != "" {
		t.Errorf("Expected metadata to be empty. Actual: %q", meta)
	}

	safeClose(t, pom)
	safeClose(t, om)
	broker.Close()
	coordinator.Close()
	safeClose(t, testClient)
}

func TestPartitionOffsetManagerNextOffset(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	pom := initPartitionOffsetManager(t, om, coordinator, 5, "test_meta")

	offset, meta := pom.NextOffset()
	if offset != 5 {
		t.Errorf("Expected offset 5. Actual: %v", offset)
	}
	if meta != "test_meta" {
		t.Errorf("Expected metadata \"test_meta\". Actual: %q", meta)
	}

	safeClose(t, pom)
	safeClose(t, om)
	broker.Close()
	coordinator.Close()
	safeClose(t, testClient)
}

func TestPartitionOffsetManagerResetOffset(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	pom := initPartitionOffsetManager(t, om, coordinator, 5, "original_meta")

	ocResponse := new(OffsetCommitResponse)
	ocResponse.AddError("my_topic", 0, ErrNoError)
	coordinator.Returns(ocResponse)

	expected := int64(1)
	pom.ResetOffset(expected, "modified_meta")
	actual, meta := pom.NextOffset()

	if actual != expected {
		t.Errorf("Expected offset %v. Actual: %v", expected, actual)
	}
	if meta != "modified_meta" {
		t.Errorf("Expected metadata \"modified_meta\". Actual: %q", meta)
	}

	safeClose(t, pom)
	safeClose(t, om)
	safeClose(t, testClient)
	broker.Close()
	coordinator.Close()
}

func TestPartitionOffsetManagerResetOffsetWithRetention(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	testClient.Config().Consumer.Offsets.Retention = time.Hour

	pom := initPartitionOffsetManager(t, om, coordinator, 5, "original_meta")

	ocResponse := new(OffsetCommitResponse)
	ocResponse.AddError("my_topic", 0, ErrNoError)
	handler := func(req *request) (res encoder) {
		if req.body.version() != 2 {
			t.Errorf("Expected to be using version 2. Actual: %v", req.body.version())
		}
		offsetCommitRequest := req.body.(*OffsetCommitRequest)
		if offsetCommitRequest.RetentionTime != (60 * 60 * 1000) {
			t.Errorf("Expected an hour retention time. Actual: %v", offsetCommitRequest.RetentionTime)
		}
		return ocResponse
	}
	coordinator.setHandler(handler)

	expected := int64(1)
	pom.ResetOffset(expected, "modified_meta")
	actual, meta := pom.NextOffset()

	if actual != expected {
		t.Errorf("Expected offset %v. Actual: %v", expected, actual)
	}
	if meta != "modified_meta" {
		t.Errorf("Expected metadata \"modified_meta\". Actual: %q", meta)
	}

	safeClose(t, pom)
	safeClose(t, om)
	safeClose(t, testClient)
	broker.Close()
	coordinator.Close()
}

func TestPartitionOffsetManagerMarkOffset(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	pom := initPartitionOffsetManager(t, om, coordinator, 5, "original_meta")

	ocResponse := new(OffsetCommitResponse)
	ocResponse.AddError("my_topic", 0, ErrNoError)
	coordinator.Returns(ocResponse)

	pom.MarkOffset(100, "modified_meta")
	offset, meta := pom.NextOffset()

	if offset != 100 {
		t.Errorf("Expected offset 100. Actual: %v", offset)
	}
	if meta != "modified_meta" {
		t.Errorf("Expected metadata \"modified_meta\". Actual: %q", meta)
	}

	safeClose(t, pom)
	safeClose(t, om)
	safeClose(t, testClient)
	broker.Close()
	coordinator.Close()
}

func TestPartitionOffsetManagerMarkOffsetWithRetention(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	testClient.Config().Consumer.Offsets.Retention = time.Hour

	pom := initPartitionOffsetManager(t, om, coordinator, 5, "original_meta")

	ocResponse := new(OffsetCommitResponse)
	ocResponse.AddError("my_topic", 0, ErrNoError)
	handler := func(req *request) (res encoder) {
		if req.body.version() != 2 {
			t.Errorf("Expected to be using version 2. Actual: %v", req.body.version())
		}
		offsetCommitRequest := req.body.(*OffsetCommitRequest)
		if offsetCommitRequest.RetentionTime != (60 * 60 * 1000) {
			t.Errorf("Expected an hour retention time. Actual: %v", offsetCommitRequest.RetentionTime)
		}
		return ocResponse
	}
	coordinator.setHandler(handler)

	pom.MarkOffset(100, "modified_meta")
	offset, meta := pom.NextOffset()

	if offset != 100 {
		t.Errorf("Expected offset 100. Actual: %v", offset)
	}
	if meta != "modified_meta" {
		t.Errorf("Expected metadata \"modified_meta\". Actual: %q", meta)
	}

	safeClose(t, pom)
	safeClose(t, om)
	safeClose(t, testClient)
	broker.Close()
	coordinator.Close()
}

func TestPartitionOffsetManagerCommitErr(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	pom := initPartitionOffsetManager(t, om, coordinator, 5, "meta")

	// Error on one partition
	ocResponse := new(OffsetCommitResponse)
	ocResponse.AddError("my_topic", 0, ErrOffsetOutOfRange)
	ocResponse.AddError("my_topic", 1, ErrNoError)
	coordinator.Returns(ocResponse)

	newCoordinator := NewMockBroker(t, 3)

	// For RefreshCoordinator()
	broker.Returns(&ConsumerMetadataResponse{
		CoordinatorID:   newCoordinator.BrokerID(),
		CoordinatorHost: "127.0.0.1",
		CoordinatorPort: newCoordinator.Port(),
	})

	// Nothing in response.Errors at all
	ocResponse2 := new(OffsetCommitResponse)
	newCoordinator.Returns(ocResponse2)

	// For RefreshCoordinator()
	broker.Returns(&ConsumerMetadataResponse{
		CoordinatorID:   newCoordinator.BrokerID(),
		CoordinatorHost: "127.0.0.1",
		CoordinatorPort: newCoordinator.Port(),
	})

	// Error on the wrong partition for this pom
	ocResponse3 := new(OffsetCommitResponse)
	ocResponse3.AddError("my_topic", 1, ErrNoError)
	newCoordinator.Returns(ocResponse3)

	// For RefreshCoordinator()
	broker.Returns(&ConsumerMetadataResponse{
		CoordinatorID:   newCoordinator.BrokerID(),
		CoordinatorHost: "127.0.0.1",
		CoordinatorPort: newCoordinator.Port(),
	})

	// ErrUnknownTopicOrPartition/ErrNotLeaderForPartition/ErrLeaderNotAvailable block
	ocResponse4 := new(OffsetCommitResponse)
	ocResponse4.AddError("my_topic", 0, ErrUnknownTopicOrPartition)
	newCoordinator.Returns(ocResponse4)

	// For RefreshCoordinator()
	broker.Returns(&ConsumerMetadataResponse{
		CoordinatorID:   newCoordinator.BrokerID(),
		CoordinatorHost: "127.0.0.1",
		CoordinatorPort: newCoordinator.Port(),
	})

	// Normal error response
	ocResponse5 := new(OffsetCommitResponse)
	ocResponse5.AddError("my_topic", 0, ErrNoError)
	newCoordinator.Returns(ocResponse5)

	pom.MarkOffset(100, "modified_meta")

	err := pom.Close()
	if err != nil {
		t.Error(err)
	}

	broker.Close()
	coordinator.Close()
	newCoordinator.Close()
	safeClose(t, om)
	safeClose(t, testClient)
}

// Test of recovery from abort
func TestAbortPartitionOffsetManager(t *testing.T) {
	om, testClient, broker, coordinator := initOffsetManager(t)
	pom := initPartitionOffsetManager(t, om, coordinator, 5, "meta")

	// this triggers an error in the CommitOffset request,
	// which leads to the abort call
	coordinator.Close()

	// Response to refresh coordinator request
	newCoordinator := NewMockBroker(t, 3)
	broker.Returns(&ConsumerMetadataResponse{
		CoordinatorID:   newCoordinator.BrokerID(),
		CoordinatorHost: "127.0.0.1",
		CoordinatorPort: newCoordinator.Port(),
	})

	ocResponse := new(OffsetCommitResponse)
	ocResponse.AddError("my_topic", 0, ErrNoError)
	newCoordinator.Returns(ocResponse)

	pom.MarkOffset(100, "modified_meta")

	safeClose(t, pom)
	safeClose(t, om)
	broker.Close()
	safeClose(t, testClient)
}
