package protohcl

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MessageDecoder provides an abstract way to decode protobuf messages from HCL
// blocks or objects.
type MessageDecoder interface {
	// EachField calls the given iterator for each field provided in the HCL source.
	EachField(iter FieldIterator) error

	// SkipFields returns a MessageDecoder that skips over the given fields. It is
	// primarily used for doing two-pass decoding of protobuf `Any` fields.
	SkipFields(fields ...string) MessageDecoder
}

// IterField represents a field discovered by the MessageDecoder.
type IterField struct {
	// Name is the HCL name of the field.
	Name string

	// Desc is the protobuf field descriptor.
	Desc protoreflect.FieldDescriptor

	// Val is the field value, only if it was given using HCL attribute syntax.
	Val *cty.Value

	// Blocks contains the HCL blocks that were given for this field.
	Blocks []*hcl.Block

	// Range determines where in the HCL source the field was given, it is useful
	// for error messages.
	Range hcl.Range
}

// FieldIterator is given to MessageDecoder.EachField to iterate over all of the
// fields in a given HCL block or object.
type FieldIterator struct {
	// IgnoreUnknown instructs the MessageDecoder to skip over any fields not
	// included in Desc.
	IgnoreUnknown bool

	// Desc is the protobuf descriptor for the message the caller is decoding into.
	// It is used to determine which fields are valid.
	Desc protoreflect.MessageDescriptor

	// Func is called for each field in the given HCL block or object.
	Func func(field *IterField) error
}

func newBodyDecoder(
	body hcl.Body,
	namer FieldNamer,
	functions map[string]function.Function,
) MessageDecoder {
	return bodyDecoder{
		body:       body,
		namer:      namer,
		functions:  functions,
		skipFields: make(map[string]struct{}),
	}
}

type bodyDecoder struct {
	body       hcl.Body
	namer      FieldNamer
	functions  map[string]function.Function
	skipFields map[string]struct{}
}

func (bd bodyDecoder) EachField(iter FieldIterator) error {
	schema, err := bd.schema(iter.Desc)
	if err != nil {
		return err
	}

	var (
		content *hcl.BodyContent
		diags   hcl.Diagnostics
	)
	if iter.IgnoreUnknown {
		content, _, diags = bd.body.PartialContent(schema)
	} else {
		content, diags = bd.body.Content(schema)
	}
	if diags.HasErrors() {
		return diags
	}

	fields := make([]*IterField, 0)

	for _, attr := range content.Attributes {
		if _, ok := bd.skipFields[attr.Name]; ok {
			continue
		}

		desc := bd.namer.GetField(iter.Desc.Fields(), attr.Name)

		val, err := attr.Expr.Value(&hcl.EvalContext{Functions: bd.functions})
		if err != nil {
			return err
		}

		fields = append(fields, &IterField{
			Name:  attr.Name,
			Desc:  desc,
			Val:   &val,
			Range: attr.Expr.Range(),
		})
	}

	for blockType, blocks := range content.Blocks.ByType() {
		if _, ok := bd.skipFields[blockType]; ok {
			continue
		}

		desc := bd.namer.GetField(iter.Desc.Fields(), blockType)

		fields = append(fields, &IterField{
			Name:   blockType,
			Desc:   desc,
			Blocks: blocks,
		})
	}

	// Always handle Any fields last, as decoding them may require type information
	// gathered from other fields (e.g. as in the case of Resource GVKs).
	sort.Slice(fields, func(a, b int) bool {
		if isAnyField(fields[b].Desc) && !isAnyField(fields[a].Desc) {
			return true
		}
		return a < b
	})

	for _, field := range fields {
		if err := iter.Func(field); err != nil {
			return err
		}
	}

	return nil
}

func (bd bodyDecoder) SkipFields(fields ...string) MessageDecoder {
	skip := make(map[string]struct{}, len(fields)+len(bd.skipFields))
	for k, v := range bd.skipFields {
		skip[k] = v
	}
	for _, field := range fields {
		skip[field] = struct{}{}
	}

	// Note: we rely on the fact bd isn't a pointer to copy the struct here.
	bd.skipFields = skip
	return bd
}

func (bd bodyDecoder) schema(desc protoreflect.MessageDescriptor) (*hcl.BodySchema, error) {
	var schema hcl.BodySchema

	fields := desc.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)

		kind := f.Kind()
		// maps are special and whether they use block or attribute syntax depends
		// on the value type
		if f.IsMap() {
			valueDesc := f.MapValue()
			valueKind := valueDesc.Kind()

			wktHint := wellKnownTypeSchemaHint(valueDesc)

			// Message types should generally be encoded as blocks unless its a special Well Known Type
			// that should use attribute encoding
			if valueKind == protoreflect.MessageKind && wktHint != wellKnownAttribute {
				schema.Blocks = append(schema.Blocks, hcl.BlockHeaderSchema{
					Type:       bd.namer.NameField(f),
					LabelNames: []string{"key"},
				})
				continue
			}

			// non-message types or Well Known Message types that need attribute encoding
			// get decoded as attributes
			schema.Attributes = append(schema.Attributes, hcl.AttributeSchema{
				Name: bd.namer.NameField(f),
			})
			continue
		}

		wktHint := wellKnownTypeSchemaHint(f)

		// message types generally will use block syntax unless its a well known
		// message type that requires attribute syntax specifically.
		if kind == protoreflect.MessageKind && wktHint != wellKnownAttribute {
			schema.Blocks = append(schema.Blocks, hcl.BlockHeaderSchema{
				Type: bd.namer.NameField(f),
			})
		}

		// by default use attribute encoding
		// - primitives
		// - repeated primitives
		// - Well Known Types requiring attribute syntax
		// - repeated Well Known Types requiring attribute syntax
		schema.Attributes = append(schema.Attributes, hcl.AttributeSchema{
			Name: bd.namer.NameField(f),
		})
		continue
	}

	// Add skipped fields to the schema so HCL doesn't throw an error when it finds them.
	for field := range bd.skipFields {
		schema.Attributes = append(schema.Attributes, hcl.AttributeSchema{Name: field})
		schema.Blocks = append(schema.Blocks, hcl.BlockHeaderSchema{Type: field})
	}

	return &schema, nil
}

func newObjectDecoder(object cty.Value, namer FieldNamer, rng hcl.Range) MessageDecoder {
	return objectDecoder{
		object:     object,
		namer:      namer,
		rng:        rng,
		skipFields: make(map[string]struct{}),
	}
}

type objectDecoder struct {
	object     cty.Value
	namer      FieldNamer
	rng        hcl.Range
	skipFields map[string]struct{}
}

func (od objectDecoder) EachField(iter FieldIterator) error {
	for attr := range od.object.Type().AttributeTypes() {
		if _, ok := od.skipFields[attr]; ok {
			continue
		}

		desc := od.namer.GetField(iter.Desc.Fields(), attr)
		if desc == nil {
			if iter.IgnoreUnknown {
				continue
			} else {
				return fmt.Errorf("%s: Unsupported argument; An argument named %q is not expected here.", od.rng, attr)
			}
		}

		val := od.object.GetAttr(attr)
		if err := iter.Func(&IterField{
			Name: attr,
			Desc: desc,
			Val:  &val,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (od objectDecoder) SkipFields(fields ...string) MessageDecoder {
	skip := make(map[string]struct{}, len(fields)+len(od.skipFields))
	for k, v := range od.skipFields {
		skip[k] = v
	}
	for _, field := range fields {
		skip[field] = struct{}{}
	}

	// Note: we rely on the fact od isn't a pointer to copy the struct here.
	od.skipFields = skip
	return od
}
