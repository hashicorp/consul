package ca

import (
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
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
			require := require.New(t)
			cfg := map[string]interface{}{
				"PrivateKeyType": tc.KeyType,
				"PrivateKeyBits": tc.KeyBits,
			}
			provider := testAWSProvider(t, testProviderConfigPrimary(t, cfg))
			defer provider.Cleanup()

			// Generate the root
			require.NoError(provider.GenerateRoot())

			// Fetch Active Root
			rootPEM, err := provider.ActiveRoot()
			require.NoError(err)

			// Generate Intermediate (not actually needed for this provider for now
			// but this simulates the calls in Server.initializeRoot).
			interPEM, err := provider.GenerateIntermediate()
			require.NoError(err)

			// Should be the same for now
			require.Equal(rootPEM, interPEM)

			// Ensure they use the right key type
			rootCert, err := connect.ParseCert(rootPEM)
			require.NoError(err)

			keyType, keyBits, err := connect.KeyInfoFromCert(rootCert)
			require.NoError(err)
			require.Equal(tc.KeyType, keyType)
			require.Equal(tc.KeyBits, keyBits)

			// Sign a leaf with it
			testSignAndValidate(t, provider, rootPEM, nil)
		})
	}
}

func testSignAndValidate(t *testing.T, p Provider, rootPEM string, intermediatePEMs []string) {
	csrPEM, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "testsvc"))
	csr, err := connect.ParseCSR(csrPEM)
	require.NoError(t, err)

	leafPEM, err := p.Sign(csr)
	require.NoError(t, err)

	err = connect.ValidateLeaf(rootPEM, leafPEM, intermediatePEMs)
	require.NoError(t, err)
}

func TestAWSBootstrapAndSignSecondary(t *testing.T) {
	// Note not parallel since we could easily hit AWS limits of too many CAs if
	// all of these tests run at once.
	skipIfAWSNotConfigured(t)

	p1 := testAWSProvider(t, testProviderConfigPrimary(t, nil))
	defer p1.Cleanup()
	rootPEM, err := p1.ActiveRoot()
	require.NoError(t, err)

	p2 := testAWSProvider(t, testProviderConfigSecondary(t, nil))
	defer p2.Cleanup()

	testSignIntermediateCrossDC(t, p1, p2)

	// Fetch intermediate from s2 now for later comparison
	intPEM, err := p2.ActiveIntermediate()
	require.NoError(t, err)

	// Capture the state of the providers we've setup
	p1State, err := p1.State()
	require.NoError(t, err)
	p2State, err := p2.State()
	require.NoError(t, err)

	// TEST LOAD FROM PREVIOUS STATE
	{
		// Now create new providers fromthe state of the first ones simulating
		// leadership change in both DCs
		t.Log("Restarting Providers with State")

		// Create new provider instances
		cfg1 := testProviderConfigPrimary(t, nil)
		cfg1.State = p1State
		p1 = testAWSProvider(t, cfg1)
		newRootPEM, err := p1.ActiveRoot()
		require.NoError(t, err)

		cfg2 := testProviderConfigPrimary(t, nil)
		cfg2.State = p2State
		p2 = testAWSProvider(t, cfg2)
		// Need call ActiveIntermediate like leader would to trigger loading from PCA
		newIntPEM, err := p2.ActiveIntermediate()
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
		newRootPEM, err := p1.ActiveRoot()
		require.NoError(t, err)

		cfg2 := testProviderConfigPrimary(t, map[string]interface{}{
			"ExistingARN": p2State[AWSStateCAARNKey],
		})
		cfg1.RawConfig["ExistingARN"] = p2State[AWSStateCAARNKey]
		p2 = testAWSProvider(t, cfg2)
		// Need call ActiveIntermediate like leader would to trigger loading from PCA
		newIntPEM, err := p2.ActiveIntermediate()
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
		require.NoError(t, p1.GenerateRoot())

		p2 := testAWSProvider(t, testProviderConfigSecondary(t, nil))
		defer p2.Cleanup()

		testSignIntermediateCrossDC(t, p1, p2)
	})

	t.Run("pri=aws,sec=consul", func(t *testing.T) {
		p1 := testAWSProvider(t, testProviderConfigPrimary(t, nil))
		defer p1.Cleanup()
		require.NoError(t, p1.GenerateRoot())

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
	defer p1.Cleanup()
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

func testAWSProvider(t *testing.T, cfg ProviderConfig) *AWSProvider {
	p := &AWSProvider{}
	logger := testutil.Logger(t)
	p.SetLogger(logger)
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
