package agent

// This file contains tests for JSON unmarshaling.
// These tests were originally written as regression tests to capture existing decoding behavior
// when we moved from mapstructure to encoding/json as a JSON decoder.
// See https://github.com/hashicorp/consul/pull/6624.
//
// Most likely, if you are adding new tests, you will only need to check your struct
// for the special values in 'translateValueTestCases' (time.Durations, etc).
// You can easily copy the structure of an existing test such as
// 'TestDecodeACLPolicyWrite'.
//
// There are two main categories of tests in this file:
//
// 1. translateValueTestCase: test decoding of special values such as:
//    - time.Duration
//    - api.ReadableDuration
//    - time.Time
//    - Hash []byte
//
// 2. translateKeyTestCase: test decoding with alias keys such as "FooBar" => "foo_bar" (see lib.TranslateKeys)
//   For these test cases, one must write an 'equalityFn' which takes an output interface{} (struct, usually)
//   as well as 'want' interface{} value, and returns an error if the test
//   condition failed, or nil if it passed.
//
// There are some test cases which are easily generalizable, and have been pulled
// out of the scope of a single test so that many tests may use them.
// These include the durationTestCases, hashTestCases etc (value) as well as
// some common field alias translations, such as translateScriptArgsTCs for
// CheckTypes.
//

import (
	"bytes"
	"fmt"

	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

// =======================================================
// TranslateValues:
// =======================================================
type translateValueTestCase struct {
	desc       string
	timestamps *timestampTC
	durations  *durationTC
	hashes     *hashTC
	wantErr    bool
}

type timestampTC struct {
	in   string
	want time.Time
}
type durationTC struct {
	in   string
	want time.Duration
}
type hashTC struct {
	in   string
	want []byte
}

var durationTestCases = append(positiveDurationTCs, negativeDurationTCs...)

var translateValueTestCases = append(append(
	timestampTestCases,
	durationTestCases...),
	hashTestCases...)

var hashTestCases = []translateValueTestCase{
	{
		desc: "hashes base64 encoded",
		hashes: &hashTC{
			in:   `"c29tZXRoaW5nIHdpY2tlZCB0aGlzIHdheSBjb21lcw=="`,
			want: []byte("c29tZXRoaW5nIHdpY2tlZCB0aGlzIHdheSBjb21lcw=="),
		},
	},
	{
		desc: "hashes not-base64 encoded",
		hashes: &hashTC{
			in:   `"something wicked this way comes"`,
			want: []byte("something wicked this way comes"),
		},
	},
	{
		desc: "hashes empty string",
		hashes: &hashTC{
			in:   `""`,
			want: []byte{},
		},
	},
	{
		desc: "hashes null",
		hashes: &hashTC{
			in:   `null`,
			want: []byte{},
		},
	},
	{
		desc: "hashes numeric value",
		hashes: &hashTC{
			in: `100`,
		},
		wantErr: true,
	},
}

var timestampTestCases = []translateValueTestCase{
	{
		desc: "timestamps correctly RFC3339 formatted",
		timestamps: &timestampTC{
			in:   `"2020-01-02T15:04:05Z"`,
			want: time.Date(2020, 01, 02, 15, 4, 5, 0, time.UTC),
		},
	},
	{
		desc: "timestamps incorrectly formatted (RFC822)",
		timestamps: &timestampTC{
			in: `"02 Jan 21 15:04"`,
		},
		wantErr: true,
	},
	{
		desc: "timestamps incorrectly formatted (RFC850)",
		timestamps: &timestampTC{
			in: `"Monday, 02-Jan-20 15:04:05"`,
		},
		wantErr: true,
	},
	{
		desc: "timestamps empty string",
		timestamps: &timestampTC{
			in: `""`,
		},
		wantErr: true,
	},
	{
		desc: "timestamps null",
		timestamps: &timestampTC{
			in:   `null`,
			want: time.Time{},
		},
	},
}

var positiveDurationTCs = []translateValueTestCase{
	{
		desc: "durations correctly formatted",
		durations: &durationTC{
			in:   `"2h0m15s"`,
			want: (2*time.Hour + 15*time.Second),
		},
	},
	{
		desc: "durations small, correctly formatted",
		durations: &durationTC{
			in:   `"50ms"`,
			want: (50 * time.Millisecond),
		},
	},
	{
		desc: "durations incorrectly formatted",
		durations: &durationTC{
			in: `"x2h0m0s"`,
		},
		wantErr: true,
	},
	{
		desc: "durations empty string",
		durations: &durationTC{
			in: `""`,
		},
		wantErr: true,
	},
	{
		desc: "durations string without quotes",
		durations: &durationTC{
			in: `2h5m`,
		},
		wantErr: true,
	},
	{
		desc: "durations numeric",
		durations: &durationTC{
			in:   `2000`,
			want: time.Duration(2000),
		},
	},
}

// Separate these negative value test cases out from others b/c some
// cases do not handle negative values correctly. This way some tests
// can write their own testCases for negative values.
var negativeDurationTCs = []translateValueTestCase{
	{
		desc: "durations negative",
		durations: &durationTC{
			in:   `"-50ms"`,
			want: -50 * time.Millisecond,
		},
	},

	{
		desc: "durations numeric and negative",
		durations: &durationTC{
			in:   `-2000`,
			want: time.Duration(-2000),
		},
	},
}

var checkTypeHeaderTestCases = []struct {
	desc    string
	in      string
	want    map[string][]string
	wantErr bool
}{
	{
		desc: "filled in map",
		in:   `{"a": ["aa", "aaa"], "b": ["bb", "bbb", "bbbb"], "c": [], "d": ["dd"]}`,
		want: map[string][]string{
			"a": {"aa", "aaa"},
			"b": {"bb", "bbb", "bbbb"},
			"d": {"dd"},
		},
	},
	{
		desc: "empty map",
		in:   `{}`,
		want: map[string][]string{},
	},
	{
		desc: "empty map",
		in:   `null`,
		want: map[string][]string{},
	},
	{
		desc:    "malformatted map",
		in:      `{"a": "aa"}`,
		wantErr: true,
	},
	{
		desc:    "not a map (slice)",
		in:      `["a", "b"]`,
		wantErr: true,
	},
	{
		desc:    "not a map (int)",
		in:      `1`,
		wantErr: true,
	},
}

// =======================================================
// TranslateKeys:
// =======================================================
type translateKeyTestCase struct {
	jsonFmtStr string
	desc       string
	in         []interface{}
	want       interface{}
	equalityFn func(outStruct, wantVal interface{}) error
}

// FixupCheckType's Translate Keys:
// 	lib.TranslateKeys(rawMap, map[string]string{
// 	"args":                              "ScriptArgs",
// 	"script_args":                       "ScriptArgs",
// 	"deregister_critical_service_after": "DeregisterCriticalServiceAfter",
// 	"docker_container_id":               "DockerContainerID",
// 	"tls_server_name":                   "TLSServerName",
// 	"tls_skip_verify":                   "TLSSkipVerify",
// 	"service_id":                        "ServiceID",

var translateCheckTypeTCs = [][]translateKeyTestCase{
	translateScriptArgsTCs,
	translateDeregisterTCs,
	translateDockerTCs,
	translateGRPCUseTLSTCs,
	translateTLSServerNameTCs,
	translateH2PingUseTLS,
	translateTLSSkipVerifyTCs,
	translateServiceIDTCs,
}

// ScriptArgs: []string
func scriptArgsEqFn(out interface{}, want interface{}) error {
	var got []string
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.ScriptArgs
	case *structs.CheckDefinition:
		got = v.ScriptArgs
	case structs.CheckType:
		got = v.ScriptArgs
	case *structs.CheckType:
		got = v.ScriptArgs
	case structs.HealthCheckDefinition:
		got = v.ScriptArgs
	case *structs.HealthCheckDefinition:
		got = v.ScriptArgs
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}

	wantSlice := want.([]string)

	if len(got) != len(wantSlice) {
		return fmt.Errorf("ScriptArgs: expected %v, got %v", wantSlice, got)
	}
	for i := range got {
		if got[i] != wantSlice[i] {
			return fmt.Errorf("ScriptArgs: [i=%d] expected %v, got %v", i, wantSlice, got)
		}
	}
	return nil
}

var scriptFields = []string{
	`"ScriptArgs": %s`,
	`"args": %s`,
	`"script_args": %s`,
}

var translateScriptArgsTCs = []translateKeyTestCase{
	{
		desc:       "scriptArgs: all set",
		in:         []interface{}{`["1"]`, `["2"]`, `["3"]`},
		want:       []string{"1"},
		jsonFmtStr: "{" + strings.Join(scriptFields, ",") + "}",
		equalityFn: scriptArgsEqFn,
	},
	{
		desc:       "scriptArgs: first and second set",
		in:         []interface{}{`["1"]`, `["2"]`},
		want:       []string{"1"},
		jsonFmtStr: "{" + scriptFields[0] + "," + scriptFields[1] + "}",
		equalityFn: scriptArgsEqFn,
	},
	{
		desc:       "scriptArgs: first and third set",
		in:         []interface{}{`["1"]`, `["3"]`},
		want:       []string{"1"},
		jsonFmtStr: "{" + scriptFields[0] + "," + scriptFields[2] + "}",
		equalityFn: scriptArgsEqFn,
	},
	{
		desc:       "scriptArgs: second and third set",
		in:         []interface{}{`["2"]`, `["3"]`},
		want:       []string{"2"},
		jsonFmtStr: "{" + scriptFields[1] + "," + scriptFields[2] + "}",
		equalityFn: scriptArgsEqFn,
	},
	{
		desc:       "scriptArgs: first set",
		in:         []interface{}{`["1"]`},
		want:       []string{"1"},
		jsonFmtStr: "{" + scriptFields[0] + "}",
		equalityFn: scriptArgsEqFn,
	},
	{
		desc:       "scriptArgs: second set",
		in:         []interface{}{`["2"]`},
		want:       []string{"2"},
		jsonFmtStr: "{" + scriptFields[1] + "}",
		equalityFn: scriptArgsEqFn,
	},
	{
		desc:       "scriptArgs: third set",
		in:         []interface{}{`["3"]`},
		want:       []string{"3"},
		jsonFmtStr: "{" + scriptFields[2] + "}",
		equalityFn: scriptArgsEqFn,
	},
	{
		desc:       "scriptArgs: none set",
		in:         []interface{}{},
		want:       []string{},
		jsonFmtStr: "{}",
		equalityFn: scriptArgsEqFn,
	},
}

