// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package generate

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/protoc-gen-resource-openapi/internal/types"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	baseResourceType = "hashicorp.consul.resource.Resource"
)

func Generate(files []*protogen.File) (map[string][]byte, error) {
	g := newGenerator()
	for _, f := range files {
		if err := g.addMessagesFromFile(f); err != nil {
			return nil, err
		}
	}
	return g.generate()
}

type generator struct {
	apis         apiGroups
	schemas      map[string]*types.Schema
	dependencies map[string][]string

	resourceProperties map[string]*types.Schema
}

func newGenerator() *generator {
	return &generator{
		apis:         newAPIGroups(),
		schemas:      make(map[string]*types.Schema),
		dependencies: make(map[string][]string),
	}
}

func (g *generator) addMessagesFromFile(f *protogen.File) error {
	if !f.Generate {
		return nil
	}

	for _, m := range f.Messages {
		if m.Desc.FullName() == baseResourceType {
			props, err := g.getPropertiesFromMessageFields(m)
			if err != nil {
				return err
			}
			g.resourceProperties = props
			continue
		}

		ext := proto.GetExtension(m.Desc.Options(), pbresource.E_Spec).(*pbresource.ResourceTypeSpec)
		if ext == nil || ext.DontMapHttp {
			continue
		}

		gvkString := strings.TrimPrefix(string(m.Desc.FullName()), "hashicorp.consul.")
		rtype, err := resource.ParseGVK(gvkString)
		if err != nil {
			return err
		}

		g.apis.addResource(&resourceKind{
			group:       rtype.Group,
			version:     rtype.GroupVersion,
			kind:        rtype.Kind,
			scope:       ext.Scope,
			dataTypeRef: schemaRef(m.Desc.FullName()),
			name:        string(m.Desc.FullName()),
		})

		if err := g.addMessage(m); err != nil {
			return err
		}
	}

	return nil
}

func (g *generator) addMessage(m *protogen.Message) error {
	name := string(m.Desc.FullName())
	if _, found := g.schemas[name]; found {
		return nil
	}

	s := &types.Schema{
		Type:        "object",
		Description: string(m.Comments.Leading),
	}

	// need to add this into the map to prevent infinite recursion
	g.schemas[name] = s
	g.dependencies[name] = make([]string, 0)

	props, err := g.getPropertiesFromMessageFields(m)
	if err != nil {
		return err
	}
	s.Properties = props

	for _, nestedMsg := range m.Messages {
		// maps will be handled in a way that the wire formats KV pair map
		// entry type doesn't need to be present in the openapi spec. See
		// the createMapFieldSchema method.
		if nestedMsg.Desc.IsMapEntry() {
			continue
		}

		if err := g.addMessage(nestedMsg); err != nil {
			return err
		}

		g.dependencies[name] = append(g.dependencies[name], string(nestedMsg.Desc.FullName()))
	}

	for _, nestedEnum := range m.Enums {
		if err := g.addEnum(nestedEnum); err != nil {
			return err
		}
		g.dependencies[name] = append(g.dependencies[name], string(nestedEnum.Desc.FullName()))
	}

	// TODO handle mutual exclusivity of oneof fields. The solution is probably to add to the description
	// field which other fields it is exclusive with.

	return nil
}

func (g *generator) getPropertiesFromMessageFields(m *protogen.Message) (map[string]*types.Schema, error) {
	props := make(map[string]*types.Schema)
	name := string(m.Desc.FullName())
	for _, f := range m.Fields {
		fs, deps, err := g.createFieldSchema(f)
		if err != nil {
			return nil, err
		}
		props[string(f.Desc.Name())] = fs

		g.dependencies[name] = append(g.dependencies[name], deps...)
	}

	return props, nil
}

func schemaRef(name protoreflect.FullName) string {
	return fmt.Sprintf("#/components/schemas/%s", string(name))
}

func paramRef(name string) string {
	return fmt.Sprintf("#/components/parameters/%s", name)
}

func (g *generator) createFieldSchema(f *protogen.Field) (*types.Schema, []string, error) {
	if f.Desc.IsList() {
		return g.createListFieldSchema(f)
	} else if f.Desc.IsMap() {
		return g.createMapFieldSchema(f)
	}
	return g.createRawFieldSchema(f, false)
}

