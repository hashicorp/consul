package service

import (
	"context"
	"fmt"
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

// ConnectContainer
type ConnectContainer struct {
	ctx         context.Context
	container   testcontainers.Container
	ip          string
	appPort     int
	adminPort   int
	serviceName string
}

var _ Service = (*ConnectContainer)(nil)

func (g ConnectContainer) GetName() string {
	name, err := g.container.Name(g.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (g ConnectContainer) GetAddr() (string, int) {
	return g.ip, g.appPort
}

func (g ConnectContainer) Start() error {
	if g.container == nil {
		return fmt.Errorf("container has not been initialized")
	}
	return g.container.Start(context.Background())
}

func (g ConnectContainer) GetAdminAddr() (string, int) {
	return "localhost", g.adminPort
}

func (c ConnectContainer) Terminate() error {
	return cluster.TerminateContainer(c.ctx, c.container, true)
}

func (g ConnectContainer) Export(partition, peer string, client *api.Client) error {
	return fmt.Errorf("ConnectContainer export unimplemented")
}

func (g ConnectContainer) GetServiceName() string {
	return g.serviceName
}

// NewConnectService returns a container that runs envoy sidecar, launched by
// "consul connect envoy", for service name (serviceName) on the specified
// node. The container exposes port serviceBindPort and envoy admin port
// (19000) by mapping them onto host ports. The container's name has a prefix
// combining datacenter and name.
func NewConnectService(ctx context.Context, name string, serviceName string, serviceBindPort int, node libcluster.Agent) (*ConnectContainer, error) {
	nodeConfig := node.GetConfig()
	if nodeConfig.ScratchDir == "" {
		return nil, fmt.Errorf("node ScratchDir is required")
	}

	namePrefix := fmt.Sprintf("%s-service-connect-%s", node.GetDatacenter(), name)
	containerName := utils.RandName(namePrefix)

	envoyVersion := getEnvoyVersion()
	buildargs := map[string]*string{
		"ENVOY_VERSION": utils.StringToPointer(envoyVersion),
	}

	dockerfileCtx, err := getDevContainerDockerfile()
	if err != nil {
		return nil, err
	}
	dockerfileCtx.BuildArgs = buildargs

	adminPort := node.ClaimAdminPort()

	req := testcontainers.ContainerRequest{
		FromDockerfile: dockerfileCtx,
		WaitingFor:     wait.ForLog("").WithStartupTimeout(10 * time.Second),
		AutoRemove:     false,
		Name:           containerName,
		Cmd: []string{
			"consul", "connect", "envoy",
			"-sidecar-for", serviceName,
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
		appPortStr   = strconv.Itoa(serviceBindPort)
		adminPortStr = strconv.Itoa(adminPort)
	)

	info, err := cluster.LaunchContainerOnNode(ctx, node, req, []string{appPortStr, adminPortStr})
	if err != nil {
		return nil, err
	}

	out := &ConnectContainer{
		ctx:         ctx,
		container:   info.Container,
		ip:          info.IP,
		appPort:     info.MappedPorts[appPortStr].Int(),
		adminPort:   info.MappedPorts[adminPortStr].Int(),
		serviceName: name,
	}

	fmt.Printf("NewConnectService: name %s, mappedAppPort %d, bind port %d\n",
		serviceName, out.appPort, serviceBindPort)

	return out, nil
}
