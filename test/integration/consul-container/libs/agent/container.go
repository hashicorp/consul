package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const bootLogLine = "Consul agent running"
const disableRYUKEnv = "TESTCONTAINERS_RYUK_DISABLED"

// consulContainerNode implements the Agent interface by running a Consul agent
// in a container.
type consulContainerNode struct {
	ctx            context.Context
	client         *api.Client
	pod            testcontainers.Container
	container      testcontainers.Container
	serverMode     bool
	ip             string
	port           int
	datacenter     string
	config         Config
	podReq         testcontainers.ContainerRequest
	consulReq      testcontainers.ContainerRequest
	dataDir        string
	terminateFuncs []func() error
}

func (c *consulContainerNode) GetName() string {
	name, err := c.container.Name(c.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (c *consulContainerNode) GetConfig() Config {
	return c.config
}

func (c *consulContainerNode) GetDatacenter() string {
	return c.datacenter
}

func (c *consulContainerNode) IsServer() bool {
	return c.serverMode
}

func startContainer(ctx context.Context, req testcontainers.ContainerRequest) (testcontainers.Container, error) {
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// NewConsulContainer starts a Consul agent in a container with the given config.
func NewConsulContainer(ctx context.Context, config Config) (Agent, error) {
	license, err := readLicense()
	if err != nil {
		return nil, err
	}

	pc, err := readSomeConfigFileFields(config.JSON)
	if err != nil {
		return nil, err
	}

	consulType := "client"
	if pc.Server {
		consulType = "server"
	}
	name := utils.RandName(fmt.Sprintf("%s-consul-%s", pc.Datacenter, consulType))

	// Inject new Agent name
	config.Cmd = append(config.Cmd, "-node", name)

	tmpDirData, err := ioutils.TempDir("", name)
	if err != nil {
		return nil, err
	}
	err = os.Chmod(tmpDirData, 0777)
	if err != nil {
		return nil, err
	}

	configFile, err := createConfigFile(config.JSON)
	if err != nil {
		return nil, err
	}

	podReq, consulReq := newContainerRequest(config, name, configFile, tmpDirData, license)

	podContainer, err := startContainer(ctx, podReq)
	if err != nil {
		return nil, err
	}

	localIP, err := podContainer.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := podContainer.MappedPort(ctx, "8500")
	if err != nil {
		return nil, err
	}

	ip, err := podContainer.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}

	consulContainer, err := startContainer(ctx, consulReq)
	if err != nil {
		return nil, err
	}

	if err := consulContainer.StartLogProducer(ctx); err != nil {
		return nil, err
	}
	consulContainer.FollowOutput(&LogConsumer{
		Prefix: name,
	})

	uri := fmt.Sprintf("http://%s:%s", localIP, mappedPort.Port())
	apiConfig := api.DefaultConfig()
	apiConfig.Address = uri
	apiClient, err := api.NewClient(apiConfig)
	if err != nil {
		return nil, err
	}

	return &consulContainerNode{
		config:     config,
		pod:        podContainer,
		container:  consulContainer,
		serverMode: pc.Server,
		ip:         ip,
		port:       mappedPort.Int(),
		datacenter: pc.Datacenter,
		client:     apiClient,
		ctx:        ctx,
		podReq:     podReq,
		consulReq:  consulReq,
		dataDir:    tmpDirData,
	}, nil
}

const pauseImage = "k8s.gcr.io/pause:3.3"

func newContainerRequest(config Config, name, configFile, dataDir, license string) (podRequest, consulRequest testcontainers.ContainerRequest) {
	skipReaper := isRYUKDisabled()

	pod := testcontainers.ContainerRequest{
		Image:        pauseImage,
		AutoRemove:   false,
		Name:         name + "-pod",
		SkipReaper:   skipReaper,
		ExposedPorts: []string{"8500/tcp"},
	}

	app := testcontainers.ContainerRequest{
		NetworkMode: dockercontainer.NetworkMode("container:" + name + "-pod"),
		Image:       config.Image + ":" + config.Version,
		WaitingFor:  wait.ForLog(bootLogLine).WithStartupTimeout(10 * time.Second),
		AutoRemove:  false,
		Name:        name,
		Mounts: []testcontainers.ContainerMount{
			{Source: testcontainers.DockerBindMountSource{HostPath: configFile}, Target: "/consul/config/config.json"},
			{Source: testcontainers.DockerBindMountSource{HostPath: dataDir}, Target: "/consul/data"},
		},
		Cmd:        config.Cmd,
		SkipReaper: skipReaper,
		Env:        map[string]string{"CONSUL_LICENSE": license},
	}

	return pod, app
}

// GetClient returns an API client that can be used to communicate with the Agent.
func (c *consulContainerNode) GetClient() *api.Client {
	return c.client
}

// GetAddr return the network address associated with the Agent.
func (c *consulContainerNode) GetAddr() (string, int) {
	return c.ip, c.port
}

func (c *consulContainerNode) RegisterTermination(f func() error) {
	c.terminateFuncs = append(c.terminateFuncs, f)
}

func (c *consulContainerNode) Upgrade(ctx context.Context, config Config) error {
	pc, err := readSomeConfigFileFields(config.JSON)
	if err != nil {
		return err
	}

	consulType := "client"
	if pc.Server {
		consulType = "server"
	}
	name := utils.RandName(fmt.Sprintf("%s-consul-%s", pc.Datacenter, consulType))

	// Inject new Agent name
	config.Cmd = append(config.Cmd, "-node", name)

	file, err := createConfigFile(config.JSON)
	if err != nil {
		return err
	}

	// We'll keep the same pod.
	_, consulReq2 := newContainerRequest(
		config,
		c.consulReq.Name,
		file,
		c.dataDir,
		"",
	)
	consulReq2.Env = c.consulReq.Env // copy license

	_ = c.container.StopLogProducer()
	if err := c.container.Terminate(ctx); err != nil {
		return err
	}

	c.consulReq = consulReq2

	container, err := startContainer(ctx, c.consulReq)
	if err != nil {
		return err
	}

	if err := container.StartLogProducer(ctx); err != nil {
		return err
	}
	container.FollowOutput(&LogConsumer{
		Prefix: name,
	})

	c.container = container

	return nil
}

// Terminate attempts to terminate the agent container.
// This might also include running termination functions for containers associated with the agent.
// On failure, an error will be returned and the reaper process (RYUK) will handle cleanup.
func (c *consulContainerNode) Terminate() error {

	// Services might register a termination function that should also fire
	// when the "agent" is cleaned up
	for _, f := range c.terminateFuncs {
		err := f()
		if err != nil {

		}
	}

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
	Datacenter string `json:"datacenter"`
	Server     bool   `json:"server"`
}

func readSomeConfigFileFields(JSON string) (parsedConfig, error) {
	var pc parsedConfig
	if err := json.Unmarshal([]byte(JSON), &pc); err != nil {
		return pc, errors.Wrap(err, "failed to parse config file")
	}
	if pc.Datacenter == "" {
		pc.Datacenter = "dc1"
	}
	return pc, nil
}
