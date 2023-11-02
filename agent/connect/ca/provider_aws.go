// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acmpca"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

const (
	// RootTemplateARN is the AWS-defined template we need to use when issuing a
	// root cert.
	RootTemplateARN = "arn:aws:acm-pca:::template/RootCACertificate/V1"

	// IntermediateTemplateARN is the AWS-defined template we need to use when
	// issuing an intermediate cert.
	IntermediateTemplateARN = "arn:aws:acm-pca:::template/SubordinateCACertificate_PathLen0/V1"

	// LeafTemplateARN is the AWS-defined template we need to use when issuing a
	// leaf cert.
	LeafTemplateARN = "arn:aws:acm-pca:::template/EndEntityCertificate/V1"

	// IntermediateTTL is the validity duration for the intermediate certs we
	// create.
	AWSIntermediateTTL = 1 * 365 * 24 * time.Hour

	// SignTimout is the maximum time we will spend waiting (polling) for a leaf
	// certificate to be signed.
	AWSSignTimeout = 45 * time.Second

	// CreateTimeout is the maximum time we will spend waiting (polling)
	// for the CA to be created.
	AWSCreateTimeout = 2 * time.Minute

	// AWSStateCAARNKey is the key in the provider State we store the ARN of the
	// CA we created if any.
	AWSStateCAARNKey = "CA_ARN"

	// day is a more readable shorthand for a duration of 24 hours. Note time
	// package doesn't provide time.Day due to ambiguity around DST and leap
	// seconds where a day may not actually be 24 hours.
	day = 24 * time.Hour
)

// AWSProvider implements Provider for AWS ACM PCA
type AWSProvider struct {
	stopped uint32 // atomically accessed, at start to prevent alignment issues
	stopCh  chan struct{}

	config          *structs.AWSCAProviderConfig
	session         *session.Session
	client          *acmpca.ACMPCA
	isPrimary       bool
	datacenter      string
	clusterID       string
	arn             string
	arnChecked      bool
	caCreated       bool
	rootPEM         string
	intermediatePEM string
	logger          hclog.Logger
}

var _ Provider = (*AWSProvider)(nil)

// NewAWSProvider returns a new AWSProvider
func NewAWSProvider(logger hclog.Logger) *AWSProvider {
	return &AWSProvider{logger: logger}
}

// Configure implements Provider
func (a *AWSProvider) Configure(cfg ProviderConfig) error {
	config, err := ParseAWSCAConfig(cfg.RawConfig)
	if err != nil {
		return err
	}

	// We only support setting IAM credentials through the normal methods ENV,
	// SharedCredentialsFile, IAM role. Per
	// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials
	// Putting them in CA config amounts to writing them to disk config file in
	// another place or sending them via API call and persisting them in state
	// store in a new place on disk. One of the existing standard solutions seems
	// better in all cases.
	awsSession, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return err
	}

	a.config = config
	a.session = awsSession
	a.isPrimary = cfg.IsPrimary
	a.clusterID = cfg.ClusterID
	a.datacenter = cfg.Datacenter
	a.client = acmpca.New(awsSession)
	a.stopCh = make(chan struct{})

	// Load the ARN from config or previous state.
	if config.ExistingARN != "" {
		a.arn = config.ExistingARN
	} else if arn := cfg.State[AWSStateCAARNKey]; arn != "" {
		a.arn = arn
		// We only pass ARN through state if we created the resource. We don't
		// "remember" previously existing resources the user configured.
		a.caCreated = true
	}

	return nil
}

// State implements Provider
func (a *AWSProvider) State() (map[string]string, error) {
	if a.arn == "" {
		return nil, nil
	}

	// Preserve the CA ARN if there is one
	state := make(map[string]string)
	state[AWSStateCAARNKey] = a.arn
	return state, nil
}

// GenerateCAChain implements Provider
func (a *AWSProvider) GenerateCAChain() (string, error) {
	if !a.isPrimary {
		return "", fmt.Errorf("provider is not the root certificate authority")
	}

	if err := a.ensureCA(); err != nil {
		return "", err
	}

	if a.rootPEM == "" {
		return "", fmt.Errorf("AWS CA provider not fully Initialized")
	}
	return a.rootPEM, nil
}

