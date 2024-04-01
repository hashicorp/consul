// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/mitchellh/copystructure"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/runner"
	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/secrets"
	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/tfgen"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/hashicorp/consul/testing/deployer/util"
)

// TODO: manage workdir externally without chdir

// Sprawl is the definition of a complete running Consul deployment topology.
type Sprawl struct {
	logger hclog.Logger
	// set after initial Launch is complete
	launchLogger hclog.Logger
	runner       *runner.Runner
	license      string
	secrets      secrets.Store

	workdir string

	// set during Run
	config    *topology.Config
	topology  *topology.Topology
	generator *tfgen.Generator

	clients        map[string]*api.Client      // one per cluster
	grpcConns      map[string]*grpc.ClientConn // one per cluster (when v2 enabled)
	grpcConnCancel map[string]func()           // one per cluster (when v2 enabled)
}

const (
	UpgradeTypeStandard  = "standard"
	UpgradeTypeAutopilot = "autopilot"
)

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

// ResourceServiceClientForCluster returns a shared common client that defaults
// to using the management token for this cluster.
func (s *Sprawl) ResourceServiceClientForCluster(clusterName string) pbresource.ResourceServiceClient {
	return pbresource.NewResourceServiceClient(s.grpcConns[clusterName])
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

	transport, err := util.ProxyHTTPTransport(network.ProxyPort)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: transport}, nil
}

// LocalAddressForNode returns the local address for the given node in the cluster
func (s *Sprawl) LocalAddressForNode(clusterName string, nid topology.NodeID) (string, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return "", fmt.Errorf("no such cluster: %s", clusterName)
	}
	node := cluster.NodeByID(nid)
	if !node.IsAgent() {
		return "", fmt.Errorf("node is not an agent")
	}
	return node.LocalAddress(), nil
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
		node.LocalProxyPort(),
		node.LocalAddress(),
		8500,
		token,
	)
}

// APIClientForCluster is a convenience wrapper for APIClientForNode that returns
// an API client for an agent node in the cluster, preferring clients, then servers
func (s *Sprawl) APIClientForCluster(clusterName, token string) (*api.Client, error) {
	clu := s.topology.Clusters[clusterName]
	// TODO: this always goes to the first client, but we might want to balance this
	firstAgent := clu.FirstClient("")
	if firstAgent == nil {
		firstAgent = clu.FirstServer()
	}
	if firstAgent == nil {
		return nil, fmt.Errorf("failed to find agent in cluster %s", clusterName)
	}
	return s.APIClientForNode(clusterName, firstAgent.ID(), token)
}

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
		logger:         logger,
		runner:         runner,
		workdir:        workdir,
		clients:        make(map[string]*api.Client),
		grpcConns:      make(map[string]*grpc.ClientConn),
		grpcConnCancel: make(map[string]func()),
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

	s.logger.Debug("compiled topology", "ct", jd(s.topology)) // TODO

	start := time.Now()
	if err := s.launch(); err != nil {
		return nil, err
	}
	s.logger.Info("topology is ready for use", "elapsed", time.Since(start))

	if err := s.PrintDetails(); err != nil {
		return nil, fmt.Errorf("error gathering diagnostic details: %w", err)
	}

	s.launchLogger = s.logger

	return s, nil
}

func (s *Sprawl) Relaunch(
	cfg *topology.Config,
) error {
	return s.RelaunchWithPhase(cfg, LaunchPhaseRegular)
}

// Upgrade upgrades the cluster to the targetImages version
// Parameters:
// - clusterName: the cluster to upgrade
// - upgradeType: the type of upgrade, standard or autopilot
// - targetImages: the target version to upgrade to
// - newServersInTopology: the number of new servers to add to the topology for autopilot upgrade only
// - validationFunc: the validation function to run during upgrade
func (s *Sprawl) Upgrade(
	cfg *topology.Config,
	clusterName string,
	upgradeType string,
	targetImages topology.Images,
	newServersInTopology []int,
	validationFunc func() error,
) error {
	cluster := cfg.Cluster(clusterName)
	if cluster == nil {
		return fmt.Errorf("cluster %s not found in topology", clusterName)
	}

	leader, err := s.Leader(cluster.Name)
	if err != nil {
		return fmt.Errorf("error get leader: %w", err)
	}
	s.logger.Info("Upgrade cluster", "cluster", cluster.Name, "type", upgradeType, "leader", leader.Name)

	switch upgradeType {
	case UpgradeTypeAutopilot:
		err = s.autopilotUpgrade(cfg, cluster, newServersInTopology, validationFunc)
	case UpgradeTypeStandard:
		err = s.standardUpgrade(cluster, targetImages, validationFunc)
	default:
		err = fmt.Errorf("upgrade type unsupported %s", upgradeType)
	}
	if err != nil {
		return fmt.Errorf("error upgrading cluster: %w", err)
	}

	s.logger.Info("After upgrade", "server_nodes", cluster.ServerNodes())
	return nil
}