func deregisterEqFn(out interface{}, want interface{}) error {
	var got interface{}
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.DeregisterCriticalServiceAfter
	case *structs.CheckDefinition:
		got = v.DeregisterCriticalServiceAfter
	case structs.CheckType:
		got = v.DeregisterCriticalServiceAfter
	case *structs.CheckType:
		got = v.DeregisterCriticalServiceAfter
	case structs.HealthCheckDefinition:
		got = v.DeregisterCriticalServiceAfter
	case *structs.HealthCheckDefinition:
		got = v.DeregisterCriticalServiceAfter
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}

	if got != want {
		return fmt.Errorf("expected DeregisterCriticalServiceAfter to be %s, got %s", want, got)
	}
	return nil
}

var deregisterFields = []string{
	`"DeregisterCriticalServiceAfter": %s`,
	`"deregister_critical_service_after": %s`,
}

var translateDeregisterTCs = []translateKeyTestCase{
	{
		desc:       "deregister: both set",
		in:         []interface{}{`"2h0m"`, `"3h0m"`},
		want:       2 * time.Hour,
		jsonFmtStr: "{" + strings.Join(deregisterFields, ",") + "}",
		equalityFn: deregisterEqFn,
	},
	{
		desc:       "deregister: first set",
		in:         []interface{}{`"2h0m"`},
		want:       2 * time.Hour,
		jsonFmtStr: "{" + deregisterFields[0] + "}",
		equalityFn: deregisterEqFn,
	},
	{
		desc:       "deregister: second set",
		in:         []interface{}{`"3h0m"`},
		want:       3 * time.Hour,
		jsonFmtStr: "{" + deregisterFields[1] + "}",
		equalityFn: deregisterEqFn,
	},
	{
		desc:       "deregister: neither set",
		in:         []interface{}{},
		want:       time.Duration(0),
		jsonFmtStr: "{}",
		equalityFn: deregisterEqFn,
	},
}

// DockerContainerID: string
func dockerEqFn(out interface{}, want interface{}) error {
	var got interface{}
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.DockerContainerID
	case *structs.CheckDefinition:
		got = v.DockerContainerID
	case structs.CheckType:
		got = v.DockerContainerID
	case *structs.CheckType:
		got = v.DockerContainerID
	case structs.HealthCheckDefinition:
		got = v.DockerContainerID
	case *structs.HealthCheckDefinition:
		got = v.DockerContainerID
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}

	if got != want {
		return fmt.Errorf("expected DockerContainerID to be %s, got %s", want, got)
	}
	return nil
}

var dockerFields = []string{`"DockerContainerID": %s`, `"docker_container_id": %s`}
var translateDockerTCs = []translateKeyTestCase{
	{
		desc:       "dockerContainerID: both set",
		in:         []interface{}{`"id-1"`, `"id-2"`},
		want:       "id-1",
		jsonFmtStr: "{" + strings.Join(dockerFields, ",") + "}",
		equalityFn: dockerEqFn,
	},
	{
		desc:       "dockerContainerID: first set",
		in:         []interface{}{`"id-1"`},
		want:       "id-1",
		jsonFmtStr: "{" + dockerFields[0] + "}",
		equalityFn: dockerEqFn,
	},
	{
		desc:       "dockerContainerID: second set",
		in:         []interface{}{`"id-2"`},
		want:       "id-2",
		jsonFmtStr: "{" + dockerFields[1] + "}",
		equalityFn: dockerEqFn,
	},
	{
		desc:       "dockerContainerID: neither set",
		in:         []interface{}{},
		want:       "", // zero value
		jsonFmtStr: "{}",
		equalityFn: dockerEqFn,
	},
}

// TLSServerName: string
func tlsServerNameEqFn(out interface{}, want interface{}) error {
	var got interface{}
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.TLSServerName
	case *structs.CheckDefinition:
		got = v.TLSServerName
	case structs.CheckType:
		got = v.TLSServerName
	case *structs.CheckType:
		got = v.TLSServerName
	case structs.HealthCheckDefinition:
		got = v.TLSServerName
	case *structs.HealthCheckDefinition:
		got = v.TLSServerName
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}
	if got != want {
		return fmt.Errorf("expected TLSServerName to be %v, got %v", want, got)
	}
	return nil
}

var tlsServerNameFields = []string{`"TLSServerName": %s`, `"tls_server_name": %s`}
var translateTLSServerNameTCs = []translateKeyTestCase{
	{
		desc:       "tlsServerName: both set",
		in:         []interface{}{`"server1"`, `"server2"`},
		want:       "server1",
		jsonFmtStr: "{" + strings.Join(tlsServerNameFields, ",") + "}",
		equalityFn: tlsServerNameEqFn,
	},
	{
		desc:       "tlsServerName: first set",
		in:         []interface{}{`"server1"`},
		want:       "server1",
		jsonFmtStr: "{" + tlsServerNameFields[0] + "}",
		equalityFn: tlsServerNameEqFn,
	},
	{
		desc:       "tlsServerName: second set",
		in:         []interface{}{`"server2"`},
		want:       "server2",
		jsonFmtStr: "{" + tlsServerNameFields[1] + "}",
		equalityFn: tlsServerNameEqFn,
	},
	{
		desc:       "tlsServerName: neither set",
		in:         []interface{}{},
		want:       "", // zero value
		jsonFmtStr: "{}",
		equalityFn: tlsServerNameEqFn,
	},
}

// TLSSkipVerify: bool
func tlsSkipVerifyEqFn(out interface{}, want interface{}) error {
	var got interface{}
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.TLSSkipVerify
	case *structs.CheckDefinition:
		got = v.TLSSkipVerify
	case structs.CheckType:
		got = v.TLSSkipVerify
	case *structs.CheckType:
		got = v.TLSSkipVerify
	case structs.HealthCheckDefinition:
		got = v.TLSSkipVerify
	case *structs.HealthCheckDefinition:
		got = v.TLSSkipVerify
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}
	if got != want {
		return fmt.Errorf("expected TLSSkipVerify to be %v, got %v", want, got)
	}
	return nil
}

var tlsSkipVerifyFields = []string{`"TLSSkipVerify": %s`, `"tls_skip_verify": %s`}
var translateTLSSkipVerifyTCs = []translateKeyTestCase{
	{
		desc:       "tlsSkipVerify: both set",
		in:         []interface{}{`true`, `false`},
		want:       true,
		jsonFmtStr: "{" + strings.Join(tlsSkipVerifyFields, ",") + "}",
		equalityFn: tlsSkipVerifyEqFn,
	},
	{
		desc:       "tlsSkipVerify: first set",
		in:         []interface{}{`true`},
		want:       true,
		jsonFmtStr: "{" + tlsSkipVerifyFields[0] + "}",
		equalityFn: tlsSkipVerifyEqFn,
	},
	{
		desc:       "tlsSkipVerify: second set",
		in:         []interface{}{`true`},
		want:       true,
		jsonFmtStr: "{" + tlsSkipVerifyFields[1] + "}",
		equalityFn: tlsSkipVerifyEqFn,
	},
	{
		desc:       "tlsSkipVerify: neither set",
		in:         []interface{}{},
		want:       false, // zero value
		jsonFmtStr: "{}",
		equalityFn: tlsSkipVerifyEqFn,
	},
}

// GRPCUseTLS: bool
func grpcUseTLSEqFn(out interface{}, want interface{}) error {
	var got interface{}
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.GRPCUseTLS
	case *structs.CheckDefinition:
		got = v.GRPCUseTLS
	case structs.CheckType:
		got = v.GRPCUseTLS
	case *structs.CheckType:
		got = v.GRPCUseTLS
	case structs.HealthCheckDefinition:
		got = v.GRPCUseTLS
	case *structs.HealthCheckDefinition:
		got = v.GRPCUseTLS
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}
	if got != want {
		return fmt.Errorf("expected GRPCUseTLS to be %v, got %v", want, got)
	}
	return nil
}

var grpcUseTLSFields = []string{`"GRPCUseTLS": %s`, `"grpc_use_tls": %s`}
var translateGRPCUseTLSTCs = []translateKeyTestCase{
	{
		desc:       "GRPCUseTLS: both set",
		in:         []interface{}{"true", "false"},
		want:       true,
		jsonFmtStr: "{" + strings.Join(grpcUseTLSFields, ",") + "}",
		equalityFn: grpcUseTLSEqFn,
	},
	{
		desc:       "GRPCUseTLS: first set",
		in:         []interface{}{`true`},
		want:       true,
		jsonFmtStr: "{" + grpcUseTLSFields[0] + "}",
		equalityFn: grpcUseTLSEqFn,
	},
	{
		desc:       "GRPCUseTLS: second set",
		in:         []interface{}{`true`},
		want:       true,
		jsonFmtStr: "{" + grpcUseTLSFields[1] + "}",
		equalityFn: grpcUseTLSEqFn,
	},
	{
		desc:       "GRPCUseTLS: neither set",
		in:         []interface{}{},
		want:       false, // zero value
		jsonFmtStr: "{}",
		equalityFn: grpcUseTLSEqFn,
	},
}

func h2pingUseTLSEqFn(out interface{}, want interface{}) error {
	var got interface{}
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.H2PingUseTLS
	case *structs.CheckDefinition:
		got = v.H2PingUseTLS
	case structs.CheckType:
		got = v.H2PingUseTLS
	case *structs.CheckType:
		got = v.H2PingUseTLS
	case structs.HealthCheckDefinition:
		got = v.H2PingUseTLS
	case *structs.HealthCheckDefinition:
		got = v.H2PingUseTLS
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}
	if got != want {
		return fmt.Errorf("expected H2PingUseTLS to be %v, got %v", want, got)
	}
	return nil
}

