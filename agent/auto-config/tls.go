// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package autoconf

import (
	"context"
	"fmt"
	"net"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbautoconf"
	"github.com/hashicorp/consul/proto/private/pbconnect"
)

const (
	// ID of the roots watch
	rootsWatchID = "roots"

	// ID of the leaf watch
	leafWatchID = "leaf"

	unknownTrustDomain = "unknown"
)

var (
	defaultDNSSANs = []string{"localhost"}

	defaultIPSANs = []net.IP{{127, 0, 0, 1}, net.ParseIP("::1")}
)

func extractPEMs(roots *structs.IndexedCARoots) []string {
	var pems []string
	for _, root := range roots.Roots {
		pems = append(pems, root.RootCert)
	}
	return pems
}

// updateTLSFromResponse will update the TLS certificate and roots in the shared
// TLS configurator.
func (ac *AutoConfig) updateTLSFromResponse(resp *pbautoconf.AutoConfigResponse) error {
	var pems []string
	for _, root := range resp.GetCARoots().GetRoots() {
		pems = append(pems, root.RootCert)
	}

	err := ac.acConfig.TLSConfigurator.UpdateAutoTLS(
		resp.ExtraCACertificates,
		pems,
		resp.Certificate.GetCertPEM(),
		resp.Certificate.GetPrivateKeyPEM(),
		resp.Config.GetTLS().GetVerifyServerHostname(),
	)

	if err != nil {
		return fmt.Errorf("Failed to update the TLS configurator with new certificates: %w", err)
	}

	return nil
}

func (ac *AutoConfig) setInitialTLSCertificates(certs *structs.SignedResponse) error {
	if certs == nil {
		return nil
	}

	if err := ac.populateCertificateCache(certs); err != nil {
		return fmt.Errorf("error populating cache with certificates: %w", err)
	}

	connectCAPems := extractPEMs(&certs.ConnectCARoots)

	err := ac.acConfig.TLSConfigurator.UpdateAutoTLS(
		certs.ManualCARoots,
		connectCAPems,
		certs.IssuedCert.CertPEM,
		certs.IssuedCert.PrivateKeyPEM,
		certs.VerifyServerHostname,
	)

	if err != nil {
		return fmt.Errorf("error updating TLS configurator with certificates: %w", err)
	}

	return nil
}

func (ac *AutoConfig) populateCertificateCache(certs *structs.SignedResponse) error {
	cert, err := connect.ParseCert(certs.IssuedCert.CertPEM)
	if err != nil {
		return fmt.Errorf("Failed to parse certificate: %w", err)
	}

	// prepolutate roots cache
	rootRes := cache.FetchResult{Value: &certs.ConnectCARoots, Index: certs.ConnectCARoots.QueryMeta.Index}
	rootsReq := ac.caRootsRequest()
	// getting the roots doesn't require a token so in order to potentially share the cache with another
	if err := ac.acConfig.Cache.Prepopulate(cachetype.ConnectCARootName, rootRes, ac.config.Datacenter, structs.DefaultPeerKeyword, "", rootsReq.CacheInfo().Key); err != nil {
		return err
	}

	leafReq := ac.leafCertRequest()

	// prepolutate leaf cache
	err = ac.acConfig.LeafCertManager.Prepopulate(
		context.Background(),
		leafReq.Key(),
		certs.IssuedCert.RaftIndex.ModifyIndex,
		&certs.IssuedCert,
		connect.EncodeSigningKeyID(cert.AuthorityKeyId),
	)
	if err != nil {
		return err
	}

	return nil
}

func (ac *AutoConfig) setupCertificateCacheWatches(ctx context.Context) (context.CancelFunc, error) {
	notificationCtx, cancel := context.WithCancel(ctx)

	rootsReq := ac.caRootsRequest()
	err := ac.acConfig.Cache.Notify(notificationCtx, cachetype.ConnectCARootName, &rootsReq, rootsWatchID, ac.cacheUpdates)
	if err != nil {
		cancel()
		return nil, err
	}

	leafReq := ac.leafCertRequest()
	err = ac.acConfig.LeafCertManager.Notify(notificationCtx, &leafReq, leafWatchID, ac.cacheUpdates)
	if err != nil {
		cancel()
		return nil, err
	}

	return cancel, nil
}

func (ac *AutoConfig) updateCARoots(roots *structs.IndexedCARoots) error {
	switch {
	case ac.config.AutoConfig.Enabled:
		ac.Lock()
		defer ac.Unlock()
		var err error
		ac.autoConfigResponse.CARoots, err = pbconnect.NewCARootsFromStructs(roots)
		if err != nil {
			return err
		}

		if err := ac.updateTLSFromResponse(ac.autoConfigResponse); err != nil {
			return err
		}
		return ac.persistAutoConfig(ac.autoConfigResponse)
	case ac.config.AutoEncryptTLS:
		pems := extractPEMs(roots)

		if err := ac.acConfig.TLSConfigurator.UpdateAutoTLSCA(pems); err != nil {
			return fmt.Errorf("failed to update Connect CA certificates: %w", err)
		}
		return nil
	default:
		return nil
	}
}

