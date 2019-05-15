package bexpr

import (
	"fmt"
	"reflect"
	"strings"
)

// Function type for usage with a SelectorConfiguration
type FieldValueCoercionFn func(value string) (interface{}, error)

// Strongly typed name of a field
type FieldName string

// Used to represent an arbitrary field name
const FieldNameAny FieldName = ""

// The FieldConfiguration struct represents how boolean expression
// validation and preparation should work for the given field. A field
// in this case is a single element of a selector.
//
// Example: foo.bar.baz has 3 fields separate by '.' characters.
type FieldConfiguration struct {
	// Name to use when looking up fields within a struct. This is useful when
	// the name(s) you want to expose to users writing the expressions does not
	// exactly match the Field name of the structure. If this is empty then the
	// user provided name will be used
	StructFieldName string

	// Nested field configurations
	SubFields FieldConfigurations

	// Function to run on the raw string value present in the expression
	// syntax to coerce into whatever form the MatchExpressionEvaluator wants
	// The coercion happens only once and will then be passed as the `value`
	// parameter to all EvaluateMatch invocations on the MatchExpressionEvaluator.
	CoerceFn FieldValueCoercionFn

	// List of MatchOperators supported for this field. This configuration
	// is used to pre-validate an expressions fields before execution.
	SupportedOperations []MatchOperator
}

// Represents all the valid fields and their corresponding configuration
type FieldConfigurations map[FieldName]*FieldConfiguration

func generateFieldConfigurationInterface(rtype reflect.Type) (FieldConfigurations, bool) {
	// Handle those types that implement our interface
	if rtype.Implements(reflect.TypeOf((*MatchExpressionEvaluator)(nil)).Elem()) {
		// TODO (mkeeler) Do we need to new a value just to call the function? Potentially we can
		// lookup the func and invoke it with a nil pointer?
		value := reflect.New(rtype)
		// have to take the Elem() of the new value because New gives us a ptr to the type that
		// we checked if it implements the interface
		configs := value.Elem().Interface().(MatchExpressionEvaluator).FieldConfigurations()
		return configs, true
	}

	return nil, false
}

func generateFieldConfigurationInternal(rtype reflect.Type) (*FieldConfiguration, error) {
	if fields, ok := generateFieldConfigurationInterface(rtype); ok {
		return &FieldConfiguration{
			SubFields: fields,
		}, nil
	}

	// must be done after checking for interface implementing
	rtype = derefType(rtype)

	// Handle primitive types
	if coerceFn, ok := primitiveCoercionFns[rtype.Kind()]; ok {
		return &FieldConfiguration{
			CoerceFn:            coerceFn,
			SupportedOperations: []MatchOperator{MatchEqual, MatchNotEqual},
		}, nil
	}

	// Handle compound types
	switch rtype.Kind() {
	case reflect.Map:
		return generateMapFieldConfiguration(derefType(rtype.Key()), rtype.Elem())
	case reflect.Array, reflect.Slice:
		return generateSliceFieldConfiguration(rtype.Elem())
	case reflect.Struct:
		subfields, err := generateStructFieldConfigurations(rtype)
		if err != nil {
			return nil, err
		}

		return &FieldConfiguration{
			SubFields: subfields,
		}, nil

	default: // unsupported types are just not filterable
		return nil, nil
	}
}

func generateSliceFieldConfiguration(elemType reflect.Type) (*FieldConfiguration, error) {
	if coerceFn, ok := primitiveCoercionFns[elemType.Kind()]; ok {
		// slices of primitives have somewhat different supported operations
		return &FieldConfiguration{
			CoerceFn:            coerceFn,
			SupportedOperations: []MatchOperator{MatchIn, MatchNotIn, MatchIsEmpty, MatchIsNotEmpty},
		}, nil
	}

	subfield, err := generateFieldConfigurationInternal(elemType)
	if err != nil {
		return nil, err
	}

	cfg := &FieldConfiguration{
		SupportedOperations: []MatchOperator{MatchIsEmpty, MatchIsNotEmpty},
	}

	if subfield != nil && len(subfield.SubFields) > 0 {
		cfg.SubFields = subfield.SubFields
	}

	return cfg, nil
}

func generateMapFieldConfiguration(keyType, valueType reflect.Type) (*FieldConfiguration, error) {
	switch keyType.Kind() {
	case reflect.String:
		subfield, err := generateFieldConfigurationInternal(valueType)
		if err != nil {
			return nil, err
		}

		cfg := &FieldConfiguration{
			CoerceFn:            CoerceString,
			SupportedOperations: []MatchOperator{MatchIsEmpty, MatchIsNotEmpty, MatchIn, MatchNotIn},
		}

		if subfield != nil {
			cfg.SubFields = FieldConfigurations{
				FieldNameAny: subfield,
			}
		}

		return cfg, nil

	default:
		// For maps with non-string keys we can really only do emptiness checks
		// and cannot index into them at all
		return &FieldConfiguration{
			SupportedOperations: []MatchOperator{MatchIsEmpty, MatchIsNotEmpty},
		}, nil
	}
}

