// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acmpca"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/sdk/testutil"
)

// skipIfAWSNotConfigured skips the test unless ENABLE_AWS_PCA_TESTS=true.
//
// These tests are not run in CI.  If you are making changes to the AWS provider
// you probably want to run these tests locally. The tests will run using any
// credentials available to the AWS SDK. See
// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials
// for a list of options.
func skipIfAWSNotConfigured(t *testing.T) {
	enabled := os.Getenv("ENABLE_AWS_PCA_TESTS")
	ok, err := strconv.ParseBool(enabled)
	if err != nil || !ok {
		t.Skip("Skipping because AWS tests are not enabled")
	}
}

func TestAWSBootstrapAndSignPrimary(t *testing.T) {
	// Note not parallel since we could easily hit AWS limits of too many CAs if
	// all of these tests run at once.
	skipIfAWSNotConfigured(t)

	for _, tc := range KeyTestCases {
		tc := tc
		t.Run(tc.Desc, func(t *testing.T) {
			cfg := map[string]interface{}{
				"PrivateKeyType": tc.KeyType,
				"PrivateKeyBits": tc.KeyBits,
				"RootCertTTL":    "8761h",
			}
			provider := testAWSProvider(t, testProviderConfigPrimary(t, cfg))
			defer provider.Cleanup(true, nil)

			root, err := provider.GenerateCAChain()
			require.NoError(t, err)
			rootPEM := root.PEM

			// Ensure they use the right key type
			rootCert, err := connect.ParseCert(rootPEM)
			require.NoError(t, err)

			keyType, keyBits, err := connect.KeyInfoFromCert(rootCert)
			require.NoError(t, err)
			require.Equal(t, tc.KeyType, keyType)
			require.Equal(t, tc.KeyBits, keyBits)

			// Ensure that the root cert ttl is withing the configured value
			// computation is similar to how we are passing the TTL thru the aws client
			expectedTime := time.Now().AddDate(0, 0, int(8761*60*time.Minute/day)).UTC()
			require.WithinDuration(t, expectedTime, rootCert.NotAfter, 10*time.Minute, "expected parsed cert ttl to be the same as the value configured")

			// Sign a leaf with it
			testSignAndValidate(t, provider, rootPEM, nil)
		})
	}

	t.Run("Test default root ttl for aws ca provider", func(t *testing.T) {
		provider := testAWSProvider(t, testProviderConfigPrimary(t, nil))
		defer provider.Cleanup(true, nil)

		root, err := provider.GenerateCAChain()
		require.NoError(t, err)
		rootPEM := root.PEM

		// Ensure they use the right key type
		rootCert, err := connect.ParseCert(rootPEM)
		require.NoError(t, err)

		// Ensure that the root cert ttl is withing the configured value
		// computation is similar to how we are passing the TTL thru the aws client
		expectedTime := time.Now().AddDate(0, 0, int(87600*60*time.Minute/day)).UTC()
		require.WithinDuration(t, expectedTime, rootCert.NotAfter, 10*time.Minute, "expected parsed cert ttl to be the same as the value configured")
	})
}

func testSignAndValidate(t *testing.T, p Provider, rootPEM string, intermediatePEMs []string) {
	csrPEM, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "testsvc"))
	csr, err := connect.ParseCSR(csrPEM)
	require.NoError(t, err)

	leafPEM, err := p.Sign(csr)
	require.NoError(t, err)

	err = connect.ValidateLeaf(rootPEM, leafPEM, intermediatePEMs)
	require.NoError(t, err)
	requireTrailingNewline(t, leafPEM)
}

