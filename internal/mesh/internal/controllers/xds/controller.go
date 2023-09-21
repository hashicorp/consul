// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cacheshim"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds/status"
	proxysnapshot "github.com/hashicorp/consul/internal/mesh/proxy-snapshot"
	proxytracker "github.com/hashicorp/consul/internal/mesh/proxy-tracker"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/lib"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const ControllerName = "consul.io/xds-controller"

const defaultTenancy = "default"

func Controller(endpointsMapper *bimapper.Mapper, updater ProxyUpdater, fetcher TrustBundleFetcher, leafCertManager *leafcert.Manager, leafMapper *LeafMapper, leafCancels *LeafCancels, datacenter string) controller.Controller {
	leafCertEvents := make(chan controller.Event, 1000)
	if endpointsMapper == nil || fetcher == nil || leafCertManager == nil || leafMapper == nil || datacenter == "" {
		panic("endpointsMapper, updater, fetcher, leafCertManager, leafMapper, and datacenter are required")
	}

	return controller.ForType(pbmesh.ProxyStateTemplateType).
		WithWatch(pbcatalog.ServiceEndpointsType, endpointsMapper.MapLink).
		WithCustomWatch(proxySource(updater), proxyMapper).
		WithCustomWatch(&controller.Source{Source: leafCertEvents}, leafMapper.EventMapLink).
		WithPlacement(controller.PlacementEachServer).
		WithReconciler(&xdsReconciler{endpointsMapper: endpointsMapper, updater: updater, fetchTrustBundle: fetcher, leafCertManager: leafCertManager, leafCancels: leafCancels, leafCertEvents: leafCertEvents, leafMapper: leafMapper, datacenter: datacenter})
}

type xdsReconciler struct {
	// Fields for fetching and watching endpoints.
	endpointsMapper *bimapper.Mapper
	// Fields for proxy management.
	updater ProxyUpdater
	// Fields for fetching and watching trust bundles.
	fetchTrustBundle TrustBundleFetcher
	// Fields for fetching and watching leaf certificates.
	leafCertManager *leafcert.Manager
	leafMapper      *LeafMapper
	leafCancels     *LeafCancels
	leafCertEvents  chan controller.Event
	datacenter      string
}

type TrustBundleFetcher func() (*pbproxystate.TrustBundle, error)

// ProxyUpdater is an interface that defines the ability to push proxy updates to the updater
// and also check its connectivity to the server.
type ProxyUpdater interface {
	// PushChange allows pushing a computed ProxyState to xds for xds resource generation to send to a proxy.
	PushChange(id *pbresource.ID, snapshot proxysnapshot.ProxySnapshot) error

	// ProxyConnectedToServer returns whether this id is connected to this server.
	ProxyConnectedToServer(id *pbresource.ID) (string, bool)

	// EventChannel returns a channel of events that are consumed by the Custom Watcher.
	EventChannel() chan controller.Event
}

