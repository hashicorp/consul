package plugin

import (
	"errors"
	"testing"

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
			pluginName: &ProviderPlugin{Impl: mockP},
		}, nil)
		defer client.Close()

		// Request the provider
		raw, err := client.Dispense(pluginName)
		require.NoError(t, err)
		provider := raw.(ca.Provider)

		// Call the test function
		f(t, mockP, provider)
	})

	t.Run("gRPC", func(t *testing.T) {
		// Create a mock provider
		mockP := new(ca.MockProvider)
		client, _ := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
			pluginName: &ProviderPlugin{Impl: mockP},
		})
		defer client.Close()

		// Request the provider
		raw, err := client.Dispense(pluginName)
		require.NoError(t, err)
		provider := raw.(ca.Provider)

		// Call the test function
		f(t, mockP, provider)
	})
}
