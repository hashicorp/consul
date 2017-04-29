package sarama

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	toxiproxy "github.com/Shopify/toxiproxy/client"
	"github.com/rcrowley/go-metrics"
)

const TestBatchSize = 1000

func TestFuncProducing(t *testing.T) {
	config := NewConfig()
	testProducingMessages(t, config)
}

func TestFuncProducingGzip(t *testing.T) {
	config := NewConfig()
	config.Producer.Compression = CompressionGZIP
	testProducingMessages(t, config)
}

func TestFuncProducingSnappy(t *testing.T) {
	config := NewConfig()
	config.Producer.Compression = CompressionSnappy
	testProducingMessages(t, config)
}

func TestFuncProducingNoResponse(t *testing.T) {
	config := NewConfig()
	config.Producer.RequiredAcks = NoResponse
	testProducingMessages(t, config)
}

func TestFuncProducingFlushing(t *testing.T) {
	config := NewConfig()
	config.Producer.Flush.Messages = TestBatchSize / 8
	config.Producer.Flush.Frequency = 250 * time.Millisecond
	testProducingMessages(t, config)
}

func TestFuncMultiPartitionProduce(t *testing.T) {
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	config := NewConfig()
	config.ChannelBufferSize = 20
	config.Producer.Flush.Frequency = 50 * time.Millisecond
	config.Producer.Flush.Messages = 200
	config.Producer.Return.Successes = true
	producer, err := NewSyncProducer(kafkaBrokers, config)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(TestBatchSize)

	for i := 1; i <= TestBatchSize; i++ {
		go func(i int) {
			defer wg.Done()
			msg := &ProducerMessage{Topic: "test.64", Key: nil, Value: StringEncoder(fmt.Sprintf("hur %d", i))}
			if _, _, err := producer.SendMessage(msg); err != nil {
				t.Error(i, err)
			}
		}(i)
	}

	wg.Wait()
	if err := producer.Close(); err != nil {
		t.Error(err)
	}
}