// ensureCA loads the CA resource to check it exists if configured by User or in
// state from previous run. Otherwise it creates a new CA of the correct type
// for this DC.
func (a *AWSProvider) ensureCA() error {
	// If we already have an ARN, we assume the CA is created and sanity check
	// it's available.
	if a.arn != "" {
		// Only check this once on startup not on every operation
		if a.arnChecked {
			return nil
		}

		// Load from the resource.
		input := &acmpca.DescribeCertificateAuthorityInput{
			CertificateAuthorityArn: aws.String(a.arn),
		}
		output, err := a.client.DescribeCertificateAuthority(input)
		if err != nil {
			return err
		}
		// Allow it to be active or pending a certificate (leadership might have
		// changed during a secondary initialization for example).
		if *output.CertificateAuthority.Status != acmpca.CertificateAuthorityStatusActive &&
			*output.CertificateAuthority.Status != acmpca.CertificateAuthorityStatusPendingCertificate {
			verb := "configured"
			if a.caCreated {
				verb = "created"
			}
			// Don't recreate CA that was manually disabled, force full deletion or
			// manual recreation. We might later support this or an explicit
			// "recreate" config option to allow rotating without a manual creation
			// but this is simpler and less surprising default behavior if user
			// disabled a CA due to a security concern and we just work around it.
			return fmt.Errorf("the %s PCA is not active: status is %s", verb,
				*output.CertificateAuthority.Status)
		}

		// Load the certs
		if err := a.loadCACerts(); err != nil {
			return err
		}
		a.arnChecked = true
		return nil
	}

	// Need to create a Private CA resource.
	err := a.createPCA()
	if err != nil {
		return err
	}

	// If we are in a secondary DC this is all we can do for now - the rest is
	// handled by the Initialization routine of calling GenerateIntermediateCSR
	// and then SetIntermediate.
	if !a.isPrimary {
		return nil
	}

	// CA is created and in PENDING_CERTIFCATE state, generate a self-signed cert
	// and install it.
	csrPEM, err := a.getCACSR()
	if err != nil {
		return err
	}

	// Self-sign it as a root
	certPEM, err := a.signCSR(csrPEM, RootTemplateARN, a.config.RootCertTTL)
	if err != nil {
		return err
	}

	// Submit the signed cert
	input := acmpca.ImportCertificateAuthorityCertificateInput{
		CertificateAuthorityArn: aws.String(a.arn),
		Certificate:             []byte(certPEM),
	}

	a.logger.Debug("uploading certificate for ARN", "arn", a.arn)
	_, err = a.client.ImportCertificateAuthorityCertificate(&input)
	if err != nil {
		return err
	}

	a.rootPEM = certPEM
	return nil
}

func keyTypeToAlgos(keyType string, keyBits int) (string, string, error) {
	switch keyType {
	case "rsa":
		switch keyBits {
		case 2048:
			return acmpca.KeyAlgorithmRsa2048, acmpca.SigningAlgorithmSha256withrsa, nil
		case 4096:
			return acmpca.KeyAlgorithmRsa4096, acmpca.SigningAlgorithmSha256withrsa, nil
		default:
			return "", "", fmt.Errorf("AWS PCA only supports RSA key lengths 2048"+
				" and 4096, PrivateKeyBits of %d configured", keyBits)
		}
	case "ec":
		if keyBits != 256 {
			return "", "", fmt.Errorf("AWS PCA only supports P256 EC curve, keyBits of %d configured", keyBits)
		}
		return acmpca.KeyAlgorithmEcPrime256v1, acmpca.SigningAlgorithmSha256withecdsa, nil
	default:
		return "", "", fmt.Errorf("AWS PCA only supports P256 EC curve, or RSA"+
			" 2048/4096. %s, %d configured", keyType, keyBits)
	}
}

