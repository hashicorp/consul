package ca

import (
	"crypto/x509"
	"time"

	//"crypto/rsa"
	//"crypto/x509"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acmpca"
	"github.com/hashicorp/consul/agent/connect"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/structs"

	"github.com/stretchr/testify/require"
)

var awsAccessKeyId string
var awsSecretAccessKey string
var awsRegion string
var awsClient *acmpca.ACMPCA

func init() {
	awsAccessKeyId = os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	awsRegion = os.Getenv("AWS_REGION")
	awsSession, _ := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKeyId, awsSecretAccessKey, ""),
	})
	awsClient = acmpca.New(awsSession)
}

func makeConfig() *structs.CAConfiguration {
	return &structs.CAConfiguration{
		ClusterID: "asdf",
		Provider:  "aws",
		Config: map[string]interface{}{
			"LeafCertTTL":      "72h",
			"Region":           awsRegion,
			"AccessKeyId":      awsAccessKeyId,
			"SecretAccessKey":  awsSecretAccessKey,
			"KeyAlgorithm":     acmpca.KeyAlgorithmEcPrime256v1,
			"SigningAlgorithm": acmpca.SigningAlgorithmSha256withecdsa,
		},
	}
}

func makeProvider(r *require.Assertions, config *structs.CAConfiguration) *AWSProvider {
	provider := &AWSProvider{}
	r.NoError(provider.Configure(config.ClusterID, true, config.Config))
	return provider
}

func TestAWSProvider_Configure(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.Equal(conf.Config["AccessKeyId"], provider.config.AccessKeyId)
	r.Equal(conf.Config["SecretAccessKey"], provider.config.SecretAccessKey)
	r.Equal(conf.Config["Region"], provider.config.Region)
}

func TestAWSProvider_ConfigureBadKeyAlgorithm(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	conf.Config["KeyAlgorithm"] = "foo"
	provider := &AWSProvider{}
	r.Error(provider.Configure(conf.ClusterID, true, conf.Config))
}

func TestAWSProvider_ConfigureBadSigningAlgorithm(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	conf.Config["SigningAlgorithm"] = "foo"
	provider := &AWSProvider{}
	r.Error(provider.Configure(conf.ClusterID, true, conf.Config))
}

func TestAWSProvider_ConfigureBadSleepTime(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	conf.Config["SleepTime"] = "-5s"
	provider := &AWSProvider{}
	r.Error(provider.Configure(conf.ClusterID, true, conf.Config))

	conf.Config["SleepTime"] = "5foo"
	provider = &AWSProvider{}
	r.Error(provider.Configure(conf.ClusterID, true, conf.Config))
}

func TestAWSProvider_ConfigureBadLeafTTL(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	conf.Config["LeafCertTTL"] = "-72h"
	provider := &AWSProvider{}
	r.Error(provider.Configure(conf.ClusterID, true, conf.Config))
}

func TestAWSProvider_GenerateRoot(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())
	r.NotEmpty(provider.rootPCA.arn)

	output, err := awsClient.DescribeCertificateAuthority(&acmpca.DescribeCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(provider.rootPCA.arn),
	})
	r.NoError(err)

	ca := output.CertificateAuthority
	caConf := ca.CertificateAuthorityConfiguration
	r.Equal(acmpca.CertificateAuthorityStatusActive, *ca.Status)
	r.Equal(acmpca.KeyAlgorithmEcPrime256v1, *caConf.KeyAlgorithm)
	r.Equal(acmpca.SigningAlgorithmSha256withecdsa, *caConf.SigningAlgorithm)
	r.Contains(*caConf.Subject.CommonName, conf.ClusterID)
}

func TestAWSProvider_GenerateRootNotRoot(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	provider.isRoot = false
	r.Error(provider.GenerateRoot())
}

func TestAWSProvider_Sign(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())

	cn := "foo"
	pk, _, err := connect.GeneratePrivateKeyWithConfig("rsa", 2048)
	r.NoError(err)

	serviceID := &connect.SpiffeIDService{
		Host:       "11111111-2222-3333-4444-555555555555.consul",
		Datacenter: "dc1",
		Namespace:  "default",
		Service:    "foo",
	}

	csrText, err := connect.CreateCSR(cn, serviceID, pk)
	r.NoError(err)

	csr, err := connect.ParseCSR(csrText)
	r.NoError(err)

	leafText, err := provider.Sign(csr)
	r.NoError(err)

	leaf, err := connect.ParseCert(leafText)
	r.NoError(err)

	r.Equal(csr.Subject.CommonName, leaf.Subject.CommonName)
	r.Equal(serviceID.URI().String(), leaf.URIs[0].String())
	r.True(leaf.NotBefore.Before(time.Now()))
	r.True(leaf.NotAfter.After(time.Now()))
}

