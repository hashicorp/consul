package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"google.golang.org/grpc/metadata"
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
	// TODO: Owner.
	Version  string            `json:"version"`
	Metadata map[string]string `json:"metadata"`
	Data     json.RawMessage   `json:"data"`
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
	case http.MethodGet:
		h.handleRead(w, r, ctx)
	case http.MethodPut:
		h.handleWrite(w, r, ctx)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (h *resourceHandler) handleRead(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	tenancyInfo, _ := checkURL(r)
	if tenancyInfo == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	rsp, err := h.client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Type:    h.reg.Type,
			Tenancy: tenancyInfo,
			Name:    r.URL.Path,
		},
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	output, err := jsonMarshal(rsp.Resource)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(output)
}

func (h *resourceHandler) handleWrite(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var req writeRequest
	// convert req data to struct
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Request body didn't follow schema."))
	}
	// struct to proto message
	data := h.reg.Proto.ProtoReflect().New().Interface()
	if err := protojson.Unmarshal(req.Data, data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Request body didn't follow schema."))
	}
	// proto message to any
	anyProtoMsg, err := anypb.New(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Error("Failed to convert proto message to any type", "error", err)
		return
	}

	tenancyInfo, resourceName := checkURL(r)
	if tenancyInfo == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing partition, peer_name or namespace in the query params"))
	}
	if resourceName == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing resource name in the URL"))
	}
	rsp, err := h.client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    h.reg.Type,
				Tenancy: tenancyInfo,
				Name:    resourceName,
			},
			Version:  req.Version,
			Metadata: req.Metadata,
			Data:     anyProtoMsg,
		},
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Error("Failed to write to GRPC resource", "error", err)
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

func checkURL(r *http.Request) (tenancy *pbresource.Tenancy, resourceName string) {
	params := r.URL.Query()
	partition := params.Get("partition")
	peerName := params.Get("peer_name")
	namespace := params.Get("namespace")
	if partition == "" || peerName == "" || namespace == "" {
		tenancy = nil
	} else {
		tenancy = &pbresource.Tenancy{
			Partition: partition,
			PeerName:  peerName,
			Namespace: namespace,
		}
	}
	resourceName = path.Base(r.URL.Path)

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
