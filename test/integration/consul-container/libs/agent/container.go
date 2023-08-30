package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	certDir        string
	dataDir        string
	network        string
	id             int
	terminateFuncs []func() error
}

// NewConsulContainer starts a Consul agent in a container with the given config.
func NewConsulContainer(ctx context.Context, config Config, network string, index int) (Agent, error) {
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
	name := utils.RandName(fmt.Sprintf("%s-consul-%s-%d", pc.Datacenter, consulType, index))

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

	tmpCertData, err := ioutils.TempDir("", fmt.Sprintf("%s-certs", name))
	if err != nil {
		return nil, err
	}
	err = os.Chmod(tmpCertData, 0777)
	if err != nil {
		return nil, err
	}

	for filename, cert := range config.Certs {
		err := createCertFile(tmpCertData, filename, cert)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to write file %s", filename)
		}
	}

	opts := containerOpts{
		name:              name,
		certDir:           tmpCertData,
		configFile:        configFile,
		dataDir:           tmpDirData,
		license:           license,
		addtionalNetworks: []string{"bridge", network},
		hostname:          fmt.Sprintf("agent-%d", index),
	}
	podReq, consulReq := newContainerRequest(config, opts)

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
		certDir:    tmpCertData,
		network:    network,
		id:         index,
	}, nil
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

func (c *consulContainerNode) Upgrade(ctx context.Context, config Config, index int) error {
	pc, err := readSomeConfigFileFields(config.JSON)
	if err != nil {
		return err
	}

	consulType := "client"
	if pc.Server {
		consulType = "server"
	}
	name := utils.RandName(fmt.Sprintf("%s-consul-%s-%d", pc.Datacenter, consulType, index))

	// Inject new Agent name
	config.Cmd = append(config.Cmd, "-node", name)

	file, err := createConfigFile(config.JSON)
	if err != nil {
		return err
	}

	for filename, cert := range config.Certs {
		err := createCertFile(c.certDir, filename, cert)
		if err != nil {
			return errors.Wrapf(err, "failed to write file %s", filename)
		}
	}

	// We'll keep the same pod.
	opts := containerOpts{
		name:              c.consulReq.Name,
		certDir:           c.certDir,
		configFile:        file,
		dataDir:           c.dataDir,
		license:           "",
		addtionalNetworks: []string{"bridge", c.network},
		hostname:          fmt.Sprintf("agent-%d", c.id),
	}
	_, consulReq2 := newContainerRequest(config, opts)
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

func startContainer(ctx context.Context, req testcontainers.ContainerRequest) (testcontainers.Container, error) {
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

const pauseImage = "k8s.gcr.io/pause:3.3"

type containerOpts struct {
	certDir           string
	configFile        string
	dataDir           string
	hostname          string
	index             int
	license           string
	name              string
	addtionalNetworks []string
}

func newContainerRequest(config Config, opts containerOpts) (podRequest, consulRequest testcontainers.ContainerRequest) {
	skipReaper := isRYUKDisabled()

	pod := testcontainers.ContainerRequest{
		Image:        pauseImage,
		AutoRemove:   false,
		Name:         opts.name + "-pod",
		SkipReaper:   skipReaper,
		ExposedPorts: []string{"8500/tcp"},
		Hostname:     opts.hostname,
		Networks:     opts.addtionalNetworks,
	}

	// For handshakes like auto-encrypt, it can take 10's of seconds for the agent to become "ready".
	// If we only wait until the log stream starts, subsequent commands to agents will fail.
	// TODO: optimize the wait strategy
	app := testcontainers.ContainerRequest{
		NetworkMode: dockercontainer.NetworkMode("container:" + opts.name + "-pod"),
		Image:       utils.ImageName(config.Image, config.Version),
		WaitingFor:  wait.ForLog(bootLogLine).WithStartupTimeout(60 * time.Second), // See note above
		AutoRemove:  false,
		Name:        opts.name,
		Mounts: []testcontainers.ContainerMount{
			{Source: testcontainers.DockerBindMountSource{HostPath: opts.certDir}, Target: "/consul/config/certs"},
			{Source: testcontainers.DockerBindMountSource{HostPath: opts.configFile}, Target: "/consul/config/config.json"},
			{Source: testcontainers.DockerBindMountSource{HostPath: opts.dataDir}, Target: "/consul/data"},
		},
		Cmd:        config.Cmd,
		SkipReaper: skipReaper,
		Env:        map[string]string{"CONSUL_LICENSE": opts.license},
	}
	return pod, app
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

func createConfigFile(JSON string) (string, error) {
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
	err = os.WriteFile(configFile, []byte(JSON), 0644)
	if err != nil {
		return "", err
	}
	return configFile, nil
}

func createCertFile(dir, filename, cert string) error {
	filename = filepath.Base(filename)
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(cert), 0644)
	if err != nil {
		return errors.Wrap(err, "could not write cert file")
	}
	return nil
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
