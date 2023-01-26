package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/go-multierror"
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
	pod            testcontainers.Container
	container      testcontainers.Container
	serverMode     bool
	datacenter     string
	config         Config
	podReq         testcontainers.ContainerRequest
	consulReq      testcontainers.ContainerRequest
	dataDir        string
	network        string
	id             int
	name           string
	terminateFuncs []func() error

	client           *api.Client
	clientAddr       string
	clientCACertFile string
	ip               string

	nextAdminPortOffset   int
	nextConnectPortOffset int

	info AgentInfo
}

func (c *consulContainerNode) GetPod() testcontainers.Container {
	return c.pod
}

func (c *consulContainerNode) ClaimAdminPort() int {
	p := 19000 + c.nextAdminPortOffset
	c.nextAdminPortOffset++
	return p
}

// NewConsulContainer starts a Consul agent in a container with the given config.
func NewConsulContainer(ctx context.Context, config Config, network string, index int) (Agent, error) {
	if config.ScratchDir == "" {
		return nil, fmt.Errorf("ScratchDir is required")
	}

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

	tmpDirData := filepath.Join(config.ScratchDir, "data")
	if err := os.MkdirAll(tmpDirData, 0777); err != nil {
		return nil, fmt.Errorf("error creating data directory %s: %w", tmpDirData, err)
	}
	if err := os.Chmod(tmpDirData, 0777); err != nil {
		return nil, fmt.Errorf("error chowning data directory %s: %w", tmpDirData, err)
	}

	var caCertFileForAPI string
	if config.CACert != "" {
		caCertFileForAPI = filepath.Join(config.ScratchDir, "ca.pem")
		if err := os.WriteFile(caCertFileForAPI, []byte(config.CACert), 0644); err != nil {
			return nil, fmt.Errorf("error writing out CA cert %s: %w", caCertFileForAPI, err)
		}
	}

	configFile, err := createConfigFile(config.ScratchDir, config.JSON)
	if err != nil {
		return nil, fmt.Errorf("error writing out config file %s: %w", configFile, err)
	}

	opts := containerOpts{
		name:              name,
		configFile:        configFile,
		dataDir:           tmpDirData,
		license:           license,
		addtionalNetworks: []string{"bridge", network},
		hostname:          fmt.Sprintf("agent-%d", index),
	}
	podReq, consulReq := newContainerRequest(config, opts)

	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	podContainer, err := startContainer(ctx, podReq)
	if err != nil {
		return nil, fmt.Errorf("error starting pod with image %q: %w", podReq.Image, err)
	}
	deferClean.Add(func() {
		_ = podContainer.Terminate(ctx)
	})

	var (
		httpPort  = pc.Ports.HTTP
		httpsPort = pc.Ports.HTTPS

		clientAddr       string
		clientCACertFile string

		info AgentInfo
	)
	if httpPort > 0 {
		uri, err := podContainer.PortEndpoint(ctx, "8500", "http")
		if err != nil {
			return nil, err
		}
		clientAddr = uri

	} else if httpsPort > 0 {
		uri, err := podContainer.PortEndpoint(ctx, "8501", "https")
		if err != nil {
			return nil, err
		}
		clientAddr = uri

		clientCACertFile = caCertFileForAPI

	} else {
		if pc.Server {
			return nil, fmt.Errorf("server container does not expose HTTP or HTTPS")
		}
	}

	if caCertFileForAPI != "" {
		if config.UseAPIWithTLS {
			if pc.Ports.HTTPS > 0 {
				info.UseTLSForAPI = true
			} else {
				return nil, fmt.Errorf("UseAPIWithTLS is set but ports.https is not for this agent")
			}
		}
		if config.UseGRPCWithTLS {
			if pc.Ports.GRPCTLS > 0 {
				info.UseTLSForGRPC = true
			} else {
				return nil, fmt.Errorf("UseGRPCWithTLS is set but ports.grpc_tls is not for this agent")
			}
		}
		info.CACertFile = clientCACertFile
	}

	ip, err := podContainer.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}

	consulContainer, err := startContainer(ctx, consulReq)
	if err != nil {
		return nil, fmt.Errorf("error starting main with image %q: %w", consulReq.Image, err)
	}
	deferClean.Add(func() {
		_ = consulContainer.Terminate(ctx)
	})

	if utils.FollowLog {
		if err := consulContainer.StartLogProducer(ctx); err != nil {
			return nil, err
		}
		deferClean.Add(func() {
			_ = consulContainer.StopLogProducer()
		})

		if config.LogConsumer != nil {
			consulContainer.FollowOutput(config.LogConsumer)
		} else {
			consulContainer.FollowOutput(&LogConsumer{
				Prefix: opts.name,
			})
		}
	}

	node := &consulContainerNode{
		config:     config,
		pod:        podContainer,
		container:  consulContainer,
		serverMode: pc.Server,
		datacenter: pc.Datacenter,
		ctx:        ctx,
		podReq:     podReq,
		consulReq:  consulReq,
		dataDir:    tmpDirData,
		network:    network,
		id:         index,
		name:       name,
		ip:         ip,
		info:       info,
	}

	if httpPort > 0 || httpsPort > 0 {
		apiConfig := api.DefaultConfig()
		apiConfig.Address = clientAddr
		if clientCACertFile != "" {
			apiConfig.TLSConfig.CAFile = clientCACertFile
		}

		apiClient, err := api.NewClient(apiConfig)
		if err != nil {
			return nil, err
		}

		node.client = apiClient
		node.clientAddr = clientAddr
		node.clientCACertFile = clientCACertFile
	}

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return node, nil
}

