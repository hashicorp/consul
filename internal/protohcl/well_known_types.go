package protohcl

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	// maximum time in seconds that a time.Time which comes from
	// an RFC 3339 timestamp can represent
	maxTimestampSeconds = 253402300799
	// minimum time in seconds that a time.Time which comes from
	// an RFC 3339 timestamp can represent. This is negative
	// because RFC 3339 base time is year 0001 whereas time.Time
	// starts in 1970
	minTimestampSeconds = -62135596800
)

var (
	// minTime is the earliest time representable as both an
	// RFC 3339 timestamp and a time.Time value
	minTime = time.Unix(minTimestampSeconds, 0)
	// maxTime is the latest time respresentable as both an
	// RFC 3339 timestamp and a time.Time value
	maxTime = time.Unix(maxTimestampSeconds, 999999999)
)

type wktSchemaHint int

const (
	notWellKnownType wktSchemaHint = iota
	wellKnownBlock
	wellKnownAttribute
)

const (
	wellKnownTypeAny = "google.protobuf.Any"
)

type AnyTypeProvider interface {
	AnyType(UnmarshalContext, hcl.Body) (protoreflect.FullName, hcl.Body, error)
}

type AnyTypeURLProvider struct {
	TypeURLFieldName string
}

func (p *AnyTypeURLProvider) AnyType(_ UnmarshalContext, body hcl.Body) (protoreflect.FullName, hcl.Body, error) {
	typeURLFieldName := "type"
	if p != nil {
		typeURLFieldName = p.TypeURLFieldName
	}
	// the initial schema we are going to use should just parse out the type url
	schema := hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     typeURLFieldName,
				Required: true,
			},
		},
	}

	// bodyRemains will be all the unparsed attributes/blocks
	typeContent, bodyRemains, diags := body.PartialContent(&schema)
	if diags.HasErrors() {
		return "", nil, diags
	}

	attr, ok := typeContent.Attributes[typeURLFieldName]
	if !ok {
		return "", nil, fmt.Errorf("error decoding Any from HCL: no %q field specified", typeURLFieldName)
	}

	attrVal, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return "", nil, diags
	}

	typeURL, err := stringFromCty(attrVal)
	if err != nil {
		return "", nil, err
	}

	slashIdx := strings.LastIndex(typeURL, "/")
	typeName := typeURL
	// strip all "hostname" parts of the URL path
	if slashIdx > 1 && slashIdx+1 < len(typeURL) {
		typeName = typeURL[slashIdx+1:]
	}

	return protoreflect.FullName(typeName), bodyRemains, nil
}

// wellKnownTypeSchemaHint returns what sort of syntax should be used to represent
// the well known type field.
//
// NotWellKnownType - use attribute block syntax based on other information
// WellKnownAttribute - use attribute syntax
// WellKnownBlock - use block syntax
func wellKnownTypeSchemaHint(desc protoreflect.FieldDescriptor) wktSchemaHint {
	if desc.Kind() != protoreflect.MessageKind {
		return notWellKnownType
	}

	switch desc.Message().FullName() {
	case "google.protobuf.DoubleValue":
		return wellKnownAttribute
	case "google.protobuf.FloatValue":
		return wellKnownAttribute
	case "google.protobuf.Int32Value":
		return wellKnownAttribute
	case "google.protobuf.Int64Value":
		return wellKnownAttribute
	case "google.protobuf.UInt32Value":
		return wellKnownAttribute
	case "google.protobuf.UInt64Value":
		return wellKnownAttribute
	case "google.protobuf.BoolValue":
		return wellKnownAttribute
	case "google.protobuf.StringValue":
		return wellKnownAttribute
	case "google.protobuf.BytesValue":
		return wellKnownAttribute
	case "google.protobuf.Empty":
		// block syntax is used for Empty to allow transitioning to using
		// a different proto message with fields in the future.
		return wellKnownBlock
	case "google.protobuf.Timestamp":
		return wellKnownAttribute
	case "google.protobuf.Duration":
		return wellKnownAttribute
	case "google.protobuf.Struct":
		// as the Struct has completely free form fields that we cannot
		// know about in advance we cannot use block syntax
		return wellKnownAttribute
	case "google.protobuf.Any":
		return wellKnownBlock
	default:
		return notWellKnownType
	}
}

