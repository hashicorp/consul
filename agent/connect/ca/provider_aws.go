package ca

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acmpca"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

type AWSProvider struct {
	config    *structs.AWSCAProviderConfig
	session   *session.Session
	client    *acmpca.ACMPCA
	isRoot    bool
	clusterId string
	logger    *log.Logger
	rootPCA   *AmazonPCA
	subPCA    *AmazonPCA
	sleepTime time.Duration
}

func (a *AWSProvider) SetLogger(l *log.Logger) {
	a.logger = l
}

func (a *AWSProvider) Printf(format string, v ...interface{}) {
	if a.logger != nil {
		a.logger.Printf(format, a)
	}
}

func (a *AWSProvider) Configure(clusterId string, isRoot bool, rawConfig map[string]interface{}) error {
	config, err := ParseAWSCAConfig(rawConfig)
	if err != nil {
		return err
	}

	sleepTime, err := time.ParseDuration(config.SleepTime)
	if err != nil {
		return fmt.Errorf("invalid sleep time specified: %s", err)
	}
	if sleepTime.Seconds() < 1 {
		return fmt.Errorf("invalid sleep time specified: must be at least 1s")
	}

	creds := credentials.NewEnvCredentials()
	if config.AccessKeyId != "" && config.SecretAccessKey != "" && config.Region != "" {
		creds = credentials.NewStaticCredentials(config.AccessKeyId, config.SecretAccessKey, "")
	}

	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String(config.Region),
		Credentials: creds,
	})
	if err != nil {
		return err
	}

	a.config = config
	a.session = awsSession
	a.isRoot = isRoot
	a.clusterId = clusterId
	a.client = acmpca.New(awsSession)
	a.sleepTime = sleepTime

	return nil
}

func (a *AWSProvider) loadFindOrCreate(arn string, pcaType string) (*AmazonPCA, error) {
	if arn != "" {
		return LoadAmazonPCA(a.client, a.config, arn, pcaType, a.clusterId,
			a.config.KeyAlgorithm, a.config.SigningAlgorithm, a.logger)
	} else {
		pca, err := FindAmazonPCA(a.client, a.config, pcaType, a.clusterId,
			a.config.KeyAlgorithm, a.config.SigningAlgorithm, a.logger)
		if err != nil {
			return nil, err
		}
		if pca == nil {
			pca, err = CreateAmazonPCA(a.client, a.config, pcaType, a.clusterId,
				a.config.KeyAlgorithm, a.config.SigningAlgorithm, a.logger)
			if err != nil {
				return nil, err
			}
		}
		return pca, nil
	}
}

func (a *AWSProvider) GenerateRoot() error {
	if !a.isRoot {
		return fmt.Errorf("provider is not the root certificate authority")
	}

	if a.rootPCA != nil {
		return nil // root PCA has already been created
	}

	rootPCA, err := a.loadFindOrCreate(a.config.RootARN, acmpca.CertificateAuthorityTypeRoot)
	if err != nil {
		return err
	}

	a.rootPCA = rootPCA
	return a.rootPCA.Generate(a.rootPCA)
}

func (a *AWSProvider) ensureIntermediate() error {
	if a.subPCA != nil {
		return nil
	}

	subPCA, err := a.loadFindOrCreate(a.config.IntermediateARN, acmpca.CertificateAuthorityTypeSubordinate)
	if err != nil {
		return err
	}

	a.subPCA = subPCA
	return a.subPCA.Generate(a.rootPCA)
}

func (a *AWSProvider) ActiveRoot() (string, error) {
	return a.rootPCA.Certificate(), nil
}

func (a *AWSProvider) GenerateIntermediateCSR() (string, error) {
	if a.isRoot {
		return "", fmt.Errorf("provider is the root certificate authority, " +
			"cannot generate an intermediate CSR")
	}

	if err := a.ensureIntermediate(); err != nil {
		return "", err
	}

	a.Printf("[INFO] requesting CSR for new intermediate CA cert")
	return a.subPCA.GetCSR()
}

func (a *AWSProvider) SetIntermediate(intermediatePEM string, rootPEM string) error {
	if err := a.ensureIntermediate(); err != nil {
		return err
	}
	return a.subPCA.SetCert(intermediatePEM, rootPEM)
}

func (a *AWSProvider) ActiveIntermediate() (string, error) {
	if err := a.ensureIntermediate(); err != nil {
		return "", err
	}
	return a.subPCA.Certificate(), nil
}

func (a *AWSProvider) GenerateIntermediate() (string, error) {
	if err := a.ensureIntermediate(); err != nil {
		return "", err
	}
	if err := a.subPCA.Generate(a.rootPCA); err != nil {
		return "", err
	}
	return a.subPCA.Certificate(), nil
}

