// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package service

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hashicorp/consul/api"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// exampleContainer
type exampleContainer struct {
	ctx         context.Context
	container   testcontainers.Container
	ip          string
	httpPort    int
	grpcPort    int
	serviceName string
}

var _ Service = (*exampleContainer)(nil)

func (g exampleContainer) Exec(ctx context.Context, cmd []string) (string, error) {
	exitCode, reader, err := g.container.Exec(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("exec with error %s", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("exec with exit code %d", exitCode)
	}
	buf, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("error reading from exec output: %w", err)
	}
	return string(buf), nil
}

func (g exampleContainer) Export(partition, peerName string, client *api.Client) error {
	config := &api.ExportedServicesConfigEntry{
		Name: partition,
		Services: []api.ExportedService{{
			Name: g.GetServiceName(),
			Consumers: []api.ServiceConsumer{
				// TODO: need to handle the changed field name in 1.13
				{Peer: peerName},
			},
		}},
	}

	_, _, err := client.ConfigEntries().Set(config, &api.WriteOptions{})
	return err
}

func (g exampleContainer) GetAddr() (string, int) {
	return g.ip, g.httpPort
}

func (g exampleContainer) GetAddrs() (string, []int) {
	return "", nil
}

func (g exampleContainer) GetPort(port int) (int, error) {
	return 0, nil
}

func (g exampleContainer) Restart() error {
	return fmt.Errorf("Restart Unimplemented by ConnectContainer")
}

func (g exampleContainer) GetLogs() (string, error) {
	rc, err := g.container.Logs(g.ctx)
	if err != nil {
		return "", fmt.Errorf("could not get logs for example service %s: %w", g.GetServiceName(), err)
	}
	defer rc.Close()

	out, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("could not read from logs for example service %s: %w", g.GetServiceName(), err)
	}
	return string(out), nil
}

func (g exampleContainer) GetName() string {
	name, err := g.container.Name(g.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (g exampleContainer) GetServiceName() string {
	return g.serviceName
}

func (g exampleContainer) Start() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Start(context.Background())
}

func (g exampleContainer) Stop() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Stop(context.Background(), nil)
}

func (c exampleContainer) Terminate() error {
	return libcluster.TerminateContainer(c.ctx, c.container, true)
}

func (c exampleContainer) GetStatus() (string, error) {
	state, err := c.container.State(c.ctx)
	return state.Status, err
}

// NewCustomService creates a new test service from a custom testcontainers.ContainerRequest.
func NewCustomService(ctx context.Context, name string, httpPort int, grpcPort int, node libcluster.Agent, request testcontainers.ContainerRequest) (Service, error) {
	namePrefix := fmt.Sprintf("%s-service-example-%s", node.GetDatacenter(), name)
	containerName := utils.RandName(namePrefix)

	pod := node.GetPod()
	if pod == nil {
		return nil, fmt.Errorf("node Pod is required")
	}

	var (
		httpPortStr = strconv.Itoa(httpPort)
		grpcPortStr = strconv.Itoa(grpcPort)
	)

	request.Name = containerName

	info, err := libcluster.LaunchContainerOnNode(ctx, node, request, []string{httpPortStr, grpcPortStr})
	if err != nil {
		return nil, err
	}

	out := &exampleContainer{
		ctx:         ctx,
		container:   info.Container,
		ip:          info.IP,
		httpPort:    info.MappedPorts[httpPortStr].Int(),
		grpcPort:    info.MappedPorts[grpcPortStr].Int(),
		serviceName: name,
	}

	fmt.Printf("Custom service exposed http port %d, gRPC port %d\n", out.httpPort, out.grpcPort)

	return out, nil
}

func NewExampleService(ctx context.Context, name string, httpPort int, grpcPort int, node libcluster.Agent, containerArgs ...string) (Service, error) {
	namePrefix := fmt.Sprintf("%s-service-example-%s", node.GetDatacenter(), name)
	containerName := utils.RandName(namePrefix)

	pod := node.GetPod()
	if pod == nil {
		return nil, fmt.Errorf("node Pod is required")
	}

	var (
		httpPortStr = strconv.Itoa(httpPort)
		grpcPortStr = strconv.Itoa(grpcPort)
	)

	command := []string{
		"server",
		"-http-port", httpPortStr,
		"-grpc-port", grpcPortStr,
		"-redirect-port", "-disabled",
	}

	command = append(command, containerArgs...)

	req := testcontainers.ContainerRequest{
		Image:      hashicorpDockerProxy + "/fortio/fortio",
		WaitingFor: wait.ForLog("").WithStartupTimeout(60 * time.Second),
		AutoRemove: false,
		Name:       containerName,
		Cmd:        command,
		Env:        map[string]string{"FORTIO_NAME": name},
	}

	info, err := libcluster.LaunchContainerOnNode(ctx, node, req, []string{httpPortStr, grpcPortStr})
	if err != nil {
		return nil, err
	}

	out := &exampleContainer{
		ctx:         ctx,
		container:   info.Container,
		ip:          info.IP,
		httpPort:    info.MappedPorts[httpPortStr].Int(),
		grpcPort:    info.MappedPorts[grpcPortStr].Int(),
		serviceName: name,
	}

	fmt.Printf("Example service exposed http port %d, gRPC port %d\n", out.httpPort, out.grpcPort)

	return out, nil
}

func (g exampleContainer) GetAdminAddr() (string, int) {
	return "localhost", 0
}
