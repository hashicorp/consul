// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"strconv"
	"time"
)

type ConsulDataplaneContainer struct {
	ctx               context.Context
	container         testcontainers.Container
	ip                string
	appPort           []int
	serviceName       string
	externalAdminPort int
	internalAdminPort int
}

func (g ConsulDataplaneContainer) GetAddr() (string, int) {
	return g.ip, g.appPort[0]
}

// GetAdminAddr returns the external admin port
func (g ConsulDataplaneContainer) GetAdminAddr() (string, int) {
	return "localhost", g.externalAdminPort
}

func (c ConsulDataplaneContainer) Terminate() error {
	return TerminateContainer(c.ctx, c.container, true)
}

func (g ConsulDataplaneContainer) GetStatus() (string, error) {
	state, err := g.container.State(g.ctx)
	return state.Status, err
}

func NewConsulDataplane(ctx context.Context, proxyID string, serverAddresses string, grpcPort int, serviceBindPorts []int,
	node Agent, containerArgs ...string) (*ConsulDataplaneContainer, error) {
	namePrefix := fmt.Sprintf("%s-consul-dataplane-%s", node.GetDatacenter(), proxyID)
	containerName := utils.RandName(namePrefix)

	internalAdminPort, err := node.ClaimAdminPort()
	if err != nil {
		return nil, err
	}

	pod := node.GetPod()
	if pod == nil {
		return nil, fmt.Errorf("node Pod is required")
	}

	var (
		appPortStrs  []string
		adminPortStr = strconv.Itoa(internalAdminPort)
	)

	for _, port := range serviceBindPorts {
		appPortStrs = append(appPortStrs, strconv.Itoa(port))
	}

	// expose the app ports and the envoy adminPortStr on the agent container
	exposedPorts := make([]string, len(appPortStrs))
	copy(exposedPorts, appPortStrs)
	exposedPorts = append(exposedPorts, adminPortStr)

	command := []string{
		"-addresses", serverAddresses,
		fmt.Sprintf("-grpc-port=%d", grpcPort),
		fmt.Sprintf("-proxy-id=%s", proxyID),
		"-proxy-namespace=default",
		"-proxy-partition=default",
		"-log-level=info",
		"-log-json=false",
		"-envoy-concurrency=2",
		"-tls-disabled",
		fmt.Sprintf("-envoy-admin-bind-port=%d", internalAdminPort),
	}

	command = append(command, containerArgs...)

	req := testcontainers.ContainerRequest{
		Image:      "consul-dataplane:local",
		WaitingFor: wait.ForLog("").WithStartupTimeout(60 * time.Second),
		AutoRemove: false,
		Name:       containerName,
		Cmd:        command,
		Env:        map[string]string{},
	}

	info, err := LaunchContainerOnNode(ctx, node, req, exposedPorts)
	if err != nil {
		return nil, err
	}
	out := &ConsulDataplaneContainer{
		ctx:               ctx,
		container:         info.Container,
		ip:                info.IP,
		serviceName:       containerName,
		externalAdminPort: info.MappedPorts[adminPortStr].Int(),
		internalAdminPort: internalAdminPort,
	}

	for _, port := range appPortStrs {
		out.appPort = append(out.appPort, info.MappedPorts[port].Int())
	}

	fmt.Printf("NewConsulDataplane: proxyID %s, mapped App Port %d, service bind port %v\n",
		proxyID, out.appPort, serviceBindPorts)
	fmt.Printf("NewConsulDataplane: proxyID %s, , mapped admin port %d, admin port %d\n",
		proxyID, out.externalAdminPort, internalAdminPort)

	return out, nil
}
