// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/go-rootcerts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/internal/resourcehcl"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
)

const JSON_INDENT = "  "

// tls.Config is used to establish communication in TLS mode
func SetupTLSConfig(c *GRPCConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !c.GRPCTLSVerify,
	}

	if c.CertFile != "" && c.KeyFile != "" {
		tlsCert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	var caConfig *rootcerts.Config
	if c.CAFile != "" || c.CAPath != "" {
		caConfig = &rootcerts.Config{
			CAFile: c.CAFile,
			CAPath: c.CAPath,
		}
	}
	// load system CA certs if user doesn't provide any
	if err := rootcerts.ConfigureTLS(tlsConfig, caConfig); err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

func removeSchemaFromGRPCAddress(addr string) string {
	// Parse as host:port with option http prefix
	grpcAddr := strings.TrimPrefix(addr, "http://")
	grpcAddr = strings.TrimPrefix(grpcAddr, "https://")
	return grpcAddr
}

type TValue[T string | bool] struct {
	v *T
}

// Set implements the flag.Value interface.
func (t *TValue[T]) Set(v string) error {
	if t.v == nil {
		t.v = new(T)
	}
	var err error
	// have to use interface{}(t.v) to do type assertion
	switch interface{}(t.v).(type) {
	case *string:
		// have to use interface{}(t.v).(*string) to assert t.v as *string
		*(interface{}(t.v).(*string)) = v
	case *bool:
		// have to use interface{}(t.v).(*bool) to assert t.v as *bool
		*(interface{}(t.v).(*bool)), err = strconv.ParseBool(v)
	default:
		err = fmt.Errorf("unsupported type %T", t.v)
	}
	return err
}

// String implements the flag.Value interface.
func (t *TValue[T]) String() string {
	var current T
	if t.v != nil {
		current = *(t.v)
	}
	return fmt.Sprintf("%v", current)
}

// Merge will overlay this value if it has been set.
func (t *TValue[T]) Merge(onto *T) error {
	if onto == nil {
		return fmt.Errorf("onto is nil")
	}
	if t.v != nil {
		*onto = *(t.v)
	}
	return nil
}

type OuterResource struct {
	ID         *ID            `json:"id"`
	Owner      *ID            `json:"owner"`
	Generation string         `json:"generation"`
	Version    string         `json:"version"`
	Metadata   map[string]any `json:"metadata"`
	Data       map[string]any `json:"data"`
}

type Tenancy struct {
	Partition string `json:"partition"`
	Namespace string `json:"namespace"`
}

type Type struct {
	Group        string `json:"group"`
	GroupVersion string `json:"groupVersion"`
	Kind         string `json:"kind"`
}

type ID struct {
	Name    string  `json:"name"`
	Tenancy Tenancy `json:"tenancy"`
	Type    Type    `json:"type"`
	UID     string  `json:"uid"`
}

func ParseResourceFromFile(filePath string) (*pbresource.Resource, error) {
	return ParseResourceInput(filePath, nil)
}

func ParseResourceInput(filePath string, stdin io.Reader) (*pbresource.Resource, error) {
	data, err := helpers.LoadDataSourceNoRaw(filePath, stdin)

	if err != nil {
		return nil, fmt.Errorf("Failed to load data: %v", err)
	}
	var parsedResource *pbresource.Resource
	if isHCL([]byte(data)) {
		parsedResource, err = resourcehcl.Unmarshal([]byte(data), consul.NewTypeRegistry())
	} else {
		parsedResource, err = parseJson(data)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to decode resource from input: %v", err)
	}

	return parsedResource, nil
}

func ParseInputParams(inputArgs []string, flags *flag.FlagSet) error {
	if err := flags.Parse(inputArgs); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			return fmt.Errorf("Failed to parse args: %v", err)
		}
	}
	return nil
}

