package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// gatewayContainer
type gatewayContainer struct {
	ctx          context.Context
	container    testcontainers.Container
	ip           string
	port         int
	adminPort    int
	serviceName  string
	portMappings map[int]int
}

var _ Service = (*gatewayContainer)(nil)

func (g gatewayContainer) Exec(ctx context.Context, cmd []string) (string, error) {
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

func (g gatewayContainer) Export(partition, peer string, client *api.Client) error {
	return fmt.Errorf("gatewayContainer export unimplemented")
}

func (g gatewayContainer) GetAddr() (string, int) {
	return g.ip, g.port
}

func (g gatewayContainer) GetAddrs() (string, []int) {
	return "", nil
}

func (g gatewayContainer) GetLogs() (string, error) {
	rc, err := g.container.Logs(context.Background())
	if err != nil {
		return "", fmt.Errorf("could not get logs for gateway service %s: %w", g.GetServiceName(), err)
	}
	defer rc.Close()

	out, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("could not read from logs for gateway service %s: %w", g.GetServiceName(), err)
	}
	return string(out), nil
}

func (g gatewayContainer) GetName() string {
	name, err := g.container.Name(g.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (g gatewayContainer) GetServiceName() string {
	return g.serviceName
}

func (g gatewayContainer) Start() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Start(context.Background())
}

func (g gatewayContainer) Stop() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Stop(context.Background(), nil)
}

func (c gatewayContainer) Terminate() error {
	return cluster.TerminateContainer(c.ctx, c.container, true)
}

func (g gatewayContainer) GetAdminAddr() (string, int) {
	return "localhost", g.adminPort
}

func (g gatewayContainer) GetPort(port int) (int, error) {
	p, ok := g.portMappings[port]
	if !ok {
		return 0, fmt.Errorf("port does not exist")
	}
	return p, nil
}

func (g gatewayContainer) Restart() error {
	_, err := g.container.State(g.ctx)
	if err != nil {
		return fmt.Errorf("error get gateway state %s", err)
	}

	fmt.Printf("Stopping container: %s\n", g.GetName())
	err = g.container.Stop(g.ctx, nil)
	if err != nil {
		return fmt.Errorf("error stop gateway %s", err)
	}

	fmt.Printf("Starting container: %s\n", g.GetName())
	err = g.container.Start(g.ctx)
	if err != nil {
		return fmt.Errorf("error start gateway %s", err)
	}
	return nil
}

func (g gatewayContainer) GetStatus() (string, error) {
	state, err := g.container.State(g.ctx)
	return state.Status, err
}

type GatewayConfig struct {
	Name      string
	Kind      string
	Namespace string
}

func NewGatewayService(ctx context.Context, gwCfg GatewayConfig, node libcluster.Agent, ports ...int) (Service, error) {
	nodeConfig := node.GetConfig()
	if nodeConfig.ScratchDir == "" {
		return nil, fmt.Errorf("node ScratchDir is required")
	}

	namePrefix := fmt.Sprintf("%s-service-gateway-%s", node.GetDatacenter(), gwCfg.Name)
	containerName := utils.RandName(namePrefix)

	envoyVersion := getEnvoyVersion()
	agentConfig := node.GetConfig()
	buildargs := map[string]*string{
		"ENVOY_VERSION": utils.StringToPointer(envoyVersion),
		"CONSUL_IMAGE":  utils.StringToPointer(agentConfig.DockerImage()),
	}

	dockerfileCtx, err := getDevContainerDockerfile()
	if err != nil {
		return nil, err
	}
	dockerfileCtx.BuildArgs = buildargs

	adminPort, err := node.ClaimAdminPort()
	if err != nil {
		return nil, err
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: dockerfileCtx,
		WaitingFor:     wait.ForLog("").WithStartupTimeout(100 * time.Second),
		AutoRemove:     false,
		Name:           containerName,
		Cmd: []string{
			"consul", "connect", "envoy",
			fmt.Sprintf("-gateway=%s", gwCfg.Kind),
			"-register",
			"-namespace", gwCfg.Namespace,
			"-service", gwCfg.Name,
			"-address", "{{ GetInterfaceIP \"eth0\" }}:8443",
			"-admin-bind", fmt.Sprintf("0.0.0.0:%d", adminPort),
			"--",
			"--log-level", envoyLogLevel,
		},
		Env: make(map[string]string),
	}

	nodeInfo := node.GetInfo()
	if nodeInfo.UseTLSForAPI || nodeInfo.UseTLSForGRPC {
		req.Mounts = append(req.Mounts, testcontainers.ContainerMount{
			Source: testcontainers.DockerBindMountSource{
				// See cluster.NewConsulContainer for this info
				HostPath: filepath.Join(nodeConfig.ScratchDir, "ca.pem"),
			},
			Target:   "/ca.pem",
			ReadOnly: true,
		})
	}

	if nodeInfo.UseTLSForAPI {
		req.Env["CONSUL_HTTP_ADDR"] = fmt.Sprintf("https://127.0.0.1:%d", 8501)
		req.Env["CONSUL_HTTP_SSL"] = "1"
		req.Env["CONSUL_CACERT"] = "/ca.pem"
	} else {
		req.Env["CONSUL_HTTP_ADDR"] = fmt.Sprintf("http://127.0.0.1:%d", 8500)
	}

	if nodeInfo.UseTLSForGRPC {
		req.Env["CONSUL_GRPC_ADDR"] = fmt.Sprintf("https://127.0.0.1:%d", 8503)
		req.Env["CONSUL_GRPC_CACERT"] = "/ca.pem"
	} else {
		req.Env["CONSUL_GRPC_ADDR"] = fmt.Sprintf("http://127.0.0.1:%d", 8502)
	}

	var (
		portStr      = "8443"
		adminPortStr = strconv.Itoa(adminPort)
	)

	extraPorts := []string{}
	for _, port := range ports {
		extraPorts = append(extraPorts, strconv.Itoa(port))
	}

	info, err := cluster.LaunchContainerOnNode(ctx, node, req, append(
		extraPorts,
		portStr,
		adminPortStr,
	))
	if err != nil {
		return nil, err
	}

	portMappings := make(map[int]int)
	for _, port := range ports {
		portMappings[port] = info.MappedPorts[strconv.Itoa(port)].Int()
	}

	out := &gatewayContainer{
		ctx:          ctx,
		container:    info.Container,
		ip:           info.IP,
		port:         info.MappedPorts[portStr].Int(),
		adminPort:    info.MappedPorts[adminPortStr].Int(),
		serviceName:  gwCfg.Name,
		portMappings: portMappings,
	}

	return out, nil
}