func (r *xdsReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	rt.Logger.Trace("reconciling  proxy state template", "id", req.ID)

	// Get the ProxyStateTemplate.
	proxyStateTemplate, err := getProxyStateTemplate(ctx, rt, req.ID)
	if err != nil {
		rt.Logger.Error("error reading proxy state template", "error", err)
		return err
	}

	token, proxyConnected := r.updater.ProxyConnectedToServer(req.ID)

	if proxyStateTemplate == nil || proxyStateTemplate.Template == nil || !proxyConnected {
		rt.Logger.Trace("proxy state template has been deleted or this controller is not responsible for this proxy state template", "id", req.ID)

		// If the proxy state template (PST) was deleted, we should:
		// 1. Remove references from endpoints mapper.
		// 2. Remove references from leaf mapper.
		// 3. Cancel all leaf watches.

		// 1. Remove PST from endpoints mapper.
		r.endpointsMapper.UntrackItem(req.ID)
		// Grab the leafs related to this PST before untracking the PST so we know which ones to cancel.
		leafLinks := r.leafMapper.LinkRefsForItem(req.ID)
		// 2. Remove PST from leaf mapper.
		r.leafMapper.UntrackItem(req.ID)

		// 3. Cancel watches for leafs that were related to this PST as long as it's not referenced by any other PST.
		r.cancelWatches(leafLinks)

		return nil
	}

	var (
		statusCondition *pbresource.Condition
		pstResource     *pbresource.Resource
	)
	pstResource = proxyStateTemplate.Resource

	if proxyStateTemplate.Template.ProxyState == nil {
		rt.Logger.Error("proxy state was missing from proxy state template")
		// Set the status.
		statusCondition = status.ConditionRejectedNilProxyState(status.KeyFromID(req.ID))
		status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

		return err
	}

	// TODO: Fetch trust bundles for all peers when peering is supported.
	trustBundle, err := r.fetchTrustBundle()
	if err != nil {
		rt.Logger.Error("error fetching root trust bundle", "error", err)
		// Set the status.
		statusCondition = status.ConditionRejectedTrustBundleFetchFailed(status.KeyFromID(req.ID))
		status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)
		return err
	}

	// Initialize ProxyState maps.
	if proxyStateTemplate.Template.ProxyState.TrustBundles == nil {
		proxyStateTemplate.Template.ProxyState.TrustBundles = make(map[string]*pbproxystate.TrustBundle)
	}
	// TODO: Figure out the correct key for the default trust bundle.
	proxyStateTemplate.Template.ProxyState.TrustBundles["local"] = trustBundle

	if proxyStateTemplate.Template.ProxyState.Endpoints == nil {
		proxyStateTemplate.Template.ProxyState.Endpoints = make(map[string]*pbproxystate.Endpoints)
	}
	if proxyStateTemplate.Template.ProxyState.LeafCertificates == nil {
		proxyStateTemplate.Template.ProxyState.LeafCertificates = make(map[string]*pbproxystate.LeafCertificate)
	}

	// Iterate through the endpoint references.
	// For endpoints, the controller should:
	//  1. Resolve ServiceEndpoint references
	//  2. Translate them into pbproxystate.Endpoints
	//  3. Add the pbproxystate.Endpoints to the ProxyState endpoints map.
	//  4. Track relationships between ProxyState and ServiceEndpoints, such that we can look up ServiceEndpoints and
	//  figure out which ProxyStates are associated with it (for mapping watches) and vice versa (for looking up
	//  references). The bimapper package is useful for tracking these relationships.
	endpointReferencesMap := proxyStateTemplate.Template.RequiredEndpoints
	var endpointsInProxyStateTemplate []resource.ReferenceOrID
	for xdsClusterName, endpointRef := range endpointReferencesMap {

		// Step 1: Resolve the reference by looking up the ServiceEndpoints.
		// serviceEndpoints will not be nil unless there is an error.
		//
		// TODO(rb/v2): note we should expose a flag on the endpointRef indicating if the user
		// wants the absence of an Endpoints to imply returning a slice of no data, vs failing outright.
		// In xdsv1 we call this the "allowEmpty" semantic. Here we are assuming "allowEmpty=true"
		var psEndpoints *pbproxystate.Endpoints
		if endpointRef.Id != nil {
			serviceEndpoints, err := getServiceEndpoints(ctx, rt, endpointRef.Id)
			if err != nil {
				rt.Logger.Error("error reading service endpoint", "id", endpointRef.Id, "error", err)
				// Set the status.
				statusCondition = status.ConditionRejectedErrorReadingEndpoints(status.KeyFromID(endpointRef.Id), err.Error())
				status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

				return err
			}

			// Step 2: Translate it into pbproxystate.Endpoints.
			psEndpoints, err = generateProxyStateEndpoints(serviceEndpoints, endpointRef.Port)
			if err != nil {
				rt.Logger.Error("error translating service endpoints to proxy state endpoints", "endpoint", endpointRef.Id, "error", err)

				// Set the status.
				statusCondition = status.ConditionRejectedCreatingProxyStateEndpoints(status.KeyFromID(endpointRef.Id), err.Error())
				status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

				return err
			}
		} else {
			psEndpoints = &pbproxystate.Endpoints{}
		}

		// Step 3: Add the endpoints to ProxyState.
		proxyStateTemplate.Template.ProxyState.Endpoints[xdsClusterName] = psEndpoints

		if endpointRef.Id != nil {
			// Track all the endpoints that are used by this ProxyStateTemplate, so we can use this for step 4.
			endpointResourceRef := resource.Reference(endpointRef.Id, "")
			endpointsInProxyStateTemplate = append(endpointsInProxyStateTemplate, endpointResourceRef)
		}
	}

	// Step 4: Track relationships between ProxyStateTemplates and ServiceEndpoints.
	r.endpointsMapper.TrackItem(req.ID, endpointsInProxyStateTemplate)
	if len(endpointsInProxyStateTemplate) == 0 {
		r.endpointsMapper.UntrackItem(req.ID)
	}

	// Iterate through leaf certificate references.
	// For each leaf certificate reference, the controller should:
	// 1. Setup a watch for the leaf certificate so that the leaf cert manager will generate and store a leaf
	// certificate if it's not already in the leaf cert manager cache.
	// 1a. Store a cancel function for that leaf certificate watch.
	// 2. Get the leaf certificate from the leaf cert manager. (This should succeed if a watch has been set up).
	// 3. Put the leaf certificate contents into the ProxyState leaf certificates map.
	// 4. Track relationships between ProxyState and leaf certificates using a bimapper.
	leafReferencesMap := proxyStateTemplate.Template.RequiredLeafCertificates
	var leafsInProxyStateTemplate []resource.ReferenceOrID
	for workloadIdentityName, leafRef := range leafReferencesMap {

		// leafRef must include the namespace and partition
		leafResourceReference := leafResourceRef(leafRef.Name, leafRef.Namespace, leafRef.Partition)
		leafKey := keyFromReference(leafResourceReference)
		leafRequest := &leafcert.ConnectCALeafRequest{
			Token:            token,
			WorkloadIdentity: leafRef.Name,
			EnterpriseMeta:   acl.NewEnterpriseMetaWithPartition(leafRef.Partition, leafRef.Namespace),
			// Add some jitter to the max query time so that all goroutines don't wake up at approximately the same time.
			// Without this, it's likely that these queries will all fire at roughly the same time, because the server
			// will have spawned many watches immediately on boot. Typically because the index number will not have changed,
			// this controller will not be notified anyway, but it's still better to space out the waking of goroutines.
			MaxQueryTime: (10 * time.Minute) + lib.RandomStagger(10*time.Minute),
		}

		// Step 1: Setup a watch for this leaf if one doesn't already exist.
		if _, ok := r.leafCancels.Get(leafKey); !ok {
			certWatchContext, cancel := context.WithCancel(ctx)
			err = r.leafCertManager.NotifyCallback(certWatchContext, leafRequest, "", func(ctx context.Context, event cacheshim.UpdateEvent) {
				cert, ok := event.Result.(*structs.IssuedCert)
				if !ok {
					panic("wrong type")
				}
				if cert == nil {
					return
				}
				controllerEvent := controller.Event{
					Obj: cert,
				}
				select {
				// This callback function is running in its own goroutine, so blocking inside this goroutine to send the
				// update event doesn't affect the controller or other leaf certificates. r.leafCertEvents is a buffered
				// channel, which should constantly be consumed by the controller custom events queue. If the controller
				// custom events consumer isn't clearing up the leafCertEvents channel, then that would be the main
				// issue to address, as opposed to this goroutine blocking.
				case r.leafCertEvents <- controllerEvent:
				// This context is the certWatchContext, so we will reach this case if the watch is canceled, and exit
				// the callback goroutine.
				case <-ctx.Done():
				}
			})
			if err != nil {
				rt.Logger.Error("error creating leaf watch", "leafRef", leafResourceReference, "error", err)
				// Set the status.
				statusCondition = status.ConditionRejectedErrorCreatingLeafWatch(keyFromReference(leafResourceReference), err.Error())
				status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

				cancel()
				return err
			}
			r.leafCancels.Set(leafKey, cancel)
		}

		// Step 2: Get the leaf certificate.
		cert, _, err := r.leafCertManager.Get(ctx, leafRequest)
		if err != nil {
			rt.Logger.Error("error getting leaf", "leafRef", leafResourceReference, "error", err)
			// Set the status.
			statusCondition = status.ConditionRejectedErrorGettingLeaf(keyFromReference(leafResourceReference), err.Error())
			status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

			return err
		}

		// Create the pbproxystate.LeafCertificate out of the structs.IssuedCert returned from the manager.
		psLeaf := generateProxyStateLeafCertificates(cert)
		if psLeaf == nil {
			rt.Logger.Error("error getting leaf certificate contents", "leafRef", leafResourceReference)

			// Set the status.
			statusCondition = status.ConditionRejectedErrorCreatingProxyStateLeaf(keyFromReference(leafResourceReference), err.Error())
			status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)

			return err
		}

		// Step 3: Add the leaf certificate to ProxyState.
		proxyStateTemplate.Template.ProxyState.LeafCertificates[workloadIdentityName] = psLeaf

		// Track all the leaf certificates that are used by this ProxyStateTemplate, so we can use this for step 4.
		leafsInProxyStateTemplate = append(leafsInProxyStateTemplate, leafResourceReference)

	}
	// Get the previously tracked leafs for this ProxyStateTemplate so we can use this to cancel watches in step 5.
	prevWatchedLeafs := r.leafMapper.LinkRefsForItem(req.ID)

	// Step 4: Track relationships between ProxyStateTemplates and leaf certificates for the current leafs referenced in
	// ProxyStateTemplate.
	r.leafMapper.TrackItem(req.ID, leafsInProxyStateTemplate)

	// Step 5: Compute whether there are leafs that are no longer referenced by this proxy state template, and cancel
	// watches for them if they aren't referenced anywhere.
	watches := prevWatchesToCancel(prevWatchedLeafs, leafsInProxyStateTemplate)
	r.cancelWatches(watches)

	// Now that the references have been resolved, push the computed proxy state to the updater.
	computedProxyState := proxyStateTemplate.Template.ProxyState

	err = r.updater.PushChange(req.ID, &proxytracker.ProxyState{ProxyState: computedProxyState})
	if err != nil {
		// Set the status.
		statusCondition = status.ConditionRejectedPushChangeFailed(status.KeyFromID(req.ID))
		status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)
		return err
	}

	// Set the status.
	statusCondition = status.ConditionAccepted()
	status.WriteStatusIfChanged(ctx, rt, pstResource, statusCondition)
	return nil
}