var h2pingUseTLSFields = []string{`"H2PING": "testing"`, `"H2PingUseTLS": %s`, `"h2ping_use_tls": %s`}
var translateH2PingUseTLS = []translateKeyTestCase{
	{
		desc:       "H2PingUseTLS: both set",
		in:         []interface{}{"false", "true"},
		want:       false,
		jsonFmtStr: "{" + strings.Join(h2pingUseTLSFields, ",") + "}",
		equalityFn: h2pingUseTLSEqFn,
	},
	{
		desc:       "H2PingUseTLS:: first set",
		in:         []interface{}{`false`},
		want:       false,
		jsonFmtStr: "{" + strings.Join(h2pingUseTLSFields[0:2], ",") + "}",
		equalityFn: h2pingUseTLSEqFn,
	},
	{
		desc:       "H2PingUseTLS: second set",
		in:         []interface{}{`false`},
		want:       false,
		jsonFmtStr: "{" + h2pingUseTLSFields[0] + "," + h2pingUseTLSFields[2] + "}",
		equalityFn: h2pingUseTLSEqFn,
	},
	{
		desc:       "H2PingUseTLS: neither set",
		in:         []interface{}{},
		want:       true, // zero value
		jsonFmtStr: "{" + h2pingUseTLSFields[0] + "}",
		equalityFn: h2pingUseTLSEqFn,
	},
}

// ServiceID: string
func serviceIDEqFn(out interface{}, want interface{}) error {
	var got interface{}
	switch v := out.(type) {
	case structs.CheckDefinition:
		got = v.ServiceID
	case *structs.CheckDefinition:
		got = v.ServiceID
	case structs.CheckType:
		return nil // CheckType does not have a ServiceID field
	case *structs.CheckType:
		return nil // CheckType does not have a ServiceID field
	case structs.HealthCheckDefinition:
		return nil // HealthCheckDefinition does not have a ServiceID field
	case *structs.HealthCheckDefinition:
		return nil // HealthCheckDefinition does not have a ServiceID field
	default:
		panic(fmt.Sprintf("unexpected type %T", out))
	}
	if got != want {
		return fmt.Errorf("expected ServiceID to be %s, got %s", want, got)
	}
	return nil
}

var serviceIDFields = []string{`"ServiceID": %s`, `"service_id": %s`}
var translateServiceIDTCs = []translateKeyTestCase{
	{
		desc:       "serviceID: both set",
		in:         []interface{}{`"id-1"`, `"id-2"`},
		want:       "id-1",
		jsonFmtStr: "{" + strings.Join(serviceIDFields, ",") + "}",
		equalityFn: serviceIDEqFn,
	},
	{
		desc:       "serviceID: first set",
		in:         []interface{}{`"id-1"`},
		want:       "id-1",
		jsonFmtStr: "{" + serviceIDFields[0] + "}",
		equalityFn: serviceIDEqFn,
	},
	{
		desc:       "serviceID: second set",
		in:         []interface{}{`"id-2"`},
		want:       "id-2",
		jsonFmtStr: "{" + serviceIDFields[1] + "}",
		equalityFn: serviceIDEqFn,
	},
	{
		desc:       "serviceID: neither set",
		in:         []interface{}{},
		want:       "", // zero value
		jsonFmtStr: "{}",
		equalityFn: serviceIDEqFn,
	},
}

// ACLPolicySetRequest:
// Policy	structs.ACLPolicy
//
//	ID	string
//	Name	string
//	Description	string
//	Rules	string
//	Syntax	acl.SyntaxVersion
//	Datacenters	[]string
//	Hash	[]uint8
//	RaftIndex	structs.RaftIndex
//	    CreateIndex	uint64
//	    ModifyIndex	uint64
//
// Datacenter	string
// WriteRequest	structs.WriteRequest
//
//	Token	string
func TestDecodeACLPolicyWrite(t *testing.T) {

	for _, tc := range hashTestCases {
		t.Run(tc.desc, func(t *testing.T) {

			jsonStr := fmt.Sprintf(`{
		"Hash": %s
	}`, tc.hashes.in)
			body := bytes.NewBuffer([]byte(jsonStr))

			var out structs.ACLPolicy
			err := decodeBody(body, &out)

			if err != nil && !tc.wantErr {
				t.Fatal(err)
			}
			if err == nil && tc.wantErr {
				t.Fatal("expected error, got nil")
			}
			if !bytes.Equal(out.Hash, tc.hashes.want) {
				t.Fatalf("expected hash to be %s, got %s", tc.hashes.want, out.Hash)
			}
		})

	}
}

// ACLTokenSetRequest:
// ACLToken	structs.ACLToken
//
//	AccessorID	string
//	SecretID	string
//	Description	string
//	Policies	[]structs.ACLTokenPolicyLink
//	    ID	string
//	    Name	string
//	Roles	[]structs.ACLTokenRoleLink
//	    ID	string
//	    Name	string
//	ServiceIdentities	[]*structs.ACLServiceIdentity
//	    ServiceName	string
//	    Datacenters	[]string
//	Type	string
//	Rules	string
//	Local	bool
//	AuthMethod	string
//	ExpirationTime	*time.Time
//	ExpirationTTL	time.Duration
//	CreateTime	time.Time
//	Hash	[]uint8
//	RaftIndex	structs.RaftIndex
//	    CreateIndex	uint64
//	    ModifyIndex	uint64
//
// Create	bool
// Datacenter	string
// WriteRequest	structs.WriteRequest
//
//	Token	string
func TestDecodeACLToken(t *testing.T) {
	for _, tc := range translateValueTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			var expTime, expTTL, createTime, hash = "null", "null", "null", "null"
			if tc.hashes != nil {
				hash = tc.hashes.in
			}
			if tc.timestamps != nil {
				expTime = tc.timestamps.in
				createTime = tc.timestamps.in
			}
			if tc.durations != nil {
				expTTL = tc.durations.in
			}
			bodyBytes := []byte(fmt.Sprintf(`{
				"ExpirationTime": %s,
				"ExpirationTTL": %s,
				"CreateTime": %s,
				"Hash": %s
			}`, expTime, expTTL, createTime, hash))

			body := bytes.NewBuffer(bodyBytes)

			// decode body
			var out structs.ACLToken

			err := decodeBody(body, &out)
			if err != nil && !tc.wantErr {
				t.Fatal(err)
			}
			if err == nil && tc.wantErr {
				t.Fatal("expected error, got nil")
			}

			// are we testing hashes in this test case?
			if tc.hashes != nil {
				if !bytes.Equal(out.Hash, tc.hashes.want) {
					t.Fatalf("expected hash to be %s, got %s", tc.hashes.want, out.Hash)
				}
			}
			// are we testing durations?
			if tc.durations != nil {
				if out.ExpirationTTL != tc.durations.want {
					t.Fatalf("expected expirationTTL to be %s, got %s", tc.durations.want, out.ExpirationTTL)
				}
			}
			// are we testing timestamps?
			if tc.timestamps != nil {
				if out.ExpirationTime != nil {
					if !out.ExpirationTime.Equal(tc.timestamps.want) {
						t.Fatalf("expected expirationTime to be %s, got %s", tc.timestamps.want, out.ExpirationTime)
					}
				} else {
					if !tc.timestamps.want.IsZero() {
						t.Fatalf("expected empty expirationTime, got %v", out.ExpirationTime)
					}
				}

				if !out.CreateTime.Equal(tc.timestamps.want) {
					t.Fatalf("expected createTime to be %s, got %s", tc.timestamps.want, out.CreateTime)
				}
			}
		})

	}
}

// ACLRoleSetRequest:
// Role	structs.ACLRole
//     ID	string
//     Name	string
//     Description	string
//     Policies	[]structs.ACLRolePolicyLink
//         ID	string
//         Name	string
//     ServiceIdentities	[]*structs.ACLServiceIdentity
//         ServiceName	string
//         Datacenters	[]string
//     Hash	[]uint8
//     RaftIndex	structs.RaftIndex
//         CreateIndex	uint64
//         ModifyIndex	uint64
// Datacenter	string
// WriteRequest	structs.WriteRequest
//     Token	string

func TestDecodeACLRoleWrite(t *testing.T) {
	for _, tc := range hashTestCases {
		t.Run(tc.desc, func(t *testing.T) {

			jsonStr := fmt.Sprintf(`{
		"Hash": %s
	}`, tc.hashes.in)
			body := bytes.NewBuffer([]byte(jsonStr))

			var out structs.ACLRole
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected error, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected no error, got: %v", err)
			}
			if !bytes.Equal(out.Hash, tc.hashes.want) {
				t.Fatalf("expected hash to be %s, got %s", tc.hashes.want, out.Hash)
			}
		})

	}
}

// CheckDefinition:
// ID	types.CheckID
// Name	string
// Notes	string
// ServiceID	string
// Token	string
// Status	string
// ScriptArgs	[]string
// HTTP	string
// Header	map[string][]string
// Method	string
// TCP	string
// Interval	time.Duration
// DockerContainerID	string
// Shell	string
// GRPC	string
// GRPCUseTLS	bool
// H2PING	string
// H2PingUseTLS	bool
// TLSServerName	string
// TLSSkipVerify	bool
// AliasNode	string
// AliasService	string
// Timeout	time.Duration
// TTL	time.Duration
// DeregisterCriticalServiceAfter	time.Duration
// OutputMaxSize	int
// ==========
// decodeCB == FixupCheckType
func TestDecodeAgentRegisterCheck(t *testing.T) {
	// Durations: Interval, Timeout, TTL, DeregisterCriticalServiceAfter
	for _, tc := range durationTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			jsonStr := fmt.Sprintf(`{

				"Interval": %[1]s,
				"Timeout": %[1]s,
				"TTL": %[1]s,
				"DeregisterCriticalServiceAfter": %[1]s
			}`, tc.durations.in)
			body := bytes.NewBuffer([]byte(jsonStr))

			var out structs.CheckDefinition
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			err = checkTypeDurationTest(out, tc.durations.want, "")
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	for _, tc := range checkTypeHeaderTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			jsonStr := fmt.Sprintf(`{"Header": %s}`, tc.in)

			body := bytes.NewBuffer([]byte(jsonStr))

			var out structs.CheckDefinition
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			if err := checkTypeHeaderTest(out, tc.want); err != nil {
				t.Fatal(err)
			}
		})
	}

	for _, tcs := range translateCheckTypeTCs {
		for _, tc := range tcs {
			t.Run(tc.desc, func(t *testing.T) {
				jsonStr := fmt.Sprintf(tc.jsonFmtStr, tc.in...)

				body := bytes.NewBuffer([]byte(jsonStr))

				var out structs.CheckDefinition
				err := decodeBody(body, &out)

				if err != nil {
					t.Fatal(err)
				}

				if err := tc.equalityFn(out, tc.want); err != nil {
					t.Fatal(err)
				}
			})
		}
	}

}

