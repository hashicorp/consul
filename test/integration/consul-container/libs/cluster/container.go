// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	goretry "github.com/avast/retry-go"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const bootLogLine = "Consul agent running"
const disableRYUKEnv = "TESTCONTAINERS_RYUK_DISABLED"

// Exposed ports info
const MaxEnvoyOnNode = 10                  // the max number of Envoy sidecar can run along with the agent, base is 19000
const ServiceUpstreamLocalBindPort = 5000  // local bind Port of service's upstream
const ServiceUpstreamLocalBindPort2 = 5001 // local bind Port of service's upstream, for services with 2 upstreams
const debugPort = "4000/tcp"

// consulContainerNode implements the Agent interface by running a Consul agent
// in a container.
type consulContainerNode struct {
	ctx            context.Context
	pod            testcontainers.Container
	container      testcontainers.Container
	serverMode     bool
	datacenter     string
	partition      string
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

	grpcConn *grpc.ClientConn

	nextAdminPortOffset   int
	nextConnectPortOffset int

	info AgentInfo
}

func (c *consulContainerNode) GetPod() testcontainers.Container {
	return c.pod
}

func (c *consulContainerNode) Logs(context context.Context) (io.ReadCloser, error) {
	return c.container.Logs(context)
}

func (c *consulContainerNode) ClaimAdminPort() (int, error) {
	if c.nextAdminPortOffset >= MaxEnvoyOnNode {
		return 0, fmt.Errorf("running out of envoy admin port, max %d, already claimed %d",
			MaxEnvoyOnNode, c.nextAdminPortOffset)
	}
	p := 19000 + c.nextAdminPortOffset
	c.nextAdminPortOffset++
	return p, nil
}

