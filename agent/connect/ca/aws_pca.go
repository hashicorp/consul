package ca

// Assumptions that need to be changed:
// 1. ECDSA is accepted everywhere. AWS only supports RSA.
// 2. CA's are able to cross-sign. Hosted CA's don't seem to allow this.
// 3. Certs don't require a Subject. AWS appears to require a CN.
// 4. Validity periods can be as short as 1h. AWS minimum is 1d.

//Current issues and problems:
// 1. Vault provider does not work if configured at bootstrap time. You have to boostrap with Consul and then switch to Vault.
//    Error: no handler for route 'consul-intermediate/sign/leaf-cert'
// 2. With any provider besides Consul, the leaf certs are constantly refreshed because the cache is storing the
//    subject key of the root cert, but Vault and AWS are signing with an intermediate cert.

import (
	"crypto/x509"
	"fmt"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/acmpca"

	"github.com/hashicorp/consul/agent/connect"
)

const (
	RootTemplateARN         = "arn:aws:acm-pca:::template/RootCACertificate/V1"
	IntermediateTemplateARN = "arn:aws:acm-pca:::template/SubordinateCACertificate_PathLen0/V1"
	LeafTemplateARN         = "arn:aws:acm-pca:::template/EndEntityCertificate/V1"
)

const (
	RootValidity         = 5 * 365 * 24 * time.Hour
	IntermediateValidity = 1 * 365 * 24 * time.Hour
)

type AmazonPCA struct {
	arn              string
	pcaType          string
	keyAlgorithm     string
	signingAlgorithm string
	sleepTime        time.Duration

	certPEM  string
	chainPEM string
	//signingKey  string
	client *acmpca.ACMPCA
	logger hclog.Logger
}

func createPCALogger(pcaType string) hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:  "pca-" + pcaType,
		Level: hclog.Trace,
	})
}

func NewAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, arn string, pcaType string,
	keyAlgorithm string, signingAlgorithm string) *AmazonPCA {
	sleepTime, err := time.ParseDuration(config.SleepTime)
	if err != nil {
		sleepTime = 5 * time.Second
	}
	return &AmazonPCA{arn: arn, pcaType: pcaType, client: client, logger: createPCALogger(pcaType),
		keyAlgorithm: keyAlgorithm, signingAlgorithm: signingAlgorithm, sleepTime: sleepTime}
}

func LoadAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, arn string, pcaType string,
	clusterId string, keyAlgorithm string, signingAlgorithm string) (*AmazonPCA, error) {
	logger := createPCALogger(pcaType)
	logger.Trace("entering LoadAmazonPCA", "arn", arn, "type", pcaType, "clusterId",
		clusterId, "keyAlgo", keyAlgorithm, "signAlgo", signingAlgorithm)

	pca := NewAmazonPCA(client, config, arn, pcaType, keyAlgorithm, signingAlgorithm)
	output, err := pca.describe()
	if err != nil {
		return nil, err
	}

	if *output.CertificateAuthority.CertificateAuthorityConfiguration.Subject.CommonName !=
		GetCommonName(pcaType, clusterId) {
		logger.Warn("name of specified PCA does not match expected, continuing anyway",
			"expected", GetCommonName(pcaType, clusterId),
			"actual", *output.CertificateAuthority.CertificateAuthorityConfiguration.Subject.CommonName)
	}
	if *output.CertificateAuthority.Status != acmpca.CertificateAuthorityStatusActive {
		return nil, fmt.Errorf("the specified PCA is not active: status is %s",
			*output.CertificateAuthority.Status)
	}
	if *output.CertificateAuthority.CertificateAuthorityConfiguration.KeyAlgorithm != keyAlgorithm {
		logger.Warn("specified PCA is using an unexpected key algorithm",
			"expected", keyAlgorithm,
			"actual", *output.CertificateAuthority.CertificateAuthorityConfiguration.KeyAlgorithm)
	}
	if *output.CertificateAuthority.CertificateAuthorityConfiguration.SigningAlgorithm != signingAlgorithm {
		logger.Warn("specified PCA is using an unexpected signing algorithm",
			"expected", signingAlgorithm,
			"actual", *output.CertificateAuthority.CertificateAuthorityConfiguration.SigningAlgorithm)
	}

	logger.Info("existing PCA passed all preflight checks")
	return pca, nil
}

func FindAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, pcaType string, clusterId string,
	keyAlgorithm string, signingAlgorithm string) (*AmazonPCA, error) {
	logger := createPCALogger(pcaType)
	logger.Trace("entering FindAmazonPCA", "type", pcaType, "clusterId", clusterId)

	var name string = GetCommonName(pcaType, clusterId)
	var nextToken *string
	for {
		input := acmpca.ListCertificateAuthoritiesInput{
			MaxResults: aws.Int64(100),
			NextToken:  nextToken,
		}
		logger.Debug("searching existing certificate authorities", "input", input)
		output, err := client.ListCertificateAuthorities(&input)
		if err != nil {
			logger.Error("error searching certificate authorities: " + err.Error())
			return nil, err
		}

		for _, ca := range output.CertificateAuthorities {
			if *ca.CertificateAuthorityConfiguration.Subject.CommonName == name &&
				*ca.CertificateAuthorityConfiguration.KeyAlgorithm == keyAlgorithm &&
				*ca.CertificateAuthorityConfiguration.SigningAlgorithm == signingAlgorithm &&
				*ca.Status == acmpca.CertificateAuthorityStatusActive {
				logger.Info("found an existing active CA", "arn", *ca.Arn)
				return NewAmazonPCA(client, config, *ca.Arn, pcaType, *ca.CertificateAuthorityConfiguration.KeyAlgorithm,
					*ca.CertificateAuthorityConfiguration.SigningAlgorithm), nil
			}
		}

		nextToken = output.NextToken
		if nextToken == nil {
			break
		}
	}

	logger.Warn("no existing active CA found")
	return nil, nil // not found
}

func CreateAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, pcaType string, clusterId string,
	keyAlgorithm string, signingAlgorithm string) (*AmazonPCA, error) {
	logger := createPCALogger(pcaType)
	logger.Trace("entering CreateAmazonPCA", "type", pcaType, "clusterId", clusterId,
		"keyAlgo", keyAlgorithm, "signAlgo", signingAlgorithm)

	createInput := acmpca.CreateCertificateAuthorityInput{
		CertificateAuthorityType: aws.String(pcaType),
		CertificateAuthorityConfiguration: &acmpca.CertificateAuthorityConfiguration{
			Subject: &acmpca.ASN1Subject{
				CommonName: aws.String(GetCommonName(pcaType, clusterId)),
			},
			KeyAlgorithm:     aws.String(keyAlgorithm),
			SigningAlgorithm: aws.String(signingAlgorithm),
		},
		RevocationConfiguration: &acmpca.RevocationConfiguration{
			CrlConfiguration: &acmpca.CrlConfiguration{
				Enabled: aws.Bool(false),
			},
		},
		Tags: []*acmpca.Tag{
			{Key: aws.String("ClusterID"), Value: aws.String(clusterId)},
		},
	}

	logger.Debug("creating new PCA", "input", createInput)
	createOutput, err := client.CreateCertificateAuthority(&createInput)
	if err != nil {
		return nil, err
	}

	// wait for PCA to be created
	newARN := *createOutput.CertificateAuthorityArn
	for {
		logger.Debug("checking to see if CA is ready", "arn", newARN)

		describeInput := acmpca.DescribeCertificateAuthorityInput{
			CertificateAuthorityArn: aws.String(newARN),
		}

		describeOutput, err := client.DescribeCertificateAuthority(&describeInput)
		if err != nil {
			logger.Error("error describing CA: " + err.Error())
			if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
				return nil, fmt.Errorf("error waiting for PCA to be created: %s", err)
			}
		}

		if *describeOutput.CertificateAuthority.Status == acmpca.CertificateAuthorityStatusPendingCertificate {
			logger.Debug("new CA is ready to accept a certificate", "arn", newARN)
			logger.Trace("leaving CreateAmazonPCA")
			return NewAmazonPCA(client, config, newARN, pcaType, keyAlgorithm, signingAlgorithm), nil
		}

		// TODO: get from provider config
		logger.Debug("sleeping until certificate is ready")
		time.Sleep(5 * time.Second)
	}
}

func (pca *AmazonPCA) describe() (*acmpca.DescribeCertificateAuthorityOutput, error) {
	input := &acmpca.DescribeCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}

	return pca.client.DescribeCertificateAuthority(input)
}

func (pca *AmazonPCA) GetCSR() (string, error) {
	pca.logger.Trace("entering GetCSR")
	input := &acmpca.GetCertificateAuthorityCsrInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}
	pca.logger.Debug("retrieving CSR", "input", input)
	output, err := pca.client.GetCertificateAuthorityCsr(input)
	if err != nil {
		pca.logger.Error("error retrieving CSR: " + err.Error())
		return "", err
	}

	pca.logger.Trace("leaving GetCSR", "csr", *output.Csr, "error", nil)
	return *output.Csr, nil
}

