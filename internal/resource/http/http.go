// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func NewHandler(
	client pbresource.ResourceServiceClient,
	registry resource.Registry,
	parseToken func(req *http.Request, token *string),
	logger hclog.Logger) http.Handler {
	mux := http.NewServeMux()
	for _, t := range registry.Types() {
		// Individual Resource Endpoints.
		prefix := strings.ToLower(fmt.Sprintf("/%s/%s/%s/", t.Type.Group, t.Type.GroupVersion, t.Type.Kind))
		logger.Info("Registered resource endpoint", "endpoint", prefix)
		mux.Handle(prefix, http.StripPrefix(prefix, &resourceHandler{t, client, parseToken, logger}))
	}

	return mux
}

type writeRequest struct {
	Metadata map[string]string `json:"metadata"`
	Data     json.RawMessage   `json:"data"`
	Owner    *pbresource.ID    `json:"owner"`
}

type resourceHandler struct {
	reg        resource.Registration
	client     pbresource.ResourceServiceClient
	parseToken func(req *http.Request, token *string)
	logger     hclog.Logger
}

func (h *resourceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var token string
	h.parseToken(r, &token)
	ctx := metadata.AppendToOutgoingContext(r.Context(), "x-consul-token", token)
	switch r.Method {
	case http.MethodPut:
		h.handleWrite(w, r, ctx)
	case http.MethodGet:
		h.handleRead(w, r, ctx)
	case http.MethodDelete:
		h.handleDelete(w, r, ctx)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (h *resourceHandler) handleWrite(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var req writeRequest
	// convert req body to writeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Request body didn't follow schema."))
	}
	// convert data struct to proto message
	data := h.reg.Proto.ProtoReflect().New().Interface()
	if err := protojson.Unmarshal(req.Data, data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Request body didn't follow schema."))
	}
	// proto message to any
	anyProtoMsg, err := anypb.New(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Error("Failed to convert proto message to any type", "error", err)
		return
	}

	tenancyInfo, params := parseParams(r)

	rsp, err := h.client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    h.reg.Type,
				Tenancy: tenancyInfo,
				Name:    params["resourceName"],
			},
			Owner:    req.Owner,
			Version:  params["version"],
			Metadata: req.Metadata,
			Data:     anyProtoMsg,
		},
	})
	if err != nil {
		handleResponseError(err, w, h)
		return
	}

	output, err := jsonMarshal(rsp.Resource)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Error("Failed to unmarshal GRPC resource response", "error", err)
		return
	}
	w.Write(output)
}

func (h *resourceHandler) handleRead(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	tenancyInfo, params := parseParams(r)
	if params["consistent"] != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-consul-consistency-mode", "consistent")
	}

	rsp, err := h.client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Type:    h.reg.Type,
			Tenancy: tenancyInfo,
			Name:    params["resourceName"],
		},
	})
	if err != nil {
		handleResponseError(err, w, h)
		return
	}

	output, err := jsonMarshal(rsp.Resource)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Error("Failed to unmarshal GRPC resource response", "error", err)
		return
	}
	w.Write(output)
}

// Note: The HTTP endpoints do not accept UID since it is quite unlikely that the user will have access to it
func (h *resourceHandler) handleDelete(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	tenancyInfo, params := parseParams(r)
	_, err := h.client.Delete(ctx, &pbresource.DeleteRequest{
		Id: &pbresource.ID{
			Type:    h.reg.Type,
			Tenancy: tenancyInfo,
			Name:    params["resourceName"],
		},
		Version: params["version"],
	})
	if err != nil {
		handleResponseError(err, w, h)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	w.Write([]byte("{}"))
}

func parseParams(r *http.Request) (tenancy *pbresource.Tenancy, params map[string]string) {
	query := r.URL.Query()
	tenancy = &pbresource.Tenancy{
		Partition: query.Get("partition"),
		PeerName:  query.Get("peer_name"),
		Namespace: query.Get("namespace"),
	}

	resourceName := path.Base(r.URL.Path)
	if resourceName == "." || resourceName == "/" {
		resourceName = ""
	}

	params = make(map[string]string)
	params["resourceName"] = resourceName
	params["version"] = query.Get("version")
	if _, ok := query["consistent"]; ok {
		params["consistent"] = "true"
	}

	return
}

func jsonMarshal(res *pbresource.Resource) ([]byte, error) {
	output, err := protojson.Marshal(res)
	if err != nil {
		return nil, err
	}

	var stuff map[string]any
	if err := json.Unmarshal(output, &stuff); err != nil {
		return nil, err
	}

	delete(stuff["data"].(map[string]any), "@type")
	return json.MarshalIndent(stuff, "", "  ")
}

func handleResponseError(err error, w http.ResponseWriter, h *resourceHandler) {
	if e, ok := status.FromError(err); ok {
		switch e.Code() {
		case codes.InvalidArgument:
			w.WriteHeader(http.StatusBadRequest)
			h.logger.Info("User has mal-formed request", "error", err)
		case codes.NotFound:
			w.WriteHeader(http.StatusNotFound)
			h.logger.Info("Received error from resource service: Not found", "error", err)
		case codes.PermissionDenied:
			w.WriteHeader(http.StatusForbidden)
			h.logger.Info("Received error from resource service: User not authenticated", "error", err)
		case codes.Aborted:
			w.WriteHeader(http.StatusConflict)
			h.logger.Info("Received error from resource service: the request conflict with the current state of the target resource", "error", err)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Error("Received error from resource service", "error", err)
		}
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Error("Received error from resource service: not able to parse error returned", "error", err)
	}
	w.Write([]byte(err.Error()))
}
