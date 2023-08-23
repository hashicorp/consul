package cluster

import (
	"context"

	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
)

func createNetwork(name string) (testcontainers.Network, error) {
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
	return network, nil
}