func GetTypeAndResourceName(args []string) (resourceType *pbresource.Type, resourceName string, e error) {
	if len(args) < 2 {
		return nil, "", fmt.Errorf("Must specify two arguments: resource type and resource name")
	}
	// it has to be resource name after the type
	if strings.HasPrefix(args[1], "-") {
		return nil, "", fmt.Errorf("Must provide resource name right after type")
	}
	resourceName = args[1]

	resourceType, e = InferTypeFromResourceType(args[0])

	return resourceType, resourceName, e
}

func InferTypeFromResourceType(resourceType string) (*pbresource.Type, error) {
	s := strings.Split(resourceType, ".")
	switch length := len(s); {
	// only kind is provided
	case length == 1:
		kindToGVKMap := BuildKindToGVKMap()
		kind := strings.ToLower(s[0])
		switch len(kindToGVKMap[kind]) {
		// no g.v.k is found
		case 0:
			return nil, fmt.Errorf("The shorthand name does not map to any existing resource type, please check `consul api-resources`")
		// only one is found
		case 1:
			// infer gvk from resource kind
			gvkSplit := strings.Split(kindToGVKMap[kind][0], ".")
			return &pbresource.Type{
				Group:        gvkSplit[0],
				GroupVersion: gvkSplit[1],
				Kind:         gvkSplit[2],
			}, nil
		// it alerts error if any conflict is found
		default:
			return nil, fmt.Errorf("The shorthand name has conflicts %v, please use the full name", kindToGVKMap[s[0]])
		}
	case length == 3:
		return &pbresource.Type{
			Group:        s[0],
			GroupVersion: s[1],
			Kind:         s[2],
		}, nil
	default:
		return nil, fmt.Errorf("Must provide resource type argument with either in group.version.kind format or its shorthand name")
	}
}

func BuildKindToGVKMap() map[string][]string {
	// this use the local copy of registration to build map
	typeRegistry := consul.NewTypeRegistry()
	kindToGVKMap := map[string][]string{}
	for _, r := range typeRegistry.Types() {
		gvkString := fmt.Sprintf("%s.%s.%s", r.Type.Group, r.Type.GroupVersion, r.Type.Kind)
		kindKey := strings.ToLower(r.Type.Kind)
		if len(kindToGVKMap[kindKey]) == 0 {
			kindToGVKMap[kindKey] = []string{gvkString}
		} else {
			kindToGVKMap[kindKey] = append(kindToGVKMap[kindKey], gvkString)
		}
	}
	return kindToGVKMap
}

// this is an inlined variant of hcl.lexMode()
func isHCL(v []byte) bool {
	var (
		r      rune
		w      int
		offset int
	)

	for {
		r, w = utf8.DecodeRune(v[offset:])
		offset += w
		if unicode.IsSpace(r) {
			continue
		}
		if r == '{' {
			return false
		}
		break
	}

	return true
}

func parseJson(js string) (*pbresource.Resource, error) {

	parsedResource := new(pbresource.Resource)

	var outerResource OuterResource

	if err := json.Unmarshal([]byte(js), &outerResource); err != nil {
		return nil, err
	}

	if outerResource.ID == nil {
		return nil, fmt.Errorf("\"id\" field need to be provided")
	}

	typ := pbresource.Type{
		Kind:         outerResource.ID.Type.Kind,
		Group:        outerResource.ID.Type.Group,
		GroupVersion: outerResource.ID.Type.GroupVersion,
	}

	reg, ok := consul.NewTypeRegistry().Resolve(&typ)
	if !ok {
		return nil, fmt.Errorf("invalid type %v", parsedResource)
	}
	data := reg.Proto.ProtoReflect().New().Interface()
	anyProtoMsg, err := anypb.New(data)
	if err != nil {
		return nil, err
	}

	outerResource.Data["@type"] = anyProtoMsg.TypeUrl

	marshal, err := json.Marshal(outerResource)
	if err != nil {
		return nil, err
	}

	if err := protojson.Unmarshal(marshal, parsedResource); err != nil {
		return nil, err
	}
	return parsedResource, nil
}
