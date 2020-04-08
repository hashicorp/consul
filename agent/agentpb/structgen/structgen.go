package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

var (
	// structsMirrorPBFileSuffixes is the list of generated protobuf go file
	// suffixes we care about building adapters for. If we mirror more structs
	// into different .proto files later we can add them here to get code gen to
	// work for them.
	structsMirrorPBFiles = []string{
		"structs.pb.go",
		"common.pb.go",
		// TODO: figure out how to exclude this and include _ent in Enterprise
		// builds when this is a go run script :(
		"common_oss.pb.go",
	}

	// pbTypesIgnore is a list of protobuf structs that are generated but don't
	// have direct equals in the structs package and so need hand-written
	// conversions.
	pbTypesIgnore = []string{
		"HeaderValue",
		"TargetDatacenter",
	}
)

func main() {
	protoStructs, eventStructs, err := findProtoGeneratedStructs()
	if err != nil {
		log.Fatalf("failed to find proto generated structs: %s", err)
	}
	structsTI, err := loadStructsTypeInfo()
	if err != nil {
		log.Fatalf("failed to load type info for structs: %s", err)
	}

	var convertBuf bytes.Buffer
	var testBuf bytes.Buffer

	err = genConvertHeader(&convertBuf)
	if err != nil {
		log.Fatalf("failed to write file header: %s", err)
	}
	err = genTestHeader(&testBuf)
	if err != nil {
		log.Fatalf("failed to write test file header: %s", err)
	}

	for _, desc := range protoStructs {
		structsType := findMatchingStructsType(structsTI, desc.Name)
		if structsType == nil {
			log.Fatalf("failed to find a matching struct def in structs package for %s", desc.Name)
		}
		if structsType.NumFields() != desc.Struct.NumFields() {
			log.Fatalf("structs type %s has %d fields, agentpb has %d", desc.Name,
				structsType.NumFields(), desc.Struct.NumFields())
		}
		err = genConvert(&convertBuf, desc.Name, desc.Struct, structsType)
		if err != nil {
			log.Fatalf("failed to write generate conversions methods for %s header: %s", desc.Name, err)
		}
		err = genTests(&testBuf, desc.Name, desc.Struct, structsType)
		if err != nil {
			log.Fatalf("failed to write generate tests for %s header: %s", desc.Name, err)
		}
	}
	//fmt.Println(convertBuf.String())

	// Dump the files somewhere
	err = writeToFile("./agent/agentpb/structs.structgen.go", convertBuf.Bytes())
	if err != nil {
		log.Fatalf("Failed to write output file: %s", err)
	}
	err = writeToFile("./agent/agentpb/structs.structgen_test.go", testBuf.Bytes())
	if err != nil {
		log.Fatalf("Failed to write test file: %s", err)
	}

	// Build simple file with all defined event types in an array so we can
	// write exhaustive test checks over event types.
	var eventTypesBuf bytes.Buffer
	err = evTypesTpl.Execute(&eventTypesBuf, eventStructs)
	if err != nil {
		log.Fatalf("Failed to generate event types list: %s", err)
	}
	err = writeToFile("./agent/agentpb/event_types.structgen.go", eventTypesBuf.Bytes())
	if err != nil {
		log.Fatalf("Failed to write event types file: %s", err)
	}
}

func writeToFile(name string, code []byte) error {
	// Format it correctly
	fCode, err := format.Source(code)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(fCode)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	return nil
}

func fileMirrorsStructs(name string) bool {
	for _, fName := range structsMirrorPBFiles {
		if name == fName {
			// We care about this file.
			return true
		}
	}
	return false
}

type structDesc struct {
	Name   string
	Struct *types.Struct
}

type structsList []structDesc

func (l structsList) Len() int           { return len(l) }
func (l structsList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l structsList) Less(i, j int) bool { return l[i].Name < l[j].Name }