func generateStructFieldConfigurations(rtype reflect.Type) (FieldConfigurations, error) {
	fieldConfigs := make(FieldConfigurations)

	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)

		fieldTag := field.Tag.Get("bexpr")

		var fieldNames []string

		if field.PkgPath != "" {
			// we cant handle unexported fields using reflection
			continue
		}

		if fieldTag != "" {
			parts := strings.Split(fieldTag, ",")

			if len(parts) > 0 {
				if parts[0] == "-" {
					continue
				}

				fieldNames = parts
			} else {
				fieldNames = append(fieldNames, field.Name)
			}
		} else {
			fieldNames = append(fieldNames, field.Name)
		}

		cfg, err := generateFieldConfigurationInternal(field.Type)
		if err != nil {
			return nil, err
		}
		cfg.StructFieldName = field.Name

		// link the config to all the correct names
		for _, name := range fieldNames {
			fieldConfigs[FieldName(name)] = cfg
		}
	}

	return fieldConfigs, nil
}

// `generateFieldConfigurations` can be used to generate the `FieldConfigurations` map
// It supports generating configurations for either a `map[string]*` or a `struct` as the `topLevelType`
//
// Internally within the top level type the following is supported:
//
// Primitive Types:
//    strings
//    integers (all width types and signedness)
//    floats (32 and 64 bit)
//    bool
//
// Compound Types
//   `map[*]*`
//       - Supports emptiness checking. Does not support further selector nesting.
//   `map[string]*`
//       - Supports in/contains operations on the keys.
//   `map[string]<supported type>`
//       - Will have a single subfield with name `FieldNameAny` (wildcard) and the rest of
//         the field configuration will come from the `<supported type>`
//   `[]*`
//       - Supports emptiness checking only. Does not support further selector nesting.
//   `[]<supported primitive type>`
//       - Supports in/contains operations against the primitive values.
//   `[]<supported compund type>`
//       - Will have subfields with the configuration of whatever the supported
//         compound type is.
//       - Does not support indexing of individual values like a map does currently
//         and with the current evaluation logic slices of slices will mostly be
//         handled as if they were flattened. One thing that cannot be done is
//         to be able to perform emptiness/contains checking against the internal
//         slice.
//   structs
//       - No operations are supported on the struct itself
//       - Will have subfield configurations generated for the fields of the struct.
//       - A struct tag like `bexpr:"<name>"` allows changing the name that allows indexing
//         into the subfield.
//       - By default unexported fields of a struct are not selectable. If The struct tag is
//         present then this behavior is overridden.
//       - Exported fields can be made unselectable by adding a tag to the field like `bexpr:"-"`
func GenerateFieldConfigurations(topLevelType interface{}) (FieldConfigurations, error) {
	return generateFieldConfigurations(reflect.TypeOf(topLevelType))
}

func generateFieldConfigurations(rtype reflect.Type) (FieldConfigurations, error) {
	if fields, ok := generateFieldConfigurationInterface(rtype); ok {
		return fields, nil
	}

	// Do this after we check for interface implementation
	rtype = derefType(rtype)

	switch rtype.Kind() {
	case reflect.Struct:
		fields, err := generateStructFieldConfigurations(rtype)
		return fields, err
	case reflect.Map:
		if rtype.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("Cannot generate FieldConfigurations for maps with keys that are not strings")
		}

		elemType := rtype.Elem()

		field, err := generateFieldConfigurationInternal(elemType)
		if err != nil {
			return nil, err
		}

		if field == nil {
			return nil, nil
		}

		return FieldConfigurations{
			FieldNameAny: field,
		}, nil
	}

	return nil, fmt.Errorf("Invalid top level type - can only use structs, map[string]* or an MatchExpressionEvaluator")
}

func (config *FieldConfiguration) stringInternal(builder *strings.Builder, level int, path string) {
	fmt.Fprintf(builder, "%sPath: %s, StructFieldName: %s, CoerceFn: %p, SupportedOperations: %v\n", strings.Repeat("   ", level), path, config.StructFieldName, config.CoerceFn, config.SupportedOperations)
	if len(config.SubFields) > 0 {
		config.SubFields.stringInternal(builder, level+1, path)
	}
}

func (config *FieldConfiguration) String() string {
	var builder strings.Builder
	config.stringInternal(&builder, 0, "")
	return builder.String()
}

func (configs FieldConfigurations) stringInternal(builder *strings.Builder, level int, path string) {
	for fieldName, cfg := range configs {
		newPath := string(fieldName)
		if level > 0 {
			newPath = fmt.Sprintf("%s.%s", path, fieldName)
		}
		cfg.stringInternal(builder, level, newPath)
	}
}

func (configs FieldConfigurations) String() string {
	var builder strings.Builder
	configs.stringInternal(&builder, 0, "")
	return builder.String()
}
