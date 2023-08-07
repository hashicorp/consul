package envoyextensions

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// TestWASMRemote Summary
// This test ensures that a WASM extension can be loaded from a remote file server then executed.
// It uses the same basic WASM extension as the TestWASMLocal test which adds the header
// "x-test:true" to the response.
// This test configures a static server and proxy, a client proxy, as well as an Nginx file server to
// serve the compiled wasm extension. The static proxy is configured to apply the wasm filter
// and pointed at the remote file server. When the filter is added with the remote wasm file configured
// envoy calls out to the nginx file server to download it.
func TestWASMRemote(t *testing.T) {
	t.Parallel()

	// build all the file paths we will need for the test
	cwd, err := os.Getwd()
	require.NoError(t, err, "could not get current working directory")
	hostWASMDir := fmt.Sprintf("%s/testdata/wasm_test_files", cwd)

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
		},
	})

	clientService, staticProxy := createTestServices(t, cluster)
	_, port := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.GetEnvoyListenerTCPFilters(t, adminPort)

	libassert.AssertContainerState(t, clientService, "running")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")

	// Check header not present
	c1 := cleanhttp.DefaultClient()
	res, err := c1.Get(fmt.Sprintf("http://localhost:%d", port))
	require.NoError(t, err)

	// check that header DOES NOT exist before wasm applied
	if value := res.Header.Get(http.CanonicalHeaderKey("x-test")); value != "" {
		t.Fatal("unexpected test header present before WASM applied")
	}

	// Create Nginx file server
	uri, nginxService, nginxProxy := createNginxFileServer(t, cluster,
		// conf file
		testcontainers.ContainerFile{
			HostFilePath:      fmt.Sprintf("%s/nginx.conf", hostWASMDir),
			ContainerFilePath: "/etc/nginx/conf.d/wasm.conf",
			FileMode:          777,
		},
		// extra files loaded after startup
		testcontainers.ContainerFile{
			HostFilePath:      fmt.Sprintf("%s/wasm_add_header.wasm", hostWASMDir),
			ContainerFilePath: "/usr/share/nginx/html/wasm_add_header.wasm",
			FileMode:          777,
		})

	defer nginxService.Terminate()
	defer nginxProxy.Terminate()

	// wire up the wasm filter
	node := cluster.Agents[0]
	client := node.GetClient()

	agentService, _, err := client.Agent().Service(libservice.StaticServerServiceName, nil)
	require.NoError(t, err)

	agentService.Connect = &api.AgentServiceConnect{
		SidecarService: &api.AgentServiceRegistration{
			Kind: api.ServiceKindConnectProxy,
			Proxy: &api.AgentServiceConnectProxyConfig{
				Upstreams: []api.Upstream{
					{
						DestinationName:  "nginx-fileserver",
						DestinationPeer:  "",
						LocalBindAddress: "0.0.0.0",
						LocalBindPort:    9595,
					},
				},
			},
		},
	}

	// Upsert the service registration to add the nginx file server as an
	// upstream so that the static server proxy can retrieve the wasm plugin.
	err = client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Kind:              agentService.Kind,
		ID:                agentService.ID,
		Name:              agentService.Service,
		Tags:              agentService.Tags,
		Port:              agentService.Port,
		Address:           agentService.Address,
		SocketPath:        agentService.SocketPath,
		TaggedAddresses:   agentService.TaggedAddresses,
		EnableTagOverride: agentService.EnableTagOverride,
		Meta:              agentService.Meta,
		Weights:           &agentService.Weights,
		Check:             nil,
		Checks:            nil,
		Proxy:             agentService.Proxy,
		Connect:           agentService.Connect,
		Namespace:         agentService.Namespace,
		Partition:         agentService.Partition,
		Locality:          agentService.Locality,
	})
	if err != nil {
		t.Fatal(err)
	}

	// wait until the nginx-fileserver is reachable from the static proxy
	t.Log("Attempting wait until nginx-fileserver-sidecar-proxy is available")
	bashScript := "for i in {1..10}; do echo Attempt $1: contacting nginx; if curl -I localhost:9595; then break; fi;" +
		"if [[ $i -ge 10 ]]; then echo Unable to connect to nginx; exit 1; fi; sleep 3; done; echo nginx available"
	_, err = staticProxy.Exec(context.Background(), []string{"/bin/bash", "-c", bashScript})
	require.NoError(t, err)

	consul := cluster.APIClient(0)
	defaults := api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "static-server",
		Protocol: "http",
		EnvoyExtensions: []api.EnvoyExtension{{
			Name: "builtin/wasm",
			Arguments: map[string]any{
				"Protocol":     "http",
				"ListenerType": "inbound",
				"PluginConfig": map[string]any{
					"VmConfig": map[string]any{
						"Code": map[string]any{
							"Remote": map[string]any{
								"HttpURI": map[string]any{
									"Service": map[string]any{
										"Name": "nginx-fileserver",
									},
									"URI": fmt.Sprintf("%s/wasm_add_header.wasm", uri),
								},
								"SHA256": sha256FromFile(t, fmt.Sprintf("%s/wasm_add_header.wasm", hostWASMDir)),
							},
						},
					},
				},
			},
		}},
	}

	_, _, err = consul.ConfigEntries().Set(&defaults, nil)
	require.NoError(t, err, "could not set config entries")

	// Check that header is present after wasm applied
	c2 := cleanhttp.DefaultClient()

	// The wasm plugin is not always applied on the first call. Retry and see if it is loaded.
	retryStrategy := func() *retry.Timer {
		return &retry.Timer{Timeout: 5 * time.Second, Wait: time.Second}
	}
	retry.RunWith(retryStrategy(), t, func(r *retry.R) {
		res2, err := c2.Get(fmt.Sprintf("http://localhost:%d", port))
		require.NoError(r, err)

		if value := res2.Header.Get(http.CanonicalHeaderKey("x-test")); value == "" {
			r.Fatal("test header missing after WASM applied")
		}
	})
}