func findProtoGeneratedStructs() (structsList, structsList, error) {
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedTypesInfo}
	pkgs, err := packages.Load(cfg, "github.com/hashicorp/consul/agent/agentpb")
	if err != nil {
		return nil, nil, err
	}
	pkg := pkgs[0]
	ss := make(structsList, 0)
	evs := make(structsList, 0)

	for ident, obj := range pkg.TypesInfo.Defs {
		// See where this type was defined
		if obj == nil {
			// Apparently this can happen..
			continue
		}

		// Only consider exported types
		if !obj.Exported() {
			continue
		}

		// Only consider types defined in the structs protobuf mirror file, or the
		// stream events.
		p := pkg.Fset.Position(obj.Pos())
		fName := filepath.Base(p.Filename)
		if !fileMirrorsStructs(fName) && fName != "subscribe.pb.go" {
			continue
		}

		// Only struct fields and methods have nil parent and we don't want those
		if obj.Parent() == nil {
			continue
		}

		// Skip some stuff
		if shouldIgnoreType(ident.Name) {
			continue
		}

		// Append to list of mirrored structs, unless this is subscribe.pb.go where
		// we just need the Event payload types.
		collect := func(fName string, id *ast.Ident, t *types.Struct) {
			if fName == "subscribe.pb.go" {
				if strings.HasPrefix(id.Name, "Event_") {
					evs = append(evs, structDesc{id.Name, nil})
				}
			} else {
				ss = append(ss, structDesc{id.Name, t})
			}
		}

		// See if it's a struct type
		switch tt := obj.Type().(type) {
		case *types.Struct:
			collect(fName, ident, tt)
		case *types.Named:
			switch st := tt.Underlying().(type) {
			case *types.Struct:
				collect(fName, ident, st)
			default:
				continue
			}
		default:
			continue
		}
	}

	// Sort them to keep the generated file deterministic
	sort.Sort(ss)
	sort.Sort(evs)

	return ss, evs, nil
}

func shouldIgnoreType(name string) bool {
	for _, ignore := range pbTypesIgnore {
		if ignore == name {
			return true
		}
	}
	return false
}

func loadStructsTypeInfo() (*types.Info, error) {
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedTypesInfo}
	pkgs, err := packages.Load(cfg, "github.com/hashicorp/consul/agent/structs")
	if err != nil {
		return nil, err
	}
	return pkgs[0].TypesInfo, nil
}

func findMatchingStructsType(structsTI *types.Info, name string) *types.Struct {
	for _, t := range structsTI.Defs {
		if t == nil {
			continue
		}
		// Skip struct fields as they can cause false positives
		if t.Parent() == nil {
			continue
		}
		if t.Name() == name {
			if tn, ok := t.Type().(*types.Named); ok {
				if st, ok := tn.Underlying().(*types.Struct); ok {
					return st
				}
			}
			// There are valid collisions e.g. a struct named CheckType and a method
			// named CheckType on HealthCheck struct. So just continue.
		}
	}
	return nil
}

func genConvertHeader(w io.Writer) error {
	header := `// Code generated by agentpb/structgen. DO NOT EDIT.

package agentpb

import (
	"github.com/hashicorp/consul/agent/structs"
)

`
	_, err := w.Write([]byte(header))
	return err
}

type tplData struct {
	Name   string
	Fields []string
}

var toStructsTpl = template.Must(template.New("ToStructs").Parse(`

// ToStructs converts the protobuf type to the original structs package type.
func (p *{{ .Name }}) ToStructs() (*structs.{{ .Name }}, error) {
	if p == nil {
		return nil, nil
	}
	s := structs.{{ .Name }}{}
	{{- range .Fields -}}
		{{- . -}}
	{{- end }}
	return &s, nil
}

{{- define "assign" }}
	s.{{ .Name }} = {{ .CastToIfNotSameType }}{{ .RefOp true }}p.{{ .Name }}{{ .EndCast }}
{{- end -}}

{{ define "struct" }}
	tmp{{ .Name }}, err := p.{{ .Name }}.ToStructs()
	if err != nil {
		return nil, err
	}
	if tmp{{ .Name }} != nil {
		s.{{ .Name }} = {{ .RefOpForced true true }}tmp{{ .Name }}
	}
{{- end -}}

{{ define "slice-assign" }}
	if p.{{ .Name }} != nil {
		s.{{ .Name }} = make({{ .StructsTypeInfo.Type }}, len(p.{{ .Name }}))
		for i, e := range p.{{ .Name }} {
			s.{{ .Name }}[i] = {{ .ElemRefOp true }}e
		}
	}
{{- end -}}

{{ define "slice-struct" }}
	if p.{{ .Name }} != nil {
		s.{{ .Name }} = make({{ .StructsTypeInfo.Type }}, 0, len(p.{{ .Name }}))
		for _, e := range p.{{ .Name }} {
			tmp, err := e.ToStructs()
			if err != nil {
				return nil, err
			}
			if tmp != nil {
				s.{{ .Name }} = append(s.{{ .Name }}, {{ .ElemRefOpForced true true }}tmp)
			}
		}
	}
{{- end -}}

{{ define "map-assign" }}
	if p.{{ .Name }} != nil {
		s.{{ .Name }} = make({{ .StructsTypeInfo.Type }}, len(p.{{ .Name }}))
		for i, e := range p.{{ .Name }} {
			s.{{ .Name }}[i] = {{ .ElemRefOp true }}e
		}
	}
{{- end -}}

{{ define "map-struct" }}
	if p.{{ .Name }} != nil {
		s.{{ .Name }} = make({{ .StructsTypeInfo.Type }}, len(p.{{ .Name }}))
		for k, v := range p.{{ .Name }} {
			tmp, err := v.ToStructs()
			if err != nil {
				return nil, err
			}
			{{- if .ElemTypeInfo.ForceDeref }}
			s.{{ .Name }}[k] = {{ .ElemRefOpForced true true }}tmp
			{{ else }}
			s.{{ .Name }}[k] = {{ .ElemRefOp true }}tmp
			{{- end }}
		}
	}
{{- end -}}

{{ define "proto.Struct" }}
	s.{{ .Name }} = MapFromPBStruct(p.{{ .Name }})
{{- end -}}

`))