func TestAWSBootstrapAndSignSecondary(t *testing.T) {
	// Note not parallel since we could easily hit AWS limits of too many CAs if
	// all of these tests run at once.
	skipIfAWSNotConfigured(t)

	p1 := testAWSProvider(t, testProviderConfigPrimary(t, nil))
	defer p1.Cleanup(true, nil)
	root, err := p1.GenerateCAChain()
	require.NoError(t, err)
	rootPEM := root.PEM

	p2 := testAWSProvider(t, testProviderConfigSecondary(t, nil))
	defer p2.Cleanup(true, nil)

	testSignIntermediateCrossDC(t, p1, p2)

	// Fetch intermediate from s2 now for later comparison
	intPEM, err := p2.ActiveLeafSigningCert()
	require.NoError(t, err)

	// Capture the state of the providers we've setup
	p1State, err := p1.State()
	require.NoError(t, err)
	p2State, err := p2.State()
	require.NoError(t, err)

	// TEST LOAD FROM PREVIOUS STATE
	{
		// Now create new providers from the state of the first ones simulating
		// leadership change in both DCs
		t.Log("Restarting Providers with State")

		// Create new provider instances
		cfg1 := testProviderConfigPrimary(t, nil)
		cfg1.State = p1State
		p1 = testAWSProvider(t, cfg1)
		root, err := p1.GenerateCAChain()
		require.NoError(t, err)
		newRootPEM := root.PEM

		cfg2 := testProviderConfigPrimary(t, nil)
		cfg2.State = p2State
		p2 = testAWSProvider(t, cfg2)
		// Need call ActiveLeafSigningCert like leader would to trigger loading from PCA
		newIntPEM, err := p2.ActiveLeafSigningCert()
		require.NoError(t, err)

		// Root cert should not have changed
		require.Equal(t, rootPEM, newRootPEM)

		// Secondary intermediate cert should not have changed
		require.NoError(t, err)
		require.Equal(t, rootPEM, newRootPEM)
		require.Equal(t, intPEM, newIntPEM)

		// Should both be able to sign leafs again
		testSignAndValidate(t, p1, rootPEM, nil)
		testSignAndValidate(t, p2, rootPEM, []string{intPEM})
	}

	// Since we have CAs created, test the use-case where User supplied CAs are
	// used.
	{
		t.Log("Starting up Providers with ExistingARNs")

		// Create new provider instances with config
		cfg1 := testProviderConfigPrimary(t, map[string]interface{}{
			"ExistingARN": p1State[AWSStateCAARNKey],
		})
		p1 = testAWSProvider(t, cfg1)
		root, err := p1.GenerateCAChain()
		require.NoError(t, err)
		newRootPEM := root.PEM

		cfg2 := testProviderConfigPrimary(t, map[string]interface{}{
			"ExistingARN": p2State[AWSStateCAARNKey],
		})
		cfg1.RawConfig["ExistingARN"] = p2State[AWSStateCAARNKey]
		p2 = testAWSProvider(t, cfg2)
		// Need call ActiveLeafSigningCert like leader would to trigger loading from PCA
		newIntPEM, err := p2.ActiveLeafSigningCert()
		require.NoError(t, err)

		// Root cert should not have changed
		require.Equal(t, rootPEM, newRootPEM)

		// Secondary intermediate cert should not have changed
		require.NoError(t, err)
		require.Equal(t, rootPEM, newRootPEM)
		require.Equal(t, intPEM, newIntPEM)

		// Should both be able to sign leafs again
		testSignAndValidate(t, p1, rootPEM, nil)
		testSignAndValidate(t, p2, rootPEM, []string{intPEM})
	}

	// Test that SetIntermediate() gives back certs with trailing new lines
	{

		// "Set" root, intermediate certs without a trailing new line
		newIntPEM := strings.TrimSuffix(intPEM, "\n")
		newRootPEM := strings.TrimSuffix(rootPEM, "\n")

		cfg2 := testProviderConfigSecondary(t, map[string]interface{}{
			"ExistingARN": p2State[AWSStateCAARNKey],
		})
		p2 = testAWSProvider(t, cfg2)
		require.NoError(t, p2.SetIntermediate(newIntPEM, newRootPEM, ""))

		root, err = p1.GenerateCAChain()
		require.NoError(t, err)
		newRootPEM = root.PEM
		newIntPEM, err = p2.ActiveLeafSigningCert()
		require.NoError(t, err)

		require.Equal(t, rootPEM, newRootPEM)
		require.Equal(t, intPEM, newIntPEM)
	}
}