func wellKnownTypeDecode(desc protoreflect.FieldDescriptor, val cty.Value) (bool, protoreflect.Value, error) {
	if desc.Kind() != protoreflect.MessageKind {
		return false, protoreflect.Value{}, nil
	}

	switch desc.Message().FullName() {
	case "google.protobuf.DoubleValue":
		protoVal, err := protoDoubleWrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.FloatValue":
		protoVal, err := protoFloatWrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.Int32Value":
		protoVal, err := protoInt32WrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.Int64Value":
		protoVal, err := protoInt64WrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.UInt32Value":
		protoVal, err := protoUint32WrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.UInt64Value":
		protoVal, err := protoUint64WrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.BoolValue":
		protoVal, err := protoBoolWrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.StringValue":
		protoVal, err := protoStringWrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.BytesValue":
		protoVal, err := protoBytesWrapperFromCty(val)
		return true, protoVal, err
	case "google.protobuf.Empty":
		protoVal, err := protoEmptyFromCty(val)
		return true, protoVal, err
	case "google.protobuf.Timestamp":
		protoVal, err := protoTimestampFromCty(val)
		return true, protoVal, err
	case "google.protobuf.Duration":
		protoVal, err := protoDurationFromCty(val)
		return true, protoVal, err
	case "google.protobuf.Struct":
		protoVal, err := protoStructFromCty(val)
		return true, protoVal, err
	case "google.protobuf.Any":
		return true, protoreflect.Value{}, fmt.Errorf("well-known Any unsupported")
	default:
		return false, protoreflect.Value{}, nil
	}
}

func protoWrapperFromBool(v bool) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.Bool(v).ProtoReflect())
}
func protoWrapperFromInt32(v int32) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.Int32(v).ProtoReflect())
}
func protoWrapperFromInt64(v int64) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.Int64(v).ProtoReflect())
}
func protoWrapperFromUint32(v uint32) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.UInt32(v).ProtoReflect())
}
func protoWrapperFromUint64(v uint64) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.UInt64(v).ProtoReflect())
}
func protoWrapperFromFloat(v float32) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.Float(v).ProtoReflect())
}
func protoWrapperFromDouble(v float64) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.Double(v).ProtoReflect())
}
func protoWrapperFromString(v string) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.String(v).ProtoReflect())
}
func protoWrapperFromBytes(v []byte) protoreflect.Value {
	return protoreflect.ValueOfMessage(wrapperspb.Bytes(v).ProtoReflect())
}

func protoBoolWrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := boolFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromBool(v), nil
}
func protoInt32WrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := int32FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromInt32(v), nil
}
func protoInt64WrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := int64FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromInt64(v), nil
}
func protoUint32WrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := uint32FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromUint32(v), nil
}
func protoUint64WrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := uint64FromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromUint64(v), nil
}
func protoFloatWrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := floatFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromFloat(v), nil
}
func protoDoubleWrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := doubleFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromDouble(v), nil
}
func protoStringWrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := stringFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromString(v), nil
}
func protoBytesWrapperFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := bytesFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}
	return protoWrapperFromBytes(v), nil
}

func protoEmptyFromCty(val cty.Value) (protoreflect.Value, error) {
	var e emptypb.Empty
	if val.IsNull() {
		return protoreflect.Value{}, nil
	}

	valType := val.Type()
	if (valType.IsObjectType() || valType.IsMapType()) && val.LengthInt() == 0 {
		return protoreflect.ValueOfMessage(e.ProtoReflect()), nil
	}

	return protoreflect.Value{}, fmt.Errorf("well known empty type can only be represented as an hcl map/object - actual type %q", valType.FriendlyName())
}

func protoTimestampFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := stringFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}

	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return protoreflect.Value{}, fmt.Errorf("error parsing timestamp: %w", err)
	}

	if t.Before(minTime) || t.After(maxTime) {
		return protoreflect.Value{}, fmt.Errorf("time is out of range %s to %s - %s", minTime.String(), maxTime.String(), v)
	}

	return protoreflect.ValueOfMessage(timestamppb.New(t).ProtoReflect()), nil
}

func protoDurationFromCty(val cty.Value) (protoreflect.Value, error) {
	v, err := stringFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return protoreflect.Value{}, fmt.Errorf("error parsing string duration: %w", err)
	}

	return protoreflect.ValueOfMessage(durationpb.New(d).ProtoReflect()), nil
}

func protoStructFromCty(val cty.Value) (protoreflect.Value, error) {
	s, err := protoStructObjectFromCty(val)
	if err != nil {
		return protoreflect.Value{}, err
	}

	return protoreflect.ValueOfMessage(s.ProtoReflect()), nil
}

