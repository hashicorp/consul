package ca

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acmpca"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	RootTemplateARN         = "arn:aws:acm-pca:::template/RootCACertificate/V1"
	IntermediateTemplateARN = "arn:aws:acm-pca:::template/SubordinateCACertificate_PathLen0/V1"
	LeafTemplateARN         = "arn:aws:acm-pca:::template/EndEntityCertificate/V1"
)

const (
	RootTTL         = 5 * 365 * 24 * time.Hour
	IntermediateTTL = 1 * 365 * 24 * time.Hour
)

type AWSProvider struct {
	stopped uint32 // atomically accessed, at start to prevent alignment issues
	stopCh  chan struct{}

	config          *structs.AWSCAProviderConfig
	session         *session.Session
	client          *acmpca.ACMPCA
	primaryDC       bool
	datacenter      string
	clusterID       string
	arn             string
	arnChecked      bool
	caCreated       bool
	rootPEM         string
	intermediatePEM string
	logger          *log.Logger
}

// SetLogger implements NeedsLogger
func (a *AWSProvider) SetLogger(l *log.Logger) {
	a.logger = l
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
	a.primaryDC = cfg.PrimaryDC
	a.clusterID = cfg.ClusterID
	a.datacenter = cfg.Datacenter
	a.client = acmpca.New(awsSession)
	a.stopCh = make(chan struct{})

	// Load the ARN from config or previous state.
	if config.ExistingARN != "" {
		a.arn = config.ExistingARN
	} else if arn := cfg.State["CA_ARN"]; arn != "" {
		a.arn = arn
		// We only pass ARN through state if we created the resource. We don't
		// "remember" previously existing resources the user configured.
		a.caCreated = true
	}

	return nil
}

func (a *AWSProvider) State() (map[string]string, error) {
	if a.arn == "" {
		return nil, nil
	}

	// Preserve the CA ARN if there is one
	state := make(map[string]string)
	state["CA_ARN"] = a.arn
	return state, nil
}

func (a *AWSProvider) GenerateRoot() error {
	if !a.primaryDC {
		return fmt.Errorf("provider is not the root certificate authority")
	}

	return a.ensureCA()
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
	if !a.primaryDC {
		return nil
	}

	// CA is created and in PENDING_CERTIFCATE state, generate a self-signed cert
	// and install it.
	csrPEM, err := a.getCACSR()
	if err != nil {
		return err
	}

	// Self-sign it as a root
	certPEM, err := a.signCSR(csrPEM, RootTemplateARN, RootTTL)
	if err != nil {
		return err
	}

	// Submit the signed cert
	input := acmpca.ImportCertificateAuthorityCertificateInput{
		CertificateAuthorityArn: aws.String(a.arn),
		Certificate:             []byte(certPEM),
	}

	a.logger.Printf("[DEBUG] connect.ca.aws: uploading certificate for %s", a.arn)
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
	if !a.primaryDC {
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
	commonName := connect.CACN("aws", uid, a.clusterID, a.primaryDC)

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
		// uid is unique each PCA we create so use it as an idempotency string. We
		// don't actually retry on failure yet but might as well!
		IdempotencyToken: aws.String(uid),
		Tags: []*acmpca.Tag{
			{Key: aws.String("consul_cluster_id"), Value: aws.String(a.clusterID)},
			{Key: aws.String("consul_datacenter"), Value: aws.String(a.datacenter)},
		},
	}

	a.logger.Printf("[DEBUG] creating new PCA %s", commonName)
	createOutput, err := a.client.CreateCertificateAuthority(&createInput)
	if err != nil {
		return err
	}

	// wait for PCA to be created
	newARN := *createOutput.CertificateAuthorityArn
	describeInput := acmpca.DescribeCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(newARN),
	}

	for {
		describeOutput, err := a.client.DescribeCertificateAuthority(&describeInput)
		if err != nil {
			if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
				a.logger.Printf("[ERR] connect.ca.aws: error describing PCA: %s", err.Error())
				return fmt.Errorf("error waiting for PCA to be created: %s", err)
			}
		}

		if *describeOutput.CertificateAuthority.Status == acmpca.CertificateAuthorityStatusPendingCertificate {
			a.logger.Printf("[DEBUG] connect.ca.aws: new PCA %s is ready to accept a certificate", newARN)
			a.arn = newARN
			// we don't need to reload this ARN since we just created it and know what state it's in
			a.arnChecked = true
			return nil
		}

		a.logger.Printf("[DEBUG] connect.ca.aws: CA is not ready, waiting %s to check again",
			a.config.PollInterval)

		select {
		case <-a.stopCh:
			// Provider discarded
			a.logger.Print("[WARN] connect.ca.aws: provider instance terminated while waiting for a new CA resource to be ready.")
			return nil
		case <-time.After(a.config.PollInterval):
			// Continue looping...
		}
	}
}

