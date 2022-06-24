package node

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcl"
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

	pc, err := readSomeConfigFileFields(config.HCL)
	if err != nil {
		return nil, err
	}

	configFile, err := createConfigFile(config.HCL)
	if err != nil {
		return nil, err
	}

	req := newContainerRequest(config, name, configFile, tmpDirData, license)
	container, err := newConsulContainerWithReq(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := container.StartLogProducer(ctx); err != nil {
		return nil, err
	}
	container.FollowOutput(&NodeLogConsumer{
		Prefix: pc.NodeName,
	})

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
	apiConfig := api.DefaultConfig()
	apiConfig.Address = uri
	apiClient, err := api.NewClient(apiConfig)
	if err != nil {
		return nil, err
	}

	return &consulContainerNode{
		config:    config,
		container: container,
		ip:        ip,
		port:      mappedPort.Int(),
		client:    apiClient,
		ctx:       ctx,
		req:       req,
		dataDir:   tmpDirData,
	}, nil
}

func newContainerRequest(config Config, name, configFile, dataDir, license string) testcontainers.ContainerRequest {
	skipReaper := isRYUKDisabled()

	return testcontainers.ContainerRequest{
		Image:        consulImage + ":" + config.Version,
		ExposedPorts: []string{"8500/tcp"},
		WaitingFor:   wait.ForLog(bootLogLine).WithStartupTimeout(10 * time.Second),
		AutoRemove:   false,
		Name:         name,
		Mounts: []testcontainers.ContainerMount{
			{Source: testcontainers.DockerBindMountSource{HostPath: configFile}, Target: "/consul/config/config.hcl"},
			{Source: testcontainers.DockerBindMountSource{HostPath: dataDir}, Target: "/consul/data"},
		},
		Cmd:        config.Cmd,
		SkipReaper: skipReaper,
		Env:        map[string]string{"CONSUL_LICENSE": license},
	}
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
	pc, err := readSomeConfigFileFields(config.HCL)
	if err != nil {
		return err
	}

	file, err := createConfigFile(config.HCL)
	if err != nil {
		return err
	}

	req2 := newContainerRequest(
		config,
		c.req.Name,
		file,
		c.dataDir,
		"",
	)
	req2.Env = c.req.Env // copy license

	_ = c.container.StopLogProducer()
	if err := c.container.Terminate(ctx); err != nil {
		return err
	}

	c.req = req2

	container, err := newConsulContainerWithReq(ctx, c.req)
	if err != nil {
		return err
	}

	if err := container.StartLogProducer(ctx); err != nil {
		return err
	}
	container.FollowOutput(&NodeLogConsumer{
		Prefix: pc.NodeName,
	})

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
	if c.container == nil {
		return nil
	}

	err := c.container.StopLogProducer()

	if err1 := c.container.Terminate(c.ctx); err == nil {
		err = err1
	}

	c.container = nil

	return err
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

type parsedConfig struct {
	NodeName string `hcl:"node_name"`
}

func readSomeConfigFileFields(HCL string) (parsedConfig, error) {
	var pc parsedConfig
	if err := hcl.Decode(&pc, HCL); err != nil {
		return pc, fmt.Errorf("Failed to parse config file: %w", err)
	}
	return pc, nil
}