func (a *AWSProvider) Sign(csr *x509.CertificateRequest) (string, error) {

	if err := a.ensureIntermediate(); err != nil {
		return "", err
	}

	var pemBuf bytes.Buffer
	if err := pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr.Raw}); err != nil {
		return "", fmt.Errorf("error encoding CSR into PEM format: %s", err)
	}

	leafPEM, err := a.subPCA.SignLeaf(pemBuf.String(), a.config.LeafCertTTL)

	return leafPEM, err
}

func (a *AWSProvider) SignIntermediate(csr *x509.CertificateRequest) (string, error) {

	spiffeID := connect.SpiffeIDSigning{ClusterID: a.clusterId, Domain: "consul"}

	if len(csr.URIs) < 1 {
		return "", fmt.Errorf("intermediate does not contain a trust domain SAN")
	}

	if csr.URIs[0].String() != spiffeID.URI().String() {
		return "", fmt.Errorf("attempt to sign intermediate from a different trust domain: "+
			"mine='%s' theirs='%s'", spiffeID.URI().String(), csr.URIs[0].String())
	}

	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr.Raw}); err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	return a.rootPCA.SignIntermediate(buf.String())
}

// I'm not sure this can actually be implemented. PCA cannot cross-sign a cert, it can only
// sign a CSR, and we cannot generate a CSR from another provider's certificate without its
// private key.
func (a *AWSProvider) CrossSignCA(newCA *x509.Certificate) (string, error) {
	return "", fmt.Errorf("not implemented in AWS PCA provider")
}

func (a *AWSProvider) Cleanup() error {
	if a.config.DeleteOnExit {
		if a.subPCA != nil {
			if err := a.subPCA.Disable(); err != nil {
				a.Printf("[WARN] error disabling subordinate PCA: %s", err.Error())
			}
			if err := a.subPCA.Delete(); err != nil {
				a.Printf("[WARN] error deleting subordinate PCA: %s", err.Error())
			}
			a.subPCA = nil
		}
		if a.rootPCA != nil {
			if err := a.rootPCA.Disable(); err != nil {
				a.Printf("[WARN] error disabling root PCA: %s", err.Error())
			}
			if err := a.rootPCA.Delete(); err != nil {
				a.Printf("[WARN] error deleting root PCA: %s", err.Error())
			}
			a.rootPCA = nil
		}
	}
	return nil
}

func (a *AWSProvider) SupportsCrossSigning() bool {
	return false
}

func (a *AWSProvider) MinLifetime() time.Duration {
	return 24 * time.Hour
}

func ParseAWSCAConfig(raw map[string]interface{}) (*structs.AWSCAProviderConfig, error) {
	config := structs.AWSCAProviderConfig{
		CommonCAProviderConfig: defaultCommonConfig(),
		SleepTime:              "5s",
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

	if config.AccessKeyId == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		return nil, fmt.Errorf("must provide the AWS access key ID")
	}

	if config.SecretAccessKey == "" && os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		return nil, fmt.Errorf("must provide the AWS secret access key")
	}

	if config.Region == "" && os.Getenv("AWS_REGION") == "" {
		return nil, fmt.Errorf("must provide the AWS region")
	}

	if config.RootARN != "" {
		if !IsValidARN(config.RootARN) {
			return nil, fmt.Errorf("root PCA ARN is not in correct format")
		}
	}

	if config.IntermediateARN != "" {
		if !IsValidARN(config.IntermediateARN) {
			return nil, fmt.Errorf("intermediate PCA ARN is not in correct format")
		}
	}

	if config.KeyAlgorithm == "" {
		config.KeyAlgorithm = acmpca.KeyAlgorithmEcPrime256v1
	} else {
		config.KeyAlgorithm, err = ValidateEnum(config.KeyAlgorithm,
			acmpca.KeyAlgorithmRsa2048, acmpca.KeyAlgorithmRsa4096,
			acmpca.KeyAlgorithmEcPrime256v1, acmpca.KeyAlgorithmEcSecp384r1)
		if err != nil {
			return nil, fmt.Errorf("invalid key algorithm specified: %s", err)
		}
	}

	if config.SigningAlgorithm == "" {
		config.SigningAlgorithm = acmpca.SigningAlgorithmSha256withecdsa
	} else {
		config.SigningAlgorithm, err = ValidateEnum(config.SigningAlgorithm,
			acmpca.SigningAlgorithmSha256withrsa, acmpca.SigningAlgorithmSha384withrsa,
			acmpca.SigningAlgorithmSha512withrsa, acmpca.SigningAlgorithmSha256withecdsa,
			acmpca.SigningAlgorithmSha384withecdsa, acmpca.SigningAlgorithmSha512withecdsa)
		if err != nil {
			return nil, fmt.Errorf("invalid signing algorithm specified: %s", err)
		}
	}

	if err := config.CommonCAProviderConfig.Validate(); err != nil {
		return nil, err
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
