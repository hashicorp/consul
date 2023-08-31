package cluster

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
)

const tooManyNetworksError = "could not find an available, non-overlapping IPv4 address pool among the defaults to assign to the network: failed to create network"

func createNetwork(t TestingT, name string) (testcontainers.Network, error) {
	req := testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           name,
			Attachable:     true,
			CheckDuplicate: true,
		},
	}
	first := true
RETRY:
	network, err := testcontainers.GenericNetwork(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), tooManyNetworksError) {
			if first {
				t.Logf("waiting until possible to get a network")
				first = false
			}
			time.Sleep(1 * time.Second)
			goto RETRY
		}
		return nil, errors.Wrap(err, "could not create network")
	}
	t.Cleanup(func() {
		_ = network.Remove(context.Background())
	})

	return network, nil
}