func (a *AWSProvider) getCACSR() (string, error) {
	input := &acmpca.GetCertificateAuthorityCsrInput{
		CertificateAuthorityArn: aws.String(a.arn),
	}
	a.logger.Printf("[DEBUG] connect.ca.aws: retrieving CSR for %s", a.arn)
	output, err := a.client.GetCertificateAuthorityCsr(input)
	if err != nil {
		a.logger.Printf("[ERR] connect.ca.aws: error retrieving CSR: %s", err)
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

	if a.primaryDC {
		// Just use the cert as a root
		a.rootPEM = *output.Certificate
	} else {
		a.intermediatePEM = *output.Certificate
		// TODO(banks) support user-supplied CA being a Subordinate even in the
		// primary DC. For now this assumes there is only one cert in the chain
		if output.CertificateChain == nil {
			return fmt.Errorf("Subordinate CA %s returned no chain", a.arn)
		}
		a.rootPEM = *output.CertificateChain
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
			Value: aws.Int64(int64(ttl.Seconds() / 86400.0)),
			Type:  aws.String(acmpca.ValidityPeriodTypeDays),
		},
	}

	issueOutput, err := a.client.IssueCertificate(&issueInput)
	if err != nil {
		return "", fmt.Errorf("error issuing certificate from PCA: %s", err)
	}

	// wait for certificate to be created
	certInput := acmpca.GetCertificateInput{
		CertificateAuthorityArn: aws.String(a.arn),
		CertificateArn:          issueOutput.CertificateArn,
	}
	for {
		certOutput, err := a.client.GetCertificate(&certInput)
		if err != nil {
			if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
				return "", fmt.Errorf("error retrieving certificate from PCA: %s", err)
			}
		}

		if certOutput.Certificate != nil {
			return *certOutput.Certificate, nil
		}

		a.logger.Printf("[DEBUG] connect.ca.aws: certificate %s is not ready"+
			", waiting %s to check again", *issueOutput.CertificateArn,
			a.config.PollInterval)

		select {
		case <-a.stopCh:
			// Provider discarded
			a.logger.Print("[WARN] connect.ca.aws: provider instance terminated"+
				" while waiting for a new certificate %s to be ready.",
				issueOutput.CertificateArn)
			return "", fmt.Errorf("provider terminated")
		case <-time.After(a.config.PollInterval):
			// Continue looping...
		}
	}
}

func (a *AWSProvider) ActiveRoot() (string, error) {
	err := a.ensureCA()
	if err != nil {
		return "", err
	}

	if a.rootPEM == "" {
		return "", fmt.Errorf("Secondary AWS CA provider not fully Initialized")
	}
	return a.rootPEM, nil
}

func (a *AWSProvider) GenerateIntermediateCSR() (string, error) {
	if a.primaryDC {
		return "", fmt.Errorf("provider is the root certificate authority, " +
			"cannot generate an intermediate CSR")
	}

	err := a.ensureCA()
	if err != nil {
		return "", err
	}

	// We should have the CA created now and should be able to generate the CSR.
	return a.getCACSR()
}