func (c *consulContainerNode) GetName() string {
	if c.container == nil {
		return c.consulReq.Name // TODO: is this safe to do all the time?
	}
	name, err := c.container.Name(c.ctx)
	if err != nil {
		return ""
	}
	return name
}

func (c *consulContainerNode) GetConfig() Config {
	return c.config.Clone()
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

func (c *consulContainerNode) GetAPIAddrInfo() (addr, caCert string) {
	return c.clientAddr, c.clientCACertFile
}

func (c *consulContainerNode) GetInfo() AgentInfo {
	return c.info
}

func (c *consulContainerNode) GetIP() string {
	return c.ip
}

func (c *consulContainerNode) RegisterTermination(f func() error) {
	c.terminateFuncs = append(c.terminateFuncs, f)
}

func (c *consulContainerNode) Exec(ctx context.Context, cmd []string) (int, error) {
	exit, _, err := c.container.Exec(ctx, cmd)
	return exit, err
}

func (c *consulContainerNode) Upgrade(ctx context.Context, config Config) error {
	if config.ScratchDir == "" {
		return fmt.Errorf("ScratchDir is required")
	}

	newConfigFile, err := createConfigFile(config.ScratchDir, config.JSON)
	if err != nil {
		return err
	}

	// We'll keep the same pod.
	opts := containerOpts{
		name:              c.consulReq.Name,
		configFile:        newConfigFile,
		dataDir:           c.dataDir,
		license:           "",
		addtionalNetworks: []string{"bridge", c.network},
		hostname:          c.consulReq.Hostname,
	}
	_, consulReq2 := newContainerRequest(config, opts)
	consulReq2.Env = c.consulReq.Env // copy license

	// sanity check two fields
	if consulReq2.Name != c.consulReq.Name {
		return fmt.Errorf("new name %q should match old name %q", consulReq2.Name, c.consulReq.Name)
	}
	if consulReq2.Hostname != c.consulReq.Hostname {
		return fmt.Errorf("new hostname %q should match old hostname %q", consulReq2.Hostname, c.consulReq.Hostname)
	}

	if err := c.TerminateAndRetainPod(); err != nil {
		return fmt.Errorf("error terminating running container during upgrade: %w", err)
	}

	c.consulReq = consulReq2

	container, err := startContainer(ctx, c.consulReq)
	c.ctx = ctx
	c.container = container
	if err != nil {
		return err
	}

	if utils.FollowLog {
		if err := container.StartLogProducer(ctx); err != nil {
			return err
		}
		container.FollowOutput(&LogConsumer{
			Prefix: opts.name,
		})
	}

	return nil
}

// Terminate attempts to terminate the agent container.
// This might also include running termination functions for containers associated with the agent.
// On failure, an error will be returned and the reaper process (RYUK) will handle cleanup.
func (c *consulContainerNode) Terminate() error {
	return c.terminate(false)
}
func (c *consulContainerNode) TerminateAndRetainPod() error {
	return c.terminate(true)
}
func (c *consulContainerNode) terminate(retainPod bool) error {
	// Services might register a termination function that should also fire
	// when the "agent" is cleaned up
	for _, f := range c.terminateFuncs {
		err := f()
		if err != nil {
			continue
		}
	}

	var merr error
	if c.container != nil {
		if err := TerminateContainer(c.ctx, c.container, true); err != nil {
			merr = multierror.Append(merr, err)
		}
		c.container = nil
	}

	if !retainPod && c.pod != nil {
		if err := TerminateContainer(c.ctx, c.pod, false); err != nil {
			merr = multierror.Append(merr, err)
		}

		c.pod = nil
	}

	return merr
}

func (c *consulContainerNode) DataDir() string {
	return c.dataDir
}

func startContainer(ctx context.Context, req testcontainers.ContainerRequest) (testcontainers.Container, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*40)
	defer cancel()
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

const pauseImage = "k8s.gcr.io/pause:3.3"

type containerOpts struct {
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
		Image:      pauseImage,
		AutoRemove: false,
		Name:       opts.name + "-pod",
		SkipReaper: skipReaper,
		ExposedPorts: []string{
			"8500/tcp",
			"8501/tcp",

			"8443/tcp", // Envoy Gateway Listener

			"5000/tcp", // Envoy App Listener
			"8079/tcp", // Envoy App Listener
			"8080/tcp", // Envoy App Listener
			"9998/tcp", // Envoy App Listener
			"9999/tcp", // Envoy App Listener

			"19000/tcp", // Envoy Admin Port
			"19001/tcp", // Envoy Admin Port
			"19002/tcp", // Envoy Admin Port
			"19003/tcp", // Envoy Admin Port
			"19004/tcp", // Envoy Admin Port
			"19005/tcp", // Envoy Admin Port
			"19006/tcp", // Envoy Admin Port
			"19007/tcp", // Envoy Admin Port
			"19008/tcp", // Envoy Admin Port
			"19009/tcp", // Envoy Admin Port
		},
		Hostname: opts.hostname,
		Networks: opts.addtionalNetworks,
	}

	// For handshakes like auto-encrypt, it can take 10's of seconds for the agent to become "ready".
	// If we only wait until the log stream starts, subsequent commands to agents will fail.
	// TODO: optimize the wait strategy
	app := testcontainers.ContainerRequest{
		NetworkMode: dockercontainer.NetworkMode("container:" + opts.name + "-pod"),
		Image:       config.DockerImage(),
		WaitingFor:  wait.ForLog(bootLogLine).WithStartupTimeout(60 * time.Second), // See note above
		AutoRemove:  false,
		Name:        opts.name,
		Mounts: []testcontainers.ContainerMount{
			{
				Source:   testcontainers.DockerBindMountSource{HostPath: opts.configFile},
				Target:   "/consul/config/config.json",
				ReadOnly: true,
			},
			{
				Source: testcontainers.DockerBindMountSource{HostPath: opts.dataDir},
				Target: "/consul/data",
			},
		},
		Cmd:        config.Cmd,
		SkipReaper: skipReaper,
		Env:        map[string]string{"CONSUL_LICENSE": opts.license},
	}

	if config.CertVolume != "" {
		app.Mounts = append(app.Mounts, testcontainers.ContainerMount{
			Source: testcontainers.DockerVolumeMountSource{
				Name: config.CertVolume,
			},
			Target:   "/consul/config/certs",
			ReadOnly: true,
		})
	}

	// fmt.Printf("app: %s\n", utils.Dump(app))

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
	if license := os.Getenv("CONSUL_LICENSE"); license != "" {
		return license, nil
	}

	licensePath := os.Getenv("CONSUL_LICENSE_PATH")
	if licensePath == "" {
		return "", nil
	}

	licenseBytes, err := os.ReadFile(licensePath)
	if err != nil {
		return "", err
	}
	return string(licenseBytes), nil
}

func createConfigFile(scratchDir string, JSON string) (string, error) {
	configDir := filepath.Join(scratchDir, "config")

	if err := os.MkdirAll(configDir, 0777); err != nil {
		return "", err
	}
	if err := os.Chmod(configDir, 0777); err != nil {
		return "", err
	}

	configFile := filepath.Join(configDir, "config.hcl")

	if err := os.WriteFile(configFile, []byte(JSON), 0644); err != nil {
		return "", err
	}
	return configFile, nil
}

type parsedConfig struct {
	Datacenter string      `json:"datacenter"`
	Server     bool        `json:"server"`
	Ports      parsedPorts `json:"ports"`
}

type parsedPorts struct {
	DNS     int `json:"dns"`
	HTTP    int `json:"http"`
	HTTPS   int `json:"https"`
	GRPC    int `json:"grpc"`
	GRPCTLS int `json:"grpc_tls"`
	SerfLAN int `json:"serf_lan"`
	SerfWAN int `json:"serf_wan"`
	Server  int `json:"server"`
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
