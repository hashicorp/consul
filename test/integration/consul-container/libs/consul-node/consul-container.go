package node

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/docker/docker/pkg/ioutils"

	"github.com/hashicorp/consul/integration/consul-container/libs/utils"

	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
)

type consulContainerNode struct {
	ctx       context.Context
	client    *api.Client
	container testcontainers.Container
	ip        string
	port      int
}

func (c *consulContainerNode) GetClient() *api.Client {
	return c.client
}

func (c *consulContainerNode) GetAddr() (string, int) {
	return c.ip, c.port
}

type Config struct {
	HCL     string
	Version string
	Cmd     []string
}

type Node interface {
	Terminate() error
	GetClient() *api.Client
	GetAddr() (string, int)
}

const bootLogLine = "Consul agent running"

func NewConsulContainer(ctx context.Context, config Config) (Node, error) {
	name := utils.RandName("consul-")
	ctx = context.WithValue(ctx, "name", name)
	tmpDir, err := ioutils.TempDir("/tmp", name)
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

	req := testcontainers.ContainerRequest{
		Image:        "consul:" + config.Version,
		ExposedPorts: []string{"8500/tcp"},
		WaitingFor:   wait.ForLog(bootLogLine).WithStartupTimeout(10 * time.Second),
		AutoRemove:   false,
		Name:         name,
		BindMounts:   map[string]string{"/consul/config/config.hcl": configFile},
		Cmd:          config.Cmd,
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

func (c *consulContainerNode) Terminate() error {
	return c.container.Terminate(c.ctx)
}
