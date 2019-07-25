package plugin

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProvider_Configure(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Basic configure
		m.On("Configure", "foo", false, map[string]interface{}{
			"string": "bar",
			"number": float64(42), // because json
		}).Once().Return(nil)
		require.NoError(p.Configure("foo", false, map[string]interface{}{
			"string": "bar",
			"number": float64(42),
		}))
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("Configure", "foo", false, map[string]interface{}{}).Once().Return(errors.New("hello world"))
		err := p.Configure("foo", false, map[string]interface{}{})
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_GenerateRoot(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Try cleanup with no error
		m.On("GenerateRoot").Once().Return(nil)
		require.NoError(p.GenerateRoot())
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("GenerateRoot").Once().Return(errors.New("hello world"))
		err := p.GenerateRoot()
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_ActiveRoot(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Try cleanup with no error
		m.On("ActiveRoot").Once().Return("foo", nil)
		actual, err := p.ActiveRoot()
		require.NoError(err)
		require.Equal(actual, "foo")
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("ActiveRoot").Once().Return("", errors.New("hello world"))
		actual, err = p.ActiveRoot()
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_GenerateIntermediateCSR(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Try cleanup with no error
		m.On("GenerateIntermediateCSR").Once().Return("foo", nil)
		actual, err := p.GenerateIntermediateCSR()
		require.NoError(err)
		require.Equal(actual, "foo")
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("GenerateIntermediateCSR").Once().Return("", errors.New("hello world"))
		actual, err = p.GenerateIntermediateCSR()
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_SetIntermediate(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Try cleanup with no error
		m.On("SetIntermediate", "foo", "bar").Once().Return(nil)
		err := p.SetIntermediate("foo", "bar")
		require.NoError(err)
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("SetIntermediate", "foo", "bar").Once().Return(errors.New("hello world"))
		err = p.SetIntermediate("foo", "bar")
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_ActiveIntermediate(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Try cleanup with no error
		m.On("ActiveIntermediate").Once().Return("foo", nil)
		actual, err := p.ActiveIntermediate()
		require.NoError(err)
		require.Equal(actual, "foo")
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("ActiveIntermediate").Once().Return("", errors.New("hello world"))
		actual, err = p.ActiveIntermediate()
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_GenerateIntermediate(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Try cleanup with no error
		m.On("GenerateIntermediate").Once().Return("foo", nil)
		actual, err := p.GenerateIntermediate()
		require.NoError(err)
		require.Equal(actual, "foo")
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("GenerateIntermediate").Once().Return("", errors.New("hello world"))
		actual, err = p.GenerateIntermediate()
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_Sign(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Create a CSR
		csrPEM, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))
		block, _ := pem.Decode([]byte(csrPEM))
		csr, err := x509.ParseCertificateRequest(block.Bytes)
		require.NoError(err)
		require.NoError(csr.CheckSignature())

		// No error
		m.On("Sign", mock.Anything).Once().Return("foo", nil).Run(func(args mock.Arguments) {
			csr := args.Get(0).(*x509.CertificateRequest)
			require.NoError(csr.CheckSignature())
		})
		actual, err := p.Sign(csr)
		require.NoError(err)
		require.Equal(actual, "foo")
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("Sign", mock.Anything).Once().Return("", errors.New("hello world"))
		actual, err = p.Sign(csr)
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_SignIntermediate(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Create a CSR
		csrPEM, _ := connect.TestCSR(t, connect.TestSpiffeIDService(t, "web"))
		block, _ := pem.Decode([]byte(csrPEM))
		csr, err := x509.ParseCertificateRequest(block.Bytes)
		require.NoError(err)
		require.NoError(csr.CheckSignature())

		// No error
		m.On("SignIntermediate", mock.Anything).Once().Return("foo", nil).Run(func(args mock.Arguments) {
			csr := args.Get(0).(*x509.CertificateRequest)
			require.NoError(csr.CheckSignature())
		})
		actual, err := p.SignIntermediate(csr)
		require.NoError(err)
		require.Equal(actual, "foo")
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("SignIntermediate", mock.Anything).Once().Return("", errors.New("hello world"))
		actual, err = p.SignIntermediate(csr)
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_CrossSignCA(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Create a CSR
		root := connect.TestCA(t, nil)
		block, _ := pem.Decode([]byte(root.RootCert))
		crt, err := x509.ParseCertificate(block.Bytes)
		require.NoError(err)

		// No error
		m.On("CrossSignCA", mock.Anything).Once().Return("foo", nil).Run(func(args mock.Arguments) {
			actual := args.Get(0).(*x509.Certificate)
			require.True(crt.Equal(actual))
		})
		actual, err := p.CrossSignCA(crt)
		require.NoError(err)
		require.Equal(actual, "foo")
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("CrossSignCA", mock.Anything).Once().Return("", errors.New("hello world"))
		actual, err = p.CrossSignCA(crt)
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

func TestProvider_Cleanup(t *testing.T) {
	testPlugin(t, func(t *testing.T, m *ca.MockProvider, p ca.Provider) {
		require := require.New(t)

		// Try cleanup with no error
		m.On("Cleanup").Once().Return(nil)
		require.NoError(p.Cleanup())
		m.AssertExpectations(t)

		// Try with an error
		m.Mock = mock.Mock{}
		m.On("Cleanup").Once().Return(errors.New("hello world"))
		err := p.Cleanup()
		require.Error(err)
		require.Contains(err.Error(), "hello")
		m.AssertExpectations(t)
	})
}

// testPlugin runs the given test function callback for all supported
// transports of the plugin RPC layer.
func testPlugin(t *testing.T, f func(t *testing.T, m *ca.MockProvider, actual ca.Provider)) {
	t.Run("net/rpc", func(t *testing.T) {
		// Create a mock provider
		mockP := new(ca.MockProvider)
		client, _ := plugin.TestPluginRPCConn(t, map[string]plugin.Plugin{
			Name: &ProviderPlugin{Impl: mockP},
		}, nil)
		defer client.Close()

		// Request the provider
		raw, err := client.Dispense(Name)
		require.NoError(t, err)
		provider := raw.(ca.Provider)

		// Call the test function
		f(t, mockP, provider)
	})

	t.Run("gRPC", func(t *testing.T) {
		// Create a mock provider
		mockP := new(ca.MockProvider)
		client, _ := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
			Name: &ProviderPlugin{Impl: mockP},
		})
		defer client.Close()

		// Request the provider
		raw, err := client.Dispense(Name)
		require.NoError(t, err)
		provider := raw.(ca.Provider)

		// Call the test function
		f(t, mockP, provider)
	})
}