// TestWASMLocal Summary
// This test ensures that a WASM extension with basic functionality is executed correctly.
// The extension takes an incoming request and adds the header "x-test:true"
func TestWASMLocal(t *testing.T) {
	t.Parallel()

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
		},
	})

	clientService, _ := createTestServices(t, cluster)
	_, port := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.GetEnvoyListenerTCPFilters(t, adminPort)

	libassert.AssertContainerState(t, clientService, "running")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")

	// Check header not present
	c1 := cleanhttp.DefaultClient()
	res, err := c1.Get(fmt.Sprintf("http://localhost:%d", port))
	require.NoError(t, err)

	// check that header DOES NOT exist before wasm applied
	if value := res.Header.Get(http.CanonicalHeaderKey("x-test")); value != "" {
		t.Fatal("unexpected test header present before WASM applied")
	}

	// wire up the wasm filter
	consul := cluster.APIClient(0)
	defaults := api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "static-server",
		Protocol: "http",
		EnvoyExtensions: []api.EnvoyExtension{{
			Name: "builtin/wasm",
			Arguments: map[string]any{
				"Protocol":     "http",
				"ListenerType": "inbound",
				"PluginConfig": map[string]any{
					"VmConfig": map[string]any{
						"Code": map[string]any{
							"Local": map[string]any{
								"Filename": "/wasm_add_header.wasm",
							},
						},
					},
				},
			},
		}},
	}

	_, _, err = consul.ConfigEntries().Set(&defaults, nil)
	require.NoError(t, err, "could not set config entries")

	// Check that header is present after wasm applied
	c2 := cleanhttp.DefaultClient()

	// The wasm plugin is not always applied on the first call. Retry and see if it is loaded.
	retryStrategy := func() *retry.Timer {
		return &retry.Timer{Timeout: 5 * time.Second, Wait: time.Second}
	}
	retry.RunWith(retryStrategy(), t, func(r *retry.R) {
		res2, err := c2.Get(fmt.Sprintf("http://localhost:%d", port))
		require.NoError(r, err)

		if value := res2.Header.Get(http.CanonicalHeaderKey("x-test")); value == "" {
			r.Fatal("test header missing after WASM applied")
		}
	})
}

func createTestServices(t *testing.T, cluster *libcluster.Cluster) (libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       libservice.StaticServerServiceName,
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	cwd, err := os.Getwd()
	require.NoError(t, err, "could not get current working directory")
	hostWASMDir := fmt.Sprintf("%s/testdata/wasm_test_files", cwd)

	wasmFile := testcontainers.ContainerFile{
		HostFilePath:      fmt.Sprintf("%s/wasm_add_header.wasm", hostWASMDir),
		ContainerFilePath: "/wasm_add_header.wasm",
		FileMode:          777,
	}

	customFn := chain(
		copyFilesToContainer([]testcontainers.ContainerFile{wasmFile}),
		chownFiles([]testcontainers.ContainerFile{wasmFile}, "envoy", true),
	)

	// Create a service and proxy instance
	_, staticProxy, err := libservice.CreateAndRegisterStaticServerAndSidecarWithCustomContainerConfig(node, serviceOpts, customFn)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)

	// Create a client proxy instance with the server as an upstream

	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)

	return clientConnectProxy, staticProxy
}

