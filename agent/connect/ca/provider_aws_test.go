// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acmpca"
	"github.com/aws/aws-sdk-go-v2/service/acmpca/types"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// skipIfAWSNotConfigured skips the test unless ENABLE_AWS_PCA_TESTS=true.
//
// These tests are not run in CI.  If you are making changes to the AWS provider
// you probably want to run these tests locally. The tests will run using any
// credentials available to the AWS SDK. See
// https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide
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
			provider := testAWSProvider(t, testProviderConfigPrimary(cfg))
			defer provider.Cleanup(true, nil)

			rootPEM, err := provider.GenerateCAChain()
			require.NoError(t, err)

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
		provider := testAWSProvider(t, testProviderConfigPrimary(nil))
		defer provider.Cleanup(true, nil)

		rootPEM, err := provider.GenerateCAChain()
		require.NoError(t, err)

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

	p1 := testAWSProvider(t, testProviderConfigPrimary(nil))
	defer p1.Cleanup(true, nil)
	rootPEM, err := p1.GenerateCAChain()
	require.NoError(t, err)

	p2 := testAWSProvider(t, testProviderConfigSecondary(nil))
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
		cfg1 := testProviderConfigPrimary(nil)
		cfg1.State = p1State
		p1 = testAWSProvider(t, cfg1)
		newRootPEM, err := p1.GenerateCAChain()
		require.NoError(t, err)

		cfg2 := testProviderConfigPrimary(nil)
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
		cfg1 := testProviderConfigPrimary(map[string]interface{}{
			"ExistingARN": p1State[AWSStateCAARNKey],
		})
		p1 = testAWSProvider(t, cfg1)
		newRootPEM, err := p1.GenerateCAChain()
		require.NoError(t, err)

		cfg2 := testProviderConfigPrimary(map[string]interface{}{
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

		cfg2 := testProviderConfigSecondary(map[string]interface{}{
			"ExistingARN": p2State[AWSStateCAARNKey],
		})
		p2 = testAWSProvider(t, cfg2)
		require.NoError(t, p2.SetIntermediate(newIntPEM, newRootPEM, ""))

		newRootPEM, err = p1.GenerateCAChain()
		require.NoError(t, err)
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

		p2 := testAWSProvider(t, testProviderConfigSecondary(nil))
		defer p2.Cleanup(true, nil)

		testSignIntermediateCrossDC(t, p1, p2)
	})

	t.Run("pri=aws,sec=consul", func(t *testing.T) {
		p1 := testAWSProvider(t, testProviderConfigPrimary(nil))
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

	p1 := testAWSProvider(t, testProviderConfigPrimary(nil))
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
		output, err := provider.client.DescribeCertificateAuthority(context.Background(), input)
		if err != nil {
			return false, err
		}
		require.NotNil(t, output)
		require.NotNil(t, output.CertificateAuthority)
		return output.CertificateAuthority.Status == types.CertificateAuthorityStatusDeleted, nil
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
		p1Conf := testProviderConfigPrimary(nil)
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

		p2Conf := testProviderConfigPrimary(map[string]interface{}{
			"ExistingARN": state[AWSStateCAARNKey],
		})
		p2 := testAWSProvider(t, p2Conf)

		// provider change should trigger deletion of the CA
		require.NoError(t, p2.Cleanup(true, nil))

		requirePCADeleted(t, p1)
	})

	t.Run("arn-change", func(t *testing.T) {
		// create a provider with the default config which will create the CA
		p1Conf := testProviderConfigPrimary(nil)
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

		p2Conf := testProviderConfigPrimary(map[string]interface{}{
			"ExistingARN": state[AWSStateCAARNKey],
		})
		p2 := testAWSProvider(t, p2Conf)

		// changing the ARN should cause the other CA to be deleted
		p2ConfAltARN := testProviderConfigPrimary(map[string]interface{}{
			"ExistingARN": "doesnt-need-to-be-real",
		})
		require.NoError(t, p2.Cleanup(false, p2ConfAltARN.RawConfig))

		requirePCADeleted(t, p1)
	})

	t.Run("arn-not-changed", func(t *testing.T) {
		// create a provider with the default config which will create the CA
		p1Conf := testProviderConfigPrimary(nil)
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

		p2Conf := testProviderConfigPrimary(map[string]interface{}{
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

func testProviderConfigPrimary(cfg map[string]interface{}) ProviderConfig {
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

func testProviderConfigSecondary(cfg map[string]interface{}) ProviderConfig {
	c := testProviderConfigPrimary(cfg)
	c.IsPrimary = false
	c.Datacenter = "dc2"
	return c
}

func TestNewAWSProvider(t *testing.T) {
	logger := testutil.Logger(t)
	provider := NewAWSProvider(logger)

	require.NotNil(t, provider)
	require.Equal(t, logger, provider.logger)
	require.Equal(t, uint32(0), provider.stopped)
}

func TestAWSProvider_State(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		expected map[string]string
	}{
		{
			name:     "no ARN returns nil",
			arn:      "",
			expected: nil,
		},
		{
			name: "ARN set returns state with ARN",
			arn:  "arn:aws:acm-pca:us-east-1:123456789012:certificate-authority/abc123",
			expected: map[string]string{
				AWSStateCAARNKey: "arn:aws:acm-pca:us-east-1:123456789012:certificate-authority/abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &AWSProvider{
				arn: tt.arn,
			}

			state, err := provider.State()
			require.NoError(t, err)
			require.Equal(t, tt.expected, state)
		})
	}
}

func TestAWSProvider_SupportsCrossSigning(t *testing.T) {
	provider := &AWSProvider{}

	supported, err := provider.SupportsCrossSigning()
	require.NoError(t, err)
	require.False(t, supported, "AWS PCA provider should not support cross-signing")
}

func TestAWSProvider_CrossSignCA(t *testing.T) {
	provider := &AWSProvider{}

	// Create a test certificate
	ca := connect.TestCA(t, nil)
	caCert, err := connect.ParseCert(ca.RootCert)
	require.NoError(t, err)

	// CrossSignCA should always return an error
	_, err = provider.CrossSignCA(caCert)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not implemented")
}

func TestAWSProvider_GenerateCAChain_NotPrimary(t *testing.T) {
	provider := &AWSProvider{
		isPrimary: false,
	}

	// Should return error when called on non-primary
	_, err := provider.GenerateCAChain()
	require.Error(t, err)
	require.Contains(t, err.Error(), "provider is not the root certificate authority")
}

func TestAWSProvider_GenerateIntermediateCSR_IsPrimary(t *testing.T) {
	provider := &AWSProvider{
		isPrimary: true,
	}

	// Should return error when called on primary
	_, _, err := provider.GenerateIntermediateCSR()
	require.Error(t, err)
	require.Contains(t, err.Error(), "provider is the root certificate authority")
}

func TestKeyTypeToAlgos(t *testing.T) {
	tests := []struct {
		name         string
		keyType      string
		keyBits      int
		wantKeyAlgo  string
		wantSignAlgo string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "RSA 2048",
			keyType:      "rsa",
			keyBits:      2048,
			wantKeyAlgo:  string(types.KeyAlgorithmRsa2048),
			wantSignAlgo: string(types.SigningAlgorithmSha256withrsa),
		},
		{
			name:         "RSA 4096",
			keyType:      "rsa",
			keyBits:      4096,
			wantKeyAlgo:  string(types.KeyAlgorithmRsa4096),
			wantSignAlgo: string(types.SigningAlgorithmSha256withrsa),
		},
		{
			name:         "EC 256",
			keyType:      "ec",
			keyBits:      256,
			wantKeyAlgo:  string(types.KeyAlgorithmEcPrime256v1),
			wantSignAlgo: string(types.SigningAlgorithmSha256withecdsa),
		},
		{
			name:        "RSA invalid key bits 1024",
			keyType:     "rsa",
			keyBits:     1024,
			wantErr:     true,
			errContains: "AWS PCA only supports RSA key lengths 2048 and 4096",
		},
		{
			name:        "RSA invalid key bits 8192",
			keyType:     "rsa",
			keyBits:     8192,
			wantErr:     true,
			errContains: "AWS PCA only supports RSA key lengths 2048 and 4096",
		},
		{
			name:        "EC invalid key bits 384",
			keyType:     "ec",
			keyBits:     384,
			wantErr:     true,
			errContains: "AWS PCA only supports P256 EC curve",
		},
		{
			name:        "EC invalid key bits 521",
			keyType:     "ec",
			keyBits:     521,
			wantErr:     true,
			errContains: "AWS PCA only supports P256 EC curve",
		},
		{
			name:        "unsupported key type ed25519",
			keyType:     "ed25519",
			keyBits:     256,
			wantErr:     true,
			errContains: "AWS PCA only supports P256 EC curve, or RSA 2048/4096",
		},
		{
			name:        "unsupported key type dsa",
			keyType:     "dsa",
			keyBits:     2048,
			wantErr:     true,
			errContains: "AWS PCA only supports P256 EC curve, or RSA 2048/4096",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyAlgo, signAlgo, err := keyTypeToAlgos(tt.keyType, tt.keyBits)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantKeyAlgo, keyAlgo)
			require.Equal(t, tt.wantSignAlgo, signAlgo)
		})
	}
}

