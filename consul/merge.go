package consul

import (
	"log"

	"github.com/hashicorp/serf/serf"
)

// lanMergeDelegate is used to handle a cluster merge on the LAN gossip
// ring. We check that the peers are in the same datacenter and abort the
// merge if there is a mis-match.
type lanMergeDelegate struct {
	logger *log.Logger
	dc     string
}

func (md *lanMergeDelegate) NotifyMerge(members []*serf.Member) (cancel bool) {
	for _, m := range members {
		ok, dc := isConsulNode(*m)
		if ok {
			if dc != md.dc {
				md.logger.Printf("[WARN] consul: Canceling cluster merge, member '%s' part of wrong datacenter '%s'",
					m.Name, dc)
				return true
			}
			continue
		}

		ok, parts := isConsulServer(*m)
		if ok && parts.Datacenter != md.dc {
			md.logger.Printf("[WARN] consul: Canceling cluster merge, member '%s' part of wrong datacenter '%s'",
				m.Name, parts.Datacenter)
			return true
		}
	}
	return false
}

// wanMergeDelegate is used to handle a cluster merge on the WAN gossip
// ring. We check that the peers are server nodes and abort the merge
// otherwise.
type wanMergeDelegate struct {
	logger *log.Logger
}

func (md *wanMergeDelegate) NotifyMerge(members []*serf.Member) (cancel bool) {
	for _, m := range members {
		ok, _ := isConsulServer(*m)
		if !ok {
			md.logger.Printf("[WARN] consul: Canceling cluster merge, member '%s' is not a server",
				m.Name)
			return true
		}
	}
	return false
}
