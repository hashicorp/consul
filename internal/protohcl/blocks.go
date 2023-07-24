package protohcl

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (u UnmarshalOptions) decodeBlocks(ctx *UnmarshalContext, blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	if f.Kind() != protoreflect.MessageKind {
		return protoreflect.Value{}, fmt.Errorf("only protobuf message kinds can use HCL block syntax")
	}

	if f.IsMap() {
		return u.decodeBlocksToMap(ctx, blocks, msg, f)
	}

	if f.IsList() {
		return u.decodeBlocksToList(ctx, blocks, msg, f)
	}

	return u.decodeBlocksToMessage(ctx, blocks, msg, f)
}

func (u UnmarshalOptions) decodeBlocksToMap(ctx *UnmarshalContext, blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	val := msg.NewField(f)
	mapVal := val.Map()

	for _, block := range blocks {
		if len(block.Labels) != 1 {
			return protoreflect.Value{}, fmt.Errorf("protobuf map fields must have 1 HCL block label")
		}

		key := protoreflect.ValueOfString(block.Labels[0])
		value := mapVal.NewValue()
		msgVal := value.Message()

		err := u.decodeMessage(
			&UnmarshalContext{
				Parent:  ctx,
				Name:    fmt.Sprintf("%s[%q]", u.FieldNamer.NameField(f), block.Labels[0]),
				Message: msgVal,
				Range:   block.DefRange,
			},
			u.bodyDecoder(block.Body),
			msgVal,
		)
		if err != nil {
			return protoreflect.Value{}, err
		}

		mapVal.Set(key.MapKey(), value)
	}
	return val, nil
}

func (u UnmarshalOptions) decodeBlocksToList(ctx *UnmarshalContext, blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	val := msg.NewField(f)
	listVal := val.List()

	var err error
	for idx, block := range blocks {
		if len(block.Labels) > 0 {
			return protoreflect.Value{}, fmt.Errorf("repeated protobuf fields must not have HCL block labels")
		}
		elem := listVal.NewElement()
		elemMsg := elem.Message()

		err = u.decodeMessage(
			&UnmarshalContext{
				Parent:  ctx,
				Name:    fmt.Sprintf("%s[%d]", u.FieldNamer.NameField(f), idx),
				Message: elemMsg,
				Range:   block.DefRange,
			},
			u.bodyDecoder(block.Body),
			elemMsg,
		)
		if err != nil {
			return protoreflect.Value{}, err
		}
		listVal.Append(elem)
	}

	return val, nil
}

func (u UnmarshalOptions) decodeBlocksToMessage(ctx *UnmarshalContext, blocks hcl.Blocks, msg protoreflect.Message, f protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	if len(blocks) > 1 {
		return protoreflect.Value{}, fmt.Errorf("only one HCL block may be specified for a non-repeated protobuf Message")
	}

	val := msg.NewField(f)
	valMsg := val.Message()

	err := u.decodeMessage(
		&UnmarshalContext{
			Parent:  ctx,
			Name:    blocks[0].Type,
			Message: valMsg,
			Range:   blocks[0].DefRange,
		},
		u.bodyDecoder(blocks[0].Body),
		valMsg,
	)
	if err != nil {
		return protoreflect.Value{}, err
	}

	return val, nil
}
