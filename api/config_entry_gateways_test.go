package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntries_IngressGateway(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	config_entries := c.ConfigEntries()

	ingress1 := &IngressGatewayConfigEntry{
		Kind: IngressGateway,
		Name: "foo",
		Meta: map[string]string{
			"foo": "bar",
			"gir": "zim",
		},
	}

	ingress2 := &IngressGatewayConfigEntry{
		Kind: IngressGateway,
		Name: "bar",
		TLS: GatewayTLSConfig{
			Enabled:       true,
			TLSMinVersion: "TLSv1_2",
		},
		Defaults: &IngressServiceConfig{
			MaxConnections:     uint32Pointer(2048),
			MaxPendingRequests: uint32Pointer(4096),
		},
	}

	global := &ProxyConfigEntry{
		Kind: ProxyDefaults,
		Name: ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}
	// set default protocol to http so that ingress gateways pass validation
	_, wm, err := config_entries.Set(global, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// set it
	_, wm, err = config_entries.Set(ingress1, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// also set the second one
	_, wm, err = config_entries.Set(ingress2, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// get it
	entry, qm, err := config_entries.Get(IngressGateway, "foo", nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)

	// verify it
	readIngress, ok := entry.(*IngressGatewayConfigEntry)
	require.True(t, ok)
	require.Equal(t, ingress1.Kind, readIngress.Kind)
	require.Equal(t, ingress1.Name, readIngress.Name)
	require.Equal(t, ingress1.Meta, readIngress.Meta)
	require.Equal(t, ingress1.Meta, readIngress.GetMeta())

	// update it
	ingress1.Listeners = []IngressListener{
		{
			Port:     2222,
			Protocol: "http",
			Services: []IngressService{
				{
					Name:  "asdf",
					Hosts: []string{"test.example.com"},
					RequestHeaders: &HTTPHeaderModifiers{
						Set: map[string]string{
							"x-foo": "bar",
						},
					},
					ResponseHeaders: &HTTPHeaderModifiers{
						Remove: []string{"x-foo"},
					},
					TLS: &GatewayServiceTLSConfig{
						SDS: &GatewayTLSSDSConfig{
							ClusterName:  "foo",
							CertResource: "bar",
						},
					},
					MaxConnections:        uint32Pointer(5120),
					MaxPendingRequests:    uint32Pointer(512),
					MaxConcurrentRequests: uint32Pointer(2048),
				},
			},
			TLS: &GatewayTLSConfig{
				SDS: &GatewayTLSSDSConfig{
					ClusterName:  "baz",
					CertResource: "qux",
				},
			},
		},
	}
	ingress1.TLS = GatewayTLSConfig{
		SDS: &GatewayTLSSDSConfig{
			ClusterName:  "qux",
			CertResource: "bug",
		},
	}

	// CAS fail
	written, _, err := config_entries.CAS(ingress1, 0, nil)
	require.NoError(t, err)
	require.False(t, written)

	// CAS success
	written, wm, err = config_entries.CAS(ingress1, readIngress.ModifyIndex, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)
	require.True(t, written)

	// update no cas
	ingress2.Listeners = []IngressListener{
		{
			Port:     3333,
			Protocol: "http",
			Services: []IngressService{
				{
					Name: "qwer",
				},
			},
		},
	}
	_, wm, err = config_entries.Set(ingress2, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// list them
	entries, qm, err := config_entries.List(IngressGateway, nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)
	require.Len(t, entries, 2)

	for _, entry = range entries {
		switch entry.GetName() {
		case "foo":
			// this also verifies that the update value was persisted and
			// the updated values are seen
			readIngress, ok = entry.(*IngressGatewayConfigEntry)
			require.True(t, ok)
			require.Equal(t, ingress1.Kind, readIngress.Kind)
			require.Equal(t, ingress1.Name, readIngress.Name)

			require.Len(t, readIngress.Listeners, 1)
			require.Len(t, readIngress.Listeners[0].Services, 1)
			// Set namespace and partition to blank so that CE and ent can utilize the same tests
			readIngress.Listeners[0].Services[0].Namespace = ""
			readIngress.Listeners[0].Services[0].Partition = ""

			require.Equal(t, ingress1.Listeners, readIngress.Listeners)
		case "bar":
			readIngress, ok = entry.(*IngressGatewayConfigEntry)
			require.True(t, ok)
			require.Equal(t, ingress2.Kind, readIngress.Kind)
			require.Equal(t, ingress2.Name, readIngress.Name)
			require.Equal(t, *ingress2.Defaults.MaxConnections, *readIngress.Defaults.MaxConnections)
			require.Equal(t, uint32(4096), *readIngress.Defaults.MaxPendingRequests)
			require.Equal(t, uint32(0), *readIngress.Defaults.MaxConcurrentRequests)
			require.Len(t, readIngress.Listeners, 1)
			require.Len(t, readIngress.Listeners[0].Services, 1)
			// Set namespace and partition to blank so that CE and ent can utilize the same tests
			readIngress.Listeners[0].Services[0].Namespace = ""
			readIngress.Listeners[0].Services[0].Partition = ""

			require.Equal(t, ingress2.Listeners, readIngress.Listeners)
		}
	}

	// delete it
	wm, err = config_entries.Delete(IngressGateway, "foo", nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// verify deletion
	_, _, err = config_entries.Get(IngressGateway, "foo", nil)
	require.Error(t, err)
}

func TestAPI_ConfigEntries_TerminatingGateway(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	configEntries := c.ConfigEntries()

	terminating1 := &TerminatingGatewayConfigEntry{
		Kind: TerminatingGateway,
		Name: "foo",
		Meta: map[string]string{
			"foo": "bar",
			"gir": "zim",
		},
	}

	terminating2 := &TerminatingGatewayConfigEntry{
		Kind: TerminatingGateway,
		Name: "bar",
	}

	// set it
	_, wm, err := configEntries.Set(terminating1, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// also set the second one
	_, wm, err = configEntries.Set(terminating2, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// get it
	entry, qm, err := configEntries.Get(TerminatingGateway, "foo", nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)

	// verify it
	readTerminating, ok := entry.(*TerminatingGatewayConfigEntry)
	require.True(t, ok)
	require.Equal(t, terminating1.Kind, readTerminating.Kind)
	require.Equal(t, terminating1.Name, readTerminating.Name)
	require.Equal(t, terminating1.Meta, readTerminating.Meta)
	require.Equal(t, terminating1.Meta, readTerminating.GetMeta())

	// update it
	terminating1.Services = []LinkedService{
		{
			Name:     "web",
			CAFile:   "/etc/web/ca.crt",
			CertFile: "/etc/web/client.crt",
			KeyFile:  "/etc/web/tls.key",
			SNI:      "mydomain",
		},
	}

	// CAS fail
	written, _, err := configEntries.CAS(terminating1, 0, nil)
	require.NoError(t, err)
	require.False(t, written)

	// CAS success
	written, wm, err = configEntries.CAS(terminating1, readTerminating.ModifyIndex, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)
	require.True(t, written)

	// re-setting should not yield an error
	_, wm, err = configEntries.Set(terminating1, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	terminating2.Services = []LinkedService{
		{
			Name:     "*",
			CAFile:   "/etc/certs/ca.crt",
			CertFile: "/etc/certs/client.crt",
			KeyFile:  "/etc/certs/tls.key",
			SNI:      "mydomain",
		},
	}
	_, wm, err = configEntries.Set(terminating2, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// list them
	entries, qm, err := configEntries.List(TerminatingGateway, nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)
	require.Len(t, entries, 2)

	for _, entry = range entries {
		switch entry.GetName() {
		case "foo":
			// this also verifies that the update value was persisted and
			// the updated values are seen
			readTerminating, ok = entry.(*TerminatingGatewayConfigEntry)
			require.True(t, ok)
			require.Equal(t, terminating1.Kind, readTerminating.Kind)
			require.Equal(t, terminating1.Name, readTerminating.Name)
			require.Len(t, readTerminating.Services, 1)
			// Set namespace to blank so that CE and ent can utilize the same tests
			readTerminating.Services[0].Namespace = ""

			require.Equal(t, terminating1.Services, readTerminating.Services)
		case "bar":
			readTerminating, ok = entry.(*TerminatingGatewayConfigEntry)
			require.True(t, ok)
			require.Equal(t, terminating2.Kind, readTerminating.Kind)
			require.Equal(t, terminating2.Name, readTerminating.Name)
			require.Len(t, readTerminating.Services, 1)
			// Set namespace to blank so that CE and ent can utilize the same tests
			readTerminating.Services[0].Namespace = ""

			require.Equal(t, terminating2.Services, readTerminating.Services)
		}
	}

	// delete it
	wm, err = configEntries.Delete(TerminatingGateway, "foo", nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// verify deletion
	_, _, err = configEntries.Get(TerminatingGateway, "foo", nil)
	require.Error(t, err)
}