// ServiceDefinition:
// Kind	structs.ServiceKind
// ID	string
// Name	string
// Tags	[]string
// Address	string
// TaggedAddresses	map[string]structs.ServiceAddress
//
//	Address	string
//	Port	int
//
// Meta	map[string]string
// Port	int
// Check	structs.CheckType
//
//	CheckID	types.CheckID
//	Name	string
//	Status	string
//	Notes	string
//	ScriptArgs	[]string
//	HTTP	string
//	Header	map[string][]string
//	Method	string
//	TCP	string
//	Interval	time.Duration
//	AliasNode	string
//	AliasService	string
//	DockerContainerID	string
//	Shell	string
//	GRPC	string
//	GRPCUseTLS	bool
//	TLSServerName	string
//	TLSSkipVerify	bool
//	Timeout	time.Duration
//	TTL	time.Duration
//	ProxyHTTP	string
//	ProxyGRPC	string
//	DeregisterCriticalServiceAfter	time.Duration
//	OutputMaxSize	int
//
// Checks	structs.CheckTypes
// Weights	*structs.Weights
//
//	Passing	int
//	Warning	int
//
// Token	string
// EnableTagOverride	bool
// Proxy	*structs.ConnectProxyConfig
//
//	DestinationServiceName	string
//	DestinationServiceID	string
//	LocalServiceAddress	string
//	LocalServicePort	int
//	Config	map[string]interface {}
//	Upstreams	structs.Upstreams
//	    DestinationType	string
//	    DestinationNamespace	string
//	    DestinationName	string
//	    Datacenter	string
//	    LocalBindAddress	string
//	    LocalBindPort	int
//	    Config	map[string]interface {}
//	    MeshGateway	structs.MeshGatewayConfig
//	        Mode	structs.MeshGatewayMode
//	MeshGateway	structs.MeshGatewayConfig
//	Expose	structs.ExposeConfig
//	    Checks	bool
//	    Paths	[]structs.ExposePath
//	        ListenerPort	int
//	        Path	string
//	        LocalPathPort	int
//	        Protocol	string
//	        ParsedFromCheck	bool
//
// Connect	*structs.ServiceConnect
//
//	Native	bool
//	SidecarService	*structs.ServiceDefinition
func TestDecodeAgentRegisterService(t *testing.T) {
	// key translation tests:
	// decodeCB fields:
	// --------------------
	// "enable_tag_override": "EnableTagOverride",
	// // Proxy Upstreams
	// "destination_name":      "DestinationName",
	// "destination_type":      "DestinationType",
	// "destination_namespace": "DestinationNamespace",
	// "local_bind_port":       "LocalBindPort",
	// "local_bind_address":    "LocalBindAddress",
	// // Proxy Config
	// "destination_service_name": "DestinationServiceName",
	// "destination_service_id":   "DestinationServiceID",
	// "local_service_port":       "LocalServicePort",
	// "local_service_address":    "LocalServiceAddress",
	// // SidecarService
	// "sidecar_service": "SidecarService",
	// // Expose Config
	// "local_path_port": "LocalPathPort",
	// "listener_port":   "ListenerPort",

	// "tagged_addresses": "TaggedAddresses",

	// EnableTagOverride: bool
	enableTagOverrideEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).EnableTagOverride
		if got != want {
			return fmt.Errorf("expected EnableTagOverride to be %v, got %v", want, got)
		}
		return nil
	}
	var enableTagOverrideFields = []string{
		`"EnableTagOverride": %s`,
		`"enable_tag_override": %s`,
	}
	var translateEnableTagOverrideTCs = []translateKeyTestCase{
		{
			desc:       "translateEnableTagTCs: both set",
			in:         []interface{}{`true`, `false`},
			want:       true,
			jsonFmtStr: "{" + strings.Join(enableTagOverrideFields, ",") + "}",
			equalityFn: enableTagOverrideEqFn,
		},
		{
			desc:       "translateEnableTagTCs: first set",
			in:         []interface{}{`true`},
			want:       true,
			jsonFmtStr: "{" + enableTagOverrideFields[0] + "}",
			equalityFn: enableTagOverrideEqFn,
		},
		{
			desc:       "translateEnableTagTCs: second set",
			in:         []interface{}{`true`},
			want:       true,
			jsonFmtStr: "{" + enableTagOverrideFields[1] + "}",
			equalityFn: enableTagOverrideEqFn,
		},
		{
			desc:       "translateEnableTagTCs: neither set",
			in:         []interface{}{},
			want:       false, // zero value
			jsonFmtStr: "{}",
			equalityFn: enableTagOverrideEqFn,
		},
	}

	// DestinationName: string (Proxy.Upstreams)
	destinationNameEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.Upstreams[0].DestinationName
		if got != want {
			return fmt.Errorf("expected DestinationName to be %s, got %s", want, got)
		}
		return nil
	}

	var destinationNameFields = []string{
		`"DestinationName": %s`,
		`"destination_name": %s`,
	}
	var translateDestinationNameTCs = []translateKeyTestCase{
		{
			desc:       "DestinationName: both set",
			in:         []interface{}{`"a"`, `"b"`},
			want:       "a",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + strings.Join(destinationNameFields, ",") + `}]}}`,
			equalityFn: destinationNameEqFn,
		},
		{
			desc:       "DestinationName: first set",
			in:         []interface{}{`"a"`},
			want:       "a",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + destinationNameFields[0] + `}]}}`,
			equalityFn: destinationNameEqFn,
		},
		{
			desc:       "DestinationName: second set",
			in:         []interface{}{`"b"`},
			want:       "b",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + destinationNameFields[1] + `}]}}`,
			equalityFn: destinationNameEqFn,
		},
		{
			desc:       "DestinationName: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Proxy": {"Upstreams": [{}]}}`,
			equalityFn: destinationNameEqFn,
		},
	}

	// DestinationType: string (Proxy.Upstreams)
	destinationTypeEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.Upstreams[0].DestinationType
		if got != want {
			return fmt.Errorf("expected DestinationType to be %s, got %s", want, got)
		}
		return nil
	}

	var destinationTypeFields = []string{
		`"DestinationType": %s`,
		`"destination_type": %s`,
	}
	var translateDestinationTypeTCs = []translateKeyTestCase{
		{
			desc:       "DestinationType: both set",
			in:         []interface{}{`"a"`, `"b"`},
			want:       "a",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + strings.Join(destinationTypeFields, ",") + `}]}}`,
			equalityFn: destinationTypeEqFn,
		},
		{
			desc:       "DestinationType: first set",
			in:         []interface{}{`"a"`},
			want:       "a",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + destinationTypeFields[0] + `}]}}`,
			equalityFn: destinationTypeEqFn,
		},
		{
			desc:       "DestinationType: second set",
			in:         []interface{}{`"b"`},
			want:       "b",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + destinationTypeFields[1] + `}]}}`,
			equalityFn: destinationTypeEqFn,
		},
		{
			desc:       "DestinationType: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Proxy": {"Upstreams": [{}]}}`,
			equalityFn: destinationTypeEqFn,
		},
	}

	// DestinationNamespace: string (Proxy.Upstreams)
	destinationNamespaceEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.Upstreams[0].DestinationNamespace
		if got != want {
			return fmt.Errorf("expected DestinationNamespace to be %s, got %s", want, got)
		}
		return nil
	}

	var destinationNamespaceFields = []string{
		`"DestinationNamespace": %s`,
		`"destination_namespace": %s`,
	}
	var translateDestinationNamespaceTCs = []translateKeyTestCase{
		{
			desc:       "DestinationNamespace: both set",
			in:         []interface{}{`"a"`, `"b"`},
			want:       "a",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + strings.Join(destinationNamespaceFields, ",") + `}]}}`,

			equalityFn: destinationNamespaceEqFn,
		},
		{
			desc:       "DestinationNamespace: first set",
			in:         []interface{}{`"a"`},
			want:       "a",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + destinationNamespaceFields[0] + `}]}}`,
			equalityFn: destinationNamespaceEqFn,
		},
		{
			desc:       "DestinationNamespace: second set",
			in:         []interface{}{`"b"`},
			want:       "b",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + destinationNamespaceFields[1] + `}]}}`,
			equalityFn: destinationNamespaceEqFn,
		},
		{
			desc:       "DestinationNamespace: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Proxy": {"Upstreams": [{}]}}`,
			equalityFn: destinationNamespaceEqFn,
		},
	}

	// LocalBindPort: int (Proxy.Upstreams)
	localBindPortEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.Upstreams[0].LocalBindPort
		if got != want {
			return fmt.Errorf("expected LocalBindPort to be %v, got %v", want, got)
		}
		return nil
	}
	var localBindPortFields = []string{
		`"LocalBindPort": %s`,
		`"local_bind_port": %s`,
	}
	var translateLocalBindPortTCs = []translateKeyTestCase{
		{
			desc:       "LocalBindPort: both set",
			in:         []interface{}{`1`, `2`},
			want:       1,
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + strings.Join(localBindPortFields, ",") + `}]}}`,
			equalityFn: localBindPortEqFn,
		},
		{
			desc:       "LocalBindPort: first set",
			in:         []interface{}{`1`},
			want:       1,
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + localBindPortFields[0] + `}]}}`,
			equalityFn: localBindPortEqFn,
		},
		{
			desc:       "LocalBindPort: second set",
			in:         []interface{}{`2`},
			want:       2,
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + localBindPortFields[1] + `}]}}`,
			equalityFn: localBindPortEqFn,
		},
		{
			desc:       "LocalBindPort: neither set",
			in:         []interface{}{},
			want:       0, // zero value
			jsonFmtStr: `{"Proxy": {"Upstreams": [{}]}}`,
			equalityFn: localBindPortEqFn,
		},
	}

	// LocalBindAddress: string (Proxy.Upstreams)
	localBindAddressEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.Upstreams[0].LocalBindAddress
		if got != want {
			return fmt.Errorf("expected LocalBindAddress to be %s, got %s", want, got)
		}
		return nil
	}

	var localBindAddressFields = []string{
		`"LocalBindAddress": %s`,
		`"local_bind_address": %s`,
	}
	var translateLocalBindAddressTCs = []translateKeyTestCase{
		{
			desc:       "LocalBindAddress: both set",
			in:         []interface{}{`"one"`, `"two"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + strings.Join(localBindAddressFields, ",") + `}]}}`,
			equalityFn: localBindAddressEqFn,
		},
		{
			desc:       "LocalBindAddress: first set",
			in:         []interface{}{`"one"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + localBindAddressFields[0] + `}]}}`,
			equalityFn: localBindAddressEqFn,
		},
		{
			desc:       "LocalBindAddress: second set",
			in:         []interface{}{`"two"`},
			want:       "two",
			jsonFmtStr: `{"Proxy": {"Upstreams": [{` + localBindAddressFields[1] + `}]}}`,
			equalityFn: localBindAddressEqFn,
		},
		{
			desc:       "LocalBindAddress: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Proxy": {"Upstreams": [{}]}}`,
			equalityFn: localBindAddressEqFn,
		},
	}

	// DestinationServiceName: string (Proxy)
	destinationServiceNameEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.DestinationServiceName
		if got != want {
			return fmt.Errorf("expected DestinationServiceName to be %s, got %s", want, got)
		}
		return nil
	}

	var destinationServiceNameFields = []string{
		`"DestinationServiceName": %s`,
		`"destination_service_name": %s`,
	}
	var translateDestinationServiceNameTCs = []translateKeyTestCase{
		{
			desc:       "DestinationServiceName: both set",
			in:         []interface{}{`"one"`, `"two"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {` + strings.Join(destinationServiceNameFields, ",") + `}}`,
			equalityFn: destinationServiceNameEqFn,
		},
		{
			desc:       "DestinationServiceName: first set",
			in:         []interface{}{`"one"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {` + destinationServiceNameFields[0] + `}}`,
			equalityFn: destinationServiceNameEqFn,
		},
		{
			desc:       "DestinationServiceName: second set",
			in:         []interface{}{`"two"`},
			want:       "two",
			jsonFmtStr: `{"Proxy": {` + destinationServiceNameFields[1] + `}}`,
			equalityFn: destinationServiceNameEqFn,
		},
		{
			desc:       "DestinationServiceName: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Proxy": {` + `}}`,
			equalityFn: destinationServiceNameEqFn,
		},
	}

	// DestinationServiceID:  string (Proxy)
	destinationServiceIDEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.DestinationServiceID
		if got != want {
			return fmt.Errorf("expected DestinationServiceID to be %s, got %s", want, got)
		}
		return nil
	}

	var destinationServiceIDFields = []string{
		`"DestinationServiceID": %s`,
		`"destination_service_id": %s`,
	}
	var translateDestinationServiceIDTCs = []translateKeyTestCase{
		{
			desc:       "DestinationServiceID: both set",
			in:         []interface{}{`"one"`, `"two"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {` + strings.Join(destinationServiceIDFields, ",") + `}}`,
			equalityFn: destinationServiceIDEqFn,
		},
		{
			desc:       "DestinationServiceID: first set",
			in:         []interface{}{`"one"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {` + destinationServiceIDFields[0] + `}}`,
			equalityFn: destinationServiceIDEqFn,
		},
		{
			desc:       "DestinationServiceID: second set",
			in:         []interface{}{`"two"`},
			want:       "two",
			jsonFmtStr: `{"Proxy": {` + destinationServiceIDFields[1] + `}}`,
			equalityFn: destinationServiceIDEqFn,
		},
		{
			desc:       "DestinationServiceID: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Proxy": {}}`,
			equalityFn: destinationServiceIDEqFn,
		},
	}

	// LocalServicePort: int (Proxy)
	localServicePortEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.LocalServicePort
		if got != want {
			return fmt.Errorf("expected LocalServicePort to be %v, got %v", want, got)
		}
		return nil
	}
	var localServicePortFields = []string{
		`"LocalServicePort": %s`,
		`"local_service_port": %s`,
	}
	var translateLocalServicePortTCs = []translateKeyTestCase{
		{
			desc:       "LocalServicePort: both set",
			in:         []interface{}{`1`, `2`},
			want:       1,
			jsonFmtStr: `{"Proxy": {` + strings.Join(localServicePortFields, ",") + `}}`,
			equalityFn: localServicePortEqFn,
		},
		{
			desc:       "LocalServicePort: first set",
			in:         []interface{}{`1`},
			want:       1,
			jsonFmtStr: `{"Proxy": {` + localServicePortFields[0] + `}}`,
			equalityFn: localServicePortEqFn,
		},
		{
			desc:       "LocalServicePort: second set",
			in:         []interface{}{`2`},
			want:       2,
			jsonFmtStr: `{"Proxy": {` + localServicePortFields[1] + `}}`,
			equalityFn: localServicePortEqFn,
		},
		{
			desc:       "LocalServicePort: neither set",
			in:         []interface{}{},
			want:       0, // zero value
			jsonFmtStr: `{"Proxy": {}}`,
			equalityFn: localServicePortEqFn,
		},
	}

	// LocalServiceAddress: string (Proxy)
	localServiceAddressEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.LocalServiceAddress
		if got != want {
			return fmt.Errorf("expected LocalServiceAddress to be %s, got %s", want, got)
		}
		return nil
	}

	var localServiceAddressFields = []string{
		`"LocalServiceAddress": %s`,
		`"local_service_address": %s`,
	}
	var translateLocalServiceAddressTCs = []translateKeyTestCase{
		{
			desc:       "LocalServiceAddress: both set",
			in:         []interface{}{`"one"`, `"two"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {` + strings.Join(localServiceAddressFields, ",") + `}}`,
			equalityFn: localServiceAddressEqFn,
		},
		{
			desc:       "LocalServiceAddress: first set",
			in:         []interface{}{`"one"`},
			want:       "one",
			jsonFmtStr: `{"Proxy": {` + localServiceAddressFields[0] + `}}`,
			equalityFn: localServiceAddressEqFn,
		},
		{
			desc:       "LocalServiceAddress: second set",
			in:         []interface{}{`"two"`},
			want:       "two",
			jsonFmtStr: `{"Proxy": {` + localServiceAddressFields[1] + `}}`,
			equalityFn: localServiceAddressEqFn,
		},
		{
			desc:       "LocalServiceAddress: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Proxy": {}}`,
			equalityFn: localServiceAddressEqFn,
		},
	}

	// SidecarService: ServiceDefinition (Connect)
	sidecarServiceEqFn := func(out interface{}, want interface{}) error {
		scService := out.(structs.ServiceDefinition).Connect.SidecarService
		if scService == nil {
			if want != "" {
				return fmt.Errorf("expected SidecarService with Name '%s', got nil service", want)
			}
			return nil
		}
		if scService.Name != want {
			return fmt.Errorf("expected SidecarService with Name '%s', got Name=%s", want, scService.Name)
		}
		return nil
	}

	var sidecarServiceFields = []string{
		`"SidecarService": %s`,
		`"sidecar_service": %s`,
	}
	var translateSidecarServiceTCs = []translateKeyTestCase{
		{
			desc:       "SidecarService: both set",
			in:         []interface{}{`{"Name": "one"}`, `{"Name": "two"}`},
			want:       "one",
			jsonFmtStr: `{"Connect": {` + strings.Join(sidecarServiceFields, ",") + `}}`,
			equalityFn: sidecarServiceEqFn,
		},
		{
			desc:       "SidecarService: first set",
			in:         []interface{}{`{"Name": "one"}`},
			want:       "one",
			jsonFmtStr: `{"Connect": {` + sidecarServiceFields[0] + `}}`,
			equalityFn: sidecarServiceEqFn,
		},
		{
			desc:       "SidecarService: second set",
			in:         []interface{}{`{"Name": "two"}`},
			want:       "two",
			jsonFmtStr: `{"Connect": {` + sidecarServiceFields[1] + `}}`,
			equalityFn: sidecarServiceEqFn,
		},
		{
			desc:       "SidecarService: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{"Connect": {}}`,
			equalityFn: sidecarServiceEqFn,
		},
	}

	// LocalPathPort: int (Proxy.Expose.Paths)
	localPathPortEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.Expose.Paths[0].LocalPathPort
		if got != want {
			return fmt.Errorf("expected LocalPathPort to be %v, got %v", want, got)
		}
		return nil
	}
	var localPathPortFields = []string{
		`"LocalPathPort": %s`,
		`"local_path_port": %s`,
	}
	var translateLocalPathPortTCs = []translateKeyTestCase{
		{
			desc:       "LocalPathPort: both set",
			in:         []interface{}{`1`, `2`},
			want:       1,
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{` + strings.Join(localPathPortFields, ",") + `}]}}}`,
			equalityFn: localPathPortEqFn,
		},
		{
			desc:       "LocalPathPort: first set",
			in:         []interface{}{`1`},
			want:       1,
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{` + localPathPortFields[0] + `}]}}}`,
			equalityFn: localPathPortEqFn,
		},
		{
			desc:       "LocalPathPort: second set",
			in:         []interface{}{`2`},
			want:       2,
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{` + localPathPortFields[1] + `}]}}}`,
			equalityFn: localPathPortEqFn,
		},
		{
			desc:       "LocalPathPort: neither set",
			in:         []interface{}{},
			want:       0, // zero value
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{}]}}}`,
			equalityFn: localPathPortEqFn,
		},
	}

	// ListenerPort: int (Proxy.Expose.Paths)
	listenerPortEqFn := func(out interface{}, want interface{}) error {
		got := out.(structs.ServiceDefinition).Proxy.Expose.Paths[0].ListenerPort
		if got != want {
			return fmt.Errorf("expected ListenerPort to be %v, got %v", want, got)
		}
		return nil
	}
	var listenerPortFields = []string{
		`"ListenerPort": %s`,
		`"listener_port": %s`,
	}
	var translateListenerPortTCs = []translateKeyTestCase{
		{
			desc:       "ListenerPort: both set",
			in:         []interface{}{`1`, `2`},
			want:       1,
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{` + strings.Join(listenerPortFields, ",") + `}]}}}`,
			equalityFn: listenerPortEqFn,
		},
		{
			desc:       "ListenerPort: first set",
			in:         []interface{}{`1`},
			want:       1,
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{` + listenerPortFields[0] + `}]}}}`,
			equalityFn: listenerPortEqFn,
		},
		{
			desc:       "ListenerPort: second set",
			in:         []interface{}{`2`},
			want:       2,
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{` + listenerPortFields[1] + `}]}}}`,
			equalityFn: listenerPortEqFn,
		},
		{
			desc:       "ListenerPort: neither set",
			in:         []interface{}{},
			want:       0, // zero value
			jsonFmtStr: `{"Proxy": {"Expose": {"Paths": [{}]}}}`,
			equalityFn: listenerPortEqFn,
		},
	}

	// TaggedAddresses: map[string]structs.ServiceAddress
	taggedAddressesEqFn := func(out interface{}, want interface{}) error {
		tgdAddresses := out.(structs.ServiceDefinition).TaggedAddresses
		if tgdAddresses == nil {
			if want != "" {
				return fmt.Errorf("expected TaggedAddresses at key='key' to have Address='%s', got nil TaggedAddress", want)
			}
			return nil
		}

		if tgdAddresses["key"].Address != want {
			return fmt.Errorf("expected TaggedAddresses at key='key' to have Address '%v', got Address=%v", want, tgdAddresses)
		}
		return nil
	}

	var taggedAddressesFields = []string{
		`"TaggedAddresses": %s`,
		`"tagged_addresses": %s`,
	}
	var translateTaggedAddressesTCs = []translateKeyTestCase{
		{
			desc:       "TaggedAddresses: both set",
			in:         []interface{}{`{"key": {"Address": "1"}}`, `{"key": {"Address": "2"}}`},
			want:       "1",
			jsonFmtStr: `{` + strings.Join(taggedAddressesFields, ",") + `}`,
			equalityFn: taggedAddressesEqFn,
		},
		{
			desc:       "TaggedAddresses: first set",
			in:         []interface{}{`{"key": {"Address": "1"}}`},
			want:       "1",
			jsonFmtStr: `{` + taggedAddressesFields[0] + `}`,
			equalityFn: taggedAddressesEqFn,
		},
		{
			desc:       "TaggedAddresses: second set",
			in:         []interface{}{`{"key": {"Address": "2"}}`},
			want:       "2",
			jsonFmtStr: `{` + taggedAddressesFields[1] + `}`,
			equalityFn: taggedAddressesEqFn,
		},
		{
			desc:       "TaggedAddresses: neither set",
			in:         []interface{}{},
			want:       "", // zero value
			jsonFmtStr: `{}`,
			equalityFn: taggedAddressesEqFn,
		},
	}

	// lib.TranslateKeys keys pasted here again to check against:
	// ---------------------------------------
	// "enable_tag_override": "EnableTagOverride",
	// // Proxy Upstreams
	// "destination_name":      "DestinationName",
	// "destination_type":      "DestinationType",
	// "destination_namespace": "DestinationNamespace",
	// "local_bind_port":       "LocalBindPort",
	// "local_bind_address":    "LocalBindAddress",
	// // Proxy Config
	// "destination_service_name": "DestinationServiceName",
	// "destination_service_id":   "DestinationServiceID",
	// "local_service_port":       "LocalServicePort",
	// "local_service_address":    "LocalServiceAddress",
	// // SidecarService
	// "sidecar_service": "SidecarService",
	// // Expose Config
	// "local_path_port": "LocalPathPort",
	// "listener_port":   "ListenerPort",
	// "tagged_addresses": "TaggedAddresses",

	var translateFieldTCs = [][]translateKeyTestCase{
		translateEnableTagOverrideTCs,
		translateDestinationNameTCs,
		translateDestinationTypeTCs,
		translateDestinationNamespaceTCs,
		translateLocalBindPortTCs,
		translateLocalBindAddressTCs,
		translateDestinationServiceNameTCs,
		translateDestinationServiceIDTCs,
		translateLocalServicePortTCs,
		translateLocalServiceAddressTCs,
		translateSidecarServiceTCs,
		translateLocalPathPortTCs,
		translateListenerPortTCs,
		translateTaggedAddressesTCs,
	}

	for _, tcGroup := range translateFieldTCs {
		for _, tc := range tcGroup {
			t.Run(tc.desc, func(t *testing.T) {
				checkJSONStr := fmt.Sprintf(tc.jsonFmtStr, tc.in...)
				body := bytes.NewBuffer([]byte(checkJSONStr))

				var out structs.ServiceDefinition
				err := decodeBody(body, &out)

				if err != nil {
					t.Fatal(err)
				}

				if err := tc.equalityFn(out, tc.want); err != nil {
					t.Fatal(err)
				}
			})
		}
	}

	// ======================================================

	for _, tc := range durationTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			jsonStr := fmt.Sprintf(`{
				"Check": {
					"Interval": %[1]s,
					"Timeout": %[1]s,
					"TTL": %[1]s,
					"DeregisterCriticalServiceAfter": %[1]s
				},
				"Checks": [
					{
						"Interval": %[1]s,
						"Timeout": %[1]s,
						"TTL": %[1]s,
						"DeregisterCriticalServiceAfter": %[1]s
					}
				]
			}`, tc.durations.in)
			body := bytes.NewBuffer([]byte(jsonStr))

			var out structs.ServiceDefinition
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			err = checkTypeDurationTest(out.Check, tc.durations.want, "")
			if err != nil {
				t.Fatal(err)
			}
			if out.Checks == nil {
				if tc.durations.want != 0 {
					t.Fatalf("Checks is nil, expected duration values to be %v", tc.durations.want)
				}
				return
			}
			err = checkTypeDurationTest(out.Checks[0], tc.durations.want, "[i=0]")
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	for _, tc := range checkTypeHeaderTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			checkJSONStr := fmt.Sprintf(`{"Header": %s}`, tc.in)
			jsonStr := fmt.Sprintf(`{
				"Check": %[1]s,
				"Checks": [%[1]s]
			}`, checkJSONStr)

			body := bytes.NewBuffer([]byte(jsonStr))

			var out structs.ServiceDefinition
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			if err := checkTypeHeaderTest(out.Check, tc.want); err != nil {
				t.Fatal(err)
			}
			if out.Checks == nil {
				if tc.want != nil {
					t.Fatalf("Checks is nil, expected Header to be %v", tc.want)
				}
				return
			}
			if err := checkTypeHeaderTest(out.Checks[0], tc.want); err != nil {
				t.Fatal(err)
			}
		})
	}

	for _, tcs := range translateCheckTypeTCs {
		for _, tc := range tcs {
			t.Run(tc.desc, func(t *testing.T) {
				checkJSONStr := fmt.Sprintf(tc.jsonFmtStr, tc.in...)
				jsonStr := fmt.Sprintf(`{
					"Check": %[1]s,
					"Checks": [%[1]s]
				}`, checkJSONStr)
				body := bytes.NewBuffer([]byte(jsonStr))

				var out structs.ServiceDefinition
				err := decodeBody(body, &out)

				if err != nil {
					t.Fatal(err)
				}

				if err := tc.equalityFn(out.Check, tc.want); err != nil {
					t.Fatal(err)
				}
				if err := tc.equalityFn(out.Checks[0], tc.want); err != nil {
					t.Fatal(err)
				}
			})
		}
	}

}

// RegisterRequest:
// Datacenter	string
// ID	types.NodeID
// Node	string
// Address	string
// TaggedAddresses	map[string]string
// NodeMeta	map[string]string
// Service	*structs.NodeService
//
//	Kind	structs.ServiceKind
//	ID	string
//	Service	string
//	Tags	[]string
//	Address	string
//	TaggedAddresses	map[string]structs.ServiceAddress
//	    Address	string
//	    Port	int
//	Meta	map[string]string
//	Port	int
//	Weights	*structs.Weights
//	    Passing	int
//	    Warning	int
//	EnableTagOverride	bool
//	Proxy	structs.ConnectProxyConfig
//	    DestinationServiceName	string
//	    DestinationServiceID	string
//	    LocalServiceAddress	string
//	    LocalServicePort	int
//	    Config	map[string]interface {}
//	    Upstreams	structs.Upstreams
//	        DestinationType	string
//	        DestinationNamespace	string
//	        DestinationName	string
//	        Datacenter	string
//	        LocalBindAddress	string
//	        LocalBindPort	int
//	        Config	map[string]interface {}
//	        MeshGateway	structs.MeshGatewayConfig
//	            Mode	structs.MeshGatewayMode
//	    MeshGateway	structs.MeshGatewayConfig
//	    Expose	structs.ExposeConfig
//	        Checks	bool
//	        Paths	[]structs.ExposePath
//	            ListenerPort	int
//	            Path	string
//	            LocalPathPort	int
//	            Protocol	string
//	            ParsedFromCheck	bool
//	Connect	structs.ServiceConnect
//	    Native	bool
//	    SidecarService	*structs.ServiceDefinition
//	        Kind	structs.ServiceKind
//	        ID	string
//	        Name	string
//	        Tags	[]string
//	        Address	string
//	        TaggedAddresses	map[string]structs.ServiceAddress
//	        Meta	map[string]string
//	        Port	int
//	        Check	structs.CheckType
//	            CheckID	types.CheckID
//	            Name	string
//	            Status	string
//	            Notes	string
//	            ScriptArgs	[]string
//	            HTTP	string
//	            Header	map[string][]string
//	            Method	string
//	            TCP	string
//	            Interval	time.Duration
//	            AliasNode	string
//	            AliasService	string
//	            DockerContainerID	string
//	            Shell	string
//	            GRPC	string
//	            GRPCUseTLS	bool
//	            TLSServerName	string
//	            TLSSkipVerify	bool
//	            Timeout	time.Duration
//	            TTL	time.Duration
//	            ProxyHTTP	string
//	            ProxyGRPC	string
//	            DeregisterCriticalServiceAfter	time.Duration
//	            OutputMaxSize	int
//	        Checks	structs.CheckTypes
//	        Weights	*structs.Weights
//	        Token	string
//	        EnableTagOverride	bool
//	        Proxy	*structs.ConnectProxyConfig
//	        Connect	*structs.ServiceConnect
//	LocallyRegisteredAsSidecar	bool
//	RaftIndex	structs.RaftIndex
//	    CreateIndex	uint64
//	    ModifyIndex	uint64
//
// Check	*structs.HealthCheck
//
//	Node	string
//	CheckID	types.CheckID
//	Name	string
//	Status	string
//	Notes	string
//	Output	string
//	ServiceID	string
//	ServiceName	string
//	ServiceTags	[]string
//	Definition	structs.HealthCheckDefinition
//	    HTTP	string
//	    TLSServerName	string
//	    TLSSkipVerify	bool
//	    Header	map[string][]string
//	    Method	string
//	    TCP	string
//	    Interval	time.Duration
//	    OutputMaxSize	uint
//	    Timeout	time.Duration
//	    DeregisterCriticalServiceAfter	time.Duration
//	    ScriptArgs	[]string
//	    DockerContainerID	string
//	    Shell	string
//	    GRPC	string
//	    GRPCUseTLS	bool
//	    AliasNode	string
//	    AliasService	string
//	    TTL	time.Duration
//	RaftIndex	structs.RaftIndex
//
// Checks	structs.HealthChecks
// SkipNodeUpdate	bool
// WriteRequest	structs.WriteRequest
//
//	Token	string
func TestDecodeCatalogRegister(t *testing.T) {
	for _, tc := range durationTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			jsonStr := fmt.Sprintf(`{
				"Service": {
					"Connect": {
						"SidecarService": {
							"Check": {
								"Interval": %[1]s,
								"Timeout": %[1]s,
								"TTL": %[1]s,
								"DeregisterCriticalServiceAfter": %[1]s
							}
						}
					}
				},
				"Check": {
					"Definition": {
						"Interval": %[1]s,
						"Timeout": %[1]s,
						"TTL": %[1]s,
						"DeregisterCriticalServiceAfter": %[1]s	
					}
				}
			}`, tc.durations.in)
			body := bytes.NewBuffer([]byte(jsonStr))

			var out structs.RegisterRequest
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			if err != nil && tc.wantErr {
				return // no point continuing
			}

			// Service and Check will be nil if tc.wantErr == true && err != nil.
			// We don't want to panic upon trying to follow a nil pointer, so we
			// check these on a higher level here.
			if out.Service == nil && tc.durations.want != 0 {
				t.Fatalf("Service is nil, expected duration values to be %v", tc.durations.want)
			}
			if out.Check == nil && tc.durations.want != 0 {
				t.Fatalf("Check is nil, expected duration values to be %v", tc.durations.want)
			}
			if out.Service == nil && out.Check == nil {
				return
			}

			// Carry on checking nested fields
			err = checkTypeDurationTest(out.Service.Connect.SidecarService.Check, tc.durations.want, "Service.Connect.SidecarService.Check")
			if err != nil {
				t.Fatal(err)
			}

			err = checkTypeDurationTest(out.Check.Definition, tc.durations.want, "Check.Definition")
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// IntentionRequest:
// Datacenter	string
// Op	structs.IntentionOp
// Intention	*structs.Intention
//
//	ID	string
//	Description	string
//	SourceNS	string
//	SourceName	string
//	DestinationNS	string
//	DestinationName	string
//	SourceType	structs.IntentionSourceType
//	Action	structs.IntentionAction
//	Meta	map[string]string
//	Precedence	int
//	CreatedAt	time.Time	mapstructure:'-'
//	UpdatedAt	time.Time	mapstructure:'-'
//	Hash	[]uint8
//	RaftIndex	structs.RaftIndex
//	    CreateIndex	uint64
//	    ModifyIndex	uint64
//
// WriteRequest	structs.WriteRequest
//
//	Token	string
func TestDecodeIntentionCreate(t *testing.T) {
	for _, tc := range append(hashTestCases, timestampTestCases...) {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			var createdAt, updatedAt, hash = "null", "null", "null"
			if tc.hashes != nil {
				hash = tc.hashes.in
			}
			if tc.timestamps != nil {
				createdAt = tc.timestamps.in
				updatedAt = tc.timestamps.in
			}
			bodyBytes := []byte(fmt.Sprintf(`{
				"CreatedAt": %s,
				"UpdatedAt": %s,
				"Hash": %s
			}`, createdAt, updatedAt, hash))

			body := bytes.NewBuffer(bodyBytes)

			// decode body
			var out structs.Intention
			err := decodeBody(body, &out)

			if tc.hashes != nil {
				// We should only check tc.wantErr for hashes in this case.
				//
				// This is because our CreatedAt and UpdatedAt timestamps have
				// `mapstructure:"-"` tags, so these fields values should always be 0,
				// and not return an error upon decoding (because they are to be ignored
				// all together).

				if err != nil && !tc.wantErr {
					t.Fatal(err)
				}
				if err == nil && tc.wantErr {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatal(err)
			}

			// are we testing hashes in this test case?
			if tc.hashes != nil {
				if !bytes.Equal(out.Hash, tc.hashes.want) {
					t.Fatalf("expected hash to be %s, got %s", tc.hashes.want, out.Hash)
				}
			}
			// are we testing timestamps?
			if tc.timestamps != nil {
				// CreatedAt and UpdatedAt should never be encoded/decoded, so we check
				// that the timestamps are 0 here instead of tc.timestamps.want.
				if !out.CreatedAt.IsZero() {
					t.Fatalf("expected CreatedAt to be zero value, got %s", out.CreatedAt)
				}

				if !out.UpdatedAt.IsZero() {
					t.Fatalf("expected UpdatedAt to be zero value, got %s", out.UpdatedAt)
				}
			}
		})

	}
}

// AutopilotConfiguration:
// CleanupDeadServers	bool
// LastContactThreshold	*api.ReadableDuration
// MaxTrailingLogs	uint64
// ServerStabilizationTime	*api.ReadableDuration
// RedundancyZoneTag	string
// DisableUpgradeMigration	bool
// UpgradeVersionTag	string
// CreateIndex	uint64
// ModifyIndex	uint64
func TestDecodeOperatorAutopilotConfiguration(t *testing.T) {
	for _, tc := range durationTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			jsonStr := fmt.Sprintf(`{
				"LastContactThreshold": %[1]s,
				"ServerStabilizationTime": %[1]s
			}`, tc.durations.in)

			body := bytes.NewBuffer([]byte(jsonStr))

			var out api.AutopilotConfiguration
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			if out.LastContactThreshold == nil {
				if tc.durations.want != 0 {
					t.Fatalf("expected LastContactThreshold to be %v, got nil.", tc.durations.want)
				}
			} else if *out.LastContactThreshold != api.ReadableDuration(tc.durations.want) {
				t.Fatalf("expected LastContactThreshold to be %s, got %s", tc.durations.want, out.LastContactThreshold)
			}

			if out.ServerStabilizationTime == nil {
				if tc.durations.want != 0 {
					t.Fatalf("expected ServerStabilizationTime to be %v, got nil.", tc.durations.want)
				}
			} else if *out.ServerStabilizationTime != api.ReadableDuration(tc.durations.want) {
				t.Fatalf("expected ServerStabilizationTime to be %s, got %s", tc.durations.want, out.ServerStabilizationTime)
			}
		})
	}
}

// SessionRequest:
// Datacenter	string
// Op	structs.SessionOp
// Session	structs.Session
//
//	ID	string
//	Name	string
//	Node	string
//	Checks	[]types.CheckID
//	LockDelay	time.Duration
//	Behavior	structs.SessionBehavior
//	TTL	string
//	RaftIndex	structs.RaftIndex
//	    CreateIndex	uint64
//	    ModifyIndex	uint64
//
// WriteRequest	structs.WriteRequest
//
//	Token	string
func TestDecodeSessionCreate(t *testing.T) {
	// outSession var is shared among test cases b/c of the
	// nature/signature of the FixupChecks callback.
	var outSession structs.Session

	// lockDelayMinThreshold = 1000

	sessionDurationTCs := append(positiveDurationTCs,
		translateValueTestCase{
			desc: "duration small, numeric (< lockDelayMinThreshold)",
			durations: &durationTC{
				in:   `20`,
				want: (20 * time.Second),
			},
		},
		translateValueTestCase{
			desc: "duration string, no unit",
			durations: &durationTC{
				in: `"20"`,
			},
			wantErr: true,
		},
		translateValueTestCase{
			desc: "duration small, string, already duration",
			durations: &durationTC{
				in:   `"20ns"`, //  ns ignored
				want: (20 * time.Second),
			},
		},
		translateValueTestCase{
			desc: "duration small, numeric, negative",
			durations: &durationTC{
				in:   `-5`,
				want: -5 * time.Second,
			},
		},
	)

	for _, tc := range sessionDurationTCs {
		t.Run(tc.desc, func(t *testing.T) {
			// outSession var is shared among test cases b/c of the
			// nature/signature of the FixupChecks callback.
			// Wipe it clean before each test case.
			outSession = structs.Session{}

			// set up request body
			jsonStr := fmt.Sprintf(`{
				"LockDelay": %s
			}`, tc.durations.in)

			body := bytes.NewBuffer([]byte(jsonStr))

			// outSession var is shared among test cases

			err := decodeBody(body, &outSession)
			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			if outSession.LockDelay != tc.durations.want {
				t.Fatalf("expected LockDelay to be %v, got %v", tc.durations.want, outSession.LockDelay)
			}
		})
	}

	checkIDTestCases := []struct {
		desc    string
		in      string
		want    []types.CheckID
		wantErr bool
	}{
		{
			desc: "many check ids",
			in:   `["one", "two", "three"]`,
			want: []types.CheckID{"one", "two", "three"},
		},
		{
			desc: "one check ids",
			in:   `["foo"]`,
			want: []types.CheckID{"foo"},
		},
		{
			desc: "empty check id slice",
			in:   `[]`,
			want: []types.CheckID{},
		},
		{
			desc: "null check ids",
			in:   `null`,
			want: []types.CheckID{},
		},
		{
			desc:    "empty value check ids",
			in:      ``,
			wantErr: true,
		},
		{
			desc:    "malformatted check ids (string)",
			in:      `"one"`,
			wantErr: true,
		},
	}

	for _, tc := range checkIDTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// outSession var is shared among test cases b/c of the
			// nature/signature of the FixupChecks callback.
			// Wipe it clean before each test case.
			outSession = structs.Session{}

			// set up request body
			jsonStr := fmt.Sprintf(`{
				"Checks": %s
			}`, tc.in)

			body := bytes.NewBuffer([]byte(jsonStr))

			err := decodeBody(body, &outSession)
			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}
			if len(outSession.Checks) != len(tc.want) {
				t.Fatalf("expected Checks to be %v, got %v", tc.want, outSession.Checks)
			}
			for i := range outSession.Checks {
				if outSession.Checks[i] != tc.want[i] {
					t.Fatalf("expected Checks to be %v, got %v", tc.want, outSession.Checks)
				}
			}
		})
	}
}

// TxnOps:
//
//	KV	*api.KVTxnOp
//	    Verb	api.KVOp
//	    Key	string
//	    Value	[]uint8
//	    Flags	uint64
//	    Index	uint64
//	    Session	string
//	Node	*api.NodeTxnOp
//	    Verb	api.NodeOp
//	    Node	api.Node
//	        ID	string
//	        Node	string
//	        Address	string
//	        Datacenter	string
//	        TaggedAddresses	map[string]string
//	        Meta	map[string]string
//	        CreateIndex	uint64
//	        ModifyIndex	uint64
//	Service	*api.ServiceTxnOp
//	    Verb	api.ServiceOp
//	    Node	string
//	    Service	api.AgentService
//	        Kind	api.ServiceKind
//	        ID	string
//	        Service	string
//	        Tags	[]string
//	        Meta	map[string]string
//	        Port	int
//	        Address	string
//	        TaggedAddresses	map[string]api.ServiceAddress
//	            Address	string
//	            Port	int
//	        Weights	api.AgentWeights
//	            Passing	int
//	            Warning	int
//	        EnableTagOverride	bool
//	        CreateIndex	uint64
//	        ModifyIndex	uint64
//	        ContentHash	string
//	        Proxy	*api.AgentServiceConnectProxyConfig
//	            DestinationServiceName	string
//	            DestinationServiceID	string
//	            LocalServiceAddress	string
//	            LocalServicePort	int
//	            Config	map[string]interface {}
//	            Upstreams	[]api.Upstream
//	                DestinationType	api.UpstreamDestType
//	                DestinationNamespace	string
//	                DestinationName	string
//	                Datacenter	string
//	                LocalBindAddress	string
//	                LocalBindPort	int
//	                Config	map[string]interface {}
//	                MeshGateway	api.MeshGatewayConfig
//	                    Mode	api.MeshGatewayMode
//	            MeshGateway	api.MeshGatewayConfig
//	            Expose	api.ExposeConfig
//	                Checks	bool
//	                Paths	[]api.ExposePath
//	                    ListenerPort	int
//	                    Path	string
//	                    LocalPathPort	int
//	                    Protocol	string
//	                    ParsedFromCheck	bool
//	        Connect	*api.AgentServiceConnect
//	            Native	bool
//	            SidecarService	*api.AgentServiceRegistration
//	                Kind	api.ServiceKind
//	                ID	string
//	                Name	string
//	                Tags	[]string
//	                Port	int
//	                Address	string
//	                TaggedAddresses	map[string]api.ServiceAddress
//	                EnableTagOverride	bool
//	                Meta	map[string]string
//	                Weights	*api.AgentWeights
//	                Check	*api.AgentServiceCheck
//	                    CheckID	string
//	                    Name	string
//	                    Args	[]string
//	                    DockerContainerID	string
//	                    Shell	string
//	                    Interval	string
//	                    Timeout	string
//	                    TTL	string
//	                    HTTP	string
//	                    Header	map[string][]string
//	                    Method	string
//	                    TCP	string
//	                    Status	string
//	                    Notes	string
//	                    TLSServerName	string
//	                    TLSSkipVerify	bool
//	                    GRPC	string
//	                    GRPCUseTLS	bool
//	                    AliasNode	string
//	                    AliasService	string
//	                    DeregisterCriticalServiceAfter	string
//	                Checks	api.AgentServiceChecks
//	                Proxy	*api.AgentServiceConnectProxyConfig
//	                Connect	*api.AgentServiceConnect
//	Check	*api.CheckTxnOp
//	    Verb	api.CheckOp
//	    Check	api.HealthCheck
//	        Node	string
//	        CheckID	string
//	        Name	string
//	        Status	string
//	        Notes	string
//	        Output	string
//	        ServiceID	string
//	        ServiceName	string
//	        ServiceTags	[]string
//	        Definition	api.HealthCheckDefinition
//	            HTTP	string
//	            Header	map[string][]string
//	            Method	string
//	            Body	string
//	            TLSServerName	string
//	            TLSSkipVerify	bool
//	            TCP	string
//	            IntervalDuration	time.Duration
//	            TimeoutDuration	time.Duration
//	            DeregisterCriticalServiceAfterDuration	time.Duration
//	            Interval	api.ReadableDuration
//	            Timeout	api.ReadableDuration
//	            DeregisterCriticalServiceAfter	api.ReadableDuration
//	        CreateIndex	uint64
//	        ModifyIndex	uint64
func TestDecodeTxnConvertOps(t *testing.T) {
	for _, tc := range durationTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			// set up request body
			jsonStr := fmt.Sprintf(`[{
				"Check": {
					"Check": {
						"Definition": {
							"IntervalDuration": %[1]s,
							"TimeoutDuration": %[1]s,
							"DeregisterCriticalServiceAfterDuration": %[1]s,
							"Interval": %[1]s,
							"Timeout": %[1]s,
							"DeregisterCriticalServiceAfter": %[1]s
						}
					}
				}
			}]`, tc.durations.in)

			body := bytes.NewBuffer([]byte(jsonStr))

			var out api.TxnOps
			err := decodeBody(body, &out)

			if err == nil && tc.wantErr {
				t.Fatal("expected err, got nil")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("expected nil error, got %v", err)
			}

			// Check will be nil if we want an error and got one (tc.wantErr == true && err != nil).
			// We don't want to panic dereferencing a nil pointer, so we
			// check this on a higher level here.
			if out == nil || out[0] == nil {
				if tc.durations.want != 0 {
					t.Fatalf("Check is nil, expected duration values to be %v", tc.durations.want)
				}
				return
			}

			outCheck := out[0].Check.Check.Definition
			if outCheck.IntervalDuration != tc.durations.want {
				t.Fatalf("expected IntervalDuration to be %v, got %v", tc.durations.want, outCheck.IntervalDuration)
			}
			if outCheck.TimeoutDuration != tc.durations.want {
				t.Fatalf("expected TimeoutDuration to be %v, got %v", tc.durations.want, outCheck.TimeoutDuration)
			}
			if outCheck.DeregisterCriticalServiceAfterDuration != tc.durations.want {
				t.Fatalf("expected DeregisterCriticalServiceAfterDuration to be %v, got %v", tc.durations.want, outCheck.DeregisterCriticalServiceAfterDuration)
			}

			if outCheck.Interval != api.ReadableDuration(tc.durations.want) {
				t.Fatalf("expected Interval to be %v, got %v", tc.durations.want, outCheck.Interval)
			}
			if outCheck.Timeout != api.ReadableDuration(tc.durations.want) {
				t.Fatalf("expected Timeout to be %v, got %v", tc.durations.want, outCheck.Timeout)
			}
			if outCheck.DeregisterCriticalServiceAfter != api.ReadableDuration(tc.durations.want) {
				t.Fatalf("expected DeregisterCriticalServiceAfter to be %v, got %v", tc.durations.want, outCheck.DeregisterCriticalServiceAfter)
			}
		})
	}
}

// =========================================
// Helper funcs:
// =========================================

// checkTypeDurationTest is a helper func to test durations in CheckTYpe or CheckDefiniton
// (to reduce repetetive typing).
func checkTypeDurationTest(check interface{}, want time.Duration, prefix string) error {
	// check for pointers first
	switch v := check.(type) {
	case *structs.CheckType:
		check = *v
	case *structs.CheckDefinition:
		check = *v
	case *structs.HealthCheckDefinition:
		check = *v
	}

	var interval, timeout, ttl, deregister time.Duration
	switch v := check.(type) {
	case structs.CheckType:
		interval = v.Interval
		timeout = v.Timeout
		ttl = v.TTL
		deregister = v.DeregisterCriticalServiceAfter
	case structs.CheckDefinition:
		interval = v.Interval
		timeout = v.Timeout
		ttl = v.TTL
		deregister = v.DeregisterCriticalServiceAfter
	case structs.HealthCheckDefinition:
		interval = v.Interval
		timeout = v.Timeout
		ttl = v.TTL
		deregister = v.DeregisterCriticalServiceAfter
	default:
		panic(fmt.Sprintf("unexpected type %T", check))
	}

	if interval != want {
		return fmt.Errorf("%s expected Check.Interval to be %s, got %s", prefix, want, interval)
	}
	if timeout != want {
		return fmt.Errorf("%s expected Check.Timeout to be %s, got %s", prefix, want, timeout)
	}
	if ttl != want {
		return fmt.Errorf("%s expected Check.TTL to be %s, got %s", prefix, want, ttl)
	}
	if deregister != want {
		return fmt.Errorf("%s expected Check.DeregisterCriticalServiceAfter to be %s, got %s", prefix, want, deregister)
	}
	return nil
}

// checkTypeDurationTest is a helper func to test the Header map in a CheckType or CheckDefiniton
// (to reduce repetetive typing).
func checkTypeHeaderTest(check interface{}, want map[string][]string) error {

	var header map[string][]string
	switch v := check.(type) {
	case structs.CheckType:
		header = v.Header
	case *structs.CheckType:
		header = v.Header
	case structs.CheckDefinition:
		header = v.Header
	case *structs.CheckDefinition:
		header = v.Header
	}
	for wantk, wantvs := range want {
		if len(header[wantk]) != len(wantvs) {
			return fmt.Errorf("expected Header to be %v, got %v", want, header)
		}
		for i, wantv := range wantvs {
			if header[wantk][i] != wantv {
				return fmt.Errorf("expected Header to be %v, got %v", want, header)
			}
		}
	}
	return nil
}