func genConvert(w io.Writer, name string, s, structsType *types.Struct) error {
	toFields := make([]string, 0, s.NumFields())
	fromFields := make([]string, 0, s.NumFields())

	var buf bytes.Buffer

	// Render each field according to type
	err := walkStructFields(s, func(i int, fld *types.Var) error {
		ti := analyzeFieldType(fld)

		// Find the equivalent field in the structs package version
		structsTI := analyzeFieldType(structsType.Field(i))
		ti.StructsTypeInfo = &structsTI

		if strings.HasSuffix(ti.Type, "invalid type") {
			return fmt.Errorf("protobuf field %s.%s has invalid type", name, ti.Name)
		}
		if strings.HasSuffix(structsTI.Type, "invalid type") {
			return fmt.Errorf("structs field %s.%s has invalid type", name, structsTI.Name)
		}

		buf.Reset()
		err := toStructsTpl.ExecuteTemplate(&buf, ti.Template, ti)
		if err != nil {
			return err
		}
		toFields = append(toFields, buf.String())

		buf.Reset()
		err = fromStructsTpl.ExecuteTemplate(&buf, ti.Template, ti)
		if err != nil {
			return err
		}
		fromFields = append(fromFields, buf.String())
		return nil
	})
	if err != nil {
		return err
	}

	toData := tplData{
		Name:   name,
		Fields: toFields,
	}
	err = toStructsTpl.Execute(w, toData)
	if err != nil {
		return err
	}
	fromData := tplData{
		Name:   name,
		Fields: fromFields,
	}
	err = fromStructsTpl.Execute(w, fromData)
	if err != nil {
		return err
	}
	return nil
}

