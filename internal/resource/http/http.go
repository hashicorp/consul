package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func NewHandler(client pbresource.ResourceServiceClient, registry resource.Registry, parseToken func(req *http.Request, token *string)) http.Handler {
	mux := http.NewServeMux()
	for _, t := range registry.Types() {
		// Individual Resource Endpoints.
		prefix := fmt.Sprintf("/%s/%s/%s/", t.Type.Group, t.Type.GroupVersion, t.Type.Kind)
		fmt.Println("REGISTERED URLS: ", prefix)
		mux.Handle(prefix, http.StripPrefix(prefix, &resourceHandler{t, client, parseToken}))
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
	rsp, err := h.client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Type:    h.reg.Type,
			Tenancy: tenancy(r),
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
	// do we introduce logger in this server?
	//logger := hclog.New(&hclog.LoggerOptions{Name: "xinyi"})
	//logger.Debug("DECODING ERROR", "error", err.Error())
	var req writeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	data := h.reg.Proto.ProtoReflect().New().Interface()
	if err := protojson.Unmarshal(req.Data, data); err != nil {
		fmt.Println("UNMARSHAL REQUEST ERROR", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	a, err := anypb.New(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rsp, err := h.client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    h.reg.Type,
				Tenancy: tenancy(r),
				Name:    r.URL.Path,
			},
			Version:  req.Version,
			Metadata: req.Metadata,
			Data:     a,
		},
	})
	if err != nil {
		fmt.Println("WRITE ERROR", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	output, err := jsonMarshal(rsp.Resource)
	if err != nil {
		fmt.Println("UNMARSHAL RESPONSE ERROR", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(output)
}

func tenancy(r *http.Request) *pbresource.Tenancy {
	// TODO: Read querystring parameters.
	return &pbresource.Tenancy{
		Partition: "default",
		PeerName:  "local",
		Namespace: "default",
	}
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