func TestAWSBootstrapAndSignSecondaryConsul(t *testing.T) {
	// Note not parallel since we could easily hit AWS limits of too many CAs if
	// all of these tests run at once.
	skipIfAWSNotConfigured(t)

	t.Run("pri=consul,sec=aws", func(t *testing.T) {
		conf := testConsulCAConfig()
		delegate := newMockDelegate(t, conf)
		p1 := TestConsulProvider(t, delegate)
		cfg := testProviderConfig(conf)
		require.NoError(t, p1.Configure(cfg))
		_, err := p1.GenerateCAChain()
		require.NoError(t, err)

		p2 := testAWSProvider(t, testProviderConfigSecondary(t, nil))
		defer p2.Cleanup(true, nil)

		testSignIntermediateCrossDC(t, p1, p2)
	})

	t.Run("pri=aws,sec=consul", func(t *testing.T) {
		p1 := testAWSProvider(t, testProviderConfigPrimary(t, nil))
		defer p1.Cleanup(true, nil)

		_, err := p1.GenerateCAChain()
		require.NoError(t, err)

		conf := testConsulCAConfig()
		delegate := newMockDelegate(t, conf)
		p2 := TestConsulProvider(t, delegate)
		cfg := testProviderConfig(conf)
		cfg.IsPrimary = false
		cfg.Datacenter = "dc2"
		require.NoError(t, p2.Configure(cfg))

		testSignIntermediateCrossDC(t, p1, p2)
	})
}