func TestAWSProvider_GenerateIntermediateCSR(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())

	_, err := provider.GenerateIntermediateCSR()
	r.Error(err)

	provider.isRoot = false
	csrText, err := provider.GenerateIntermediateCSR()

	csr, err := connect.ParseCSR(csrText)
	r.NoError(err)

	r.Contains(csr.Subject.CommonName, conf.ClusterID)
	r.Equal(x509.ECDSA, csr.PublicKeyAlgorithm)
	r.Equal(x509.ECDSAWithSHA256, csr.SignatureAlgorithm)
}

func TestAWSProvider_ActiveRoot(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())

	rootText, err := provider.ActiveRoot()
	r.NoError(err)

	root, err := connect.ParseCert(rootText)
	r.NoError(err)

	r.Equal(x509.ECDSA, root.PublicKeyAlgorithm)
	r.Equal(x509.ECDSAWithSHA256, root.SignatureAlgorithm)
	r.True(root.NotBefore.Before(time.Now()))
	r.True(root.NotAfter.After(time.Now()))
	r.True(root.IsCA)
}

func TestAWSProvider_GenerateIntermediate(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())

	interText, err := provider.GenerateIntermediate()
	r.NoError(err)

	inter, err := connect.ParseCert(interText)
	r.NoError(err)

	r.Contains(inter.Subject.CommonName, conf.ClusterID)
	r.Equal(x509.ECDSA, inter.PublicKeyAlgorithm)
	r.Equal(x509.ECDSAWithSHA256, inter.SignatureAlgorithm)
	r.True(inter.NotBefore.Before(time.Now()))
	r.True(inter.NotAfter.After(time.Now()))
	r.True(inter.IsCA)
	r.True(inter.MaxPathLenZero)
}

func TestAWSProvider_ActiveIntermediate(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())

	interText, err := provider.ActiveIntermediate()
	r.NoError(err)

	inter, err := connect.ParseCert(interText)
	r.NoError(err)

	r.Contains(inter.Subject.CommonName, conf.ClusterID)
	r.Equal(x509.ECDSA, inter.PublicKeyAlgorithm)
	r.Equal(x509.ECDSAWithSHA256, inter.SignatureAlgorithm)
	r.True(inter.NotBefore.Before(time.Now()))
	r.True(inter.NotAfter.After(time.Now()))
	r.True(inter.IsCA)
	r.True(inter.MaxPathLenZero)
}

func TestAWSProvider_SignIntermediate(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	conf := makeConfig()
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())

	conf2 := testConsulCAConfig()
	delegate2 := newMockDelegate(t, conf2)
	provider2 := &ConsulProvider{Delegate: delegate2}
	r.NoError(provider2.Configure(conf2.ClusterID, false, conf2.Config))

	testSignIntermediateCrossDC(t, provider, provider2)
}

func TestAWSProvider_Cleanup(t *testing.T) {
	// THIS TEST CANNOT BE RUN IN PARALLEL.
	// It disables and deletes the PCA, which will cause other tests to fail if they are running
	// at the same time.

	r := require.New(t)
	conf := makeConfig()
	conf.Config["DeleteOnExit"] = true
	provider := makeProvider(r, conf)

	r.NoError(provider.GenerateRoot())
	_, err := provider.GenerateIntermediate()
	r.NoError(err)

	rootPCA := provider.rootPCA
	subPCA := provider.subPCA
	r.NoError(provider.Cleanup())

	output, err := awsClient.DescribeCertificateAuthority(&acmpca.DescribeCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(rootPCA.arn),
	})
	r.NoError(err)

	ca := output.CertificateAuthority
	r.Equal(acmpca.CertificateAuthorityStatusDeleted, *ca.Status)
	r.Equal((*AmazonPCA)(nil), provider.rootPCA)

	r.NoError(rootPCA.Undelete())
	r.NoError(rootPCA.Enable())

	output, err = awsClient.DescribeCertificateAuthority(&acmpca.DescribeCertificateAuthorityInput{
		CertificateAuthorityArn: aws.String(subPCA.arn),
	})
	r.NoError(err)

	ca = output.CertificateAuthority
	r.Equal(acmpca.CertificateAuthorityStatusDeleted, *ca.Status)
	r.Equal((*AmazonPCA)(nil), provider.subPCA)

	r.NoError(subPCA.Undelete())
	r.NoError(subPCA.Enable())
}