// standardUpgrade upgrades server agents in the cluster to the targetImages
// individually
func (s *Sprawl) standardUpgrade(cluster *topology.Cluster,
	targetImages topology.Images, validationFunc func() error) error {
	upgradeFn := func(nodeID topology.NodeID) error {
		cfgUpgrade := s.Config()
		clusterCopy := cfgUpgrade.Cluster(cluster.Name)

		// update the server node's image
		node := clusterCopy.NodeByID(nodeID)
		node.Images = targetImages
		s.logger.Info("Upgrading", "node", nodeID.Name, "to_version", node.Images)
		err := s.RelaunchWithPhase(cfgUpgrade, LaunchPhaseUpgrade)
		if err != nil {
			return fmt.Errorf("error relaunch for upgrade: %w", err)
		}
		s.logger.Info("Relaunch completed", "node", node.Name)
		return nil
	}

	s.logger.Info("Upgrade to", "version", targetImages)

	// upgrade servers one at a time
	for _, node := range cluster.Nodes {
		if node.Kind != topology.NodeKindServer {
			s.logger.Info("Skip non-server node", "node", node.Name)
			continue
		}
		if err := upgradeFn(node.ID()); err != nil {
			return fmt.Errorf("error upgrading node %s: %w", node.Name, err)
		}

		// run the validation function after upgrading each server agent
		if validationFunc != nil {
			if err := validationFunc(); err != nil {
				return fmt.Errorf("error validating cluster: %w", err)
			}
		}
	}

	// upgrade client agents one at a time
	for _, node := range cluster.Nodes {
		if node.Kind != topology.NodeKindClient {
			s.logger.Info("Skip non-client node", "node", node.Name)
			continue
		}
		if err := upgradeFn(node.ID()); err != nil {
			return fmt.Errorf("error upgrading node %s: %w", node.Name, err)
		}

		// run the validation function after upgrading each client agent
		if validationFunc != nil {
			if err := validationFunc(); err != nil {
				return fmt.Errorf("error validating cluster: %w", err)
			}
		}
	}

	return nil
}

// autopilotUpgrade upgrades server agents by joining new servers with
// higher version. After upgrade completes, the number of server agents
// are doubled
func (s *Sprawl) autopilotUpgrade(cfg *topology.Config, cluster *topology.Cluster, newServersInTopology []int, validationFunc func() error) error {
	leader, err := s.Leader(cluster.Name)
	if err != nil {
		return fmt.Errorf("error get leader: %w", err)
	}

	// sanity check for autopilot upgrade
	if len(newServersInTopology) < len(cluster.ServerNodes()) {
		return fmt.Errorf("insufficient new nodes for autopilot upgrade, expect %d, got %d",
			len(cluster.ServerNodes()), len(newServersInTopology))
	}

	for _, nodeIdx := range newServersInTopology {
		node := cluster.Nodes[nodeIdx]
		if node.Kind != topology.NodeKindServer {
			return fmt.Errorf("node %s kind is not server", node.Name)
		}

		if !node.Disabled {
			return fmt.Errorf("node %s is already enabled", node.Name)
		}

		node.Disabled = false
		node.IsNewServer = true

		s.logger.Info("Joining new server", "node", node.Name)
	}

	err = s.RelaunchWithPhase(cfg, LaunchPhaseUpgrade)
	if err != nil {
		return fmt.Errorf("error relaunch for upgrade: %w", err)
	}
	s.logger.Info("Relaunch completed for autopilot upgrade")

	// Verify leader is transferred - if upgrade type is autopilot
	s.logger.Info("Waiting for leader transfer")
	time.Sleep(20 * time.Second)
	err = retry.Do(
		func() error {
			newLeader, err := s.Leader(cluster.Name)
			if err != nil {
				return fmt.Errorf("error get new leader: %w", err)
			}
			s.logger.Info("New leader", "addr", newLeader)

			if newLeader.Name == leader.Name {
				return fmt.Errorf("waiting for leader transfer")
			}

			return nil
		},
		retry.MaxDelay(5*time.Second),
		retry.Attempts(20),
	)
	if err != nil {
		return fmt.Errorf("Leader transfer failed: %w", err)
	}

	// Nodes joined the cluster, so we can set all new servers to false
	for _, node := range cluster.Nodes {
		node.IsNewServer = false
	}

	// Run the validation code
	if validationFunc != nil {
		if err := validationFunc(); err != nil {
			return fmt.Errorf("error validating cluster: %w", err)
		}
	}

	return nil
}

