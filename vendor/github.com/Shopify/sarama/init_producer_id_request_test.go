package sarama

import (
	"testing"
	"time"
)

var (
	initProducerIDRequestNull = []byte{
		255, 255,
		0, 0, 0, 100,
	}

	initProducerIDRequest = []byte{
		0, 3, 't', 'x', 'n',
		0, 0, 0, 100,
	}
)

func TestInitProducerIDRequest(t *testing.T) {
	req := &InitProducerIDRequest{
		TransactionTimeout: 100 * time.Millisecond,
	}

	testRequest(t, "null transaction id", req, initProducerIDRequestNull)

	transactionID := "txn"
	req.TransactionalID = &transactionID

	testRequest(t, "transaction id", req, initProducerIDRequest)
}
