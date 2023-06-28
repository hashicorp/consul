package secrets

import (
	"net/url"
	"strings"

	"github.com/hashicorp/consul/testingconsul/topology"
)

type Store struct {
	m map[string]string
}

const (
	GossipKey      = "gossip"
	BootstrapToken = "bootstrap-token"
	AgentRecovery  = "agent-recovery"
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

func (s *Store) SaveServiceToken(cluster string, sid topology.ServiceID, value string) {
	s.save(encode(cluster, "service", sid.String()), value)
}

func (s *Store) ReadServiceToken(cluster string, sid topology.ServiceID) string {
	return s.read(encode(cluster, "service", sid.String()))
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
