package protohcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty/function"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// UnmarshalContext provides information about the context in which we are
// unmarshalling HCL. It is primarily used for decoding Any fields based on
// surrounding information (e.g. the resource Type block).
type UnmarshalContext struct {
	// Parent context.
	Parent *UnmarshalContext

	// Name of the field that we are unmarshalling.
	Name string

	// Message is the protobuf message that we are unmarshalling into (may be nil).
	Message protoreflect.Message

	// Range of where this field was in the HCL source.
	Range hcl.Range
}

// ErrorRange returns a range that can be used in error messages.
func (ctx *UnmarshalContext) ErrorRange() hcl.Range {
	for {
		if !ctx.Range.Empty() || ctx.Parent == nil {
			return ctx.Range
		}
		ctx = ctx.Parent
	}
}

func Unmarshal(src []byte, dest protoreflect.ProtoMessage) error {
	return UnmarshalOptions{}.Unmarshal(src, dest)
}

type UnmarshalOptions struct {
	AnyTypeProvider AnyTypeProvider
	SourceFileName  string
	FieldNamer      FieldNamer
	Functions       map[string]function.Function
}

func (u UnmarshalOptions) Unmarshal(src []byte, dest protoreflect.ProtoMessage) error {
	rmsg := dest.ProtoReflect()

	file, diags := hclparse.NewParser().ParseHCL(src, u.SourceFileName)

	// error performing basic HCL parsing
	if diags.HasErrors() {
		return diags
	}

	u.clearAll(rmsg)

	if u.FieldNamer == nil {
		u.FieldNamer = textFieldNamer{}
	}

	return u.decodeMessage(
		&UnmarshalContext{Message: rmsg},
		u.bodyDecoder(file.Body),
		rmsg,
	)
}

func (u UnmarshalOptions) bodyDecoder(body hcl.Body) MessageDecoder {
	return newBodyDecoder(body, u.FieldNamer, u.Functions)
}

func (u UnmarshalOptions) decodeMessage(ctx *UnmarshalContext, decoder MessageDecoder, msg protoreflect.Message) error {
	desc := msg.Descriptor()

	if desc.FullName() == wellKnownTypeAny {
		return u.decodeAny(ctx, decoder, msg)
	}

	tracker := newOneOfTracker(u.FieldNamer)

	return decoder.EachField(FieldIterator{
		Desc: desc,
		Func: func(field *IterField) error {
			if err := tracker.markFieldAsSet(field.Desc); err != nil {
				return err
			}

			var (
				protoVal protoreflect.Value
				err      error
			)
			switch {
			case field.Val != nil:
				protoVal, err = u.decodeAttribute(
					&UnmarshalContext{
						Parent: ctx,
						Name:   field.Name,
						Range:  field.Range,
					},
					func() protoreflect.Value { return msg.NewField(field.Desc) },
					field.Desc,
					*field.Val,
					true,
				)
			case len(field.Blocks) != 0:
				protoVal, err = u.decodeBlocks(ctx, field.Blocks, msg, field.Desc)
			default:
				panic("decoder yielded no blocks or attributes")
			}
			if err != nil {
				return err
			}

			if protoVal.IsValid() {
				msg.Set(field.Desc, protoVal)
			}

			return nil
		},
	})
}

type newMessageFn func() protoreflect.Value

func (u UnmarshalOptions) clearAll(msg protoreflect.Message) {
	fields := msg.Descriptor().Fields()

	for i := 0; i < fields.Len(); i++ {
		msg.Clear(fields.Get(i))
	}
}
