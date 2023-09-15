// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package selectiontracker

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/radix"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/lib/stringslice"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type WorkloadSelectionTracker struct {
	lock     sync.Mutex
	prefixes *radix.Tree[[]controller.Request]
	exact    *radix.Tree[[]controller.Request]

	// workloadSelectors contains a map keyed on resource names with values
	// being the selector that resource is currently associated with. This map
	// is kept mainly to make tracking removal operations more efficient.
	// Generally any operation that could take advantage of knowing where
	// in the trees the resource id is referenced can use this to prevent
	// needing to search the whole tree.
	workloadSelectors map[string]*pbcatalog.WorkloadSelector
}

func New() *WorkloadSelectionTracker {
	return &WorkloadSelectionTracker{
		prefixes:          radix.New[[]controller.Request](),
		exact:             radix.New[[]controller.Request](),
		workloadSelectors: make(map[string]*pbcatalog.WorkloadSelector),
	}
}

// MapWorkload will return a slice of controller.Requests with 1 resource for
// each resource that selects the specified Workload resource.
func (t *WorkloadSelectionTracker) MapWorkload(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var reqs []controller.Request

	// gather the list of all resources that select the specified workload using a prefix match
	t.prefixes.WalkPath(res.Id.Name, func(path string, requests []controller.Request) bool {
		reqs = append(reqs, requests...)
		return false
	})

	// gather the list of all resources that select the specified workload using an exact match
	exactReqs, _ := t.exact.Get(res.Id.Name)

	// return the combined list of all resources that select the specified workload
	return append(reqs, exactReqs...), nil
}

// TrackIDForSelector will associate workloads matching the specified workload
// selector with the given resource id.
func (t *WorkloadSelectionTracker) TrackIDForSelector(id *pbresource.ID, selector *pbcatalog.WorkloadSelector) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if previousSelector, found := t.workloadSelectors[id.Name]; found {
		if stringslice.Equal(previousSelector.Names, selector.Names) &&
			stringslice.Equal(previousSelector.Prefixes, selector.Prefixes) {
			// the selector is unchanged so do nothing
			return
		}

		// Potentially we could detect differences and do more minimal work. However
		// users are not expected to alter workload selectors often and therefore
		// not optimizing this further is probably fine. Therefore we are going
		// to wipe all tracking of the id and reinsert things.
		t.untrackID(id)
	}

	// loop over all the exact matching rules and associate those workload names
	// with the given resource id
	for _, name := range selector.GetNames() {
		// lookup any resource id associations for the given workload name
		leaf, _ := t.exact.Get(name)

		// append the ID to the existing request list
		t.exact.Insert(name, append(leaf, controller.Request{ID: id}))
	}

	// loop over all the prefix matching rules and associate those prefixes
	// with the given resource id.
	for _, prefix := range selector.GetPrefixes() {
		// lookup any resource id associations for the given workload name prefix
		leaf, _ := t.prefixes.Get(prefix)

		// append the new resource ID to the existing request list
		t.prefixes.Insert(prefix, append(leaf, controller.Request{ID: id}))
	}

	t.workloadSelectors[id.Name] = selector
}

// UntrackID causes the tracker to stop tracking the given resource ID
func (t *WorkloadSelectionTracker) UntrackID(id *pbresource.ID) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.untrackID(id)
}

// untrackID should be called to stop tracking a resource ID.
// This method assumes the lock is already held. Besides modifying
// the prefix & name trees to not reference this ID, it will also
// delete any corresponding entry within the workloadSelectors map
func (t *WorkloadSelectionTracker) untrackID(id *pbresource.ID) {
	selector, found := t.workloadSelectors[id.Name]
	if !found {
		return
	}

	removeIDFromTreeAtPaths(t.exact, id, selector.Names)
	removeIDFromTreeAtPaths(t.prefixes, id, selector.Prefixes)

	// If we don't do this deletion then reinsertion of the id for
	// tracking in the future could prevent selection criteria from
	// being properly inserted into the radix trees.
	delete(t.workloadSelectors, id.Name)
}

// removeIDFromTree will remove the given resource ID from all leaf nodes in the radix tree.
func removeIDFromTreeAtPaths(t *radix.Tree[[]controller.Request], id *pbresource.ID, paths []string) {
	for _, path := range paths {
		requests, _ := t.Get(path)

		foundIdx := -1
		for idx, req := range requests {
			if resource.EqualID(req.ID, id) {
				foundIdx = idx
				break
			}
		}

		if foundIdx != -1 {
			l := len(requests)

			if l == 1 {
				requests = nil
			} else if foundIdx == l-1 {
				requests = requests[:foundIdx]
			} else if foundIdx == 0 {
				requests = requests[1:]
			} else {
				requests = append(requests[:foundIdx], requests[foundIdx+1:]...)
			}

			if len(requests) > 1 {
				t.Insert(path, requests)
			} else {
				t.Delete(path)
			}
		}
	}
}
