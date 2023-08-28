// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	resourceSvc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	pbdemov1 "github.com/hashicorp/consul/proto/private/pbdemo/v1"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/sdk/testutil"
)

const testACLTokenArtistReadPolicy = "00000000-0000-0000-0000-000000000001"
const testACLTokenArtistWritePolicy = "00000000-0000-0000-0000-000000000002"
const testACLTokenArtistListPolicy = "00000000-0000-0000-0000-000000000003"
const fakeToken = "fake-token"

func parseToken(req *http.Request, token *string) {
	*token = req.Header.Get("x-consul-token")
}

func TestResourceHandler_InputValidation(t *testing.T) {
	type testCase struct {
		description          string
		request              *http.Request
		response             *httptest.ResponseRecorder
		expectedResponseCode int
	}
	client, _ := svctest.RunResourceService(t, demo.RegisterTypes)
	resourceHandler := resourceHandler{
		resource.Registration{
			Type:  demo.TypeV2Artist,
			Proto: &pbdemov2.Artist{},
		},
		client,
		func(req *http.Request, token *string) { return },
		hclog.NewNullLogger(),
	}

	testCases := []testCase{
		{
			description: "missing resource name",
			request: httptest.NewRequest("PUT", "/?partition=default&peer_name=local&namespace=default", strings.NewReader(`
				{
					"metadata": {
						"foo": "bar"
					},
					"data": {
						"name": "Keith Urban",
						"genre": "GENRE_COUNTRY"
					}
				}
			`)),
			response:             httptest.NewRecorder(),
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			description: "wrong schema",
			request: httptest.NewRequest("PUT", "/keith-urban?partition=default&peer_name=local&namespace=default", strings.NewReader(`
				{
					"metadata": {
						"foo": "bar"
					},
					"dada": {
						"name": "Keith Urban",
						"genre": "GENRE_COUNTRY"
					}
				}
			`)),
			response:             httptest.NewRecorder(),
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			description:          "no id",
			request:              httptest.NewRequest("DELETE", "/?partition=default&peer_name=local&namespace=default", strings.NewReader("")),
			response:             httptest.NewRecorder(),
			expectedResponseCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			resourceHandler.ServeHTTP(tc.response, tc.request)

			require.Equal(t, tc.expectedResponseCode, tc.response.Result().StatusCode)
		})
	}
}