// RelaunchWithPhase relaunch the toplogy with the given phase
// and wait for the cluster to be ready (i.e, leadership is established)
func (s *Sprawl) RelaunchWithPhase(
	cfg *topology.Config,
	launchPhase LaunchPhase,
) error {
	// Copy this BEFORE compiling so we capture the original definition, without denorms.
	var err error
	s.config, err = copyConfig(cfg)
	if err != nil {
		return err
	}

	s.logger = s.launchLogger.Named(launchPhase.String())

	newTopology, err := topology.Recompile(s.logger.Named("recompile"), cfg, s.topology)
	if err != nil {
		return fmt.Errorf("topology.Compile: %w", err)
	}

	s.topology = newTopology

	s.logger.Debug("compiled replacement topology", "ct", jd(s.topology)) // TODO

	start := time.Now()
	if err := s.relaunch(launchPhase); err != nil {
		return err
	}
	s.logger.Info("topology is ready for use", "elapsed", time.Since(start))

	if err := s.PrintDetails(); err != nil {
		return fmt.Errorf("error gathering diagnostic details: %w", err)
	}

	return nil
}

// SnapshotSaveAndRestore saves a snapshot of a cluster and then restores the snapshot
func (s *Sprawl) SnapshotSaveAndRestore(clusterName string) error {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return fmt.Errorf("no such cluster: %s", clusterName)
	}
	var (
		client = s.clients[cluster.Name]
	)
	snapshot := client.Snapshot()
	snap, _, err := snapshot.Save(nil)
	if err != nil {
		return fmt.Errorf("error saving snapshot: %w", err)
	}
	s.logger.Info("snapshot saved")
	time.Sleep(3 * time.Second)
	defer snap.Close()

	// Restore the snapshot.
	if err := snapshot.Restore(nil, snap); err != nil {
		return fmt.Errorf("error restoring snapshot: %w", err)
	}
	s.logger.Info("snapshot restored")
	return nil
}

func (s *Sprawl) GetKV(cluster string, key string, queryOpts *api.QueryOptions) ([]byte, error) {
	client := s.clients[cluster]
	kvClient := client.KV()

	data, _, err := kvClient.Get(key, queryOpts)
	if err != nil {
		return nil, fmt.Errorf("error getting key: %w", err)
	}
	return data.Value, nil
}

func (s *Sprawl) LoadKVDataToCluster(cluster string, numberOfKeys int, writeOpts *api.WriteOptions) error {
	client := s.clients[cluster]
	kvClient := client.KV()

	for i := 0; i <= numberOfKeys; i++ {
		p := &api.KVPair{
			Key: fmt.Sprintf("key-%d", i),
		}
		token := make([]byte, 131072) // 128K size of value
		rand.Read(token)
		p.Value = token
		_, err := kvClient.Put(p, writeOpts)
		if err != nil {
			return fmt.Errorf("error writing kv: %w", err)
		}
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

func (s *Sprawl) DisabledServers(clusterName string) ([]*topology.Node, error) {
	cluster, ok := s.topology.Clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster: %s", clusterName)
	}

	var servers []*topology.Node

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
			for _, wrk := range n.Workloads {
				if wrk.Disabled || wrk.EnvoyAdminPort <= 0 {
					continue
				}
				prefix := fmt.Sprintf("http://%s:%d", n.LocalAddress(), wrk.EnvoyAdminPort)

				for fn, target := range targets {
					u := prefix + "/" + target

					body, err := scrapeURL(client, u)
					if err != nil {
						merr = multierror.Append(merr, fmt.Errorf("could not scrape %q for %s on %s: %w",
							target, wrk.ID.String(), n.ID().String(), err,
						))
						continue
					}

					outFn := filepath.Join(snapDir, n.DockerName()+"--"+wrk.ID.TFString()+"."+fn)

					if err := os.WriteFile(outFn+".tmp", body, 0644); err != nil {
						merr = multierror.Append(merr, fmt.Errorf("could not write output %q for %s on %s: %w",
							target, wrk.ID.String(), n.ID().String(), err,
						))
						continue
					}

					if err := os.Rename(outFn+".tmp", outFn); err != nil {
						merr = multierror.Append(merr, fmt.Errorf("could not write output %q for %s on %s: %w",
							target, wrk.ID.String(), n.ID().String(), err,
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

	s.logger.Debug("Capturing logs")

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

func (s *Sprawl) GetFileFromContainer(ctx context.Context, containerName string, filePath string) error {
	return s.runner.DockerExec(ctx, []string{"cp", containerName + ":" + filePath, filePath}, nil, nil)
}
