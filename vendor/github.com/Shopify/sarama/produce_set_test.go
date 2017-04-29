package sarama

import (
	"testing"
	"time"
)

func makeProduceSet() (*asyncProducer, *produceSet) {
	parent := &asyncProducer{
		conf: NewConfig(),
	}
	return parent, newProduceSet(parent)
}

func safeAddMessage(t *testing.T, ps *produceSet, msg *ProducerMessage) {
	if err := ps.add(msg); err != nil {
		t.Error(err)
	}
}

func TestProduceSetInitial(t *testing.T) {
	_, ps := makeProduceSet()

	if !ps.empty() {
		t.Error("New produceSet should be empty")
	}

	if ps.readyToFlush() {
		t.Error("Empty produceSet must never be ready to flush")
	}
}

func TestProduceSetAddingMessages(t *testing.T) {
	parent, ps := makeProduceSet()
	parent.conf.Producer.Flush.MaxMessages = 1000

	msg := &ProducerMessage{Key: StringEncoder(TestMessage), Value: StringEncoder(TestMessage)}
	safeAddMessage(t, ps, msg)

	if ps.empty() {
		t.Error("set shouldn't be empty when a message is added")
	}

	if !ps.readyToFlush() {
		t.Error("by default set should be ready to flush when any message is in place")
	}

	for i := 0; i < 999; i++ {
		if ps.wouldOverflow(msg) {
			t.Error("set shouldn't fill up after only", i+1, "messages")
		}
		safeAddMessage(t, ps, msg)
	}

	if !ps.wouldOverflow(msg) {
		t.Error("set should be full after 1000 messages")
	}
}

func TestProduceSetPartitionTracking(t *testing.T) {
	_, ps := makeProduceSet()

	m1 := &ProducerMessage{Topic: "t1", Partition: 0}
	m2 := &ProducerMessage{Topic: "t1", Partition: 1}
	m3 := &ProducerMessage{Topic: "t2", Partition: 0}
	safeAddMessage(t, ps, m1)
	safeAddMessage(t, ps, m2)
	safeAddMessage(t, ps, m3)

	seenT1P0 := false
	seenT1P1 := false
	seenT2P0 := false

	ps.eachPartition(func(topic string, partition int32, msgs []*ProducerMessage) {
		if len(msgs) != 1 {
			t.Error("Wrong message count")
		}

		if topic == "t1" && partition == 0 {
			seenT1P0 = true
		} else if topic == "t1" && partition == 1 {
			seenT1P1 = true
		} else if topic == "t2" && partition == 0 {
			seenT2P0 = true
		}
	})

	if !seenT1P0 {
		t.Error("Didn't see t1p0")
	}
	if !seenT1P1 {
		t.Error("Didn't see t1p1")
	}
	if !seenT2P0 {
		t.Error("Didn't see t2p0")
	}

	if len(ps.dropPartition("t1", 1)) != 1 {
		t.Error("Got wrong messages back from dropping partition")
	}

	if ps.bufferCount != 2 {
		t.Error("Incorrect buffer count after dropping partition")
	}
}

func TestProduceSetRequestBuilding(t *testing.T) {
	parent, ps := makeProduceSet()
	parent.conf.Producer.RequiredAcks = WaitForAll
	parent.conf.Producer.Timeout = 10 * time.Second

	msg := &ProducerMessage{
		Topic:     "t1",
		Partition: 0,
		Key:       StringEncoder(TestMessage),
		Value:     StringEncoder(TestMessage),
	}
	for i := 0; i < 10; i++ {
		safeAddMessage(t, ps, msg)
	}
	msg.Partition = 1
	for i := 0; i < 10; i++ {
		safeAddMessage(t, ps, msg)
	}
	msg.Topic = "t2"
	for i := 0; i < 10; i++ {
		safeAddMessage(t, ps, msg)
	}

	req := ps.buildRequest()

	if req.RequiredAcks != WaitForAll {
		t.Error("RequiredAcks not set properly")
	}

	if req.Timeout != 10000 {
		t.Error("Timeout not set properly")
	}

	if len(req.msgSets) != 2 {
		t.Error("Wrong number of topics in request")
	}
}

func TestProduceSetCompressedRequestBuilding(t *testing.T) {
	parent, ps := makeProduceSet()
	parent.conf.Producer.RequiredAcks = WaitForAll
	parent.conf.Producer.Timeout = 10 * time.Second
	parent.conf.Producer.Compression = CompressionGZIP
	parent.conf.Version = V0_10_0_0

	msg := &ProducerMessage{
		Topic:     "t1",
		Partition: 0,
		Key:       StringEncoder(TestMessage),
		Value:     StringEncoder(TestMessage),
		Timestamp: time.Now(),
	}
	for i := 0; i < 10; i++ {
		safeAddMessage(t, ps, msg)
	}

	req := ps.buildRequest()

	if req.Version != 2 {
		t.Error("Wrong request version")
	}

	for _, msgBlock := range req.msgSets["t1"][0].Messages {
		msg := msgBlock.Msg
		err := msg.decodeSet()
		if err != nil {
			t.Error("Failed to decode set from payload")
		}
		for _, compMsgBlock := range msg.Set.Messages {
			compMsg := compMsgBlock.Msg
			if compMsg.Version != 1 {
				t.Error("Wrong compressed message version")
			}
		}
		if msg.Version != 1 {
			t.Error("Wrong compressed parent message version")
		}
	}
}
