package sarama

import (
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestFuncConsumerOffsetOutOfRange(t *testing.T) {
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	consumer, err := NewConsumer(kafkaBrokers, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := consumer.ConsumePartition("test.1", 0, -10); err != ErrOffsetOutOfRange {
		t.Error("Expected ErrOffsetOutOfRange, got:", err)
	}

	if _, err := consumer.ConsumePartition("test.1", 0, math.MaxInt64); err != ErrOffsetOutOfRange {
		t.Error("Expected ErrOffsetOutOfRange, got:", err)
	}

	safeClose(t, consumer)
}

func TestConsumerHighWaterMarkOffset(t *testing.T) {
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	p, err := NewSyncProducer(kafkaBrokers, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer safeClose(t, p)

	_, offset, err := p.SendMessage(&ProducerMessage{Topic: "test.1", Value: StringEncoder("Test")})
	if err != nil {
		t.Fatal(err)
	}

	c, err := NewConsumer(kafkaBrokers, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer safeClose(t, c)

	pc, err := c.ConsumePartition("test.1", 0, offset)
	if err != nil {
		t.Fatal(err)
	}

	<-pc.Messages()

	if hwmo := pc.HighWaterMarkOffset(); hwmo != offset+1 {
		t.Logf("Last produced offset %d; high water mark should be one higher but found %d.", offset, hwmo)
	}

	safeClose(t, pc)
}

// Makes sure that messages produced by all supported client versions/
// compression codecs (except LZ4) combinations can be consumed by all
// supported consumer versions. It relies on the KAFKA_VERSION environment
// variable to provide the version of the test Kafka cluster.
//
// Note that LZ4 codec was introduced in v0.10.0.0 and therefore is excluded
// from this test case. It has a similar version matrix test case below that
// only checks versions from v0.10.0.0 until KAFKA_VERSION.
func TestVersionMatrix(t *testing.T) {
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	// Produce lot's of message with all possible combinations of supported
	// protocol versions and compressions for the except of LZ4.
	testVersions := versionRange(V0_8_2_0)
	allCodecsButLZ4 := []CompressionCodec{CompressionNone, CompressionGZIP, CompressionSnappy}
	producedMessages := produceMsgs(t, testVersions, allCodecsButLZ4, 17, 100)

	// When/Then
	consumeMsgs(t, testVersions, producedMessages)
}

// Support for LZ4 codec was introduced in v0.10.0.0 so a version matrix to
// test LZ4 should start with v0.10.0.0.
func TestVersionMatrixLZ4(t *testing.T) {
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	// Produce lot's of message with all possible combinations of supported
	// protocol versions starting with v0.10 (first where LZ4 was supported)
	// and all possible compressions.
	testVersions := versionRange(V0_10_0_0)
	allCodecs := []CompressionCodec{CompressionNone, CompressionGZIP, CompressionSnappy, CompressionLZ4}
	producedMessages := produceMsgs(t, testVersions, allCodecs, 17, 100)

	// When/Then
	consumeMsgs(t, testVersions, producedMessages)
}

func prodMsg2Str(prodMsg *ProducerMessage) string {
	return fmt.Sprintf("{offset: %d, value: %s}", prodMsg.Offset, string(prodMsg.Value.(StringEncoder)))
}

func consMsg2Str(consMsg *ConsumerMessage) string {
	return fmt.Sprintf("{offset: %d, value: %s}", consMsg.Offset, string(consMsg.Value))
}

func versionRange(lower KafkaVersion) []KafkaVersion {
	// Get the test cluster version from the environment. If there is nothing
	// there then assume the highest.
	upper, err := ParseKafkaVersion(os.Getenv("KAFKA_VERSION"))
	if err != nil {
		upper = MaxVersion
	}

	versions := make([]KafkaVersion, 0, len(SupportedVersions))
	for _, v := range SupportedVersions {
		if !v.IsAtLeast(lower) {
			continue
		}
		if !upper.IsAtLeast(v) {
			return versions
		}
		versions = append(versions, v)
	}
	return versions
}

func produceMsgs(t *testing.T, clientVersions []KafkaVersion, codecs []CompressionCodec, flush int, countPerVerCodec int) []*ProducerMessage {
	var wg sync.WaitGroup
	var producedMessagesMu sync.Mutex
	var producedMessages []*ProducerMessage
	for _, prodVer := range clientVersions {
		for _, codec := range codecs {
			prodCfg := NewConfig()
			prodCfg.Version = prodVer
			prodCfg.Producer.Return.Successes = true
			prodCfg.Producer.Return.Errors = true
			prodCfg.Producer.Flush.MaxMessages = flush
			prodCfg.Producer.Compression = codec

			p, err := NewSyncProducer(kafkaBrokers, prodCfg)
			if err != nil {
				t.Errorf("Failed to create producer: version=%s, compression=%s, err=%v", prodVer, codec, err)
				continue
			}
			defer safeClose(t, p)
			for i := 0; i < countPerVerCodec; i++ {
				msg := &ProducerMessage{
					Topic: "test.1",
					Value: StringEncoder(fmt.Sprintf("msg:%s:%s:%d", prodVer, codec, i)),
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, _, err := p.SendMessage(msg)
					if err != nil {
						t.Errorf("Failed to produce message: %s, err=%v", msg.Value, err)
					}
					producedMessagesMu.Lock()
					producedMessages = append(producedMessages, msg)
					producedMessagesMu.Unlock()
				}()
			}
		}
	}
	wg.Wait()

	// Sort produced message in ascending offset order.
	sort.Slice(producedMessages, func(i, j int) bool {
		return producedMessages[i].Offset < producedMessages[j].Offset
	})
	t.Logf("*** Total produced %d, firstOffset=%d, lastOffset=%d\n",
		len(producedMessages), producedMessages[0].Offset, producedMessages[len(producedMessages)-1].Offset)
	return producedMessages
}

func consumeMsgs(t *testing.T, clientVersions []KafkaVersion, producedMessages []*ProducerMessage) {
	// Consume all produced messages with all client versions supported by the
	// cluster.
consumerVersionLoop:
	for _, consVer := range clientVersions {
		t.Logf("*** Consuming with client version %s\n", consVer)
		// Create a partition consumer that should start from the first produced
		// message.
		consCfg := NewConfig()
		consCfg.Version = consVer
		c, err := NewConsumer(kafkaBrokers, consCfg)
		if err != nil {
			t.Fatal(err)
		}
		defer safeClose(t, c)
		pc, err := c.ConsumePartition("test.1", 0, producedMessages[0].Offset)
		if err != nil {
			t.Fatal(err)
		}
		defer safeClose(t, pc)

		// Consume as many messages as there have been produced and make sure that
		// order is preserved.
		for i, prodMsg := range producedMessages {
			select {
			case consMsg := <-pc.Messages():
				if consMsg.Offset != prodMsg.Offset {
					t.Errorf("Consumed unexpected offset: version=%s, index=%d, want=%s, got=%s",
						consVer, i, prodMsg2Str(prodMsg), consMsg2Str(consMsg))
					continue consumerVersionLoop
				}
				if string(consMsg.Value) != string(prodMsg.Value.(StringEncoder)) {
					t.Errorf("Consumed unexpected msg: version=%s, index=%d, want=%s, got=%s",
						consVer, i, prodMsg2Str(prodMsg), consMsg2Str(consMsg))
					continue consumerVersionLoop
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("Timeout waiting for: index=%d, offset=%d, msg=%s", i, prodMsg.Offset, prodMsg.Value)
			}
		}
	}
}