// NewConsulContainer starts a Consul agent in a container with the given config.
func NewConsulContainer(ctx context.Context, config Config, cluster *Cluster, ports ...int) (Agent, error) {
	network := cluster.NetworkName
	index := cluster.Index
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

	name := config.NodeName
	if name == "" {
		// Generate a random name for the agent
		consulType := "client"
		if pc.Server {
			consulType = "server"
		}
		name = utils.RandName(fmt.Sprintf("%s-consul-%s-%d", pc.Datacenter, consulType, index))
	}

	// Inject new Agent name
	config.Cmd = append(config.Cmd, "-node", name)

	tmpDirData := filepath.Join(config.ScratchDir, "data")
	if err := os.MkdirAll(tmpDirData, 0777); err != nil {
		return nil, fmt.Errorf("error creating data directory %s: %w", tmpDirData, err)
	}
	if err := os.Chmod(tmpDirData, 0777); err != nil {
		return nil, fmt.Errorf("error chowning data directory %s: %w", tmpDirData, err)
	}

	if config.ExternalDataDir != "" {
		// copy consul persistent state from an external dir
		err := copy.Copy(config.ExternalDataDir, tmpDirData)
		if err != nil {
			return nil, fmt.Errorf("error copying persistent data from %s: %w", config.ExternalDataDir, err)
		}
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
	podReq, consulReq := newContainerRequest(config, opts, ports...)

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

		info     AgentInfo
		grpcConn *grpc.ClientConn
	)
	debugURI := ""
	if utils.Debug {
		if err := goretry.Do(
			func() (err error) {
				debugURI, err = podContainer.PortEndpoint(ctx, "4000", "tcp")
				return err
			},
			goretry.Delay(10*time.Second),
			goretry.RetryIf(func(err error) bool {
				return err != nil
			}),
		); err != nil {
			return nil, fmt.Errorf("container creating: %s", err)
		}
		info.DebugURI = debugURI
	}
	if httpPort > 0 {
		for i := 0; i < 10; i++ {
			uri, err := podContainer.PortEndpoint(ctx, "8500", "http")
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			clientAddr = uri
		}
		if err != nil {
			return nil, err
		}

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

	// TODO: Support gRPC+TLS port.
	if pc.Ports.GRPC > 0 {
		port, err := nat.NewPort("tcp", strconv.Itoa(pc.Ports.GRPC))
		if err != nil {
			return nil, fmt.Errorf("failed to parse gRPC TLS port: %w", err)
		}
		endpoint, err := podContainer.PortEndpoint(ctx, port, "tcp")
		if err != nil {
			return nil, fmt.Errorf("failed to get gRPC TLS endpoint: %w", err)
		}
		url, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse gRPC endpoint URL: %w", err)
		}
		conn, err := grpc.Dial(url.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("failed to dial gRPC connection: %w", err)
		}
		deferClean.Add(func() { _ = conn.Close() })
		grpcConn = conn
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
		partition:  pc.Partition,
		ctx:        ctx,
		podReq:     podReq,
		consulReq:  consulReq,
		dataDir:    tmpDirData,
		network:    network,
		id:         index,
		name:       name,
		ip:         ip,
		info:       info,
		grpcConn:   grpcConn,
	}

	if httpPort > 0 || httpsPort > 0 {
		apiConfig := api.DefaultConfig()
		apiConfig.Address = clientAddr
		if clientCACertFile != "" {
			apiConfig.TLSConfig.CAFile = clientCACertFile
		}

		if cluster.TokenBootstrap != "" {
			apiConfig.Token = cluster.TokenBootstrap
		}
		apiClient, err := api.NewClient(apiConfig)
		if err != nil {
			return nil, err
		}

		node.client = apiClient
		node.clientAddr = clientAddr
		node.clientCACertFile = clientCACertFile
	}

	// Inject node token if ACL is enabled and the bootstrap token is generated
	if cluster.TokenBootstrap != "" && cluster.ACLEnabled {
		agentToken, err := cluster.CreateAgentToken(pc.Datacenter, name)
		if err != nil {
			return nil, err
		}
		cmd := []string{"consul", "acl", "set-agent-token",
			"-token", cluster.TokenBootstrap,
			"agent", agentToken}

		// retry in case agent has not fully initialized
		err = goretry.Do(
			func() error {
				_, err := node.Exec(context.Background(), cmd)
				if err != nil {
					return fmt.Errorf("error setting the agent token, error %s", err)
				}
				return nil
			},
			goretry.Delay(time.Second*1),
		)
		if err != nil {
			return nil, fmt.Errorf("error setting agent token: %s", err)
		}
	}

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return node, nil
}

