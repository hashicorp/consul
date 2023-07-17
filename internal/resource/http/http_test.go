package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestHandler(t *testing.T) {
	svc := svctest.RunResourceService(t, demo.RegisterTypes)

	r := resource.NewRegistry()
	demo.RegisterTypes(r)

	h := NewHandler(svc, r)

	t.Run("Write", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/demo/v2/artist/keith-urban", strings.NewReader(`
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

		h.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))
		require.Equal(t, "Keith Urban", result["data"].(map[string]any)["name"])

		readRsp, err := svc.Read(testutil.TestContext(t), &pbresource.ReadRequest{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: demo.TenancyDefault,
				Name:    "keith-urban",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, readRsp.Resource)

		var artist pbdemov2.Artist
		require.NoError(t, readRsp.Resource.Data.UnmarshalTo(&artist))
		require.Equal(t, "Keith Urban", artist.Name)
	})

	t.Run("Read", func(t *testing.T) {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/demo/v2/artist/keith-urban", nil)

		h.ServeHTTP(rsp, req)

		require.Equal(t, http.StatusOK, rsp.Result().StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(rsp.Body).Decode(&result))
		require.Equal(t, "Keith Urban", result["data"].(map[string]any)["name"])
	})
}