func (pca *AmazonPCA) SetCert(certPEM string, chainPEM string) error {
	pca.logger.Trace("entering SetCert")
	chainBytes := []byte(chainPEM)
	if chainPEM == "" {
		chainBytes = nil
	}
	input := acmpca.ImportCertificateAuthorityCertificateInput{
		CertificateAuthorityArn: aws.String(pca.arn),
		Certificate:             []byte(certPEM),
		CertificateChain:        chainBytes,
	}

	pca.logger.Debug("uploading certificate", "input", input)
	_, err := pca.client.ImportCertificateAuthorityCertificate(&input)
	if err != nil {
		pca.logger.Error("error importing certificates: " + err.Error())
		return err
	}

	pca.certPEM = certPEM
	pca.chainPEM = chainPEM

	pca.logger.Trace("leaving SetCert", "error", nil)
	return nil
}

func (pca *AmazonPCA) getCerts() error {
	if pca.certPEM == "" || pca.chainPEM == "" {
		input := &acmpca.GetCertificateAuthorityCertificateInput{
			CertificateAuthorityArn: aws.String(pca.arn),
		}

		output, err := pca.client.GetCertificateAuthorityCertificate(input)
		if err != nil {
			return err
		}

		pca.certPEM = *output.Certificate
		pca.chainPEM = *output.CertificateChain
	}

	return nil
}

func (pca *AmazonPCA) Certificate() string {
	if pca.certPEM == "" {
		_ = pca.getCerts()
	}
	return pca.certPEM
}

func (pca *AmazonPCA) CertificateChain() string {
	if pca.chainPEM == "" {
		_ = pca.getCerts()
	}
	return pca.certPEM
}

func (pca *AmazonPCA) Generate(signingPCA *AmazonPCA) error {
	pca.logger.Trace("entering Generate", "signingARN", signingPCA.arn)
	csrPEM, err := pca.GetCSR()
	if err != nil {
		return err
	}

	templateARN := GetTemplateARN(pca.pcaType)
	chainPEM := ""
	validity := RootValidity
	if pca.pcaType == acmpca.CertificateAuthorityTypeSubordinate {
		chainPEM = signingPCA.Certificate()
		validity = IntermediateValidity
	}

	newCertPEM, err := signingPCA.Sign(csrPEM, templateARN, validity)
	if err != nil {
		return err
	}

	err = pca.SetCert(newCertPEM, chainPEM)
	pca.logger.Trace("leaving SetCert", "error", err)
	return err
}

func (pca *AmazonPCA) Sign(csrPEM string, templateARN string, validity time.Duration) (string, error) {
	pca.logger.Trace("entering Sign")
	issueInput := acmpca.IssueCertificateInput{
		CertificateAuthorityArn: aws.String(pca.arn),
		Csr:                     []byte(csrPEM),
		SigningAlgorithm:        aws.String(pca.signingAlgorithm),
		TemplateArn:             aws.String(templateARN),
		Validity: &acmpca.Validity{
			Value: aws.Int64(int64(validity.Seconds() / 86400.0)),
			Type:  aws.String(acmpca.ValidityPeriodTypeDays),
		},
	}

	pca.logger.Debug("issuing certificate", "input", issueInput)
	issueOutput, err := pca.client.IssueCertificate(&issueInput)
	if err != nil {
		pca.logger.Error("error issuing certificate: " + err.Error())
		return "", fmt.Errorf("error issuing certificate from PCA: %s", err)
	}

	// wait for certificate to be created
	for {
		pca.logger.Debug("checking to see if certificate is ready", "arn", *issueOutput.CertificateArn)

		certInput := acmpca.GetCertificateInput{
			CertificateAuthorityArn: aws.String(pca.arn),
			CertificateArn:          issueOutput.CertificateArn,
		}
		certOutput, err := pca.client.GetCertificate(&certInput)
		if err != nil {
			if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
				pca.logger.Error("error retrieving new certificate: "+err.Error(), "arn", *issueOutput.CertificateArn)
				return "", fmt.Errorf("error retrieving certificate from PCA: %s", err)
			}
		}

		if certOutput.Certificate != nil {
			pca.logger.Debug("certificate is ready", "arn", *issueOutput.CertificateArn)

			newCert, err := connect.ParseCert(*certOutput.Certificate)
			if err == nil {
				pca.logger.Debug("certificate created",
					"commonName", newCert.Subject.CommonName,
					"subjectKey", connect.HexString(newCert.SubjectKeyId),
					"authorityKey", connect.HexString(newCert.AuthorityKeyId))
			}

			pca.logger.Trace("leaving Sign", "error", nil)
			return *certOutput.Certificate, nil
		}

		pca.logger.Debug("sleeping until certificate is ready", "duration", pca.sleepTime)
		time.Sleep(pca.sleepTime)
	}
}