// createNginxFileServer creates an nginx container configured to serve a wasm file for download, as well as a sidecar
// registered for the service.
func createNginxFileServer(t *testing.T,
	cluster *libcluster.Cluster,
	conf testcontainers.ContainerFile,
	files ...testcontainers.ContainerFile) (string, libservice.Service, libservice.Service) {

	nginxName := "nginx-fileserver"
	nginxPort := 80
	svc := &libservice.ServiceOpts{
		Name:     nginxName,
		ID:       nginxName,
		HTTPPort: nginxPort,
		GRPCPort: 9999,
	}

	node := cluster.Agents[0]

	req := testcontainers.ContainerRequest{
		// nginx:stable
		Image:      "nginx@sha256:b07a5ab5292bd90c4271a55a44761899cc1b14814172cf7f186e3afb8bdbec28",
		Name:       nginxName,
		WaitingFor: wait.ForLog("").WithStartupTimeout(time.Second * 30),
		LifecycleHooks: []testcontainers.ContainerLifecycleHooks{
			{
				PostStarts: []testcontainers.ContainerHook{
					func(ctx context.Context, c testcontainers.Container) error {
						_, _, err := c.Exec(ctx, []string{"mkdir", "-p", "/www/downloads"})
						if err != nil {
							return err
						}

						for _, f := range files {
							fBytes, err := os.ReadFile(f.HostFilePath)
							if err != nil {
								return err
							}
							err = c.CopyToContainer(ctx, fBytes, f.ContainerFilePath, f.FileMode)
							if err != nil {
								return err
							}

							_, _, err = c.Exec(ctx, []string{"chmod", "+r", f.ContainerFilePath})
							if err != nil {
								return err
							}
						}

						return err
					},
				},
			},
		},
		Files: []testcontainers.ContainerFile{conf},
	}

	nginxService, nginxProxy, err := libservice.CreateAndRegisterCustomServiceAndSidecar(node, svc, req, nil)
	require.NoError(t, err, "could not create custom server and sidecar")

	_, port := nginxService.GetAddr()

	client := node.GetClient()
	libassert.CatalogServiceExists(t, client, nginxName, nil)
	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", nginxName), nil)

	return fmt.Sprintf("http://nginx-fileserver:%d", port), nginxService, nginxProxy
}

// chain takes multiple setup functions for testcontainers.ContainerRequest and chains them together into a single function
// of testcontainers.ContainerRequest to testcontainers.ContainerRequest.
func chain(fns ...func(testcontainers.ContainerRequest) testcontainers.ContainerRequest) func(testcontainers.ContainerRequest) testcontainers.ContainerRequest {
	return func(req testcontainers.ContainerRequest) testcontainers.ContainerRequest {
		for _, fn := range fns {
			req = fn(req)
		}

		return req
	}
}

// copyFilesToContainer is a convenience function to build custom testcontainers.ContainerRequest. It takes a list of files
// which need to be copied to the container. It returns a function which updates a given testcontainers.ContainerRequest
// to include the files which need to be copied to the container on startup.
func copyFilesToContainer(files []testcontainers.ContainerFile) func(testcontainers.ContainerRequest) testcontainers.ContainerRequest {
	return func(req testcontainers.ContainerRequest) testcontainers.ContainerRequest {
		req.Files = files
		return req
	}
}

// chownFiles is a convenience function to build custom testcontainers.ContainerRequest. It takes a list of files,
// a user to make the owner, and whether the command requires sudo. It then returns a function which updates
// a testcontainers.ContainerRequest with a lifecycle hook which will chown the files to the user after container startup.
func chownFiles(files []testcontainers.ContainerFile, user string, sudo bool) func(request testcontainers.ContainerRequest) testcontainers.ContainerRequest {
	return func(req testcontainers.ContainerRequest) testcontainers.ContainerRequest {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostStarts: []testcontainers.ContainerHook{
				func(ctx context.Context, c testcontainers.Container) error {
					cmd := []string{}
					if sudo {
						cmd = append(cmd, "sudo")
					}

					cmd = append(cmd, "chown", user)

					for _, f := range files {
						cmd = append(cmd, f.ContainerFilePath)
					}

					_, _, err := c.Exec(ctx, cmd)
					return err
				},
			},
		})

		return req
	}
}

// sha256FromFile reads in the file from filepath and computes a sha256 of its contents.
func sha256FromFile(t *testing.T, filepath string) string {
	f, err := os.Open(filepath)
	require.NoError(t, err, "could not open file for sha")
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	require.NoError(t, err, "could not copy file to sha")

	return fmt.Sprintf("%x", h.Sum(nil))
}

// safeDelete removes a given file if it exists.
func safeDelete(t *testing.T, filePath string) {
	t.Logf("cleaning up stale build file: %s", filePath)
	if _, err := os.Stat(filePath); err != nil {
		return
	}

	// build is out of date, wipe compiled wasm
	err := os.Remove(filePath)
	require.NoError(t, err, "could not remove file")
}
