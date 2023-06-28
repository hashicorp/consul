package sprawl

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul/testingconsul"
	"github.com/hashicorp/consul/testingconsul/sprawl/internal/runner"
	"github.com/hashicorp/consul/testingconsul/sprawl/internal/secrets"
	"github.com/hashicorp/consul/testingconsul/sprawl/internal/tfgen"
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
	config    *testingconsul.Config
	topology  *testingconsul.Topology
	generator *tfgen.Generator

	clients map[string]*api.Client // one per cluster
}

// Topology allows access to the topology that defines the resources. Do not
// write to any of these fields.
func (s *Sprawl) Topology() *testingconsul.Topology {
	return s.topology
}

func (s *Sprawl) Config() *testingconsul.Config {
	c2, err := copyConfig(s.config)
	if err != nil {
		panic(err)
	}
	return c2
}

func (s *Sprawl) HTTPClientForCluster(clusterName string) (*http.Client, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", clusterName)
	}

	// grab the local network for the cluster
	network, ok := s.topology.Networks[cluster.NetworkName]
	if !ok {
		return nil, fmt.Errorf("no such network: %s", cluster.NetworkName)
	}

	transport, err := testingconsul.ProxyHTTPTransport(network.ProxyPort)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: transport}, nil
}

// APIClientForNode gets a pooled api.Client connected to the agent running on
// the provided node.
//
// Passing an empty token will assume the bootstrap token. If you want to
// actually use the anonymous token say "-".
func (s *Sprawl) APIClientForNode(clusterName string, nid testingconsul.NodeID, token string) (*api.Client, error) {
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

	return testingconsul.ProxyAPIClient(
		node.LocalProxyPort(),
		node.LocalAddress(),
		8500,
		token,
	)
}

func copyConfig(cfg *testingconsul.Config) (*testingconsul.Config, error) {
	dup, err := copystructure.Copy(cfg)
	if err != nil {
		return nil, err
	}
	return dup.(*testingconsul.Config), nil
}

// Launch will create the topology defined by the provided configuration and
// bring up all of the relevant clusters. Once created the Stop method must be
// called to destroy everything.
func Launch(
	logger hclog.Logger,
	workdir string,
	cfg *testingconsul.Config,
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

	s.topology, err = testingconsul.Compile(logger.Named("compile"), cfg)
	if err != nil {
		return nil, fmt.Errorf("testingconsul.Compile: %w", err)
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
	cfg *testingconsul.Config,
) error {
	// Copy this BEFORE compiling so we capture the original definition, without denorms.
	var err error
	s.config, err = copyConfig(cfg)
	if err != nil {
		return err
	}

	newTopology, err := testingconsul.Recompile(s.logger.Named("recompile"), cfg, s.topology)
	if err != nil {
		return fmt.Errorf("testingconsul.Compile: %w", err)
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
func (s *Sprawl) Leader(clusterName string) (*testingconsul.Node, error) {
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
func (s *Sprawl) Followers(clusterName string) ([]*testingconsul.Node, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", clusterName)
	}

	leaderNode, err := s.Leader(clusterName)
	if err != nil {
		return nil, fmt.Errorf("could not determine leader: %w", err)
	}

	var followers []*testingconsul.Node

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

func (s *Sprawl) DisabledServers(clusterName string) ([]*testingconsul.Node, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", clusterName)
	}

	var servers []*testingconsul.Node

	for _, node := range cluster.Nodes {
		if !node.IsServer() || !node.Disabled {
			continue
		}
		servers = append(servers, node)
	}

	return servers, nil
}

func (s *Sprawl) StopContainer(ctx context.Context, containerName string) error {
	return s.runner.DockerExec(ctx, []string{"stop", containerName}, nil, nil)
}

func (s *Sprawl) SnapshotEnvoy(ctx context.Context) error {
	snapDir := filepath.Join(s.workdir, "envoy-snapshots")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return fmt.Errorf("could not create envoy snapshot output dir %s: %w", snapDir, err)
	}

	targets := map[string]string{
		"config_dump.json":     "config_dump",
		"clusters.json":        "clusters?format=json",
		"stats.txt":            "stats",
		"stats_prometheus.txt": "stats/prometheus",
	}

	var merr error
	for _, c := range s.topology.Clusters {
		client, err := s.HTTPClientForCluster(c.Name)
		if err != nil {
			return fmt.Errorf("could not get http client for cluster %q: %w", c.Name, err)
		}

		for _, n := range c.Nodes {
			if n.Disabled {
				continue
			}
			for _, s := range n.Services {
				if s.Disabled || s.EnvoyAdminPort <= 0 {
					continue
				}
				prefix := fmt.Sprintf("http://%s:%d", n.LocalAddress(), s.EnvoyAdminPort)

				for fn, target := range targets {
					u := prefix + "/" + target

					body, err := scrapeURL(client, u)
					if err != nil {
						merr = multierror.Append(merr, fmt.Errorf("could not scrape %q for %s on %s: %w",
							target, s.ID.String(), n.ID().String(), err,
						))
						continue
					}

					outFn := filepath.Join(snapDir, n.DockerName()+"--"+s.ID.TFString()+"."+fn)

					if err := os.WriteFile(outFn+".tmp", body, 0644); err != nil {
						merr = multierror.Append(merr, fmt.Errorf("could not write output %q for %s on %s: %w",
							target, s.ID.String(), n.ID().String(), err,
						))
						continue
					}

					if err := os.Rename(outFn+".tmp", outFn); err != nil {
						merr = multierror.Append(merr, fmt.Errorf("could not write output %q for %s on %s: %w",
							target, s.ID.String(), n.ID().String(), err,
						))
						continue
					}
				}
			}
		}
	}
	return merr
}

func scrapeURL(client *http.Client, url string) ([]byte, error) {
	res, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Sprawl) CaptureLogs(ctx context.Context) error {
	logDir := filepath.Join(s.workdir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("could not create log output dir %s: %w", logDir, err)
	}

	containers, err := s.listContainers(ctx)
	if err != nil {
		return err
	}

	s.logger.Info("Capturing logs")

	var merr error
	for _, container := range containers {
		if err := s.dumpContainerLogs(ctx, container, logDir); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("could not dump logs for container %s: %w", container, err))
		}
	}

	return merr
}

// Dump known containers out of terraform state file.
func (s *Sprawl) listContainers(ctx context.Context) ([]string, error) {
	tfdir := filepath.Join(s.workdir, "terraform")

	var buf bytes.Buffer
	if err := s.runner.TerraformExec(ctx, []string{"state", "list"}, &buf, tfdir); err != nil {
		return nil, fmt.Errorf("error listing containers in terraform state file: %w", err)
	}

	var (
		scan       = bufio.NewScanner(&buf)
		containers []string
	)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())

		name := strings.TrimPrefix(line, "docker_container.")
		if name != line {
			containers = append(containers, name)
			continue
		}
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}

	return containers, nil
}

func (s *Sprawl) dumpContainerLogs(ctx context.Context, containerName, outputRoot string) error {
	path := filepath.Join(outputRoot, containerName+".log")

	f, err := os.Create(path + ".tmp")
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		_ = f.Close()
		if !keep {
			_ = os.Remove(path + ".tmp")
			_ = os.Remove(path)
		}
	}()

	err = s.runner.DockerExecWithStderr(
		ctx,
		[]string{"logs", containerName},
		f,
		f,
		nil,
	)
	if err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(path+".tmp", path); err != nil {
		return err
	}

	keep = true
	return nil
}