// leafResourceRef translates a leaf certificate reference in ProxyState template to an internal resource reference. The
// bimapper package uses resource references, so we use an internal type to create a leaf resource reference since leaf
// certificates are not v2 resources.
func leafResourceRef(workloadIdentity, namespace, partition string) *pbresource.Reference {
	// Since leaf certificate references aren't resources in the resource API, we don't have the same guarantees that
	// namespace and partition are set. So this is to ensure that we always do set values for tenancy.
	if namespace == "" {
		namespace = defaultTenancy
	}
	if partition == "" {
		partition = defaultTenancy
	}
	ref := &pbresource.Reference{
		Name: workloadIdentity,
		Type: InternalLeafType,
		Tenancy: &pbresource.Tenancy{
			Partition: partition,
			Namespace: namespace,
		},
	}
	return ref
}

// InternalLeafType sets up an internal resource type to use for leaf certificates, since they are not yet a v2
// resource. It's exported because it's used by the mesh controller registration which needs to set up the bimapper for
// leaf certificates.
var InternalLeafType = &pbresource.Type{
	Group:        "internal",
	GroupVersion: "v2beta1",
	Kind:         "leaf",
}

// keyFromReference is used to create string keys from resource references.
func keyFromReference(ref resource.ReferenceOrID) string {
	return fmt.Sprintf("%s/%s/%s",
		resource.ToGVK(ref.GetType()),
		tenancyToString(ref.GetTenancy()),
		ref.GetName())
}

