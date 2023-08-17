// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxystateconverter

import (
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/configfetcher"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

// Converter converts a single snapshot into a ProxyState.
type Converter struct {
	Logger     hclog.Logger
	CfgFetcher configfetcher.ConfigFetcher
	proxyState *pbmesh.ProxyState
}

func NewConverter(
	logger hclog.Logger,
	cfgFetcher configfetcher.ConfigFetcher,
) *Converter {
	return &Converter{
		Logger:     logger,
		CfgFetcher: cfgFetcher,
		proxyState: &pbmesh.ProxyState{
			Listeners: make([]*pbproxystate.Listener, 0),
			Clusters:  make(map[string]*pbproxystate.Cluster),
			Routes:    make(map[string]*pbproxystate.Route),
			Endpoints: make(map[string]*pbproxystate.Endpoints),
		},
	}
}

func (g *Converter) ProxyStateFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) (*pbmesh.ProxyState, error) {
	err := g.resourcesFromSnapshot(cfgSnap)
	if err != nil {
		return nil, fmt.Errorf("failed to generate FullProxyState: %v", err)
	}

	return g.proxyState, nil
}

func (g *Converter) resourcesFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) error {
	err := g.tlsConfigFromSnapshot(cfgSnap)
	if err != nil {
		return err
	}

	err = g.listenersFromSnapshot(cfgSnap)
	if err != nil {
		return err
	}

	err = g.endpointsFromSnapshot(cfgSnap)
	if err != nil {
		return err
	}
	err = g.clustersFromSnapshot(cfgSnap)
	if err != nil {
		return err
	}

	err = g.routesFromSnapshot(cfgSnap)
	if err != nil {
		return err
	}

	//g.secretsFromSnapshot(cfgSnap)
	return nil
}

const localPeerKey = "local"

func (g *Converter) tlsConfigFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) error {
	proxyStateTLS := &pbproxystate.TLS{}
	g.proxyState.TrustBundles = make(map[string]*pbproxystate.TrustBundle)
	g.proxyState.LeafCertificates = make(map[string]*pbproxystate.LeafCertificate)

	// Set the TLS in the top level proxyState
	g.proxyState.Tls = proxyStateTLS

	// Add local trust bundle
	g.proxyState.TrustBundles[localPeerKey] = &pbproxystate.TrustBundle{
		TrustDomain: cfgSnap.Roots.TrustDomain,
		Roots:       []string{cfgSnap.RootPEMs()},
	}

	// Add peered trust bundles for remote peers that will dial this proxy.
	for _, peeringTrustBundle := range cfgSnap.PeeringTrustBundles() {
		g.proxyState.TrustBundles[peeringTrustBundle.PeerName] = &pbproxystate.TrustBundle{
			TrustDomain: peeringTrustBundle.GetTrustDomain(),
			Roots:       peeringTrustBundle.RootPEMs,
		}
	}

	// Add upstream peer trust bundles for dialing upstreams in remote peers.
	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil {
		if !(cfgSnap.Kind == structs.ServiceKindMeshGateway || cfgSnap.Kind == structs.ServiceKindTerminatingGateway) {
			return err
		}
	}
	if upstreamsSnapshot != nil {
		upstreamsSnapshot.UpstreamPeerTrustBundles.ForEachKeyE(func(k proxycfg.PeerName) error {
			tbs, ok := upstreamsSnapshot.UpstreamPeerTrustBundles.Get(k)
			if ok {
				g.proxyState.TrustBundles[k] = &pbproxystate.TrustBundle{
					TrustDomain: tbs.TrustDomain,
					Roots:       tbs.RootPEMs,
				}
			}
			return nil
		})
	}

	if cfgSnap.MeshConfigTLSOutgoing() != nil {
		proxyStateTLS.OutboundTlsParameters = makeTLSParametersFromTLSConfig(cfgSnap.MeshConfigTLSOutgoing().TLSMinVersion,
			cfgSnap.MeshConfigTLSOutgoing().TLSMaxVersion, cfgSnap.MeshConfigTLSOutgoing().CipherSuites)
	}

	if cfgSnap.MeshConfigTLSIncoming() != nil {
		proxyStateTLS.InboundTlsParameters = makeTLSParametersFromTLSConfig(cfgSnap.MeshConfigTLSIncoming().TLSMinVersion,
			cfgSnap.MeshConfigTLSIncoming().TLSMaxVersion, cfgSnap.MeshConfigTLSIncoming().CipherSuites)
	}

	return nil
}
