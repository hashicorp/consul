package handler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-openapi/spec"
)

var returnedSwagger = []byte(`{
  "swagger": "2.0",
  "info": {
   "title": "Kubernetes",
   "version": "v1.11.0"
  }}`)

func TestRegisterOpenAPIVersionedService(t *testing.T) {
	var s spec.Swagger
	err := s.UnmarshalJSON(returnedSwagger)
	if err != nil {
		t.Errorf("Unexpected error in unmarshalling SwaggerJSON: %v", err)
	}

	returnedJSON, err := json.MarshalIndent(s, " ", " ")
	if err != nil {
		t.Errorf("Unexpected error in preparing returnedJSON: %v", err)
	}
	returnedPb, err := toProtoBinary(returnedJSON)
	if err != nil {
		t.Errorf("Unexpected error in preparing returnedPb: %v", err)
	}

	mux := http.NewServeMux()
	_, err = RegisterOpenAPIVersionedService(&s, "/openapi/v2", mux)
	if err != nil {
		t.Errorf("Unexpected error in register OpenAPI versioned service: %v", err)
	}
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	tcs := []struct {
		acceptHeader string
		respStatus   int
		respBody     []byte
	}{
		{"", 200, returnedJSON},
		{"*/*", 200, returnedJSON},
		{"application/*", 200, returnedJSON},
		{"application/json", 200, returnedJSON},
		{"test/test", 406, []byte{}},
		{"application/test", 406, []byte{}},
		{"application/test, */*", 200, returnedJSON},
		{"application/test, application/json", 200, returnedJSON},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf", 200, returnedPb},
		{"application/json, application/com.github.proto-openapi.spec.v2@v1.0+protobuf", 200, returnedJSON},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf, application/json", 200, returnedPb},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf; q=0.5, application/json", 200, returnedJSON},
	}

	for _, tc := range tcs {
		req, err := http.NewRequest("GET", server.URL+"/openapi/v2", nil)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in creating new request: %v", tc.acceptHeader, err)
		}

		req.Header.Add("Accept", tc.acceptHeader)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in serving HTTP request: %v", tc.acceptHeader, err)
		}

		if resp.StatusCode != tc.respStatus {
			t.Errorf("Accept: %v: Unexpected response status code, want: %v, got: %v", tc.acceptHeader, tc.respStatus, resp.StatusCode)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in reading response body: %v", tc.acceptHeader, err)
		}
		if !reflect.DeepEqual(body, tc.respBody) {
			t.Errorf("Accept: %v: Response body mismatches, want: %v, got: %v", tc.acceptHeader, tc.respBody, body)
		}
	}
}