func (pca *AmazonPCA) SignLeaf(csrPEM string, validity time.Duration) (string, error) {
	pca.logger.Trace("entering SignLeaf")
	certPEM, err := pca.Sign(csrPEM, LeafTemplateARN, validity)
	pca.logger.Trace("leaving SignLeaf", "error", err)
	return certPEM, err
}

func (pca *AmazonPCA) SignIntermediate(csrPEM string) (string, error) {
	pca.logger.Trace("entering SignIntermediate")
	certPEM, err := pca.Sign(csrPEM, IntermediateTemplateARN, IntermediateValidity)
	pca.logger.Trace("leaving SignIntermediate", "error", err)
	return certPEM, err
}

func (pca *AmazonPCA) SignRoot(csrPEM string) (string, error) {
	pca.logger.Trace("entering SignRoot")
	certPEM, err := pca.Sign(csrPEM, RootTemplateARN, RootValidity)
	pca.logger.Trace("leaving SignRoot", "error", err)
	return certPEM, err
}

func (pca *AmazonPCA) Disable() error {
	pca.logger.Trace("entering Disable")
	input := acmpca.UpdateCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
		Status:                  aws.String(acmpca.CertificateAuthorityStatusDisabled),
	}
	pca.logger.Info("disabling PCA", "arn", pca.arn)
	_, err := pca.client.UpdateCertificateAuthority(&input)

	pca.logger.Trace("leaving Disable")
	return err
}

func (pca *AmazonPCA) Enable() error {
	pca.logger.Trace("entering Enable")
	input := acmpca.UpdateCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
		Status:                  aws.String(acmpca.CertificateAuthorityStatusActive),
	}
	pca.logger.Info("enabling PCA", "arn", pca.arn)
	_, err := pca.client.UpdateCertificateAuthority(&input)

	pca.logger.Trace("leaving Enable")
	return err
}

func (pca *AmazonPCA) Delete() error {
	pca.logger.Trace("entering Delete")
	input := acmpca.DeleteCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}
	pca.logger.Info("deleting PCA", "arn", pca.arn)
	_, err := pca.client.DeleteCertificateAuthority(&input)

	pca.logger.Trace("leaving Delete")
	return err
}

func (pca *AmazonPCA) Undelete() error {
	pca.logger.Trace("entering Undelete")
	input := acmpca.RestoreCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}
	pca.logger.Info("undeleting PCA", "arn", pca.arn)
	_, err := pca.client.RestoreCertificateAuthority(&input)

	pca.logger.Trace("leaving Undelete")
	return err
}

// utility functions

func GetCommonName(pcaType string, clusterId string) string {
	return fmt.Sprintf("Consul %s %s", pcaType, clusterId)
}

func GetTemplateARN(pcaType string) string {
	if pcaType == acmpca.CertificateAuthorityTypeRoot {
		return RootTemplateARN
	} else {
		return IntermediateTemplateARN
	}
}

func IsValidARN(arn string) bool {
	const PcaArnRegex = "^arn:([\\w-]+):([\\w-]+):(\\w{2}-\\w+-\\d+):(\\d+):(?:([\\w-]+)[/:])?([[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12})$"

	matched, _ := regexp.MatchString(PcaArnRegex, arn)
	return matched
}

func ToSignatureAlgorithm(algo string) x509.SignatureAlgorithm {
	switch algo {
	case acmpca.SigningAlgorithmSha256withrsa:
		return x509.SHA256WithRSA
	case acmpca.SigningAlgorithmSha384withrsa:
		return x509.SHA384WithRSA
	case acmpca.SigningAlgorithmSha512withrsa:
		return x509.SHA512WithRSA
	case acmpca.SigningAlgorithmSha256withecdsa:
		return x509.ECDSAWithSHA256
	case acmpca.SigningAlgorithmSha384withecdsa:
		return x509.ECDSAWithSHA384
	case acmpca.SigningAlgorithmSha512withecdsa:
		return x509.ECDSAWithSHA512
	default:
		return x509.SHA256WithRSA
	}
}
