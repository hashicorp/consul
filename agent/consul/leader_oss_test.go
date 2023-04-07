//go:build !consulent
// +build !consulent

package consul

import libserf "github.com/hashicorp/consul/lib/serf"

func updateSerfTags(s *Server, key, value string) {
	libserf.UpdateTag(s.serfLAN, key, value)

	if s.serfWAN != nil {
		libserf.UpdateTag(s.serfWAN, key, value)
	}
}