func tenancyToString(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s.%s", tenancy.Partition, tenancy.Namespace)
}

// generateProxyStateLeafCertificates translates a *structs.IssuedCert into a *pbproxystate.LeafCertificate.
func generateProxyStateLeafCertificates(cert *structs.IssuedCert) *pbproxystate.LeafCertificate {
	if cert.CertPEM == "" || cert.PrivateKeyPEM == "" {
		return nil
	}
	return &pbproxystate.LeafCertificate{
		Cert: cert.CertPEM,
		Key:  cert.PrivateKeyPEM,
	}
}

// cancelWatches cancels watches for leafs that no longer need to be watched, as long as it is referenced by zero ProxyStateTemplates.
func (r *xdsReconciler) cancelWatches(leafResourceRefs []*pbresource.Reference) {
	for _, leaf := range leafResourceRefs {
		pstItems := r.leafMapper.ItemRefsForLink(leaf)
		if len(pstItems) > 0 {
			// Don't delete and cancel watches, since this leaf is referenced elsewhere.
			continue
		}
		cancel, ok := r.leafCancels.Get(keyFromReference(leaf))
		if ok {
			cancel()
			r.leafCancels.Delete(keyFromReference(leaf))
		}
	}
}

// prevWatchesToCancel computes if there are any items in prevWatchedLeafs that are not in currentLeafs, and returns a list of those items.
func prevWatchesToCancel(prevWatchedLeafs []*pbresource.Reference, currentLeafs []resource.ReferenceOrID) []*pbresource.Reference {
	prevWatchedLeafsToCancel := make([]*pbresource.Reference, 0, len(prevWatchedLeafs))
	newLeafs := make(map[string]struct{})
	for _, newLeaf := range currentLeafs {
		newLeafs[keyFromReference(newLeaf)] = struct{}{}
	}
	for _, prevLeaf := range prevWatchedLeafs {
		if _, ok := newLeafs[keyFromReference(prevLeaf)]; !ok {
			prevWatchedLeafsToCancel = append(prevWatchedLeafsToCancel, prevLeaf)
		}
	}
	return prevWatchedLeafsToCancel
}
