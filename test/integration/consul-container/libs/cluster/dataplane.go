// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
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

func (g ConsulDataplaneContainer) GetServiceName() string {
	return g.serviceName
}

// GetAdminAddr returns the external admin port
func (g ConsulDataplaneContainer) GetAdminAddr() (string, int) {
	return "localhost", g.externalAdminPort
}

func (c ConsulDataplaneContainer) Terminate() error {
	return TerminateContainer(c.ctx, c.container, true)
}

func (g ConsulDataplaneContainer) Exec(ctx context.Context, cmd []string) (string, error) {
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

func (g ConsulDataplaneContainer) GetStatus() (string, error) {
	state, err := g.container.State(g.ctx)
	return state.Status, err
}

func NewConsulDataplane(ctx context.Context, proxyID string, serverAddresses string, grpcPort int, serviceBindPorts []int,
	node Agent, tproxy bool, bootstrapToken string, containerArgs ...string) (*ConsulDataplaneContainer, error) {
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

	req := testcontainers.ContainerRequest{
		Image:      "consul-dataplane:local",
		WaitingFor: wait.ForLog("").WithStartupTimeout(60 * time.Second),
		AutoRemove: false,
		Name:       containerName,
		Env:        map[string]string{},
	}

	var command []string

	if tproxy {
		req.Entrypoint = []string{"sh", "/bin/tproxy-startup.sh"}
		req.Env["REDIRECT_TRAFFIC_ARGS"] = strings.Join(
			[]string{
				// TODO once we run this on a different pod from Consul agents, we can eliminate most of this.
				"-exclude-inbound-port", fmt.Sprint(internalAdminPort),
				"-exclude-inbound-port", "8300",
				"-exclude-inbound-port", "8301",
				"-exclude-inbound-port", "8302",
				"-exclude-inbound-port", "8500",
				"-exclude-inbound-port", "8502",
				"-exclude-inbound-port", "8600",
				"-proxy-inbound-port", "20000",
				"-consul-dns-ip", "127.0.0.1",
				"-consul-dns-port", "8600",
			},
			" ",
		)
		req.CapAdd = append(req.CapAdd, "NET_ADMIN")
		command = append(command, "consul-dataplane")
	}

	command = append(command,
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
	)

	if bootstrapToken != "" {
		command = append(command,
			"-credential-type=static",
			fmt.Sprintf("-static-token=%s", bootstrapToken))
	}

	req.Cmd = append(command, containerArgs...)

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
