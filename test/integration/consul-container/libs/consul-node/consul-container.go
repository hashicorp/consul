package consulcontainer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/docker/docker/pkg/ioutils"

	"github.com/hashicorp/consul/integration/ca/libs/utils"

	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
)

type ConsulNode struct {
	ctx       context.Context
	Client    *api.Client
	container testcontainers.Container
	IP        string
	Port      int
}

type Config struct {
	ConsulConfig string
	Image        string
	ConsulPath   string
	Cmd          []string
}

const bootLogLine = "Consul agent running"

func NewNodeWitConfig(ctx context.Context, config Config) (*ConsulNode, error) {
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
	err = os.WriteFile(configFile, []byte(config.ConsulConfig), 0644)
	if err != nil {
		return nil, err
	}

	req := testcontainers.ContainerRequest{
		Image:        config.Image,
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
	c := new(ConsulNode)
	c.container = container
	c.IP = ip
	c.Port = mappedPort.Int()
	apiConfig := api.DefaultConfig()
	apiConfig.Address = uri
	c.Client, err = api.NewClient(apiConfig)
	c.ctx = ctx

	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewNode() (*ConsulNode, error) {
	return NewNodeWitConfig(context.Background(), Config{})
}

func (c *ConsulNode) Terminate() error {
	return c.container.Terminate(c.ctx)
}