func (g *generator) createListFieldSchema(f *protogen.Field) (*types.Schema, []string, error) {
	items, deps, err := g.createRawFieldSchema(f, true)
	if err != nil {
		return nil, nil, err
	}

	return &types.Schema{
		Type:        "array",
		Items:       items,
		Description: f.Comments.Leading.String(),
	}, deps, nil
}

func getMapValueField(m *protogen.Field) *protogen.Field {
	const mapEntryFieldLength = 2
	const mapEntryValueFieldName = "value"
	const mapEntryValueFieldNumber = 2

	if m.Message == nil {
		panic("getMapValueField called on non-map value")
	}

	// The Golang protobuf implementation will auto-generate a MapEntry type to hold the Key/Value pair
	// that represents one entry in the map. Within that auto-generated message type their will be two
	// fields called "key" (index 1) and "value" (index 2).
	//
	// see https://pkg.go.dev/google.golang.org/protobuf@v1.31.0/reflect/protoreflect#MessageDescriptor
	if !m.Message.Desc.IsMapEntry() {
		panic("getMapValueField called on a message which is not a MapEntry type")
	}

	// Ensure that the message contains the documented number of fields exactly.
	if len(m.Message.Fields) != mapEntryFieldLength {
		panic("MapEntry type doesn't have the required 2 fields")
	}

	// if we didn't want the protogen.Field but were fine with the protoreflect.FieldDescriptor
	// we could just call m.Message.Desc.Fields().Get(mapEntryValueFieldNumber).
	// As we do want the protogen.Field the only way to get it is to loop through all the fields
	// and try to match on the elements we are looking for.
	for _, field := range m.Message.Fields {
		if field.Desc.Name() == mapEntryValueFieldName && field.Desc.Number() == mapEntryValueFieldNumber {
			return field
		}
	}

	panic("the MapEntry \"value\" field was not found")
}

func (g *generator) createMapFieldSchema(f *protogen.Field) (*types.Schema, []string, error) {
	// f should point to a Field that was defined as map<K, V> within the proto files. protoc/buf
	// will translate `map<K, V> field_name` into a `repeated FieldNameMapEntry` with the FieldNameMapEntry
	// message type being synthesized. The structure of that message is documented and well known:
	//
	// https://pkg.go.dev/google.golang.org/protobuf@v1.31.0/reflect/protoreflect#MessageDescriptor
	//
	// For the purposes of generating an openapi schema, we want to expose the field as an object with
	// whose properties have the type defined as the type of the "value" field within the synthesized
	// MapEntry. Therefore we have to first peer through the MapEntry type to pull out the protogen.Field
	// for the "value" field.
	valueSchema, deps, err := g.createRawFieldSchema(getMapValueField(f), true)
	if err != nil {
		return nil, nil, err
	}

	return &types.Schema{
		Type:                 "object",
		AdditionalProperties: valueSchema,
		Description:          f.Comments.Leading.String(),
	}, deps, nil
}

func (g *generator) createRawFieldSchema(f *protogen.Field, omitDescription bool) (*types.Schema, []string, error) {
	var deps []string

	description := func() string {
		if !omitDescription {
			return f.Comments.Leading.String()
		}

		return ""
	}

	addDescription := func(s *types.Schema) *types.Schema {
		s.Description = description()
		return s
	}

	factory, found := types.PrimitiveSchemas[f.Desc.Kind()]
	if found {
		return addDescription(factory()), nil, nil
	}

	switch f.Desc.Kind() {
	case protoreflect.EnumKind:
		name := f.Enum.Desc.FullName()
		deps = append(deps, string(name))
		if err := g.addEnum(f.Enum); err != nil {
			return nil, nil, err
		}

		return &types.Schema{
			Ref:         schemaRef(name),
			Description: f.Comments.Leading.String(),
		}, deps, nil
	case protoreflect.MessageKind:
		name := f.Message.Desc.FullName()
		if factory, found := types.WrapperSchemas[string(name)]; found {
			return addDescription(factory()), nil, nil
		}

		if factory, found := types.WKTSchemas[string(name)]; found {
			return addDescription(factory()), nil, nil
		}

		deps = append(deps, string(name))
		if err := g.addMessage(f.Message); err != nil {
			return nil, nil, err
		}

		return &types.Schema{
			Ref:         schemaRef(name),
			Description: description(),
		}, deps, nil
	default:
		panic(fmt.Sprintf("unknown/unsupported protobuf kind: %v", f.Desc.Kind()))
	}
}