var fromStructsTpl = template.Must(template.New("ToStructs").Parse(`

// FromStructs populates the protobuf type from an original structs package type.
func (p *{{ .Name }}) FromStructs(s *structs.{{ .Name }}) error {
	if s == nil {
		return nil
	}
	{{- range .Fields }}
		{{- . -}}
	{{- end }}
	return nil
}

{{- define "assign" }}
	p.{{ .Name }} = {{ .CastFromIfNotSameType }}{{ .RefOp false }}s.{{ .Name }}{{ .EndCast }}
{{- end -}}

{{ define "struct" }}
	{{ if .StructsTypeInfo.IsPtr }}
	if s.{{ .Name }} != nil {
	{{ end }}
		var tmp{{ .Name }} {{ .Type }}
		if err := tmp{{ .Name }}.FromStructs({{ .StructsTypeAsPtrOp }}s.{{ .Name }}); err != nil {
			return err
		}
		p.{{ .Name }} = {{ .RefOpForced false false }}tmp{{ .Name }}
	{{ if .StructsTypeInfo.IsPtr -}}
	}
	{{- end }}
{{- end -}}

{{ define "slice-assign" }}
	if s.{{ .Name }} != nil {
		p.{{ .Name }} = make({{ .Type }}, len(s.{{ .Name }}))
		for i, e := range s.{{ .Name }} {
			p.{{ .Name }}[i] = {{ .ElemRefOp false }}e
		}
	}
{{- end -}}

{{ define "slice-struct" }}
	if s.{{ .Name }} != nil {
		p.{{ .Name }} = make({{ .Type }}, 0, len(s.{{ .Name }}))
		for _, e := range s.{{ .Name }} {
			{{ if .StructsTypeInfo.ElemTypeInfo.IsPtr -}}
			if e == nil {
				continue
			}
			{{- end }}
			var tmp {{ .ElemTypeInfo.Type }}
			if err := tmp.FromStructs({{ .StructsElemTypeAsPtrOp }}e); err != nil {
				return err
			}
			p.{{ .Name }} = append(p.{{ .Name }}, {{ .ElemRefOpForced false false }}tmp)
		}
	}
{{- end -}}

{{ define "map-assign" }}
	if s.{{ .Name }} != nil {
		p.{{ .Name }} = make({{ .Type }}, len(s.{{ .Name }}))
		for i, e := range s.{{ .Name }} {
			p.{{ .Name }}[i] = {{ .ElemRefOp false }}e
		}
	}
{{- end -}}

{{ define "map-struct" }}
	if s.{{ .Name }} != nil {
		p.{{ .Name }} = make({{ .Type }}, len(s.{{ .Name }}))
		for k, v := range s.{{ .Name }} {
			var tmp {{ .ElemTypeInfo.Type }}
			if err := tmp.FromStructs({{ .StructsElemTypeAsPtrOp }}v); err != nil {
				return err
			}
			{{- if .ElemTypeInfo.ForceDeref }}
			p.{{ .Name }}[k] = {{ .ElemRefOpForced false false }}tmp
			{{ else }}
			p.{{ .Name }}[k] = {{ .ElemRefOp false }}tmp
			{{- end }}
		}
	}
{{- end -}}

{{ define "proto.Struct" }}
	p.{{ .Name }} = MapToPBStruct(s.{{ .Name }})
{{- end -}}
`))

func walkStructFields(s *types.Struct, f func(i int, field *types.Var) error) error {
	for i := 0; i < s.NumFields(); i++ {
		fld := s.Field(i)
		if !fld.Exported() {
			continue
		}
		if err := f(i, fld); err != nil {
			return err
		}
	}
	return nil
}

func genTestHeader(w io.Writer) error {
	header := `// Code generated by agentpb/structgen. DO NOT EDIT.

package agentpb

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func testFuzzer() *fuzz.Fuzzer {
	f := fuzz.New()
	f.NilChance(0)
	f.NumElements(1, 3)

	// Populate map[string]interface{} since gofuzz panics on these. We force them
	// to be interface{} rather than concrete types otherwise they won't compare
	// equal when coming back out the other side.
	f.Funcs(func(m map[string]interface{}, c fuzz.Continue) {
		// Populate it with some random stuff of different types
		// Int -> Float since trip through protobuf.Value will force this.
		m[c.RandString()] = interface{}(float64(c.RandUint64()))
		m[c.RandString()] = interface{}(c.RandString())
		m[c.RandString()] = interface{}([]interface{}{c.RandString(), c.RandString()})
		m[c.RandString()] = interface{}(map[string]interface{}{c.RandString(): c.RandString()})
	},
	func(i *int, c fuzz.Continue) {
		// Potentially controversial but all of the int values we care about
		// instructs are expected to be lower than 32 bits - if they weren't then
		// we'd use (u)int64 and would already be breaking 32-bit compat. So we
		// explicitly call those int32 in protobuf. But gofuzz will happily assign
		// them values out of range of an in32 so we need to restrict it or the trip
		// through PB truncates them and fails the tests.
		*i = int(int32(c.RandUint64()))
	},
	func(i *uint, c fuzz.Continue) {
		// See above
		*i = uint(uint32(c.RandUint64()))
	},
	func(v *structs.CheckTypes, c fuzz.Continue) {
		// For some reason gofuzz keeps populating structs.CheckTypes arrays with
		// nils even though NilChance is zero. It's probably a bug but I don't
		// have time to figure that out with a minimal repro and report it right
		// now. Just work around it.
		*v = make(structs.CheckTypes, 2)
		for i := range *v {
			ct := structs.CheckType{}
			c.Fuzz(&ct)
			(*v)[i] = &ct
		}
	})
	return f
}

`
	_, err := w.Write([]byte(header))
	return err
}

