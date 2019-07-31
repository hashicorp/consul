package ca

import (
	"crypto/x509"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/acmpca"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
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
	certPEM          string
	chainPEM         string
	client           *acmpca.ACMPCA
	logger           *log.Logger
}

func NewAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, arn string, pcaType string,
	keyAlgorithm string, signingAlgorithm string, logger *log.Logger) *AmazonPCA {
	sleepTime, err := time.ParseDuration(config.SleepTime)
	if err != nil {
		sleepTime = 5 * time.Second
	}
	return &AmazonPCA{arn: arn, pcaType: pcaType, client: client, logger: logger,
		keyAlgorithm: keyAlgorithm, signingAlgorithm: signingAlgorithm, sleepTime: sleepTime}
}

func LoadAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, arn string, pcaType string,
	clusterId string, keyAlgorithm string, signingAlgorithm string, logger *log.Logger) (*AmazonPCA, error) {
	pca := NewAmazonPCA(client, config, arn, pcaType, keyAlgorithm, signingAlgorithm, logger)
	output, err := pca.describe()
	if err != nil {
		return nil, err
	}

	warned := false
	if *output.CertificateAuthority.Status != acmpca.CertificateAuthorityStatusActive {
		return nil, fmt.Errorf("the specified PCA is not active: status is %s",
			*output.CertificateAuthority.Status)
	}
	if *output.CertificateAuthority.CertificateAuthorityConfiguration.Subject.CommonName !=
		GetCommonName(pcaType, clusterId) {
		logger.Printf("[WARN] name of specified PCA '%s' does not match expected '%s'",
			*output.CertificateAuthority.CertificateAuthorityConfiguration.Subject.CommonName,
			GetCommonName(pcaType, clusterId))
		warned = true
	}
	if *output.CertificateAuthority.CertificateAuthorityConfiguration.KeyAlgorithm != keyAlgorithm {
		logger.Printf("[WARN] specified PCA is using an unexpected key algorithm: expected=%s actual=%s",
			keyAlgorithm, *output.CertificateAuthority.CertificateAuthorityConfiguration.KeyAlgorithm)
		warned = true
	}
	if *output.CertificateAuthority.CertificateAuthorityConfiguration.SigningAlgorithm != signingAlgorithm {
		logger.Printf("[WARN] specified PCA is using an unexpected signing algorithm: expected=%s actual=%s",
			signingAlgorithm, *output.CertificateAuthority.CertificateAuthorityConfiguration.SigningAlgorithm)
		warned = true
	}

	if warned {
		logger.Print("[WARN] existing PCA failed some preflight checks, trying to continue anyway")
	} else {
		logger.Print("[WARN] existing PCA passed all preflight checks")
	}
	return pca, nil
}

func FindAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, pcaType string, clusterId string,
	keyAlgorithm string, signingAlgorithm string, logger *log.Logger) (*AmazonPCA, error) {
	var name string = GetCommonName(pcaType, clusterId)
	var nextToken *string
	for {
		input := acmpca.ListCertificateAuthoritiesInput{
			MaxResults: aws.Int64(100),
			NextToken:  nextToken,
		}
		logger.Print("[DEBUG] listing existing certificate authorities")
		output, err := client.ListCertificateAuthorities(&input)
		if err != nil {
			logger.Printf("[ERR] error searching certificate authorities: %s", err.Error())
			return nil, err
		}

		for _, ca := range output.CertificateAuthorities {
			if *ca.CertificateAuthorityConfiguration.Subject.CommonName == name &&
				*ca.CertificateAuthorityConfiguration.KeyAlgorithm == keyAlgorithm &&
				*ca.CertificateAuthorityConfiguration.SigningAlgorithm == signingAlgorithm &&
				*ca.Status == acmpca.CertificateAuthorityStatusActive {
				logger.Printf("[INFO] found an existing active CA %s", *ca.Arn)
				return NewAmazonPCA(client, config, *ca.Arn, pcaType, *ca.CertificateAuthorityConfiguration.KeyAlgorithm,
					*ca.CertificateAuthorityConfiguration.SigningAlgorithm, logger), nil
			}
		}

		nextToken = output.NextToken
		if nextToken == nil {
			break
		}
	}

	logger.Print("[WARN] no existing active CA found")
	return nil, nil // not found
}

