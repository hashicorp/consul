package mocks

import (
	"sync"

	"github.com/Shopify/sarama"
)

// AsyncProducer implements sarama's Producer interface for testing purposes.
// Before you can send messages to it's Input channel, you have to set expectations
// so it knows how to handle the input; it returns an error if the number of messages
// received is bigger then the number of expectations set. You can also set a
// function in each expectation so that the message value is checked by this function
// and an error is returned if the match fails.
type AsyncProducer struct {
	l            sync.Mutex
	t            ErrorReporter
	expectations []*producerExpectation
	closed       chan struct{}
	input        chan *sarama.ProducerMessage
	successes    chan *sarama.ProducerMessage
	errors       chan *sarama.ProducerError
	lastOffset   int64
}

// NewAsyncProducer instantiates a new Producer mock. The t argument should
// be the *testing.T instance of your test method. An error will be written to it if
// an expectation is violated. The config argument is used to determine whether it
// should ack successes on the Successes channel.
func NewAsyncProducer(t ErrorReporter, config *sarama.Config) *AsyncProducer {
	if config == nil {
		config = sarama.NewConfig()
	}
	mp := &AsyncProducer{
		t:            t,
		closed:       make(chan struct{}, 0),
		expectations: make([]*producerExpectation, 0),
		input:        make(chan *sarama.ProducerMessage, config.ChannelBufferSize),
		successes:    make(chan *sarama.ProducerMessage, config.ChannelBufferSize),
		errors:       make(chan *sarama.ProducerError, config.ChannelBufferSize),
	}

	go func() {
		defer func() {
			close(mp.successes)
			close(mp.errors)
		}()

		for msg := range mp.input {
			mp.l.Lock()
			if mp.expectations == nil || len(mp.expectations) == 0 {
				mp.expectations = nil
				mp.t.Errorf("No more expectation set on this mock producer to handle the input message.")
			} else {
				expectation := mp.expectations[0]
				mp.expectations = mp.expectations[1:]
				if expectation.CheckFunction != nil {
					if val, err := msg.Value.Encode(); err != nil {
						mp.t.Errorf("Input message encoding failed: %s", err.Error())
						mp.errors <- &sarama.ProducerError{Err: err, Msg: msg}
					} else {
						err = expectation.CheckFunction(val)
						if err != nil {
							mp.t.Errorf("Check function returned an error: %s", err.Error())
							mp.errors <- &sarama.ProducerError{Err: err, Msg: msg}
						}
					}
				}
				if expectation.Result == errProduceSuccess {
					mp.lastOffset++
					if config.Producer.Return.Successes {
						msg.Offset = mp.lastOffset
						mp.successes <- msg
					}
				} else {
					if config.Producer.Return.Errors {
						mp.errors <- &sarama.ProducerError{Err: expectation.Result, Msg: msg}
					}
				}
			}
			mp.l.Unlock()
		}

		mp.l.Lock()
		if len(mp.expectations) > 0 {
			mp.t.Errorf("Expected to exhaust all expectations, but %d are left.", len(mp.expectations))
		}
		mp.l.Unlock()

		close(mp.closed)
	}()

	return mp
}

////////////////////////////////////////////////
// Implement Producer interface
////////////////////////////////////////////////

// AsyncClose corresponds with the AsyncClose method of sarama's Producer implementation.
// By closing a mock producer, you also tell it that no more input will be provided, so it will
// write an error to the test state if there's any remaining expectations.
func (mp *AsyncProducer) AsyncClose() {
	close(mp.input)
}

// Close corresponds with the Close method of sarama's Producer implementation.
// By closing a mock producer, you also tell it that no more input will be provided, so it will
// write an error to the test state if there's any remaining expectations.
func (mp *AsyncProducer) Close() error {
	mp.AsyncClose()
	<-mp.closed
	return nil
}

// Input corresponds with the Input method of sarama's Producer implementation.
// You have to set expectations on the mock producer before writing messages to the Input
// channel, so it knows how to handle them. If there is no more remaining expectations and
// a messages is written to the Input channel, the mock producer will write an error to the test
// state object.
func (mp *AsyncProducer) Input() chan<- *sarama.ProducerMessage {
	return mp.input
}

// Successes corresponds with the Successes method of sarama's Producer implementation.
func (mp *AsyncProducer) Successes() <-chan *sarama.ProducerMessage {
	return mp.successes
}

// Errors corresponds with the Errors method of sarama's Producer implementation.
func (mp *AsyncProducer) Errors() <-chan *sarama.ProducerError {
	return mp.errors
}

////////////////////////////////////////////////
// Setting expectations
////////////////////////////////////////////////

// ExpectInputWithCheckerFunctionAndSucceed sets an expectation on the mock producer that a message
// will be provided on the input channel. The mock producer will call the given function to check
// the message value. If an error is returned it will be made available on the Errors channel
// otherwise the mock will handle the message as if it produced successfully, i.e. it will make
// it available on the Successes channel if the Producer.Return.Successes setting is set to true.
func (mp *AsyncProducer) ExpectInputWithCheckerFunctionAndSucceed(cf ValueChecker) {
	mp.l.Lock()
	defer mp.l.Unlock()
	mp.expectations = append(mp.expectations, &producerExpectation{Result: errProduceSuccess, CheckFunction: cf})
}

// ExpectInputWithCheckerFunctionAndSucceed sets an expectation on the mock producer that a message
// will be provided on the input channel. The mock producer will first call the given function to
// check the message value. If an error is returned it will be made available on the Errors channel
// otherwise the mock will handle the message as if it failed to produce successfully. This means
// it will make a ProducerError available on the Errors channel.
func (mp *AsyncProducer) ExpectInputWithCheckerFunctionAndFail(cf ValueChecker, err error) {
	mp.l.Lock()
	defer mp.l.Unlock()
	mp.expectations = append(mp.expectations, &producerExpectation{Result: err, CheckFunction: cf})
}

// ExpectInputAndSucceed sets an expectation on the mock producer that a message will be provided
// on the input channel. The mock producer will handle the message as if it is produced successfully,
// i.e. it will make it available on the Successes channel if the Producer.Return.Successes setting
// is set to true.
func (mp *AsyncProducer) ExpectInputAndSucceed() {
	mp.ExpectInputWithCheckerFunctionAndSucceed(nil)
}

// ExpectInputAndFail sets an expectation on the mock producer that a message will be provided
// on the input channel. The mock producer will handle the message as if it failed to produce
// successfully. This means it will make a ProducerError available on the Errors channel.
func (mp *AsyncProducer) ExpectInputAndFail(err error) {
	mp.ExpectInputWithCheckerFunctionAndFail(nil, err)
}