func TestPollWait(t *testing.T) {
	tests := []struct {
		name         string
		attemptsMade int
		expected     time.Duration
	}{
		{
			name:         "first attempt",
			attemptsMade: 0,
			expected:     100 * time.Millisecond,
		},
		{
			name:         "second attempt",
			attemptsMade: 1,
			expected:     200 * time.Millisecond,
		},
		{
			name:         "third attempt",
			attemptsMade: 2,
			expected:     500 * time.Millisecond,
		},
		{
			name:         "fourth attempt",
			attemptsMade: 3,
			expected:     1 * time.Second,
		},
		{
			name:         "fifth attempt",
			attemptsMade: 4,
			expected:     3 * time.Second,
		},
		{
			name:         "sixth attempt",
			attemptsMade: 5,
			expected:     5 * time.Second,
		},
		{
			name:         "many attempts caps at max",
			attemptsMade: 10,
			expected:     5 * time.Second,
		},
		{
			name:         "excessive attempts still caps at max",
			attemptsMade: 100,
			expected:     5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pollWait(tt.attemptsMade)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAWSCAConfig(t *testing.T) {
	tests := []struct {
		name        string
		raw         map[string]interface{}
		wantErr     bool
		errContains string
		validate    func(t *testing.T, cfg *structs.AWSCAProviderConfig)
	}{
		{
			name: "valid minimal config uses defaults",
			raw:  map[string]interface{}{},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.NotNil(t, cfg)
				require.Equal(t, "ec", cfg.PrivateKeyType)
				require.Equal(t, 256, cfg.PrivateKeyBits)
				require.False(t, cfg.DeleteOnExit)
			},
		},
		{
			name: "valid config with ExistingARN",
			raw: map[string]interface{}{
				"ExistingARN": "arn:aws:acm-pca:us-east-1:123456789012:certificate-authority/abc123",
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, "arn:aws:acm-pca:us-east-1:123456789012:certificate-authority/abc123", cfg.ExistingARN)
			},
		},
		{
			name: "valid config with DeleteOnExit true",
			raw: map[string]interface{}{
				"DeleteOnExit": true,
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.True(t, cfg.DeleteOnExit)
			},
		},
		{
			name: "valid config with DeleteOnExit false",
			raw: map[string]interface{}{
				"DeleteOnExit": false,
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.False(t, cfg.DeleteOnExit)
			},
		},
		{
			name: "valid RSA 2048",
			raw: map[string]interface{}{
				"PrivateKeyType": "rsa",
				"PrivateKeyBits": 2048,
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, "rsa", cfg.PrivateKeyType)
				require.Equal(t, 2048, cfg.PrivateKeyBits)
			},
		},
		{
			name: "valid RSA 4096",
			raw: map[string]interface{}{
				"PrivateKeyType": "rsa",
				"PrivateKeyBits": 4096,
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, "rsa", cfg.PrivateKeyType)
				require.Equal(t, 4096, cfg.PrivateKeyBits)
			},
		},
		{
			name: "valid EC 256",
			raw: map[string]interface{}{
				"PrivateKeyType": "ec",
				"PrivateKeyBits": 256,
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, "ec", cfg.PrivateKeyType)
				require.Equal(t, 256, cfg.PrivateKeyBits)
			},
		},
		{
			name: "invalid RSA key bits",
			raw: map[string]interface{}{
				"PrivateKeyType": "rsa",
				"PrivateKeyBits": 1024,
			},
			wantErr:     true,
			errContains: "RSA key length must be 2048 or 4096 bits",
		},
		{
			name: "invalid EC key bits",
			raw: map[string]interface{}{
				"PrivateKeyType": "ec",
				"PrivateKeyBits": 384,
			},
			wantErr:     true,
			errContains: "AWS PCA only supports P256 EC curve",
		},
		{
			name: "unsupported key type",
			raw: map[string]interface{}{
				"PrivateKeyType": "ed25519",
				"PrivateKeyBits": 256,
			},
			wantErr:     true,
			errContains: "private key type must be either 'ec' or 'rsa'",
		},
		{
			name: "LeafCertTTL less than 24 hours",
			raw: map[string]interface{}{
				"LeafCertTTL": "12h",
			},
			wantErr:     true,
			errContains: "AWS PCA doesn't support certificates that are valid for less than 24 hours",
		},
		{
			name: "LeafCertTTL exactly 24 hours is valid",
			raw: map[string]interface{}{
				"LeafCertTTL": "24h",
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, 24*time.Hour, cfg.LeafCertTTL)
			},
		},
		{
			name: "LeafCertTTL more than 24 hours is valid",
			raw: map[string]interface{}{
				"LeafCertTTL": "72h",
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, 72*time.Hour, cfg.LeafCertTTL)
			},
		},
		{
			name: "RootCertTTL can be specified",
			raw: map[string]interface{}{
				"RootCertTTL": "87600h",
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, 87600*time.Hour, cfg.RootCertTTL)
			},
		},
		{
			name: "IntermediateCertTTL can be specified",
			raw: map[string]interface{}{
				"IntermediateCertTTL": "43800h",
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, 43800*time.Hour, cfg.IntermediateCertTTL)
			},
		},
		{
			name: "all valid options together",
			raw: map[string]interface{}{
				"ExistingARN":         "arn:aws:acm-pca:us-east-1:123456789012:certificate-authority/test",
				"DeleteOnExit":        true,
				"PrivateKeyType":      "rsa",
				"PrivateKeyBits":      4096,
				"LeafCertTTL":         "72h",
				"RootCertTTL":         "87600h",
				"IntermediateCertTTL": "43800h",
			},
			validate: func(t *testing.T, cfg *structs.AWSCAProviderConfig) {
				require.Equal(t, "arn:aws:acm-pca:us-east-1:123456789012:certificate-authority/test", cfg.ExistingARN)
				require.True(t, cfg.DeleteOnExit)
				require.Equal(t, "rsa", cfg.PrivateKeyType)
				require.Equal(t, 4096, cfg.PrivateKeyBits)
				require.Equal(t, 72*time.Hour, cfg.LeafCertTTL)
				require.Equal(t, 87600*time.Hour, cfg.RootCertTTL)
				require.Equal(t, 43800*time.Hour, cfg.IntermediateCertTTL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseAWSCAConfig(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}