func (a *AWSProvider) createPCA() error {
	pcaType := "ROOT" // For some reason there is no constant for this in the SDK
	if !a.isPrimary {
		pcaType = acmpca.CertificateAuthorityTypeSubordinate
	}

	keyAlg, signAlg, err := keyTypeToAlgos(a.config.PrivateKeyType, a.config.PrivateKeyBits)
	if err != nil {
		return err
	}

	uid, err := connect.CompactUID()
	if err != nil {
		return err
	}
	commonName := connect.CACN("aws", uid, a.clusterID, a.isPrimary)

	createInput := acmpca.CreateCertificateAuthorityInput{
		CertificateAuthorityType: aws.String(pcaType),
		CertificateAuthorityConfiguration: &acmpca.CertificateAuthorityConfiguration{
			Subject: &acmpca.ASN1Subject{
				CommonName: aws.String(commonName),
			},
			KeyAlgorithm:     aws.String(keyAlg),
			SigningAlgorithm: aws.String(signAlg),
		},
		RevocationConfiguration: &acmpca.RevocationConfiguration{
			// TODO support CRL in future when we manage revocation in Connect more
			// generally.
			CrlConfiguration: &acmpca.CrlConfiguration{
				Enabled: aws.Bool(false),
			},
		},
		// uid is unique to each PCA we create so use it as an idempotency string. We
		// don't actually retry on failure yet but might as well!
		IdempotencyToken: aws.String(uid),
		Tags: []*acmpca.Tag{
			{Key: aws.String("consul_cluster_id"), Value: aws.String(a.clusterID)},
			{Key: aws.String("consul_datacenter"), Value: aws.String(a.datacenter)},
		},
	}

	a.logger.Debug("creating new PCA", "common_name", commonName)
	createOutput, err := a.client.CreateCertificateAuthority(&createInput)
	if err != nil {
		a.logger.Error("failed to create new PCA", "common_name", commonName, "error", err)
		return err
	}

	// wait for PCA to be created
	newARN := *createOutput.CertificateAuthorityArn
	describeInput := acmpca.DescribeCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(newARN),
	}
	_, err = a.pollLoop("Private CA", AWSCreateTimeout, func() (bool, string, error) {
		describeOutput, err := a.client.DescribeCertificateAuthority(&describeInput)
		if err != nil {
			if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
				return true, "", fmt.Errorf("error waiting for PCA to be created: %s", err)
			}
		}
		if *describeOutput.CertificateAuthority.Status == acmpca.CertificateAuthorityStatusPendingCertificate {
			a.logger.Debug("new PCA is ready to accept a certificate", "pca", newARN)
			a.arn = newARN
			// We don't need to reload this ARN since we just created it and know what
			// state it's in
			a.arnChecked = true
			return true, "", nil
		}
		// Retry
		return false, "", nil
	})
	return err
}

func (a *AWSProvider) getCACSR() (string, error) {
	input := &acmpca.GetCertificateAuthorityCsrInput{
		CertificateAuthorityArn: aws.String(a.arn),
	}
	a.logger.Debug("retrieving CSR for PCA", "pca", a.arn)
	output, err := a.client.GetCertificateAuthorityCsr(input)
	if err != nil {
		return "", err
	}

	csrPEM := output.Csr
	if csrPEM == nil {
		// Probably shouldn't be able to happen but being defensive.
		return "", fmt.Errorf("invalid response from AWS PCA: CSR is nil")
	}
	return *csrPEM, nil
}

func (a *AWSProvider) loadCACerts() error {
	input := &acmpca.GetCertificateAuthorityCertificateInput{
		CertificateAuthorityArn: aws.String(a.arn),
	}
	output, err := a.client.GetCertificateAuthorityCertificate(input)
	if err != nil {
		return err
	}

	if a.isPrimary {
		// Just use the cert as a root
		a.rootPEM = lib.EnsureTrailingNewline(*output.Certificate)
	} else {
		a.intermediatePEM = lib.EnsureTrailingNewline(*output.Certificate)
		// TODO(banks) support user-supplied CA being a Subordinate even in the
		// primary DC. For now this assumes there is only one cert in the chain
		if output.CertificateChain == nil {
			return fmt.Errorf("Subordinate CA %s returned no chain", a.arn)
		}
		a.rootPEM = lib.EnsureTrailingNewline(*output.CertificateChain)
	}
	return nil
}

func (a *AWSProvider) signCSRRaw(csr *x509.CertificateRequest, templateARN string, ttl time.Duration) (string, error) {
	// PEM encode the CSR
	var pemBuf bytes.Buffer
	if err := pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr.Raw}); err != nil {
		return "", err
	}

	return a.signCSR(pemBuf.String(), templateARN, ttl)
}

// pollWait returns how long to wait for the next poll of an async operation. We
// optimize for times typically seen in the API. This is called _before_ the
// first poll so we can provide a typical delay since operations are never
// complete immediately so it's a waste to try.
func pollWait(attemptsMade int) time.Duration {
	// Hard code times for now
	waits := []time.Duration{
		// Never seen it complete first time with a lower value
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		3 * time.Second,
		5 * time.Second,
	}
	maxIdx := len(waits) - 1
	if attemptsMade > maxIdx {
		attemptsMade = maxIdx
	}
	return waits[attemptsMade]
}

func (a *AWSProvider) pollLoop(desc string, timeout time.Duration, f func() (bool, string, error)) (string, error) {
	attemptsMade := 0
	start := time.Now()
	wait := pollWait(attemptsMade)
	for {
		elapsed := time.Since(start)
		if elapsed >= timeout {
			return "", fmt.Errorf("timeout after %s waiting for %s", elapsed, desc)
		}

		a.logger.Debug(fmt.Sprintf("%s pending, waiting to check readiness", desc),
			"wait_time", wait,
		)
		select {
		case <-a.stopCh:
			// Provider discarded
			a.logger.Warn(fmt.Sprintf("provider instance terminated while waiting for %s.", desc))
			return "", fmt.Errorf("provider terminated")
		case <-time.After(wait):
			// Continue looping...
		}

		done, out, err := f()
		if err != nil {
			return "", err
		}
		if done {
			return out, err
		}

		attemptsMade++
		wait = pollWait(attemptsMade)
	}
}

