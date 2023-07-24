package protohcl

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type UnmarshalContext interface{}

func Unmarshal(src []byte, dest protoreflect.ProtoMessage) error {
	return UnmarshalOptions{}.Unmarshal(src, dest)
}

type UnmarshalOptions struct {
	AnyTypeProvider AnyTypeProvider

	SourceFileName string
}

func (u UnmarshalOptions) Unmarshal(src []byte, dest protoreflect.ProtoMessage) error {
	rmsg := dest.ProtoReflect()

	file, diags := hclparse.NewParser().ParseHCL(src, u.SourceFileName)

	// error performing basic HCL parsing
	if diags.HasErrors() {
		return diags
	}

	u.clearAll(rmsg)

	return u.decodeHCLBody(file.Body, rmsg)
}

func (u UnmarshalOptions) decodeHCLBody(body hcl.Body, msg protoreflect.Message) error {
	if msg.Descriptor().FullName() == wellKnownTypeAny {
		return u.decodeWellKnownAnyFromHCLBody(body, msg)
	}

	schema, err := u.hclBodySchemaForFields(msg.Descriptor().Fields())
	if err != nil {
		return err
	}

	return u.decodeHCLBodyWithSchema(body, schema, msg)
}

func (u UnmarshalOptions) decodeHCLBodyWithSchema(body hcl.Body, schema *hcl.BodySchema, msg protoreflect.Message) error {
	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return diags
	}

	return u.decodeHCLBodyContent(content, msg)
}

func (u UnmarshalOptions) decodeHCLBodyContent(content *hcl.BodyContent, msg protoreflect.Message) error {
	fields := msg.Descriptor().Fields()

	tracker := newOneOfTracker()
	for _, attr := range content.Attributes {
		// All value types (list elements, map values or top-level fields) will be either
		// primitive types or Well Known primitive wrapper types. If not they would have
		// been decoded as blocks

		f := fields.ByTextName(attr.Name)
		if f == nil {
			return fmt.Errorf("protobuf message has no field with text name %q", attr.Name)
		}

		err := tracker.markFieldAsSet(f)
		if err != nil {
			return err
		}

		val, err := u.protoValFromHCLAttr(attr, msg, f)
		if err != nil {
			return err
		}

		if val.IsValid() {
			msg.Set(f, val)
		}
	}

	for blockType, blocks := range content.Blocks.ByType() {
		// All value types (list elements, map values, or top level fields) will be
		// message types as only these get block encoding.

		f := fields.ByTextName(blockType)
		if f == nil {
			return fmt.Errorf("protobuf message has no field with text name %q", blockType)
		}

		err := tracker.markFieldAsSet(f)
		if err != nil {
			return err
		}

		val, err := u.protoValFromHCLBlocks(blocks, msg, f)
		if err != nil {
			return err
		}

		if val.IsValid() {
			msg.Set(f, val)
		}
	}

	return nil
}

func (u UnmarshalOptions) protoValFromHCLAttr(attr *hcl.Attribute, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	attrVal, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return protoreflect.Value{}, diags
	}

	if f.IsMap() {
		return u.protoMapValueFromAttr(msg, f, attrVal)
	}

	if f.IsList() {
		return u.protoListValueFromAttr(msg, f, attrVal)
	}

	ok, value, err := wellKnownTypeDecode(f, attrVal)
	if ok {
		return value, err
	}

	return protoPrimitiveFromCty(f, attrVal)
}

func (u UnmarshalOptions) protoListValueFromAttr(msg protoreflect.Message, desc protoreflect.FieldDescriptor, value cty.Value) (protoreflect.Value, error) {
	if value.IsNull() {
		return protoreflect.Value{}, nil
	}

	valueType := value.Type()
	if !valueType.IsListType() && !valueType.IsTupleType() {
		return protoreflect.Value{}, fmt.Errorf("expected list/tuple type after HCL decode but the actual type was %s", valueType.FriendlyName())
	}

	if value.LengthInt() < 1 {
		return protoreflect.Value{}, nil
	}

	protoList := msg.NewField(desc).List()

	var err error
	value.ForEachElement(func(_ cty.Value, val cty.Value) bool {
		var protoVal protoreflect.Value
		var ok bool

		ok, protoVal, err = wellKnownTypeDecode(desc, val)
		if !ok {
			protoVal, err = protoPrimitiveFromCty(desc, val)
		}
		if err != nil {
			return true
		}

		protoList.Append(protoVal)
		return false
	})

	if err != nil {
		return protoreflect.Value{}, fmt.Errorf("error processing HCL list elements: %w", err)
	}

	return protoreflect.ValueOfList(protoList), nil
}