func CreateAmazonPCA(client *acmpca.ACMPCA, config *structs.AWSCAProviderConfig, pcaType string, clusterId string,
	keyAlgorithm string, signingAlgorithm string, logger *log.Logger) (*AmazonPCA, error) {
	commonName := GetCommonName(pcaType, clusterId)
	createInput := acmpca.CreateCertificateAuthorityInput{
		CertificateAuthorityType: aws.String(pcaType),
		CertificateAuthorityConfiguration: &acmpca.CertificateAuthorityConfiguration{
			Subject: &acmpca.ASN1Subject{
				CommonName: aws.String(commonName),
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

	logger.Printf("[DEBUG] creating new PCA %s", commonName)
	createOutput, err := client.CreateCertificateAuthority(&createInput)
	if err != nil {
		return nil, err
	}

	// wait for PCA to be created
	newARN := *createOutput.CertificateAuthorityArn
	for {
		logger.Printf("[DEBUG] checking to see if PCA %s is ready", newARN)

		describeInput := acmpca.DescribeCertificateAuthorityInput{
			CertificateAuthorityArn: aws.String(newARN),
		}

		describeOutput, err := client.DescribeCertificateAuthority(&describeInput)
		if err != nil {
			logger.Printf("[ERR] error describing PCA: %s", err.Error())
			if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
				return nil, fmt.Errorf("error waiting for PCA to be created: %s", err)
			}
		}

		if *describeOutput.CertificateAuthority.Status == acmpca.CertificateAuthorityStatusPendingCertificate {
			logger.Printf("[DEBUG] new PCA %s is ready to accept a certificate", newARN)
			return NewAmazonPCA(client, config, newARN, pcaType, keyAlgorithm, signingAlgorithm, logger), nil
		}

		logger.Print("[DEBUG] sleeping until certificate is ready")
		time.Sleep(5 * time.Second) // TODO: get from provider config
	}
}

func (pca *AmazonPCA) describe() (*acmpca.DescribeCertificateAuthorityOutput, error) {
	input := &acmpca.DescribeCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}

	return pca.client.DescribeCertificateAuthority(input)
}

func (pca *AmazonPCA) GetCSR() (string, error) {
	input := &acmpca.GetCertificateAuthorityCsrInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}
	pca.logger.Printf("[DEBUG] retrieving CSR for %s", pca.arn)
	output, err := pca.client.GetCertificateAuthorityCsr(input)
	if err != nil {
		pca.logger.Printf("[ERR] error retrieving CSR: %s", err.Error())
		return "", err
	}

	return *output.Csr, nil
}

func (pca *AmazonPCA) SetCert(certPEM string, chainPEM string) error {
	chainBytes := []byte(chainPEM)
	if chainPEM == "" {
		chainBytes = nil
	}
	input := acmpca.ImportCertificateAuthorityCertificateInput{
		CertificateAuthorityArn: aws.String(pca.arn),
		Certificate:             []byte(certPEM),
		CertificateChain:        chainBytes,
	}

	pca.logger.Printf("[DEBUG] uploading certificate for %s", pca.arn)
	_, err := pca.client.ImportCertificateAuthorityCertificate(&input)
	if err != nil {
		pca.logger.Printf("[ERR] error importing certificates: %s", err.Error())
		return err
	}

	pca.certPEM = certPEM
	pca.chainPEM = chainPEM

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
	return err
}

