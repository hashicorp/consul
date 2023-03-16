package sprawl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul-topology/sprawl/internal/runner"
	"github.com/hashicorp/consul-topology/sprawl/internal/secrets"
	"github.com/hashicorp/consul-topology/sprawl/internal/tfgen"
	"github.com/hashicorp/consul-topology/topology"
	"github.com/hashicorp/consul-topology/util"
)

// TODO: manage workdir externally without chdir

// Sprawl is the definition of a complete running Consul deployment topology.
type Sprawl struct {
	logger  hclog.Logger
	runner  *runner.Runner
	license string
	secrets secrets.Store

	workdir string

	// set during Run
	config    *topology.Config
	topology  *topology.Topology
	generator *tfgen.Generator

	clients map[string]*api.Client // one per cluster
}

// Topology allows access to the topology that defines the resources. Do not
// write to any of these fields.
func (s *Sprawl) Topology() *topology.Topology {
	return s.topology
}

func (s *Sprawl) Config() *topology.Config {
	c2, err := copyConfig(s.config)
	if err != nil {
		panic(err)
	}
	return c2
}

// APIClientForNode gets a pooled api.Client connected to the agent running on
// the provided node.
//
// Passing an empty token will assume the bootstrap token. If you want to
// actually use the anonymous token say "-".
func (s *Sprawl) APIClientForNode(clusterName string, nid topology.NodeID, token string) (*api.Client, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", clusterName)
	}

	nid.Normalize()

	node := cluster.NodeByID(nid)
	if !node.IsAgent() {
		return nil, fmt.Errorf("node is not an agent")
	}

	switch token {
	case "":
		token = s.secrets.ReadGeneric(clusterName, secrets.BootstrapToken)
	case "-":
		token = ""
	}

	return util.ProxyAPIClient(
		node.LocalSquidPort(),
		node.LocalAddress(),
		8500,
		token,
	)
}

// func (s *Sprawl) HTTPClientForCluster(clusterName string) (*http.Client, error) {
// 	cluster, ok := s.topology.Clusters[clusterName]
// 	if !ok {
// 		return nil, fmt.Errorf("no such cluster: %s", clusterName)
// 	}

// 	proxyPort :=

// 	proxyURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(proxyPort))
// 	if err != nil {
// 		return nil, err
// 	}

// 	t := cleanhttp.DefaultPooledTransport()
// 	t.Proxy =
// }

func copyConfig(cfg *topology.Config) (*topology.Config, error) {
	dup, err := copystructure.Copy(cfg)
	if err != nil {
		return nil, err
	}
	return dup.(*topology.Config), nil
}

// Launch will create the topology defined by the provided configuration and
// bring up all of the relevant clusters. Once created the Stop method must be
// called to destroy everything.
func Launch(
	logger hclog.Logger,
	workdir string,
	cfg *topology.Config,
) (*Sprawl, error) {
	if logger == nil {
		panic("logger is required")
	}
	if workdir == "" {
		panic("workdir is required")
	}

	if err := os.MkdirAll(filepath.Join(workdir, "terraform"), 0755); err != nil {
		return nil, err
	}

	runner, err := runner.Load(logger)
	if err != nil {
		return nil, err
	}

	// Copy this to avoid leakage.
	cfg, err = copyConfig(cfg)
	if err != nil {
		return nil, err
	}

	s := &Sprawl{
		logger:  logger,
		runner:  runner,
		workdir: workdir,
		clients: make(map[string]*api.Client),
	}

	if err := s.ensureLicense(); err != nil {
		return nil, err
	}

	// Copy this AGAIN, BEFORE compiling so we capture the original definition, without denorms.
	s.config, err = copyConfig(cfg)
	if err != nil {
		return nil, err
	}

	s.topology, err = topology.Compile(logger.Named("compile"), cfg)
	if err != nil {
		return nil, fmt.Errorf("topology.Compile: %w", err)
	}

	s.logger.Info("compiled topology", "ct", jd(s.topology)) // TODO

	start := time.Now()
	if err := s.launch(); err != nil {
		return nil, err
	}
	s.logger.Info("topology is ready for use", "elapsed", time.Since(start))

	if err := s.PrintDetails(); err != nil {
		return nil, fmt.Errorf("error gathering diagnostic details: %w", err)
	}

	return s, nil
}

func (s *Sprawl) Relaunch(
	cfg *topology.Config,
) error {
	// Copy this BEFORE compiling so we capture the original definition, without denorms.
	var err error
	s.config, err = copyConfig(cfg)
	if err != nil {
		return err
	}

	newTopology, err := topology.Recompile(s.logger.Named("recompile"), cfg, s.topology)
	if err != nil {
		return fmt.Errorf("topology.Compile: %w", err)
	}

	s.topology = newTopology

	s.logger.Info("compiled replacement topology", "ct", jd(s.topology)) // TODO

	start := time.Now()
	if err := s.relaunch(); err != nil {
		return err
	}
	s.logger.Info("topology is ready for use", "elapsed", time.Since(start))

	if err := s.PrintDetails(); err != nil {
		return fmt.Errorf("error gathering diagnostic details: %w", err)
	}

	return nil
}

// Leader returns the cluster leader agent, or an error if no leader is
// available.
func (s *Sprawl) Leader(clusterName string) (*topology.Node, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", clusterName)
	}

	var (
		client = s.clients[cluster.Name]
		// logger = s.logger.With("cluster", cluster.Name)
	)

	leaderAddr, err := getLeader(client)
	if err != nil {
		return nil, err
	}

	for _, node := range cluster.Nodes {
		if !node.IsServer() || node.Disabled {
			continue
		}
		if strings.HasPrefix(leaderAddr, node.LocalAddress()+":") {
			return node, nil
		}
	}

	return nil, fmt.Errorf("leader not found")
}

// Followers returns the cluster following servers.
func (s *Sprawl) Followers(clusterName string) ([]*topology.Node, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", clusterName)
	}

	leaderNode, err := s.Leader(clusterName)
	if err != nil {
		return nil, fmt.Errorf("could not determine leader: %w", err)
	}

	var followers []*topology.Node

	for _, node := range cluster.Nodes {
		if !node.IsServer() || node.Disabled {
			continue
		}
		if node.ID() != leaderNode.ID() {
			followers = append(followers, node)
		}
	}

	return followers, nil
}