func protoStructObjectFromCty(val cty.Value) (*structpb.Struct, error) {
	valType := val.Type()
	if !valType.IsObjectType() && !valType.IsMapType() {
		return nil, fmt.Errorf("Struct type must be either an object or map type")
	}

	structValues := make(map[string]*structpb.Value)

	for k, v := range val.AsValueMap() {
		vType := v.Type()

		if v.IsNull() {
			structValues[k] = structpb.NewNullValue()
		} else if vType.IsListType() || vType.IsSetType() || vType.IsTupleType() {
			listVal, err := protoStructListValueFromCty(v)
			if err != nil {
				return nil, err
			}
			structValues[k] = structpb.NewListValue(listVal)
		} else if vType.IsMapType() || vType.IsObjectType() {
			objVal, err := protoStructObjectFromCty(v)
			if err != nil {
				return nil, err
			}
			structValues[k] = structpb.NewStructValue(objVal)
		} else if vType.IsPrimitiveType() {
			switch vType {
			case cty.String:
				stringVal, err := stringFromCty(v)
				if err != nil {
					return nil, err
				}

				structValues[k] = structpb.NewStringValue(stringVal)
			case cty.Bool:
				boolVal, err := boolFromCty(v)
				if err != nil {
					return nil, err
				}

				structValues[k] = structpb.NewBoolValue(boolVal)
			case cty.Number:
				doubleVal, err := doubleFromCty(v)
				if err != nil {
					return nil, err
				}

				structValues[k] = structpb.NewNumberValue(doubleVal)
			default:
				return nil, fmt.Errorf("unknown cty primitive type: %s", vType.FriendlyName())
			}
		} else {
			return nil, fmt.Errorf("unsupported cty type: %s", vType.FriendlyName())
		}
	}

	return &structpb.Struct{
		Fields: structValues,
	}, nil
}

func protoStructListValueFromCty(val cty.Value) (*structpb.ListValue, error) {
	var values []*structpb.Value

	var err error
	val.ForEachElement(func(_ cty.Value, value cty.Value) bool {
		vType := value.Type()

		if value.IsNull() {
			values = append(values, structpb.NewNullValue())
		} else if vType.IsListType() || vType.IsSetType() || vType.IsTupleType() {
			var listVal *structpb.ListValue
			listVal, err = protoStructListValueFromCty(value)
			if err != nil {
				return true
			}
			values = append(values, structpb.NewListValue(listVal))
		} else if vType.IsMapType() || vType.IsObjectType() {
			var objVal *structpb.Struct
			objVal, err = protoStructObjectFromCty(value)
			if err != nil {
				return true
			}
			values = append(values, structpb.NewStructValue(objVal))
		} else if vType.IsPrimitiveType() {
			switch vType {
			case cty.String:
				var stringVal string
				stringVal, err = stringFromCty(value)
				if err != nil {
					return true
				}

				values = append(values, structpb.NewStringValue(stringVal))
			case cty.Bool:
				var boolVal bool
				boolVal, err = boolFromCty(value)
				if err != nil {
					return true
				}

				values = append(values, structpb.NewBoolValue(boolVal))
			case cty.Number:
				var doubleVal float64
				doubleVal, err = doubleFromCty(value)
				if err != nil {
					return true
				}

				values = append(values, structpb.NewNumberValue(doubleVal))
			default:
				err = fmt.Errorf("unknown cty primitive type: %s", vType.FriendlyName())
				return true
			}
		} else {
			err = fmt.Errorf("unsupported cty type: %s", vType.FriendlyName())
			return true
		}
		return false
	})

	if err != nil {
		return nil, err
	}

	return &structpb.ListValue{
		Values: values,
	}, nil
}

func protoAnyFromCty(val cty.Value) (protoreflect.Value, error) {
	return protoreflect.Value{}, fmt.Errorf("Any type cannot be coded from a single cty Value")
}

func (u UnmarshalOptions) decodeWellKnownAnyFromHCLBody(body hcl.Body, msg protoreflect.Message) error {
	var typeProvider AnyTypeProvider = &AnyTypeURLProvider{TypeURLFieldName: "type"}
	if u.AnyTypeProvider != nil {
		typeProvider = u.AnyTypeProvider
	}

	typeName, bodyRemains, err := typeProvider.AnyType(nil, body)
	if err != nil {
		return fmt.Errorf("error getting type for Any field: %w", err)
	}

	// the type.googleapis.come/ should be optional
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(typeName))
	if err != nil {
		return fmt.Errorf("error looking up type information for %s: %w", typeName, err)
	}

	newMsg := mt.New()

	err = u.decodeHCLBody(bodyRemains, newMsg)
	if err != nil {
		return err
	}

	enc, err := proto.Marshal(newMsg.Interface())
	if err != nil {
		return fmt.Errorf("error marshalling Any data as protobuf value: %w", err)
	}

	anyValue := msg.Interface().(*anypb.Any)

	// This will look like <proto package>.<proto Message name> and not quite like a full URL with a path
	anyValue.TypeUrl = string(newMsg.Descriptor().FullName())
	anyValue.Value = enc

	return nil
}