func (c *consulContainerNode) GetNetwork() string {
	return c.network
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

func (c *consulContainerNode) GetAgentName() string {
	return c.name
}

func (c *consulContainerNode) GetConfig() Config {
	return c.config.Clone()
}

func (c *consulContainerNode) GetDatacenter() string {
	return c.datacenter
}

func (c *consulContainerNode) GetPartition() string {
	return c.partition
}

func (c *consulContainerNode) IsServer() bool {
	return c.serverMode
}

// GetClient returns an API client that can be used to communicate with the Agent.
func (c *consulContainerNode) GetClient() *api.Client {
	return c.client
}

func (c *consulContainerNode) GetGRPCConn() *grpc.ClientConn {
	return c.grpcConn
}

// NewClient returns an API client by making a new one based on the provided token
// - updateDefault: if true update the default client
func (c *consulContainerNode) NewClient(token string, updateDefault bool) (*api.Client, error) {
	apiConfig := api.DefaultConfig()
	apiConfig.Address = c.clientAddr
	if c.clientCACertFile != "" {
		apiConfig.TLSConfig.CAFile = c.clientCACertFile
	}

	if token != "" {
		apiConfig.Token = token
	}
	apiClient, err := api.NewClient(apiConfig)
	if err != nil {
		return nil, err
	}

	if updateDefault {
		c.client = apiClient
	}
	return apiClient, nil
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

func (c *consulContainerNode) Exec(ctx context.Context, cmd []string) (string, error) {
	exitcode, reader, err := c.container.Exec(ctx, cmd)
	if exitcode != 0 {
		return "", fmt.Errorf("exec with exit code %d", exitcode)
	}
	if err != nil {
		return "", fmt.Errorf("exec with error %s", err)
	}

	buf, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("error reading from exe output: %s", err)
	}

	return string(buf), err
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

	if err := c.TerminateAndRetainPod(true); err != nil {
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
	return c.terminate(false, false)
}
func (c *consulContainerNode) TerminateAndRetainPod(skipFuncs bool) error {
	return c.terminate(true, skipFuncs)
}
func (c *consulContainerNode) terminate(retainPod bool, skipFuncs bool) error {
	// Services might register a termination function that should also fire
	// when the "agent" is cleaned up.
	// If skipFuncs is tru, We skip the terminateFuncs of connect sidecar, e.g.,
	// during upgrade
	if !skipFuncs {
		for _, f := range c.terminateFuncs {
			err := f()
			if err != nil {
				continue
			}
		}

		// if the pod is retained and therefore the IP then the grpc conn
		// should handle reconnecting so there is no reason to close it.
		c.closeGRPC()
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

func (c *consulContainerNode) closeGRPC() error {
	if c.grpcConn != nil {
		if err := c.grpcConn.Close(); err != nil {
			return err
		}
		c.grpcConn = nil
	}
	return nil
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

const pauseImage = "registry.k8s.io/pause:3.3"

type containerOpts struct {
	configFile        string
	dataDir           string
	hostname          string
	index             int
	license           string
	name              string
	addtionalNetworks []string
}

func newContainerRequest(config Config, opts containerOpts, ports ...int) (podRequest, consulRequest testcontainers.ContainerRequest) {
	skipReaper := isRYUKDisabled()

	pod := testcontainers.ContainerRequest{
		Image:      pauseImage,
		AutoRemove: false,
		Name:       opts.name + "-pod",
		SkipReaper: skipReaper,
		ExposedPorts: []string{
			"8500/tcp", // Consul HTTP API
			"8501/tcp", // Consul HTTPs API
			"8502/tcp", // Consul gRPC API

			"8443/tcp", // Envoy Gateway Listener

			"8079/tcp", // Envoy App Listener - grpc port used by static-server
			"8078/tcp", // Envoy App Listener - grpc port used by static-server-v1
			"8077/tcp", // Envoy App Listener - grpc port used by static-server-v2
			"8076/tcp", // Envoy App Listener - grpc port used by static-server-v3

			"8080/tcp", // Envoy App Listener - http port used by static-server
			"8081/tcp", // Envoy App Listener - http port used by static-server-v1
			"8082/tcp", // Envoy App Listener - http port used by static-server-v2
			"8083/tcp", // Envoy App Listener - http port used by static-server-v3

			"9997/tcp", // Envoy App Listener
			"9998/tcp", // Envoy App Listener
			"9999/tcp", // Envoy App Listener

			"80/tcp", // Nginx - http port used in wasm tests
		},
		Hostname: opts.hostname,
		Networks: opts.addtionalNetworks,
	}

	// Envoy upstream listener
	pod.ExposedPorts = append(pod.ExposedPorts, fmt.Sprintf("%d/tcp", ServiceUpstreamLocalBindPort))
	pod.ExposedPorts = append(pod.ExposedPorts, fmt.Sprintf("%d/tcp", ServiceUpstreamLocalBindPort2))

	// Reserve the exposed ports for Envoy admin port, e.g., 19000 - 19009
	basePort := 19000
	for i := 0; i < MaxEnvoyOnNode; i++ {
		pod.ExposedPorts = append(pod.ExposedPorts, fmt.Sprintf("%d/tcp", basePort+i))
	}

	for _, port := range ports {
		pod.ExposedPorts = append(pod.ExposedPorts, fmt.Sprintf("%d/tcp", port))
	}
	if utils.Debug {
		pod.ExposedPorts = append(pod.ExposedPorts, debugPort)
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
	Partition  string      `json:"partition"`
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
