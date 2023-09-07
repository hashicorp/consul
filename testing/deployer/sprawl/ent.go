// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

func (s *Sprawl) ensureLicense() error {
	if s.license != "" {
		return nil
	}
	v, err := readLicense()
	if err != nil {
		return err
	}
	s.license = v
	return nil
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
	return strings.TrimSpace(string(licenseBytes)), nil
}

func (s *Sprawl) initTenancies(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	// TODO: change this to UPSERT

	var (
		partClient = client.Partitions()
		nsClient   = client.Namespaces()

		partitionNameList []string
	)
	for _, ap := range cluster.Partitions {
		if ap.Name != "default" {
			old, _, err := partClient.Read(context.Background(), ap.Name, nil)
			if err != nil {
				return fmt.Errorf("error reading partition %q: %w", ap.Name, err)
			}
			if old == nil {
				obj := &api.Partition{
					Name: ap.Name,
				}

				_, _, err := partClient.Create(context.Background(), obj, nil)
				if err != nil {
					return fmt.Errorf("error creating partition %q: %w", ap.Name, err)
				}
				logger.Info("created partition", "partition", ap.Name)
			}

			partitionNameList = append(partitionNameList, ap.Name)
		}

		if err := s.createCrossNamespaceCatalogReadPolicies(cluster, ap.Name); err != nil {
			return fmt.Errorf("createCrossNamespaceCatalogReadPolicies[%s]: %w", ap.Name, err)
		}

		for _, ns := range ap.Namespaces {
			old, _, err := nsClient.Read(ns, &api.QueryOptions{Partition: ap.Name})
			if err != nil {
				return err
			}

			if old == nil {
				obj := &api.Namespace{
					Partition: ap.Name,
					Name:      ns,
					ACLs: &api.NamespaceACLConfig{
						PolicyDefaults: []api.ACLLink{
							{Name: "cross-ns-catalog-read"},
						},
					},
				}
				if ns == "default" {
					_, _, err := nsClient.Update(obj, nil)
					if err != nil {
						return err
					}
					logger.Info("updated namespace", "namespace", ns, "partition", ap.Name)
				} else {
					_, _, err := nsClient.Create(obj, nil)
					if err != nil {
						return err
					}
					logger.Info("created namespace", "namespace", ns, "partition", ap.Name)
				}
			}
		}
	}

	if err := s.waitUntilPartitionedSerfIsReady(context.TODO(), cluster, partitionNameList); err != nil {
		return fmt.Errorf("waitUntilPartitionedSerfIsReady: %w", err)
	}

	return nil
}

func (s *Sprawl) waitUntilPartitionedSerfIsReady(ctx context.Context, cluster *topology.Cluster, partitions []string) error {
	var (
		logger = s.logger.With("cluster", cluster.Name)
	)

	readyLogs := make(map[string]string)
	for _, partition := range partitions {
		readyLogs[partition] = `agent.server: Added serf partition to gossip network: partition=` + partition
	}

	start := time.Now()
	logger.Info("waiting for partitioned serf to be ready on all servers", "partitions", partitions)
	for _, node := range cluster.Nodes {
		if !node.IsServer() || node.Disabled {
			continue
		}

		var buf bytes.Buffer
		for {
			buf.Reset()

			err := s.runner.DockerExec(ctx, []string{
				"logs", node.DockerName(),
			}, &buf, nil)
			if err != nil {
				return fmt.Errorf("could not fetch docker logs from node %q: %w", node.ID(), err)
			}

			var (
				body  = buf.String()
				found []string
			)

			for partition, readyLog := range readyLogs {
				if strings.Contains(body, readyLog) {
					found = append(found, partition)
				}
			}

			if len(found) == len(readyLogs) {
				break
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	logger.Info("partitioned serf is ready on all servers", "partitions", partitions, "elapsed", time.Since(start))

	return nil
}
