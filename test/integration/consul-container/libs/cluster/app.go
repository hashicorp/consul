// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cluster

import (
	"context"
	"fmt"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// LaunchInfo is the resutl of LaunchContainerOnNode.
type LaunchInfo struct {
	Container   testcontainers.Container
	IP          string
	MappedPorts map[string]nat.Port
}

// LaunchContainerOnNode will run a new container attached to the same network
// namespace as the provided agent, in the same manner in Kubernetes where
// you'd run two containers in the same pod so you can share localhost.
//
// This is supposed to mimic more accurately how consul/CLI/envoy/etc all are
// co-located on localhost with the consul client agent in typical deployment
// topologies.
func LaunchContainerOnNode(
	ctx context.Context,
	node Agent,
	req testcontainers.ContainerRequest,
	mapPorts []string,
) (*LaunchInfo, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("ContainerRequest requires the Name field")
	}
	if req.NetworkMode != "" {
		return nil, fmt.Errorf("caller should not configure ContainerRequest.NetworkMode")
	}

	req.NetworkMode = dockercontainer.NetworkMode("container:" + node.GetName() + "-pod")

	pod := node.GetPod()
	if pod == nil {
		return nil, fmt.Errorf("node Pod is required")
	}

	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	launchCtx, cancel := context.WithTimeout(ctx, time.Second*40)
	defer cancel()

	container, err := testcontainers.GenericContainer(launchCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	fmt.Printf("creating container with image: %s(%s)\n", req.Name, req.Image)
	if err != nil {
		return nil, fmt.Errorf("creating container: %s(%s), %w", req.Name, req.Image, err)
	}
	deferClean.Add(func() {
		_ = container.Terminate(ctx)
	})

	ip, err := container.ContainerIP(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching container IP: %w", err)
	}

	if utils.FollowLog {
		if err := container.StartLogProducer(ctx); err != nil {
			return nil, fmt.Errorf("starting log producer: %w", err)
		}
		container.FollowOutput(&LogConsumer{
			Prefix: req.Name,
		})
		deferClean.Add(func() {
			_ = container.StopLogProducer()
		})
	}

	ports := make(map[string]nat.Port)
	for _, portStr := range mapPorts {
		mapped, err := pod.MappedPort(ctx, nat.Port(portStr))
		if err != nil {
			return nil, fmt.Errorf("mapping port %s: %w", portStr, err)
		}
		ports[portStr] = mapped
	}

	info := &LaunchInfo{
		Container:   container,
		IP:          ip,
		MappedPorts: ports,
	}

	node.RegisterTermination(func() error {
		return TerminateContainer(ctx, container, true)
	})

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return info, nil
}

// TerminateContainer attempts to terminate the container. On failure, an error
// will be returned and the reaper process (RYUK) will handle cleanup.
func TerminateContainer(ctx context.Context, c testcontainers.Container, stopLogs bool) error {
	if c == nil {
		return nil
	}

	var merr error

	if utils.FollowLog && stopLogs {
		if state, err := c.State(ctx); err == nil && state.Running {
			// StopLogProducer can only be called on running containers
			if err := c.StopLogProducer(); err != nil {
				merr = multierror.Append(merr, err)
			}
		}
	}

	if err := c.Stop(ctx, nil); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := c.Terminate(ctx); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}