func (a *AWSProvider) signCSR(csrPEM string, templateARN string, ttl time.Duration) (string, error) {
	_, signAlg, err := keyTypeToAlgos(a.config.PrivateKeyType, a.config.PrivateKeyBits)
	if err != nil {
		return "", err
	}

	issueInput := acmpca.IssueCertificateInput{
		CertificateAuthorityArn: aws.String(a.arn),
		Csr:                     []byte(csrPEM),
		SigningAlgorithm:        aws.String(signAlg),
		TemplateArn:             aws.String(templateARN),
		Validity: &acmpca.Validity{
			Value: aws.Int64(int64(ttl / day)),
			Type:  aws.String(acmpca.ValidityPeriodTypeDays),
		},
	}

	issueOutput, err := a.client.IssueCertificate(&issueInput)
	// ErrCodeLimitExceededException is used for both hard and soft limits in AWS
	// SDK :(. In this specific context though (issuing a certificate) there is no
	// hard limit on number of certs so a limit exceeded here is a rate limit.
	if aerr, ok := err.(awserr.Error); ok && err != nil {
		if aerr.Code() == acmpca.ErrCodeLimitExceededException {
			return "", ErrRateLimited
		}
	}
	if err != nil {
		return "", fmt.Errorf("error issuing certificate from PCA: %s", err)
	}

	// wait for certificate to be created
	certInput := acmpca.GetCertificateInput{
		CertificateAuthorityArn: aws.String(a.arn),
		CertificateArn:          issueOutput.CertificateArn,
	}
	return a.pollLoop(fmt.Sprintf("certificate %s", *issueOutput.CertificateArn),
		AWSSignTimeout,
		func() (bool, string, error) {
			certOutput, err := a.client.GetCertificate(&certInput)
			if err != nil {
				if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
					return true, "", fmt.Errorf("error retrieving certificate from PCA: %s", err)
				}
			}

			if certOutput.Certificate != nil {
				return true, lib.EnsureTrailingNewline(*certOutput.Certificate), nil
			}

			return false, "", nil
		})
}

// GenerateIntermediateCSR implements Provider
func (a *AWSProvider) GenerateIntermediateCSR() (string, string, error) {
	if a.isPrimary {
		return "", "", fmt.Errorf("provider is the root certificate authority, " +
			"cannot generate an intermediate CSR")
	}

	err := a.ensureCA()
	if err != nil {
		return "", "", err
	}

	// We should have the CA created now and should be able to generate the CSR.
	pem, err := a.getCACSR()
	return pem, "", err
}

// SetIntermediate implements Provider
func (a *AWSProvider) SetIntermediate(intermediatePEM string, rootPEM string, _ string) error {
	err := a.ensureCA()
	if err != nil {
		return err
	}

	// Install the certificate
	input := acmpca.ImportCertificateAuthorityCertificateInput{
		CertificateAuthorityArn: aws.String(a.arn),
		Certificate:             []byte(intermediatePEM),
		CertificateChain:        []byte(rootPEM),
	}
	a.logger.Debug("uploading certificate for PCA", "pca", a.arn)
	_, err = a.client.ImportCertificateAuthorityCertificate(&input)
	if err != nil {
		return err
	}

	// We successfully initialized, keep track of the root and intermediate certs.
	a.rootPEM = lib.EnsureTrailingNewline(rootPEM)
	a.intermediatePEM = lib.EnsureTrailingNewline(intermediatePEM)

	return nil
}

// ActiveLeafSigningCert implements Provider
func (a *AWSProvider) ActiveLeafSigningCert() (string, error) {
	err := a.ensureCA()
	if err != nil {
		return "", err
	}

	if a.rootPEM == "" {
		return "", fmt.Errorf("AWS CA provider not fully Initialized")
	}

	if a.isPrimary {
		// In the simple case the primary DC owns a Root CA and signs with it
		// directly so just return that for "intermediate" too since that is what we
		// will sign leafs with.
		//
		// TODO(banks) support user-supplied CA being a Subordinate even in the
		// primary DC. We'd have to figure that out here and return the actual
		// signing cert as well as somehow populate the intermediate chain.
		return a.rootPEM, nil
	}

	if a.intermediatePEM == "" {
		return "", fmt.Errorf("secondary AWS CA provider not fully Initialized")
	}

	return a.intermediatePEM, nil
}