func (a *AWSProvider) SetIntermediate(intermediatePEM string, rootPEM string) error {
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
	a.logger.Printf("[DEBUG] uploading certificate for %s", a.arn)
	_, err = a.client.ImportCertificateAuthorityCertificate(&input)
	if err != nil {
		return err
	}

	// We succsefully initialized, keep track of the root and intermediate certs.
	a.rootPEM = rootPEM
	a.intermediatePEM = intermediatePEM

	return nil
}

func (a *AWSProvider) ActiveIntermediate() (string, error) {
	err := a.ensureCA()
	if err != nil {
		return "", err
	}

	if a.rootPEM == "" {
		return "", fmt.Errorf("AWS CA provider not fully Initialized")
	}

	if a.primaryDC {
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

func (a *AWSProvider) GenerateIntermediate() (string, error) {
	// Like the consul provider, for now the PrimaryDC just gets a root and no
	// intermediate to sign with. so just return this. Secondaries use
	// intermediates but this method is only called during primary DC (root)
	// initialization in case a provider generates separate root and
	// intermediates.
	//
	// TODO(banks) support user-supplied CA being a Subordinate even in the
	// primary DC.
	return a.ActiveIntermediate()
}

func (a *AWSProvider) Sign(csr *x509.CertificateRequest) (string, error) {
	if a.rootPEM == "" {
		return "", fmt.Errorf("AWS CA provider not fully Initialized")
	}

	a.logger.Printf("[DEBUG] connect.ca.aws: signing csr for %s",
		csr.Subject.CommonName)

	return a.signCSRRaw(csr, LeafTemplateARN, a.config.LeafCertTTL)
}

func (a *AWSProvider) SignIntermediate(csr *x509.CertificateRequest) (string, error) {
	err := validateSignIntermediate(csr, &connect.SpiffeIDSigning{ClusterID: a.clusterID, Domain: "consul"})
	if err != nil {
		return "", err
	}

	// Sign it!
	return a.signCSRRaw(csr, IntermediateTemplateARN, IntermediateTTL)
}

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
	a.logger.Printf("[INFO] connect.ca.aws: disabling PCA %s", a.arn)
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
	a.logger.Printf("[INFO] connect.ca.aws: deleting PCA %s", a.arn)
	_, err := a.client.DeleteCertificateAuthority(&input)
	return err
}

func (a *AWSProvider) Cleanup() error {
	old := atomic.SwapUint32(&a.stopped, 1)
	if old == 0 {
		close(a.stopCh)
	}

	if a.config.DeleteOnExit {
		if err := a.disablePCA(); err != nil {
			// Log the error but continue trying to delete as some errors may still
			// allow that and this is best-effort delete anyway.
			a.logger.Printf("[ERR] connect.ca.aws: failed to disable PCA %s: %s",
				a.arn, err)
		}
		if err := a.deletePCA(); err != nil {
			// Log the error but continue trying to delete as some errors may still
			// allow that and this is best-effort delete anyway.
			a.logger.Printf("[ERR] connect.ca.aws: failed to delete PCA %s: %s",
				a.arn, err)
		}
		// Don't stall leader shutdown, non of the failures here are fatal.
		return nil
	}
	return nil
}

func (a *AWSProvider) SupportsCrossSigning() (bool, error) {
	return false, nil
}

func ParseAWSCAConfig(raw map[string]interface{}) (*structs.AWSCAProviderConfig, error) {
	config := structs.AWSCAProviderConfig{
		CommonCAProviderConfig: defaultCommonConfig(),
		PollInterval:           5 * time.Second,
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

func ValidateEnum(value string, choices ...string) (string, error) {
	for _, choice := range choices {
		if strings.ToLower(value) == strings.ToLower(choice) {
			return choice, nil
		}
	}

	return "", fmt.Errorf("must be one of %s or %s",
		strings.Join(choices[:len(choices)-1], ","),
		choices[len(choices)-1])
}