var testTpl = template.Must(template.New("test").Parse(`
func Test{{ .Name }}StructsConvert( t *testing.T) {
	// Create a "full" version of the structs package version. This should mean
	// any fields added to structs package but not mirrored in agentpb will cause
	// this test to fail.
	f := testFuzzer()
	var s structs.{{ .Name }}

	for i := 0; i < 10; i++ {
		f.Fuzz(&s)

		// Convert to protobuf and back
		var p {{ .Name }}
		require.NoError(t, p.FromStructs(&s))
		got, err := p.ToStructs()
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, s, *got)
	}
}

`))

func genTests(w io.Writer, name string, s, structsType *types.Struct) error {
	data := tplData{
		Name: name,
	}
	return testTpl.Execute(w, data)
}

type fieldTypeInfo struct {
	Name            string
	Type            string
	Template        string
	IsPtr           bool
	ForceDeref      bool
	ElemTypeInfo    *fieldTypeInfo
	StructsTypeInfo *fieldTypeInfo
}

// RefOp return the "*" or "&"" (de)reference operator if one is needed to
// assign the type. Left hand type is the structs package field type if
// toStructs is true and the agentpb field type if false. If forceRHSPtr is true
// then we assume that whatever the underlying field types are the right hand
// side is currently a pointer - e.g. just obtained from calling ToStructs which
// always returns a pointer.
func (i fieldTypeInfo) RefOp(toStructs bool) string {
	lhsPtr := i.IsPtr
	rhsPtr := i.StructsTypeInfo.IsPtr
	if toStructs {
		lhsPtr = i.StructsTypeInfo.IsPtr
		rhsPtr = i.IsPtr
	}
	return i.refOp(lhsPtr, rhsPtr)
}

// RefOpForced is like RefOp except that the rhs is forced to be a pointer or
// not. It's needed when converting from the result of a ToStructs of
// FromStructs call which will always result in a pointer or a non-pointer in
// general and we need to correct it for whtever the assignment needs regardless
// of the type in the original source field.
func (i fieldTypeInfo) RefOpForced(toStructs bool, rhsPtr bool) string {
	lhsPtr := i.IsPtr
	if toStructs {
		lhsPtr = i.StructsTypeInfo.IsPtr
	}
	return i.refOp(lhsPtr, rhsPtr)
}

// ElemRefOp is like RefOp but for the container's element types.
func (i fieldTypeInfo) ElemRefOp(toStructs bool) string {
	lhsPtr := i.ElemTypeInfo.IsPtr
	rhsPtr := i.StructsTypeInfo.ElemTypeInfo.IsPtr
	if toStructs {
		lhsPtr = i.StructsTypeInfo.ElemTypeInfo.IsPtr
		rhsPtr = i.ElemTypeInfo.IsPtr
	}
	return i.refOp(lhsPtr, rhsPtr)
}

// ElemRefOpForced is like RefOpForced but for the container's element types.
func (i fieldTypeInfo) ElemRefOpForced(toStructs bool, rhsPtr bool) string {
	lhsPtr := i.ElemTypeInfo.IsPtr
	if toStructs {
		lhsPtr = i.StructsTypeInfo.ElemTypeInfo.IsPtr
	}
	return i.refOp(lhsPtr, rhsPtr)
}

func (i fieldTypeInfo) StructsTypeAsPtrOp() string {
	if i.StructsTypeInfo.IsPtr {
		return ""
	}
	// Don't need to take a reference to reference types.
	if reTypePrefix.MatchString(i.StructsTypeInfo.Type) {
		return ""
	}
	return "&"
}

func (i fieldTypeInfo) StructsElemTypeAsPtrOp() string {
	if i.StructsTypeInfo.ElemTypeInfo.IsPtr {
		return ""
	}
	// Don't need to take a reference to reference types.
	if reTypePrefix.MatchString(i.StructsTypeInfo.ElemTypeInfo.Type) {
		return ""
	}
	return "&"
}

func (i fieldTypeInfo) refOp(lhsPtr, rhsPtr bool) string {
	// Both the same no operator needed
	if lhsPtr == rhsPtr {
		return ""
	}

	if lhsPtr {
		// rhs must not be a ptr, so reference it
		return "&"
	}

	// rhsSide must be a pointer but lhs isn't so deref it
	return "*"
}

