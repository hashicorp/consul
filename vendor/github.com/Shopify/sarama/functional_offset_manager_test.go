package sarama

import (
	"testing"
)

func TestFuncOffsetManager(t *testing.T) {
	checkKafkaVersion(t, "0.8.2")
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	client, err := NewClient(kafkaBrokers, nil)
	if err != nil {
		t.Fatal(err)
	}

	offsetManager, err := NewOffsetManagerFromClient("sarama.TestFuncOffsetManager", client)
	if err != nil {
		t.Fatal(err)
	}

	pom1, err := offsetManager.ManagePartition("test.1", 0)
	if err != nil {
		t.Fatal(err)
	}

	pom1.MarkOffset(10, "test metadata")
	safeClose(t, pom1)

	pom2, err := offsetManager.ManagePartition("test.1", 0)
	if err != nil {
		t.Fatal(err)
	}

	offset, metadata := pom2.NextOffset()

	if offset != 10 {
		t.Errorf("Expected the next offset to be 10, found %d.", offset)
	}
	if metadata != "test metadata" {
		t.Errorf("Expected metadata to be 'test metadata', found %s.", metadata)
	}

	safeClose(t, pom2)
	safeClose(t, offsetManager)
	safeClose(t, client)
}