func TestAWSNoCrossSigning(t *testing.T) {
	skipIfAWSNotConfigured(t)

	p1 := testAWSProvider(t, testProviderConfigPrimary(t, nil))
	defer p1.Cleanup(true, nil)
	// Don't bother initializing a PCA as that is slow and unnecessary for this
	// test

	ok, err := p1.SupportsCrossSigning()
	require.NoError(t, err)
	require.False(t, ok)

	// Attempt to cross sign a CA should fail with sensible error
	ca := connect.TestCA(t, nil)

	caCert, err := connect.ParseCert(ca.RootCert)
	require.NoError(t, err)
	_, err = p1.CrossSignCA(caCert)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

func TestAWSProvider_Cleanup(t *testing.T) {
	// Note not parallel since we could easily hit AWS limits of too many CAs if
	// all of these tests run at once.
	skipIfAWSNotConfigured(t)

	describeCA := func(t *testing.T, provider *AWSProvider) (bool, error) {
		t.Helper()
		state, err := provider.State()
		require.NoError(t, err)

		// Load from the resource.
		input := &acmpca.DescribeCertificateAuthorityInput{
			CertificateAuthorityArn: aws.String(state[AWSStateCAARNKey]),
		}
		output, err := provider.client.DescribeCertificateAuthority(input)
		if err != nil {
			return false, err
		}
		require.NotNil(t, output)
		require.NotNil(t, output.CertificateAuthority)
		require.NotNil(t, output.CertificateAuthority.Status)
		return *output.CertificateAuthority.Status == acmpca.CertificateAuthorityStatusDeleted, nil
	}

	requirePCADeleted := func(t *testing.T, provider *AWSProvider) {
		t.Helper()
		deleted, err := describeCA(t, provider)
		require.True(t, err != nil || deleted, "The AWS PCA instance has not been deleted")
	}

	requirePCANotDeleted := func(t *testing.T, provider *AWSProvider) {
		t.Helper()
		deleted, err := describeCA(t, provider)
		require.NoError(t, err)
		require.False(t, deleted, "The AWS PCA instance should not have been deleted")
	}

	t.Run("provider-change", func(t *testing.T) {
		// create a provider with the default config which will create the CA
		p1Conf := testProviderConfigPrimary(t, nil)
		p1 := testAWSProvider(t, p1Conf)
		p1.GenerateCAChain()

		t.Cleanup(func() {
			// This is a fail safe just in case the Cleanup routine of the
			// second provider fails to delete the CA. In that case we want
			// to request that the main provider delete it during Cleanup.
			if deleted, err := describeCA(t, p1); err == nil && deleted {
				p1.Cleanup(false, p1Conf.RawConfig)
			} else {
				p1.Cleanup(true, nil)
			}
		})

		// just ensure that it got created
		requirePCANotDeleted(t, p1)

		state, err := p1.State()
		require.NoError(t, err)

		p2Conf := testProviderConfigPrimary(t, map[string]interface{}{
			"ExistingARN": state[AWSStateCAARNKey],
		})
		p2 := testAWSProvider(t, p2Conf)

		// provider change should trigger deletion of the CA
		require.NoError(t, p2.Cleanup(true, nil))

		requirePCADeleted(t, p1)
	})

	t.Run("arn-change", func(t *testing.T) {
		// create a provider with the default config which will create the CA
		p1Conf := testProviderConfigPrimary(t, nil)
		p1 := testAWSProvider(t, p1Conf)
		p1.GenerateCAChain()

		t.Cleanup(func() {
			// This is a fail safe just in case the Cleanup routine of the
			// second provider fails to delete the CA. In that case we want
			// to request that the main provider delete it during Cleanup.
			if deleted, err := describeCA(t, p1); err == nil || deleted {
				p1.Cleanup(false, p1Conf.RawConfig)
			} else {
				p1.Cleanup(true, nil)
			}
		})

		// just ensure that it got created
		requirePCANotDeleted(t, p1)

		state, err := p1.State()
		require.NoError(t, err)

		p2Conf := testProviderConfigPrimary(t, map[string]interface{}{
			"ExistingARN": state[AWSStateCAARNKey],
		})
		p2 := testAWSProvider(t, p2Conf)

		// changing the ARN should cause the other CA to be deleted
		p2ConfAltARN := testProviderConfigPrimary(t, map[string]interface{}{
			"ExistingARN": "doesnt-need-to-be-real",
		})
		require.NoError(t, p2.Cleanup(false, p2ConfAltARN.RawConfig))

		requirePCADeleted(t, p1)
	})

	t.Run("arn-not-changed", func(t *testing.T) {
		// create a provider with the default config which will create the CA
		p1Conf := testProviderConfigPrimary(t, nil)
		p1 := testAWSProvider(t, p1Conf)
		p1.GenerateCAChain()

		t.Cleanup(func() {
			// the p2 provider should not remove the CA but we need to ensure that
			// we do clean it up
			p1.Cleanup(true, nil)
		})

		// just ensure that it got created
		requirePCANotDeleted(t, p1)

		state, err := p1.State()
		require.NoError(t, err)

		p2Conf := testProviderConfigPrimary(t, map[string]interface{}{
			"ExistingARN": state[AWSStateCAARNKey],
		})
		p2 := testAWSProvider(t, p2Conf)

		// because the ARN isn't changing we don't want to remove the CA
		require.NoError(t, p2.Cleanup(false, p2Conf.RawConfig))

		requirePCANotDeleted(t, p1)
	})
}

func testAWSProvider(t *testing.T, cfg ProviderConfig) *AWSProvider {
	p := NewAWSProvider(testutil.Logger(t))
	require.NoError(t, p.Configure(cfg))
	return p
}

func testProviderConfigPrimary(t *testing.T, cfg map[string]interface{}) ProviderConfig {
	rawCfg := make(map[string]interface{})
	for k, v := range cfg {
		rawCfg[k] = v
	}
	rawCfg["DeleteOnExit"] = true
	return ProviderConfig{
		ClusterID:  connect.TestClusterID,
		Datacenter: "dc1",
		IsPrimary:  true,
		RawConfig:  rawCfg,
	}
}

func testProviderConfigSecondary(t *testing.T, cfg map[string]interface{}) ProviderConfig {
	c := testProviderConfigPrimary(t, cfg)
	c.IsPrimary = false
	c.Datacenter = "dc2"
	return c
}
