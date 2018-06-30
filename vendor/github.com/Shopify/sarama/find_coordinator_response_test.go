package sarama

import (
	"testing"
	"time"
)

func TestFindCoordinatorResponse(t *testing.T) {
	errMsg := "kaboom"

	for _, tc := range []struct {
		desc     string
		response *FindCoordinatorResponse
		encoded  []byte
	}{{
		desc: "version 0 - no error",
		response: &FindCoordinatorResponse{
			Version: 0,
			Err:     ErrNoError,
			Coordinator: &Broker{
				id:   7,
				addr: "host:9092",
			},
		},
		encoded: []byte{
			0, 0, // Err
			0, 0, 0, 7, // Coordinator.ID
			0, 4, 'h', 'o', 's', 't', // Coordinator.Host
			0, 0, 35, 132, // Coordinator.Port
		},
	}, {
		desc: "version 1 - no error",
		response: &FindCoordinatorResponse{
			Version:      1,
			ThrottleTime: 100 * time.Millisecond,
			Err:          ErrNoError,
			Coordinator: &Broker{
				id:   7,
				addr: "host:9092",
			},
		},
		encoded: []byte{
			0, 0, 0, 100, // ThrottleTime
			0, 0, // Err
			255, 255, // ErrMsg: empty
			0, 0, 0, 7, // Coordinator.ID
			0, 4, 'h', 'o', 's', 't', // Coordinator.Host
			0, 0, 35, 132, // Coordinator.Port
		},
	}, {
		desc: "version 0 - error",
		response: &FindCoordinatorResponse{
			Version:     0,
			Err:         ErrConsumerCoordinatorNotAvailable,
			Coordinator: NoNode,
		},
		encoded: []byte{
			0, 15, // Err
			255, 255, 255, 255, // Coordinator.ID: -1
			0, 0, // Coordinator.Host: ""
			255, 255, 255, 255, // Coordinator.Port: -1
		},
	}, {
		desc: "version 1 - error",
		response: &FindCoordinatorResponse{
			Version:      1,
			ThrottleTime: 100 * time.Millisecond,
			Err:          ErrConsumerCoordinatorNotAvailable,
			ErrMsg:       &errMsg,
			Coordinator:  NoNode,
		},
		encoded: []byte{
			0, 0, 0, 100, // ThrottleTime
			0, 15, // Err
			0, 6, 'k', 'a', 'b', 'o', 'o', 'm', // ErrMsg
			255, 255, 255, 255, // Coordinator.ID: -1
			0, 0, // Coordinator.Host: ""
			255, 255, 255, 255, // Coordinator.Port: -1
		},
	}} {
		testResponse(t, tc.desc, tc.response, tc.encoded)
	}
}
