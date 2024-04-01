// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"net/url"
	"strings"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

type Store struct {
	m map[string]string
}

const (
	GossipKey      = "gossip"
	BootstrapToken = "bootstrap-token"
	AgentRecovery  = "agent-recovery"
	CAPEM          = "ca-pem"
)

func (s *Store) SaveGeneric(cluster, name, value string) {
	s.save(encode(cluster, "generic", name), value)
}

func (s *Store) ReadGeneric(cluster, name string) string {
	return s.read(encode(cluster, "generic", name))
}

func (s *Store) SaveAgentToken(cluster string, nid topology.NodeID, value string) {
	s.save(encode(cluster, "agent", nid.String()), value)
}

func (s *Store) ReadAgentToken(cluster string, nid topology.NodeID) string {
	return s.read(encode(cluster, "agent", nid.String()))
}

// Deprecated: SaveWorkloadToken
func (s *Store) SaveServiceToken(cluster string, wid topology.ID, value string) {
	s.SaveWorkloadToken(cluster, wid, value)
}

func (s *Store) SaveWorkloadToken(cluster string, wid topology.ID, value string) {
	s.save(encode(cluster, "workload", wid.String()), value)
}

// Deprecated: ReadWorkloadToken
func (s *Store) ReadServiceToken(cluster string, wid topology.ID) string {
	return s.ReadWorkloadToken(cluster, wid)
}

func (s *Store) ReadWorkloadToken(cluster string, wid topology.ID) string {
	return s.read(encode(cluster, "workload", wid.String()))
}

func (s *Store) save(key, value string) {
	if s.m == nil {
		s.m = make(map[string]string)
	}

	s.m[key] = value
}

func (s *Store) read(key string) string {
	if s.m == nil {
		return ""
	}

	v, ok := s.m[key]
	if !ok {
		return ""
	}
	return v
}

func encode(parts ...string) string {
	var out []string
	for _, p := range parts {
		out = append(out, url.QueryEscape(p))
	}
	return strings.Join(out, "/")
}
