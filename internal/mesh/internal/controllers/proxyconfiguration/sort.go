// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxyconfiguration

import (
	"sort"
	"strings"

	"github.com/oklog/ulid/v2"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
)

// SortProxyConfigurations sorts proxy configurations using the following rules:
//
//  1. Proxy config with a more specific selector wins. For example,
//     if there's a proxy config with a name selector and another conflicting
//     with a prefix selector, we will choose the one that selects by name because
//     it's more specific. For two prefix-based conflicting proxy configs, we will choose
//     the one that has the longer prefix.
//  2. Otherwise, the proxy configuration created first (i.e. with an earlier timestamp) wins.
//  3. Lastly, if creation timestamps are the same, the conflict will be resolved using lexicographic
//     order.
//
// It returns them in order such that proxy configurations that take precedence occur first in the list.
func SortProxyConfigurations(proxyCfgs []*types.DecodedProxyConfiguration, workloadName string) []*types.DecodedProxyConfiguration {
	// Copy proxy configs so that we don't mutate the original.
	proxyCfgsToSort := make([]*types.DecodedProxyConfiguration, len(proxyCfgs))
	for i, cfg := range proxyCfgs {
		proxyCfgsToSort[i] = cfg
	}

	sorter := proxyCfgsSorter{
		proxyCfgs:    proxyCfgsToSort,
		workloadName: workloadName,
	}

	sort.Sort(sorter)

	return sorter.proxyCfgs
}

type proxyCfgsSorter struct {
	proxyCfgs    []*types.DecodedProxyConfiguration
	workloadName string
}

func (p proxyCfgsSorter) Len() int { return len(p.proxyCfgs) }

// Less returns true if i-th element is less than j-th element.
func (p proxyCfgsSorter) Less(i, j int) bool {
	iPrefixMatch := p.findLongestPrefixMatch(i)
	iMatchesByName := p.matchesByName(i)

	jPrefixMatch := p.findLongestPrefixMatch(j)
	jMatchesByName := p.matchesByName(j)

	switch {
	// If i matches by name but j doesn't, then i should come before j.
	case iMatchesByName && !jMatchesByName:
		return true
	case !iMatchesByName && jMatchesByName:
		return false
	case !iMatchesByName && !jMatchesByName:
		if len(iPrefixMatch) != len(jPrefixMatch) {
			// In this case, the longest prefix wins.
			return len(iPrefixMatch) > len(jPrefixMatch)
		}

		// Fallthrough to the default case if lengths of prefix matches are the same.
		fallthrough
	case iMatchesByName && jMatchesByName:
		fallthrough
	default:
		iID := ulid.MustParse(p.proxyCfgs[i].Resource.Id.Uid)
		jID := ulid.MustParse(p.proxyCfgs[j].Resource.Id.Uid)
		if iID.Time() != jID.Time() {
			return iID.Time() < jID.Time()
		} else {
			// It's impossible for names to be equal, and so we are checking if
			// i's name is "less" lexicographically than j's name
			return strings.Compare(p.proxyCfgs[i].GetResource().GetId().GetName(),
				p.proxyCfgs[j].GetResource().GetId().GetName()) == -1
		}
	}
}

func (p proxyCfgsSorter) Swap(i, j int) {
	p.proxyCfgs[i], p.proxyCfgs[j] = p.proxyCfgs[j], p.proxyCfgs[i]
}

func (p proxyCfgsSorter) matchesByName(idx int) bool {
	return slices.Contains(p.proxyCfgs[idx].GetData().GetWorkloads().GetNames(), p.workloadName)
}

func (p proxyCfgsSorter) findLongestPrefixMatch(idx int) string {
	var prefixMatch string
	for _, prefix := range p.proxyCfgs[idx].GetData().GetWorkloads().GetPrefixes() {
		if strings.Contains(p.workloadName, prefix) &&
			len(prefix) > len(prefixMatch) {

			prefixMatch = prefix
		}
	}

	return prefixMatch
}