func (i fieldTypeInfo) CastToIfNotSameType() string {
	if i.Type != i.StructsTypeInfo.Type {
		return i.StructsTypeInfo.Type + "("
	}
	return ""
}
func (i fieldTypeInfo) EndCast() string {
	if i.Type != i.StructsTypeInfo.Type {
		return ")"
	}
	return ""
}

func (i fieldTypeInfo) CastFromIfNotSameType() string {
	if i.Type != i.StructsTypeInfo.Type {
		return i.Type + "("
	}
	return ""
}

func analyzeFieldType(f *types.Var) fieldTypeInfo {
	ti := fieldTypeInfoForType(f.Type())
	// Override the name with the _field's_ name not it's type name
	ti.Name = f.Name()
	return ti
}

var reTypePrefix = regexp.MustCompile(`^(map)?\[[^\]]*\]\*?`)

func fieldTypeInfoForType(t types.Type) fieldTypeInfo {
	// Find the first period in the fully qualified type name and strip the
	// prefix.
	typeName := t.String()
	pos := strings.LastIndexByte(typeName, '/')
	if pos > -1 {
		typeName = typeName[pos+1:]
	}
	typeName = strings.TrimPrefix(typeName, "agentpb.")

	// Match any pointer/slice/map prefixes we may have removed and put them back
	// if needed.
	prefix := reTypePrefix.FindString(t.String())
	if !strings.HasPrefix(typeName, prefix) {
		typeName = prefix + typeName
	}

	ti := fieldTypeInfo{
		Type:     typeName,
		Template: "assign",
		// Generally all generated ToStructs methods return a ptr so need to be
		// coerced as a ptr.
		ForceDeref: true,
	}
	fType := t
	if ptrType, ok := fType.(*types.Pointer); ok {
		fType = ptrType.Elem()
		ti.IsPtr = true
	}

	// Special case protobuf-defined types since we have no parallel in structs
	// package and these need custom helpers to convert them.
	if strings.HasSuffix(fType.String(), "protobuf/types.Struct") {
		ti.Template = "proto.Struct"
		return ti
	}

	// Special case our HeaderValue type as a slice element since it maps to a
	// []string in structs package and so needs slightly different treatment(i.e.
	// not dereferencing the slice returned when assigning). But don't mangle
	// map[string]... which also has the type suffix - only apply this to the
	// recursive call for the map elem.
	if fType.String() == "github.com/hashicorp/consul/agent/agentpb.HeaderValue" {
		// Use same pattern as for structs but don't force deref
		ti.Template = "struct"
		ti.ForceDeref = false
		return ti
	}

	switch oType := fType.(type) {
	case *types.Struct:
		ti.Template = "struct"
	case *types.Slice:
		eti := fieldTypeInfoForType(oType.Elem())
		ti.ElemTypeInfo = &eti
		ti.Template = "slice-" + eti.Template
	case *types.Map:
		eti := fieldTypeInfoForType(oType.Elem())
		ti.ElemTypeInfo = &eti
		ti.Template = "map-" + eti.Template
	case *types.Named:
		uType := fType.Underlying()
		switch nType := uType.(type) {
		case *types.Struct:
			ti.Template = "struct"
		case *types.Slice:
			eti := fieldTypeInfoForType(nType.Elem())
			ti.ElemTypeInfo = &eti
			ti.Template = "slice-" + eti.Template
		case *types.Map:
			eti := fieldTypeInfoForType(nType.Elem())
			ti.ElemTypeInfo = &eti
			ti.Template = "map-" + eti.Template
		}
	}
	return ti
}

var evTypesTpl = template.Must(template.New("test").Parse(`// Code generated by agentpb/structgen. DO NOT EDIT.

package agentpb

// allEventTypes is used internally in tests or places we need an exhaustive
// list of Event Payload types. We use this in tests to ensure that we don't
// miss defining something for a new test type when adding new ones. If we ever
// need to machine-genereate a human-readable list of event type strings for
// something we could easily do that here too.
var allEventTypes []isEvent_Payload

func init() {
	allEventTypes = []isEvent_Payload{
		{{ range . -}}
			&{{ .Name }}{},
		{{ end }}
	}
}
`))
