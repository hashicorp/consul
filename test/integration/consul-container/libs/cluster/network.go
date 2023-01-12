package cluster

import (
	"context"

	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
)

func createNetwork(t TestingT, name string) (testcontainers.Network, error) {
	req := testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           name,
			Attachable:     true,
			CheckDuplicate: true,
		},
	}
	network, err := testcontainers.GenericNetwork(context.Background(), req)
	if err != nil {
		return nil, errors.Wrap(err, "could not create network")
	}
	t.Cleanup(func() {
		_ = network.Remove(context.Background())
	})
	return network, nil
}
