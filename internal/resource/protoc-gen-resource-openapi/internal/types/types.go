// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// The types package contains structs which comprimse elements of an OpenAPI 3.0 configuration.
// This is basically just used as a means to marshal the generated configuration.

package types

import "google.golang.org/protobuf/reflect/protoreflect"

type Document struct {
	Version    string                `yaml:"openapi"`
	Info       Info                  `yaml:"info"`
	Paths      map[string]Path       `yaml:"paths,omitempty"`
	Components Components            `yaml:"components,omitempty"`
	Security   []map[string][]string `yaml:"security,omitempty"`
}

type Info struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

type Path struct {
	Summary     string `yaml:"summary,omitempty"`
	Description string `yaml:"description,omitempty"`

	Get    *Operation `yaml:"get,omitempty"`
	Put    *Operation `yaml:"put,omitempty"`
	Delete *Operation `yaml:"delete,omitempty"`

	Parameters []*Parameter `yaml:"parameters,omitempty"`
}

type Operation struct {
	Summary     string              `yaml:"summary,omitempty"`
	Description string              `yaml:"description,omitempty"`
	OperationID string              `yaml:"operationId"`
	Parameters  []*Parameter        `yaml:"parameters,omitempty"`
	RequestBody *RequestBody        `yaml:"requestBody,omitempty"`
	Responses   map[string]Response `yaml:"responses"`
}

type Parameter struct {
	Name        string  `yaml:"name,omitempty"`
	In          string  `yaml:"in,omitempty"`
	Description string  `yaml:"description,omitempty"`
	Required    bool    `yaml:"required,omitempty"`
	Schema      *Schema `yaml:"schema,omitempty"`
	Ref         string  `yaml:"$ref,omitempty"`
}

type RequestBody struct {
	Description string             `yaml:"description,omitempty"`
	Content     map[string]Content `yaml:"content,omitempty"`
	Required    bool               `yaml:"required,omitempty"`
}

type Content struct {
	Schema *Schema `yaml:"schema"`
}

type Response struct {
	Description string                  `yaml:"description,omitempty"`
	Headers     map[string]HeaderObject `yaml:"headers,omitempty"`
	Content     map[string]Content      `yaml:"content,omitempty"`
}

type HeaderObject struct {
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
	Schema      Schema `yaml:"schema,omitempty"`
}

type Components struct {
	Schemas         map[string]*Schema         `yaml:"schemas"`
	Parameters      map[string]*Parameter      `yaml:"parameters"`
	SecuritySchemes map[string]*SecurityScheme `yaml:"securitySchemes"`
}

type Schema struct {
	Type                 string             `yaml:"type,omitempty"`
	OneOf                []*Schema          `yaml:"oneof,omitempty"`
	Enum                 []string           `yaml:"enum,omitempty"`
	Properties           map[string]*Schema `yaml:"properties,omitempty"`
	Ref                  string             `yaml:"$ref,omitempty"`
	Items                *Schema            `yaml:"items,omitempty"`
	AdditionalProperties *Schema            `yaml:"additionalProperties,omitempty"`
	Format               string             `yaml:"format,omitempty"`
	Description          string             `yaml:"description,omitempty"`
	Minimum              int                `yaml:"minimum,omitempty"`
	Pattern              string             `yaml:"pattern,omitempty"`
}

type SecurityScheme struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
	Scheme      string `yaml:"scheme,omitempty"`
	In          string `yaml:"in,omitempty"`
	Name        string `yaml:"name,omitempty"`
}

func BoolSchema() *Schema {
	return &Schema{
		Type: "boolean",
	}
}

func FloatSchema() *Schema {
	return &Schema{
		Type:   "number",
		Format: "float",
	}
}

func DoubleSchema() *Schema {
	return &Schema{
		Type:   "number",
		Format: "double",
	}
}

func Uint32Schema() *Schema {
	return &Schema{
		Type:    "integer",
		Format:  "int32",
		Minimum: 0,
	}
}

func Int32Schema() *Schema {
	return &Schema{
		Type:   "integer",
		Format: "int32",
	}
}

func Uint64Schema() *Schema {
	return &Schema{
		Type:    "integer",
		Format:  "int64",
		Minimum: 0,
	}
}

func Int64Schema() *Schema {
	return &Schema{
		Type:   "integer",
		Format: "int64",
	}
}

func StringSchema() *Schema {
	return &Schema{
		Type: "string",
	}
}

func BytesSchema() *Schema {
	return &Schema{
		Type:   "string",
		Format: "byte",
	}
}

func DurationSchema() *Schema {
	return &Schema{
		Type:        "string",
		Pattern:     `^-?(?:0|[1-9][0-9]{0,11})(?:\.[0-9]{1,9})?s$`,
		Description: "Represents a a duration between -315,576,000,000s and 315,576,000,000s (around 10000 years). Precision is in nanoseconds. 1 nanosecond is represented as 0.000000001s",
	}
}

func TimestampSchema() *Schema {
	return &Schema{
		Type:   "string",
		Format: "date-time",
	}
}

func StructSchema() *Schema {
	return &Schema{
		Type: "object",
	}
}

func AnySchema() *Schema {
	return &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"@type": {
				Type:        "string",
				Description: "The type of the serialize message",
			},
		},
	}
}

// SchemaCreators are used instead of variables/constants for known
// schemas to allow for consumers to add additional fields like descriptions
// without having to worry about deep copying data.
type SchemaCreator func() *Schema

var (
	HTTPAuthzBearerScheme = &SecurityScheme{
		Type:   "http",
		Scheme: "bearer",
	}

	WrapperSchemas = map[string]SchemaCreator{
		"google.protobuf.BoolValue":   BoolSchema,
		"google.protobuf.BytesValue":  BytesSchema,
		"google.protobuf.StringValue": StringSchema,
		"google.protobuf.Uint32Value": Uint32Schema,
		"google.protobuf.Uint64Value": Uint64Schema,
		"google.protobuf.Int32Value":  Int32Schema,
		"google.protobuf.Int64Value":  Int64Schema,
		"google.protobuf.FloatValue":  FloatSchema,
		"google.protobuf.DoubleValue": DoubleSchema,
	}

	WKTSchemas = map[string]SchemaCreator{
		"google.protobuf.Duration":  DurationSchema,
		"google.protobuf.Timestamp": TimestampSchema,
		"google.protobuf.Struct":    StructSchema,
		"google.protobuf.Any":       AnySchema,
	}

	PrimitiveSchemas = map[protoreflect.Kind]SchemaCreator{
		protoreflect.BoolKind:     BoolSchema,
		protoreflect.Int32Kind:    Int32Schema,
		protoreflect.Int64Kind:    Int64Schema,
		protoreflect.Uint32Kind:   Uint32Schema,
		protoreflect.Uint64Kind:   Uint64Schema,
		protoreflect.BytesKind:    BytesSchema,
		protoreflect.DoubleKind:   DoubleSchema,
		protoreflect.FloatKind:    FloatSchema,
		protoreflect.StringKind:   StringSchema,
		protoreflect.Fixed32Kind:  Uint32Schema,
		protoreflect.Fixed64Kind:  Uint64Schema,
		protoreflect.Sfixed32Kind: Int32Schema,
		protoreflect.Sfixed64Kind: Int64Schema,
	}
)
