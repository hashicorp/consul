package node

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/consul/api"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
)

const bootLogLine = "Consul agent running"
const disableRYUKEnv = "TESTCONTAINERS_RYUK_DISABLED"

// consulContainerNode implements the Node interface by running a Consul node
// in a container.
type consulContainerNode struct {
	ctx       context.Context
	client    *api.Client
	container testcontainers.Container
	ip        string
	port      int
}

// NewConsulContainer starts a Consul node in a container with the given config.
func NewConsulContainer(ctx context.Context, config Config) (Node, error) {

	name := utils.RandName("consul-")
	tmpDir, err := ioutils.TempDir("", name)
	if err != nil {
		return nil, err
	}
	err = os.Chmod(tmpDir, 0777)
	if err != nil {
		return nil, err
	}
	err = os.Mkdir(tmpDir+"/config", 0777)
	if err != nil {
		return nil, err
	}
	configFile := tmpDir + "/config/config.hcl"
	err = os.WriteFile(configFile, []byte(config.HCL), 0644)
	if err != nil {
		return nil, err
	}
	skipReaper := isRYUKDisabled()
	req := testcontainers.ContainerRequest{
		Image:        "consul:" + config.Version,
		ExposedPorts: []string{"8500/tcp"},
		WaitingFor:   wait.ForLog(bootLogLine).WithStartupTimeout(10 * time.Second),
		AutoRemove:   false,
		Name:         name,
		Mounts:       testcontainers.ContainerMounts{testcontainers.ContainerMount{Source: testcontainers.DockerBindMountSource{HostPath: configFile}, Target: "/consul/config/config.hcl"}},
		Cmd:          config.Cmd,
		SkipReaper:   skipReaper,
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	localIP, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "8500")
	if err != nil {
		return nil, err
	}

	ip, err := container.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("http://%s:%s", localIP, mappedPort.Port())
	c := new(consulContainerNode)
	c.container = container
	c.ip = ip
	c.port = mappedPort.Int()
	apiConfig := api.DefaultConfig()
	apiConfig.Address = uri
	c.client, err = api.NewClient(apiConfig)
	c.ctx = ctx

	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetClient returns an API client that can be used to communicate with the Node.
func (c *consulContainerNode) GetClient() *api.Client {
	return c.client
}

// GetAddr return the network address associated with the Node.
func (c *consulContainerNode) GetAddr() (string, int) {
	return c.ip, c.port
}

// Terminate attempts to terminate the container. On failure, an error will be
// returned and the reaper process (RYUK) will handle cleanup.
func (c *consulContainerNode) Terminate() error {
	return c.container.Terminate(c.ctx)
}

// isRYUKDisabled returns whether the reaper process (RYUK) has been disabled
// by an environment variable.
//
// https://github.com/testcontainers/moby-ryuk
func isRYUKDisabled() bool {
	skipReaperStr := os.Getenv(disableRYUKEnv)
	skipReaper, err := strconv.ParseBool(skipReaperStr)
	if err != nil {
		return false
	}
	return skipReaper
}