func TestFuncProducingToInvalidTopic(t *testing.T) {
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	producer, err := NewSyncProducer(kafkaBrokers, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := producer.SendMessage(&ProducerMessage{Topic: "in/valid"}); err != ErrUnknownTopicOrPartition {
		t.Error("Expected ErrUnknownTopicOrPartition, found", err)
	}

	if _, _, err := producer.SendMessage(&ProducerMessage{Topic: "in/valid"}); err != ErrUnknownTopicOrPartition {
		t.Error("Expected ErrUnknownTopicOrPartition, found", err)
	}

	safeClose(t, producer)
}

func testProducingMessages(t *testing.T, config *Config) {
	setupFunctionalTest(t)
	defer teardownFunctionalTest(t)

	// Configure some latency in order to properly validate the request latency metric
	for _, proxy := range Proxies {
		if _, err := proxy.AddToxic("", "latency", "", 1, toxiproxy.Attributes{"latency": 10}); err != nil {
			t.Fatal("Unable to configure latency toxicity", err)
		}
	}

	config.Producer.Return.Successes = true
	config.Consumer.Return.Errors = true

	client, err := NewClient(kafkaBrokers, config)
	if err != nil {
		t.Fatal(err)
	}

	// Keep in mind the current offset
	initialOffset, err := client.GetOffset("test.1", 0, OffsetNewest)
	if err != nil {
		t.Fatal(err)
	}

	producer, err := NewAsyncProducerFromClient(client)
	if err != nil {
		t.Fatal(err)
	}

	expectedResponses := TestBatchSize
	for i := 1; i <= TestBatchSize; {
		msg := &ProducerMessage{Topic: "test.1", Key: nil, Value: StringEncoder(fmt.Sprintf("testing %d", i))}
		select {
		case producer.Input() <- msg:
			i++
		case ret := <-producer.Errors():
			t.Fatal(ret.Err)
		case <-producer.Successes():
			expectedResponses--
		}
	}
	for expectedResponses > 0 {
		select {
		case ret := <-producer.Errors():
			t.Fatal(ret.Err)
		case <-producer.Successes():
			expectedResponses--
		}
	}
	safeClose(t, producer)

	// Validate producer metrics before using the consumer minus the offset request
	validateMetrics(t, client)

	master, err := NewConsumerFromClient(client)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := master.ConsumePartition("test.1", 0, initialOffset)
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= TestBatchSize; i++ {
		select {
		case <-time.After(10 * time.Second):
			t.Fatal("Not received any more events in the last 10 seconds.")

		case err := <-consumer.Errors():
			t.Error(err)

		case message := <-consumer.Messages():
			if string(message.Value) != fmt.Sprintf("testing %d", i) {
				t.Fatalf("Unexpected message with index %d: %s", i, message.Value)
			}
		}

	}
	safeClose(t, consumer)
	safeClose(t, client)
}

func validateMetrics(t *testing.T, client Client) {
	// Get the broker used by test1 topic
	var broker *Broker
	if partitions, err := client.Partitions("test.1"); err != nil {
		t.Error(err)
	} else {
		for _, partition := range partitions {
			if b, err := client.Leader("test.1", partition); err != nil {
				t.Error(err)
			} else {
				if broker != nil && b != broker {
					t.Fatal("Expected only one broker, got at least 2")
				}
				broker = b
			}
		}
	}

	metricValidators := newMetricValidators()
	noResponse := client.Config().Producer.RequiredAcks == NoResponse
	compressionEnabled := client.Config().Producer.Compression != CompressionNone

	// We are adding 10ms of latency to all requests with toxiproxy
	minRequestLatencyInMs := 10
	if noResponse {
		// but when we do not wait for a response it can be less than 1ms
		minRequestLatencyInMs = 0
	}

	// We read at least 1 byte from the broker
	metricValidators.registerForAllBrokers(broker, minCountMeterValidator("incoming-byte-rate", 1))
	// in at least 3 global requests (1 for metadata request, 1 for offset request and N for produce request)
	metricValidators.register(minCountMeterValidator("request-rate", 3))
	metricValidators.register(minCountHistogramValidator("request-size", 3))
	metricValidators.register(minValHistogramValidator("request-size", 1))
	metricValidators.register(minValHistogramValidator("request-latency-in-ms", minRequestLatencyInMs))
	// and at least 2 requests to the registered broker (offset + produces)
	metricValidators.registerForBroker(broker, minCountMeterValidator("request-rate", 2))
	metricValidators.registerForBroker(broker, minCountHistogramValidator("request-size", 2))
	metricValidators.registerForBroker(broker, minValHistogramValidator("request-size", 1))
	metricValidators.registerForBroker(broker, minValHistogramValidator("request-latency-in-ms", minRequestLatencyInMs))

	// We send at least 1 batch
	metricValidators.registerForGlobalAndTopic("test_1", minCountHistogramValidator("batch-size", 1))
	metricValidators.registerForGlobalAndTopic("test_1", minValHistogramValidator("batch-size", 1))
	if compressionEnabled {
		// We record compression ratios between [0.50,-10.00] (50-1000 with a histogram) for at least one "fake" record
		metricValidators.registerForGlobalAndTopic("test_1", minCountHistogramValidator("compression-ratio", 1))
		metricValidators.registerForGlobalAndTopic("test_1", minValHistogramValidator("compression-ratio", 50))
		metricValidators.registerForGlobalAndTopic("test_1", maxValHistogramValidator("compression-ratio", 1000))
	} else {
		// We record compression ratios of 1.00 (100 with a histogram) for every TestBatchSize record
		metricValidators.registerForGlobalAndTopic("test_1", countHistogramValidator("compression-ratio", TestBatchSize))
		metricValidators.registerForGlobalAndTopic("test_1", minValHistogramValidator("compression-ratio", 100))
		metricValidators.registerForGlobalAndTopic("test_1", maxValHistogramValidator("compression-ratio", 100))
	}

	// We send exactly TestBatchSize messages
	metricValidators.registerForGlobalAndTopic("test_1", countMeterValidator("record-send-rate", TestBatchSize))
	// We send at least one record per request
	metricValidators.registerForGlobalAndTopic("test_1", minCountHistogramValidator("records-per-request", 1))
	metricValidators.registerForGlobalAndTopic("test_1", minValHistogramValidator("records-per-request", 1))

	// We receive at least 1 byte from the broker
	metricValidators.registerForAllBrokers(broker, minCountMeterValidator("outgoing-byte-rate", 1))
	if noResponse {
		// in exactly 2 global responses (metadata + offset)
		metricValidators.register(countMeterValidator("response-rate", 2))
		metricValidators.register(minCountHistogramValidator("response-size", 2))
		metricValidators.register(minValHistogramValidator("response-size", 1))
		// and exactly 1 offset response for the registered broker
		metricValidators.registerForBroker(broker, countMeterValidator("response-rate", 1))
		metricValidators.registerForBroker(broker, minCountHistogramValidator("response-size", 1))
		metricValidators.registerForBroker(broker, minValHistogramValidator("response-size", 1))
	} else {
		// in at least 3 global responses (metadata + offset + produces)
		metricValidators.register(minCountMeterValidator("response-rate", 3))
		metricValidators.register(minCountHistogramValidator("response-size", 3))
		metricValidators.register(minValHistogramValidator("response-size", 1))
		// and at least 2 for the registered broker
		metricValidators.registerForBroker(broker, minCountMeterValidator("response-rate", 2))
		metricValidators.registerForBroker(broker, minCountHistogramValidator("response-size", 2))
		metricValidators.registerForBroker(broker, minValHistogramValidator("response-size", 1))
	}

	// Run the validators
	metricValidators.run(t, client.Config().MetricRegistry)
}

// Benchmarks

func BenchmarkProducerSmall(b *testing.B) {
	benchmarkProducer(b, nil, "test.64", ByteEncoder(make([]byte, 128)))
}
func BenchmarkProducerMedium(b *testing.B) {
	benchmarkProducer(b, nil, "test.64", ByteEncoder(make([]byte, 1024)))
}
func BenchmarkProducerLarge(b *testing.B) {
	benchmarkProducer(b, nil, "test.64", ByteEncoder(make([]byte, 8192)))
}
func BenchmarkProducerSmallSinglePartition(b *testing.B) {
	benchmarkProducer(b, nil, "test.1", ByteEncoder(make([]byte, 128)))
}
func BenchmarkProducerMediumSnappy(b *testing.B) {
	conf := NewConfig()
	conf.Producer.Compression = CompressionSnappy
	benchmarkProducer(b, conf, "test.1", ByteEncoder(make([]byte, 1024)))
}

func benchmarkProducer(b *testing.B, conf *Config, topic string, value Encoder) {
	setupFunctionalTest(b)
	defer teardownFunctionalTest(b)

	metricsDisable := os.Getenv("METRICS_DISABLE")
	if metricsDisable != "" {
		previousUseNilMetrics := metrics.UseNilMetrics
		Logger.Println("Disabling metrics using no-op implementation")
		metrics.UseNilMetrics = true
		// Restore previous setting
		defer func() {
			metrics.UseNilMetrics = previousUseNilMetrics
		}()
	}

	producer, err := NewAsyncProducer(kafkaBrokers, conf)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 1; i <= b.N; {
		msg := &ProducerMessage{Topic: topic, Key: StringEncoder(fmt.Sprintf("%d", i)), Value: value}
		select {
		case producer.Input() <- msg:
			i++
		case ret := <-producer.Errors():
			b.Fatal(ret.Err)
		}
	}
	safeClose(b, producer)
}