func (u UnmarshalOptions) protoMapValueFromAttr(msg protoreflect.Message, desc protoreflect.FieldDescriptor, value cty.Value) (protoreflect.Value, error) {
	if value.IsNull() {
		return protoreflect.Value{}, nil
	}

	valueType := value.Type()
	if !valueType.IsMapType() && !valueType.IsObjectType() {
		return protoreflect.Value{}, fmt.Errorf("expected map/object type after HCL decode but the actual type was %s", valueType.FriendlyName())
	}

	if value.LengthInt() < 1 {
		return protoreflect.Value{}, nil
	}

	protoMap := msg.NewField(desc).Map()
	protoValueDesc := desc.MapValue()
	var err error

	value.ForEachElement(func(key cty.Value, val cty.Value) (stop bool) {
		var ok bool
		var protoVal protoreflect.Value

		ok, protoVal, err = wellKnownTypeDecode(protoValueDesc, val)
		if !ok {
			protoVal, err = protoPrimitiveFromCty(protoValueDesc, val)
		}
		if err != nil {
			return true
		}

		if protoVal.IsValid() {
			// HCL doesn't support non-string keyed maps so we blindly use string keys
			protoMap.Set(protoreflect.ValueOfString(key.AsString()).MapKey(), protoVal)
		}
		return false
	})

	if err != nil {
		return protoreflect.Value{}, fmt.Errorf("error processing HCL map elements: %w", err)
	}

	return protoreflect.ValueOfMap(protoMap), nil
}

func (u UnmarshalOptions) protoValFromHCLBlocks(blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	if f.Kind() != protoreflect.MessageKind {
		return protoreflect.Value{}, fmt.Errorf("only protobuf message kinds can use HCL block syntax")
	}
	if f.IsMap() {
		return u.protoMapValueFromBlocks(blocks, msg, f)
	}

	if f.IsList() {
		return u.protoListValueFromBlocks(blocks, msg, f)
	}

	return u.protoMessageValueFromBlocks(blocks, msg, f)
}

func (u UnmarshalOptions) protoMapValueFromBlocks(blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	val := msg.NewField(f)
	mapVal := val.Map()

	var schema *hcl.BodySchema
	var err error
	for _, block := range blocks {
		if len(block.Labels) != 1 {
			return protoreflect.Value{}, fmt.Errorf("protobuf map fields must have 1 HCL block label")
		}

		key := protoreflect.ValueOfString(block.Labels[0])
		value := mapVal.NewValue()
		msgVal := value.Message()

		// for readability reasons I would love to stick this outside the for loop
		// however you have to allocate a map value before you can get access to
		// the message descriptor
		if schema == nil {
			schema, err = u.hclBodySchemaForFields(msgVal.Descriptor().Fields())
			if err != nil {
				return protoreflect.Value{}, err
			}
		}

		err = u.decodeHCLBodyWithSchema(block.Body, schema, msgVal)
		if err != nil {
			return protoreflect.Value{}, err
		}

		mapVal.Set(key.MapKey(), value)
	}
	return val, nil
}

func (u UnmarshalOptions) protoListValueFromBlocks(blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	val := msg.NewField(f)
	listVal := val.List()

	var schema *hcl.BodySchema
	var err error
	for _, block := range blocks {
		if len(block.Labels) > 0 {
			return protoreflect.Value{}, fmt.Errorf("repeated protobuf fields must not have HCL block labels")
		}
		elem := listVal.NewElement()
		elemMsg := elem.Message()

		// for readability reasons I would love to stick this outside the for loop
		// however you have to allocate a list element before you can get access to
		// the message descriptor
		if schema == nil {
			schema, err = u.hclBodySchemaForFields(elemMsg.Descriptor().Fields())
			if err != nil {
				return protoreflect.Value{}, err
			}
		}

		err = u.decodeHCLBodyWithSchema(block.Body, schema, elemMsg)
		if err != nil {
			return protoreflect.Value{}, err
		}

		listVal.Append(elem)
	}

	return val, nil
}

func (u UnmarshalOptions) protoMessageValueFromBlocks(blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	if len(blocks) > 1 {
		return protoreflect.Value{}, fmt.Errorf("only one HCL block may be specified for a non-repeated protobuf Message")
	}

	val := msg.NewField(f)
	err := u.decodeHCLBody(blocks[0].Body, val.Message())
	if err != nil {
		return protoreflect.Value{}, err
	}

	return val, nil
}

func (u UnmarshalOptions) hclBodySchemaForFields(fields protoreflect.FieldDescriptors) (*hcl.BodySchema, error) {
	schema := hcl.BodySchema{}

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
					Type:       f.TextName(),
					LabelNames: []string{"key"},
				})
				continue
			}

			// non-message types or Well Known Message types that need attribute encoding
			// get decoded as attributes
			schema.Attributes = append(schema.Attributes, hcl.AttributeSchema{
				Name: f.TextName(),
			})
			continue
		}

		wktHint := wellKnownTypeSchemaHint(f)

		// message types generally will use block syntax unless its a well known
		// message type that requires attribute syntax specifically.
		if kind == protoreflect.MessageKind && wktHint != wellKnownAttribute {
			schema.Blocks = append(schema.Blocks, hcl.BlockHeaderSchema{
				Type: f.TextName(),
			})
		}

		// by default use attribute encoding
		// - primitives
		// - repeated primitives
		// - Well Known Types requiring attribute syntax
		// - repeated Well Known Types requiring attribute syntax
		schema.Attributes = append(schema.Attributes, hcl.AttributeSchema{
			Name: f.TextName(),
		})
		continue
	}

	return &schema, nil
}

func (u UnmarshalOptions) clearAll(msg protoreflect.Message) {
	fields := msg.Descriptor().Fields()

	for i := 0; i < fields.Len(); i++ {
		msg.Clear(fields.Get(i))
	}
}
