// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/exp/maps"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Manager) ValidateDependencies(registrations []resource.Registration) error {
	deps := m.CalculateDependencies(registrations)

	return deps.validate()
}

type Dependencies map[string][]string

func (deps Dependencies) validate() error {
	var merr error
	seen := make(map[string]map[string]struct{})

	mkErr := func(src, dst string) error {
		vals := []string{src, dst}
		sort.Strings(vals)
		return fmt.Errorf("circular dependency between %q and %q", vals[0], vals[1])
	}

	for src, dsts := range deps {
		seenDsts := seen[src]
		if len(seenDsts) == 0 {
			seen[src] = make(map[string]struct{})
		}

		for _, dst := range dsts {
			if _, ok := seenDsts[dst]; ok {
				merr = multierror.Append(merr, mkErr(src, dst))
			}

			if inverseDsts := seen[dst]; len(inverseDsts) > 0 {
				if _, ok := inverseDsts[src]; ok {
					merr = multierror.Append(merr, mkErr(src, dst))
				}
			}
			seen[src][dst] = struct{}{}
		}
	}

	return merr
}

func (m *Manager) CalculateDependencies(registrations []resource.Registration) Dependencies {
	typeToString := func(t *pbresource.Type) string {
		return strings.ToLower(fmt.Sprintf("%s/%s/%s", t.Group, t.GroupVersion, t.Kind))
	}

	out := make(map[string][]string)
	for _, r := range registrations {
		out[typeToString(r.Type)] = nil
	}

	for _, c := range m.controllers {
		watches := map[string]struct{}{}

		// Extend existing watch list if one is present. This is necessary
		// because there can be multiple controllers for a given type.
		// ProxyStateTemplate, for example, is controlled by sidecar proxy and
		// gateway proxy controllers.
		if existing, ok := out[typeToString(c.managedTypeWatch.watchedType)]; ok {
			for _, w := range existing {
				watches[w] = struct{}{}
			}
		}

		for _, w := range c.watches {
			watches[typeToString(w.watchedType)] = struct{}{}
		}

		out[typeToString(c.managedTypeWatch.watchedType)] = maps.Keys(watches)
	}

	return out
}

func (deps Dependencies) ToMermaid() string {
	depStrings := make([]string, 0, len(deps))

	for src, dsts := range deps {
		if len(dsts) == 0 {
			depStrings = append(depStrings, fmt.Sprintf("  %s", src))
			continue
		}

		for _, dst := range dsts {
			depStrings = append(depStrings, fmt.Sprintf("  %s --> %s", src, dst))
		}
	}

	sort.Slice(depStrings, func(a, b int) bool {
		return depStrings[a] < depStrings[b]
	})
	out := "flowchart TD\n" + strings.Join(depStrings, "\n")

	return out
}