func TestResourceWriteHandler(t *testing.T) {
	aclResolver := &resourceSvc.MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistReadPolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV1ReadPolicy, demo.ArtistV2ReadPolicy), nil)
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistWritePolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV1WritePolicy, demo.ArtistV2WritePolicy), nil)

	client, r := svctest.RunResourceServiceWithACL(t, aclResolver, demo.RegisterTypes)
	handler := NewHandler(client, r, parseToken, hclog.NewNullLogger())

	t.Run("should be blocked if the token is not authorized", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default", strings.NewReader(`
			{
				"metadata": {
					"foo": "bar"
				},
				"data": {
					"name": "Keith Urban",
					"genre": "GENRE_COUNTRY"
				}
			}
		`))

		req.Header.Add("x-consul-token", testACLTokenArtistReadPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusForbidden, rsp.Result().StatusCode)
	})

	t.Run("should write to the resource backend", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default", strings.NewReader(`
			{
				"metadata": {
					"foo": "bar"
				},
				"data": {
					"name": "Keith Urban",
					"genre": "GENRE_COUNTRY"
				}
			}
		`))

		req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))
		require.Equal(t, "Keith Urban", result["data"].(map[string]any)["name"])
		require.Equal(t, "keith-urban", result["id"].(map[string]any)["name"])

		readRsp, err := client.Read(testutil.TestContext(t), &pbresource.ReadRequest{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    "keith-urban",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, readRsp.Resource)

		var artist pbdemov2.Artist
		require.NoError(t, readRsp.Resource.Data.UnmarshalTo(&artist))
		require.Equal(t, "Keith Urban", artist.Name)
	})

	t.Run("should update the record with version parameter", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default&version=1", strings.NewReader(`
			{
				"metadata": {
					"foo": "bar"
				},
				"data": {
					"name": "Keith Urban Two",
					"genre": "GENRE_COUNTRY"
				}
			}
		`))

		req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)
		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))
		require.Equal(t, "Keith Urban Two", result["data"].(map[string]any)["name"])
		require.Equal(t, "keith-urban", result["id"].(map[string]any)["name"])
	})

	t.Run("should fail the update if the resource's version doesn't match the version of the existing resource", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default&version=1", strings.NewReader(`
			{
				"metadata": {
					"foo": "bar"
				},
				"data": {
					"name": "Keith Urban",
					"genre": "GENRE_COUNTRY"
				}
			}
		`))

		req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusConflict, rsp.Result().StatusCode)
	})

	t.Run("should write to the resource backend with owner", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/demo/v1/artist/keith-urban-v1?partition=default&peer_name=local&namespace=default", strings.NewReader(`
			{
				"metadata": {
					"foo": "bar"
				},
				"data": {
					"name": "Keith Urban V1",
					"genre": "GENRE_COUNTRY"
				},
				"owner": {
					"name": "keith-urban",
					"type": {
						"group": "demo",
						"group_version": "v2",
						"kind": "Artist"
					},
					"tenancy": {
						"partition": "default",
						"peer_name": "local",
						"namespace": "default"
					}
				}
			}
		`))

		req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))
		require.Equal(t, "Keith Urban V1", result["data"].(map[string]any)["name"])
		require.Equal(t, "keith-urban-v1", result["id"].(map[string]any)["name"])

		readRsp, err := client.Read(testutil.TestContext(t), &pbresource.ReadRequest{
			Id: &pbresource.ID{
				Type:    demo.TypeV1Artist,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    "keith-urban-v1",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, readRsp.Resource)
		require.Equal(t, "keith-urban", readRsp.Resource.Owner.Name)

		var artist pbdemov1.Artist
		require.NoError(t, readRsp.Resource.Data.UnmarshalTo(&artist))
		require.Equal(t, "Keith Urban V1", artist.Name)
	})
}

type ResourceUri struct {
	group        string
	version      string
	kind         string
	resourceName string
}

func createResource(t *testing.T, artistHandler http.Handler, resourceUri *ResourceUri) map[string]any {
	rsp := httptest.NewRecorder()

	if resourceUri == nil {
		resourceUri = &ResourceUri{group: "demo", version: "v2", kind: "artist", resourceName: "keith-urban"}
	}

	req := httptest.NewRequest("PUT", fmt.Sprintf("/%s/%s/%s/%s?partition=default&peer_name=local&namespace=default", resourceUri.group, resourceUri.version, resourceUri.kind, resourceUri.resourceName), strings.NewReader(`
		{
			"metadata": {
				"foo": "bar"
			},
			"data": {
				"name": "test",
				"genre": "GENRE_COUNTRY"
			}
		}
	`))

	req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

	artistHandler.ServeHTTP(rsp, req)
	require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))
	return result
}

func deleteResource(t *testing.T, artistHandler http.Handler, resourceUri *ResourceUri) {
	rsp := httptest.NewRecorder()

	if resourceUri == nil {
		resourceUri = &ResourceUri{group: "demo", version: "v2", kind: "artist", resourceName: "keith-urban"}
	}

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/%s/%s/%s/%s?partition=default&peer_name=local&namespace=default", resourceUri.group, resourceUri.version, resourceUri.kind, resourceUri.resourceName), strings.NewReader(""))

	req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

	artistHandler.ServeHTTP(rsp, req)
	require.Equal(t, http.StatusNoContent, rsp.Result().StatusCode)
}

func TestResourceReadHandler(t *testing.T) {
	aclResolver := &resourceSvc.MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistReadPolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV1ReadPolicy, demo.ArtistV2ReadPolicy), nil)
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistWritePolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV1WritePolicy, demo.ArtistV2WritePolicy), nil)
	aclResolver.On("ResolveTokenAndDefaultMeta", fakeToken, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, ""), nil)

	client, r := svctest.RunResourceServiceWithACL(t, aclResolver, demo.RegisterTypes)
	handler := NewHandler(client, r, parseToken, hclog.NewNullLogger())

	createdResource := createResource(t, handler, nil)

	t.Run("Read resource", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default&consistent", nil)

		req.Header.Add("x-consul-token", testACLTokenArtistReadPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))
		require.Equal(t, result, createdResource)
	})

	t.Run("should not be found if resource not exist", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist/keith-not-exist?partition=default&peer_name=local&namespace=default&consistent", nil)

		req.Header.Add("x-consul-token", testACLTokenArtistReadPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusNotFound, rsp.Result().StatusCode)
	})

	t.Run("should be blocked if the token is not authorized", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default&consistent", nil)

		req.Header.Add("x-consul-token", fakeToken)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusForbidden, rsp.Result().StatusCode)
	})
}

func TestResourceDeleteHandler(t *testing.T) {
	aclResolver := &resourceSvc.MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistReadPolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV2ReadPolicy), nil)
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistWritePolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV2WritePolicy), nil)

	client, r := svctest.RunResourceServiceWithACL(t, aclResolver, demo.RegisterTypes)
	handler := NewHandler(client, r, parseToken, hclog.NewNullLogger())

	t.Run("should surface PermissionDenied error from resource service", func(t *testing.T) {
		createResource(t, handler, nil)

		deleteRsp := httptest.NewRecorder()
		deletReq := httptest.NewRequest("DELETE", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default", strings.NewReader(""))

		deletReq.Header.Add("x-consul-token", testACLTokenArtistReadPolicy)

		handler.ServeHTTP(deleteRsp, deletReq)

		require.Equal(t, http.StatusForbidden, deleteRsp.Result().StatusCode)
	})

	t.Run("should delete a resource without version", func(t *testing.T) {
		createResource(t, handler, nil)

		deleteRsp := httptest.NewRecorder()
		deletReq := httptest.NewRequest("DELETE", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default", strings.NewReader(""))

		deletReq.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

		handler.ServeHTTP(deleteRsp, deletReq)

		require.Equal(t, http.StatusNoContent, deleteRsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(deleteRsp.Body).Decode(&result))
		require.Empty(t, result)

		_, err := client.Read(testutil.TestContext(t), &pbresource.ReadRequest{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    "keith-urban",
			},
		})
		require.ErrorContains(t, err, "resource not found")
	})

	t.Run("should delete a resource with version", func(t *testing.T) {
		createResource(t, handler, nil)

		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/demo/v2/artist/keith-urban?partition=default&peer_name=local&namespace=default&version=1", strings.NewReader(""))

		req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)
		req.Header.Add("x-consul-token", testACLTokenArtistListPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusNoContent, rsp.Result().StatusCode)

		_, err := client.Read(testutil.TestContext(t), &pbresource.ReadRequest{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    "keith-urban",
			},
		})
		require.ErrorContains(t, err, "resource not found")
	})
}

func TestResourceListHandler(t *testing.T) {
	aclResolver := &resourceSvc.MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistListPolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV2ListPolicy), nil)
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLTokenArtistWritePolicy, mock.Anything, mock.Anything).
		Return(svctest.AuthorizerFrom(t, demo.ArtistV2WritePolicy), nil)

	client, r := svctest.RunResourceServiceWithACL(t, aclResolver, demo.RegisterTypes)
	handler := NewHandler(client, r, parseToken, hclog.NewNullLogger())

	t.Run("should return MethodNotAllowed", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/demo/v2/artist?partition=default&peer_name=local&namespace=default", strings.NewReader(""))

		req.Header.Add("x-consul-token", testACLTokenArtistListPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusMethodNotAllowed, rsp.Result().StatusCode)
	})

	t.Run("should be blocked if the token is not authorized", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist?partition=default&peer_name=local&namespace=default", strings.NewReader(""))

		req.Header.Add("x-consul-token", testACLTokenArtistWritePolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusForbidden, rsp.Result().StatusCode)
	})

	t.Run("should return list of resources", func(t *testing.T) {
		resourceUri1 := &ResourceUri{group: "demo", version: "v2", kind: "artist", resourceName: "steve"}
		resource1 := createResource(t, handler, resourceUri1)
		resourceUri2 := &ResourceUri{group: "demo", version: "v2", kind: "artist", resourceName: "elvis"}
		resource2 := createResource(t, handler, resourceUri2)

		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist?partition=default&peer_name=local&namespace=default", strings.NewReader(""))

		req.Header.Add("x-consul-token", testACLTokenArtistListPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))

		resources, _ := result["resources"].([]any)
		require.Len(t, resources, 2)

		expected := []map[string]any{resource1, resource2}
		require.Contains(t, expected, resources[0])
		require.Contains(t, expected, resources[1])

		// clean up
		deleteResource(t, handler, resourceUri1)
		deleteResource(t, handler, resourceUri2)
	})

	t.Run("should return empty list when no resources are found", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist?partition=default&peer_name=local&namespace=default", strings.NewReader(""))

		req.Header.Add("x-consul-token", testACLTokenArtistListPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))

		resources, _ := result["resources"].([]any)
		require.Len(t, resources, 0)
	})

	t.Run("should return empty list when name prefix matches don't match", func(t *testing.T) {
		createResource(t, handler, nil)

		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist?partition=default&peer_name=local&namespace=default&name_prefix=noname", strings.NewReader(""))

		req.Header.Add("x-consul-token", testACLTokenArtistListPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))

		resources, _ := result["resources"].([]any)
		require.Len(t, resources, 0)

		// clean up
		deleteResource(t, handler, nil)
	})

	t.Run("should return list of resources matching name prefix", func(t *testing.T) {
		resourceUri1 := &ResourceUri{group: "demo", version: "v2", kind: "artist", resourceName: "steve"}
		resource1 := createResource(t, handler, resourceUri1)
		resourceUri2 := &ResourceUri{group: "demo", version: "v2", kind: "artist", resourceName: "elvis"}
		createResource(t, handler, resourceUri2)

		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist?partition=default&peer_name=local&namespace=default&name_prefix=steve", strings.NewReader(""))

		req.Header.Add("x-consul-token", testACLTokenArtistListPolicy)

		handler.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))

		resources, _ := result["resources"].([]any)
		require.Len(t, resources, 1)

		require.Equal(t, resource1, resources[0])

		// clean up
		deleteResource(t, handler, resourceUri1)
		deleteResource(t, handler, resourceUri2)
	})
}