func (g *generator) addEnum(e *protogen.Enum) error {
	name := string(e.Desc.FullName())
	if _, found := g.schemas[name]; found {
		return nil
	}

	g.schemas[name] = g.createEnumSchema(e)

	return nil
}

func (g *generator) createEnumSchema(e *protogen.Enum) *types.Schema {
	// For now we are going to emit enums with a string type with possible
	// values being the string form of the enum values. We could allow
	// Enum's to use the integer encoding instead.

	s := types.Schema{
		Type:        "string",
		Description: string(e.Comments.Leading),
	}

	for _, v := range e.Values {
		s.Enum = append(s.Enum, string(v.Desc.Name()))
	}

	return &s
}

func (g *generator) generate() (map[string][]byte, error) {
	files := make(map[string][]byte)
	// loop over each api group
	for group, versions := range g.apis {
		// loop over each api group version
		for version, kinds := range versions {
			doc := types.Document{
				Version: "3.0.0",
				Info: types.Info{
					Title:       fmt.Sprintf("Consul %s", group),
					Description: fmt.Sprintf("Consul APIs for interacting with the %s resource kinds at version %s", group, version),
					Version:     version,
				},
				Security: security,
				Components: types.Components{
					Schemas:         make(map[string]*types.Schema),
					Parameters:      allParameters,
					SecuritySchemes: securitySchemes,
				},
				Paths: make(map[string]types.Path),
			}

			// Add all the base resource dependent schemas
			for name, s := range g.getTypesSchemaDependencies(baseResourceType) {
				doc.Components.Schemas[name] = s
			}

			// add all paths for the different resource kinds in the group version
			for _, rsc := range kinds {
				// Get all component schemas and merge them with the final set
				for name, s := range g.getComponentSchemasForType(rsc.name) {
					doc.Components.Schemas[name] = s
				}

				for path, pathConfig := range g.generatePathsForResource(rsc) {
					doc.Paths[path] = pathConfig
				}
			}

			content, err := yaml.Marshal(doc)
			if err != nil {
				return nil, err
			}
			fname := fmt.Sprintf("%s-%s.openapi.yml", group, version)
			files[fname] = content
		}
	}

	return files, nil
}

func (g *generator) getComponentSchemasForType(name string) map[string]*types.Schema {
	schemas := g.getTypesSchemaDependencies(name)
	schemas[name] = g.schemas[name]

	return schemas
}

func (g *generator) getTypesSchemaDependencies(name string) map[string]*types.Schema {
	schemas := make(map[string]*types.Schema)

	deps := g.dependencies[name]
	for len(deps) > 0 {
		dep := deps[0]
		if _, found := schemas[dep]; !found {
			schemas[dep] = g.schemas[dep]
			deps = append(deps[1:], g.dependencies[dep]...)
		} else {
			deps = deps[1:]
		}
	}

	return schemas
}

type apiGroups map[string]groupVersions

func newAPIGroups() apiGroups {
	return make(apiGroups)
}

func (g apiGroups) addResource(rsc *resourceKind) {
	grp, ok := g[rsc.group]
	if !ok {
		grp = newGroupVersions()
		g[rsc.group] = grp
	}
	grp.addResource(rsc)
}

type groupVersions map[string]groupKinds

func newGroupVersions() groupVersions {
	return make(groupVersions)
}

func (v groupVersions) addResource(rsc *resourceKind) {
	k, ok := v[rsc.version]
	if !ok {
		k = newGroupKinds()
		v[rsc.version] = k
	}
	k.addResource(rsc)
}

type groupKinds map[string]*resourceKind

func newGroupKinds() groupKinds {
	return make(groupKinds)
}

func (k groupKinds) addResource(rsc *resourceKind) {
	k[rsc.kind] = rsc
}

type resourceKind struct {
	name    string
	group   string
	version string
	kind    string

	scope       pbresource.Scope
	dataTypeRef string
}
