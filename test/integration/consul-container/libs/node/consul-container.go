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
	config    Config
	req       testcontainers.ContainerRequest
	dataDir   string
}

func (c *consulContainerNode) GetConfig() Config {
	return c.config
}

func newConsulContainerWithReq(ctx context.Context, req testcontainers.ContainerRequest) (testcontainers.Container, error) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}
	return container, nil
}

// NewConsulContainer starts a Consul node in a container with the given config.
func NewConsulContainer(ctx context.Context, config Config) (Node, error) {

	license, err := readLicense()
	if err != nil {
		return nil, err
	}
	name := utils.RandName("consul-")

	tmpDirData, err := ioutils.TempDir("", name)
	if err != nil {
		return nil, err
	}
	err = os.Chmod(tmpDirData, 0777)
	if err != nil {
		return nil, err
	}

	configFile, err := createConfigFile(config.HCL)
	if err != nil {
		return nil, err
	}
	skipReaper := isRYUKDisabled()
	req := testcontainers.ContainerRequest{
		Image:        consulImage + ":" + config.Version,
		ExposedPorts: []string{"8500/tcp"},
		WaitingFor:   wait.ForLog(bootLogLine).WithStartupTimeout(10 * time.Second),
		AutoRemove:   false,
		Name:         name,
		Mounts: testcontainers.ContainerMounts{
			testcontainers.ContainerMount{Source: testcontainers.DockerBindMountSource{HostPath: configFile}, Target: "/consul/config/config.hcl"},
			testcontainers.ContainerMount{Source: testcontainers.DockerBindMountSource{HostPath: tmpDirData}, Target: "/consul/data"},
		},
		Cmd:        config.Cmd,
		SkipReaper: skipReaper,
		Env:        map[string]string{"CONSUL_LICENSE": license},
	}
	container, err := newConsulContainerWithReq(ctx, req)
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
	c.config = config
	c.container = container
	c.ip = ip
	c.port = mappedPort.Int()
	apiConfig := api.DefaultConfig()
	apiConfig.Address = uri
	c.client, err = api.NewClient(apiConfig)
	c.ctx = ctx
	c.req = req
	c.dataDir = tmpDirData
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

func (c *consulContainerNode) Upgrade(ctx context.Context, config Config) error {
	file, err := createConfigFile(config.HCL)
	if err != nil {
		return err
	}
	c.req.Cmd = config.Cmd
	c.req.Mounts = testcontainers.ContainerMounts{
		testcontainers.ContainerMount{Source: testcontainers.DockerBindMountSource{HostPath: file}, Target: "/consul/config/config.hcl"},
		testcontainers.ContainerMount{Source: testcontainers.DockerBindMountSource{HostPath: c.dataDir}, Target: "/consul/data"},
	}
	c.req.Image = consulImage + ":" + config.Version
	err = c.container.Terminate(ctx)
	if err != nil {
		return err
	}
	container, err := newConsulContainerWithReq(ctx, c.req)
	if err != nil {
		return err
	}

	c.container = container

	localIP, err := container.Host(ctx)
	if err != nil {
		return err
	}

	mappedPort, err := container.MappedPort(ctx, "8500")
	if err != nil {
		return err
	}

	ip, err := container.ContainerIP(ctx)
	if err != nil {
		return err
	}
	uri := fmt.Sprintf("http://%s:%s", localIP, mappedPort.Port())
	c.ip = ip
	c.port = mappedPort.Int()
	apiConfig := api.DefaultConfig()
	apiConfig.Address = uri
	c.client, err = api.NewClient(apiConfig)
	c.ctx = ctx
	c.config = config

	if err != nil {
		return err
	}
	return nil
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

func readLicense() (string, error) {
	license := os.Getenv("CONSUL_LICENSE")
	if license == "" {
		licensePath := os.Getenv("CONSUL_LICENSE_PATH")
		if licensePath != "" {
			licenseBytes, err := os.ReadFile(licensePath)
			if err != nil {
				return "", err
			}
			license = string(licenseBytes)
		}
	}
	return license, nil
}

func createConfigFile(HCL string) (string, error) {
	tmpDir, err := ioutils.TempDir("", "consul-container-test-config")
	if err != nil {
		return "", err
	}
	err = os.Chmod(tmpDir, 0777)
	if err != nil {
		return "", err
	}
	err = os.Mkdir(tmpDir+"/config", 0777)
	if err != nil {
		return "", err
	}
	configFile := tmpDir + "/config/config.hcl"
	err = os.WriteFile(configFile, []byte(HCL), 0644)
	if err != nil {
		return "", err
	}
	return configFile, nil
}