func (ac *AutoConfig) updateLeafCert(cert *structs.IssuedCert) error {
	switch {
	case ac.config.AutoConfig.Enabled:
		ac.Lock()
		defer ac.Unlock()
		var err error
		ac.autoConfigResponse.Certificate, err = pbconnect.NewIssuedCertFromStructs(cert)
		if err != nil {
			return err
		}

		if err := ac.updateTLSFromResponse(ac.autoConfigResponse); err != nil {
			return err
		}
		return ac.persistAutoConfig(ac.autoConfigResponse)
	case ac.config.AutoEncryptTLS:
		if err := ac.acConfig.TLSConfigurator.UpdateAutoTLSCert(cert.CertPEM, cert.PrivateKeyPEM); err != nil {
			return fmt.Errorf("failed to update the agent leaf cert: %w", err)
		}
		return nil
	default:
		return nil
	}
}

func (ac *AutoConfig) caRootsRequest() structs.DCSpecificRequest {
	return structs.DCSpecificRequest{Datacenter: ac.config.Datacenter}
}

func (ac *AutoConfig) leafCertRequest() leafcert.ConnectCALeafRequest {
	return leafcert.ConnectCALeafRequest{
		Datacenter:     ac.config.Datacenter,
		Agent:          ac.config.NodeName,
		DNSSAN:         ac.getDNSSANs(),
		IPSAN:          ac.getIPSANs(),
		Token:          ac.acConfig.Tokens.AgentToken(),
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(ac.config.PartitionOrEmpty()),
	}
}

// generateCSR will generate a CSR for an Agent certificate. This should
// be sent along with the AutoConfig.InitialConfiguration RPC or the
// AutoEncrypt.Sign RPC. The generated CSR does NOT have a real trust domain
// as when generating this we do not yet have the CA roots. The server will
// update the trust domain for us though.
func (ac *AutoConfig) generateCSR() (csr string, key string, err error) {
	// We don't provide the correct host here, because we don't know any
	// better at this point. Apart from the domain, we would need the
	// ClusterID, which we don't have. This is why we go with
	// unknownTrustDomain the first time. Subsequent CSRs will have the
	// correct TrustDomain.
	id := &connect.SpiffeIDAgent{
		// will be replaced
		Host:       unknownTrustDomain,
		Datacenter: ac.config.Datacenter,
		Agent:      ac.config.NodeName,
		Partition:  ac.config.PartitionOrDefault(),
	}

	caConfig, err := ac.config.ConnectCAConfiguration()
	if err != nil {
		return "", "", fmt.Errorf("Cannot generate CSR: %w", err)
	}

	conf, err := caConfig.GetCommonConfig()
	if err != nil {
		return "", "", fmt.Errorf("Failed to load common CA configuration: %w", err)
	}

	if conf.PrivateKeyType == "" {
		conf.PrivateKeyType = connect.DefaultPrivateKeyType
	}
	if conf.PrivateKeyBits == 0 {
		conf.PrivateKeyBits = connect.DefaultPrivateKeyBits
	}

	// Create a new private key
	pk, pkPEM, err := connect.GeneratePrivateKeyWithConfig(conf.PrivateKeyType, conf.PrivateKeyBits)
	if err != nil {
		return "", "", fmt.Errorf("Failed to generate private key: %w", err)
	}

	dnsNames := ac.getDNSSANs()
	ipAddresses := ac.getIPSANs()

	// Create a CSR.
	csr, err = connect.CreateCSR(id, pk, dnsNames, ipAddresses)
	if err != nil {
		return "", "", err
	}

	return csr, pkPEM, nil
}

func (ac *AutoConfig) getDNSSANs() []string {
	sans := defaultDNSSANs
	switch {
	case ac.config.AutoConfig.Enabled:
		sans = append(sans, ac.config.AutoConfig.DNSSANs...)
	case ac.config.AutoEncryptTLS:
		sans = append(sans, ac.config.AutoEncryptDNSSAN...)
	}
	return sans
}

func (ac *AutoConfig) getIPSANs() []net.IP {
	sans := defaultIPSANs
	switch {
	case ac.config.AutoConfig.Enabled:
		sans = append(sans, ac.config.AutoConfig.IPSANs...)
	case ac.config.AutoEncryptTLS:
		sans = append(sans, ac.config.AutoEncryptIPSAN...)
	}
	return sans
}