// Sign implements Provider
func (a *AWSProvider) Sign(csr *x509.CertificateRequest) (string, error) {
	connect.HackSANExtensionForCSR(csr)

	if a.rootPEM == "" {
		return "", fmt.Errorf("AWS CA provider not fully Initialized")
	}

	a.logger.Debug("signing csr for requester",
		"requester", csr.Subject.CommonName,
	)

	return a.signCSRRaw(csr, LeafTemplateARN, a.config.LeafCertTTL)
}

// SignIntermediate implements Provider
func (a *AWSProvider) SignIntermediate(csr *x509.CertificateRequest) (string, error) {
	err := validateSignIntermediate(csr, connect.SpiffeIDSigningForCluster(a.clusterID))
	if err != nil {
		return "", err
	}

	// Sign it!
	return a.signCSRRaw(csr, IntermediateTemplateARN, AWSIntermediateTTL)
}

// CrossSignCA implements Provider
func (a *AWSProvider) CrossSignCA(newCA *x509.Certificate) (string, error) {
	return "", fmt.Errorf("not implemented in AWS PCA provider")
}

func (a *AWSProvider) disablePCA() error {
	if a.arn == "" {
		return nil
	}
	input := acmpca.UpdateCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(a.arn),
		Status:                  aws.String(acmpca.CertificateAuthorityStatusDisabled),
	}
	a.logger.Info("disabling PCA", "pca", a.arn)
	_, err := a.client.UpdateCertificateAuthority(&input)
	return err
}

func (a *AWSProvider) deletePCA() error {
	if a.arn == "" {
		return nil
	}
	input := acmpca.DeleteCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(a.arn),
		// We only ever use this to clean up after tests so delete as quickly as
		// possible (7 days).
		PermanentDeletionTimeInDays: aws.Int64(7),
	}
	a.logger.Info("deleting PCA", "pca", a.arn)
	_, err := a.client.DeleteCertificateAuthority(&input)
	return err
}

// Cleanup implements Provider
func (a *AWSProvider) Cleanup(providerTypeChange bool, otherConfig map[string]interface{}) error {
	old := atomic.SwapUint32(&a.stopped, 1)
	if old == 0 {
		close(a.stopCh)
	}

	if !providerTypeChange {
		awsConfig, err := ParseAWSCAConfig(otherConfig)
		if err != nil {
			return err
		}

		// if the provider is being replaced and using an existing PCA instance
		// then prevent deletion of that instance if the new provider uses
		// the same instance.
		if a.config.ExistingARN == awsConfig.ExistingARN {
			return nil
		}
	}

	if a.config.DeleteOnExit {
		if err := a.disablePCA(); err != nil {
			// Log the error but continue trying to delete as some errors may still
			// allow that and this is best-effort delete anyway.
			a.logger.Error("failed to disable PCA",
				"pca", a.arn,
				"error", err,
			)
		}
		if err := a.deletePCA(); err != nil {
			// Log the error but continue trying to delete as some errors may still
			// allow that and this is best-effort delete anyway.
			a.logger.Error("failed to delete PCA",
				"pca", a.arn,
				"error", err,
			)
		}
		// Don't stall leader shutdown, non of the failures here are fatal.
		return nil
	}
	return nil
}

// SupportsCrossSigning implements Provider
func (a *AWSProvider) SupportsCrossSigning() (bool, error) {
	return false, nil
}

// ParseAWSCAConfig parses and validates AWS CA Provider configuration.
func ParseAWSCAConfig(raw map[string]interface{}) (*structs.AWSCAProviderConfig, error) {
	config := structs.AWSCAProviderConfig{
		CommonCAProviderConfig: defaultCommonConfig(),
	}

	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook:       structs.ParseDurationFunc(),
		Result:           &config,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(raw); err != nil {
		return nil, fmt.Errorf("error decoding config: %s", err)
	}

	if err := config.CommonCAProviderConfig.Validate(); err != nil {
		return nil, err
	}

	// Extra keytype validation since PCA is more limited than other providers
	_, _, err = keyTypeToAlgos(config.PrivateKeyType, config.PrivateKeyBits)
	if err != nil {
		return nil, err
	}

	if config.LeafCertTTL < 24*time.Hour {
		return nil, fmt.Errorf("AWS PCA doesn't support certificates that are valid"+
			" for less than 24 hours, LeafTTL of %s configured", config.LeafCertTTL)
	}

	return &config, nil
}