func (pca *AmazonPCA) Sign(csrPEM string, templateARN string, validity time.Duration) (string, error) {
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

	csr, err := connect.ParseCSR(csrPEM)
	if err != nil {
		return "", fmt.Errorf("unable to parse CSR: %s", err)
	}

	pca.logger.Printf("[DEBUG] issuing certificate for %s with %s", csr.Subject.String(), pca.arn)
	issueOutput, err := pca.client.IssueCertificate(&issueInput)
	if err != nil {
		pca.logger.Printf("[ERR] error issuing certificate: %s", err.Error())
		return "", fmt.Errorf("error issuing certificate from PCA: %s", err)
	}

	// wait for certificate to be created
	for {
		pca.logger.Printf("[DEBUG] checking to see if certificate %s is ready", *issueOutput.CertificateArn)

		certInput := acmpca.GetCertificateInput{
			CertificateAuthorityArn: aws.String(pca.arn),
			CertificateArn:          issueOutput.CertificateArn,
		}
		certOutput, err := pca.client.GetCertificate(&certInput)
		if err != nil {
			if err.(awserr.Error).Code() != acmpca.ErrCodeRequestInProgressException {
				pca.logger.Printf("[ERR] error retrieving new certificate from %s: %s",
					*issueOutput.CertificateArn, err.Error())
				return "", fmt.Errorf("error retrieving certificate from PCA: %s", err)
			}
		}

		if certOutput.Certificate != nil {
			pca.logger.Printf("[DEBUG] certificate is ready, ARN is %s", *issueOutput.CertificateArn)

			newCert, err := connect.ParseCert(*certOutput.Certificate)
			if err == nil {
				pca.logger.Printf("[DEBUG] certificate created: commonName=%s subjectKey=%s authorityKey=%s",
					newCert.Subject.CommonName,
					connect.HexString(newCert.SubjectKeyId),
					connect.HexString(newCert.AuthorityKeyId))
			}

			return *certOutput.Certificate, nil
		}

		pca.logger.Printf("[DEBUG] sleeping for %s until certificate is ready", pca.sleepTime)
		time.Sleep(pca.sleepTime)
	}
}

func (pca *AmazonPCA) SignLeaf(csrPEM string, validity time.Duration) (string, error) {
	certPEM, err := pca.Sign(csrPEM, LeafTemplateARN, validity)
	return certPEM, err
}

func (pca *AmazonPCA) SignIntermediate(csrPEM string) (string, error) {
	certPEM, err := pca.Sign(csrPEM, IntermediateTemplateARN, IntermediateValidity)
	return certPEM, err
}

func (pca *AmazonPCA) SignRoot(csrPEM string) (string, error) {
	certPEM, err := pca.Sign(csrPEM, RootTemplateARN, RootValidity)
	return certPEM, err
}

func (pca *AmazonPCA) Disable() error {
	input := acmpca.UpdateCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
		Status:                  aws.String(acmpca.CertificateAuthorityStatusDisabled),
	}
	pca.logger.Printf("[INFO] disabling PCA %s", pca.arn)
	_, err := pca.client.UpdateCertificateAuthority(&input)

	return err
}

func (pca *AmazonPCA) Enable() error {
	input := acmpca.UpdateCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
		Status:                  aws.String(acmpca.CertificateAuthorityStatusActive),
	}
	pca.logger.Printf("[INFO] enabling PCA %s", pca.arn)
	_, err := pca.client.UpdateCertificateAuthority(&input)

	return err
}

func (pca *AmazonPCA) Delete() error {
	input := acmpca.DeleteCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}
	pca.logger.Printf("[INFO] deleting PCA %s", pca.arn)
	_, err := pca.client.DeleteCertificateAuthority(&input)

	return err
}

func (pca *AmazonPCA) Undelete() error {
	input := acmpca.RestoreCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(pca.arn),
	}
	pca.logger.Printf("[INFO] undeleting PCA %s", pca.arn)
	_, err := pca.client.RestoreCertificateAuthority(&input)

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
