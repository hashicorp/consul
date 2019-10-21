package agent

import (
    "bytes"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/hashicorp/consul/agent/structs"
    "github.com/hashicorp/consul/api"
    "github.com/hashicorp/consul/types"
    "github.com/hashicorp/serf/coordinate"
    "github.com/stretchr/testify/assert"
)

// Test cases generated with help from github.com/bxcodec/faker/v3

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint.go:
//   327    s.parseToken(req, &args.Token)
//   328
//   329:   if err := decodeBody(req, &args.Policy, fixTimeAndHashFields); err != nil {
//   330        return nil, BadRequestError{Reason: fmt.Sprintf("Policy decoding failed: %v", err)}
//   331    }
// ==================================

// ACLPolicySetRequest:
// Policy   structs.ACLPolicy
//     ID   string
//     Name string
//     Description  string
//     Rules    string
//     Syntax   acl.SyntaxVersion
//     Datacenters  []string
//     Hash []uint8
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// Datacenter   string
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityACLPolicyWrite(t *testing.T) {

    jsonBlob := `{
    "ID": "kcBeYqTJqIhuSWHCjpiecwdqu",
    "Name": "pSdFokHZkWfOhCbqmCrVQDwNA",
    "Description": "wPTmQZVmhdAlVjVMoTCUVsbPB",
    "Rules": "SGgOHhzRnAUJnSCVeeqHQgOPz",
    "Datacenters": [
        "kySKHQmOafpnwXcOzQHTIkCwX",
        "UYDfHxkatDNxufMSEWXswlHXm"
    ],
    "Hash": "Pw==",
    "CreateIndex": 67,
    "ModifyIndex": 16
}`
    // ------
    want := structs.ACLPolicy{
        ID:          "kcBeYqTJqIhuSWHCjpiecwdqu",
        Name:        "pSdFokHZkWfOhCbqmCrVQDwNA",
        Description: "wPTmQZVmhdAlVjVMoTCUVsbPB",
        Rules:       "SGgOHhzRnAUJnSCVeeqHQgOPz",
        Datacenters: []string{
            "kySKHQmOafpnwXcOzQHTIkCwX",
            "UYDfHxkatDNxufMSEWXswlHXm",
        },
        Hash: []byte("Pw=="),
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACLPolicy{}
    if err := decodeBody(req, &out, fixTimeAndHashFields); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint.go:
//   511    s.parseToken(req, &args.Token)
//   512
//   513:   if err := decodeBody(req, &args.ACLToken, fixTimeAndHashFields); err != nil {
//   514        return nil, BadRequestError{Reason: fmt.Sprintf("Token decoding failed: %v", err)}
//   515    }
// ==================================

// ACLTokenSetRequest:
// ACLToken structs.ACLToken
//     AccessorID   string
//     SecretID string
//     Description  string
//     Policies []structs.ACLTokenPolicyLink
//         ID   string
//         Name string
//     Roles    []structs.ACLTokenRoleLink
//         ID   string
//         Name string
//     ServiceIdentities    []*structs.ACLServiceIdentity
//         ServiceName  string
//         Datacenters  []string
//     Type string
//     Rules    string
//     Local    bool
//     AuthMethod   string
//     ExpirationTime   *time.Time
//     ExpirationTTL    time.Duration
//     CreateTime   time.Time
//     Hash []uint8
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// Create   bool
// Datacenter   string
// WriteRequest structs.WriteRequest
//     Token    string

func TestDecodeSanityACLToken(t *testing.T) {

    jsonBlob := `{
    "AccessorID": "ZkZiJHEevNXieErECcnDFpBKT",
    "SecretID": "HqfVWQENRLnBrgKnzfTgLGBRf",
    "Description": "QGhpppbLTCtoXCRPoIKBBenrQ",
    "Policies": [
        {
            "ID": "1eTEeYUsOmICjQCdvtdHJXHyVZ",
            "Name": "1OohIEGpNzJRohkObYVdORTwsi"
        },
        {
            "ID": "1EpdmrVFuUFyJYdBblgSqBBbKk",
            "Name": "1dVeOeiaWAievLrMNTTJgclWbJ"
        }
    ],
    "Roles": [
        {
            "ID": "zrUjuFSRFxaOwgfnasCLEqQgq",
            "Name": "YAtupcIOkpgxsZFwrbXqmZmxs"
        },
        {
            "ID": "zDqDnELtwWmtGqkznEgKUtRqF",
            "Name": "VZgqisDbFqrlfAlOpTaubVNCO"
        },
        {
            "ID": "eTEeYUsOmICjQCdvtdHJXHyVZ",
            "Name": "OohIEGpNzJRohkObYVdORTwsi"
        },
        {
            "ID": "EpdmrVFuUFyJYdBblgSqBBbKk",
            "Name": "dVeOeiaWAievLrMNTTJgclWbJ"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "1EpdmrVFuUFyJYdBblgSqBBbKk",
            "Datacenters": ["1", "2"]
        }
    ],
    "Rules": "DrLkIucfvnaKeKPaVTPNtBgxV",
    "Local": false,
    "AuthMethod": "lCXxfNKLRNorGKcJNqGRebLsJ",
    "ExpirationTime": "2124-04-17T11:03:15.0Z",
    "ExpirationTTL": 68,
    "CreateTime": "2308-06-04T04:53:33.0Z",
    "Hash": "NA==",
    "CreateIndex": 73,
    "ModifyIndex": 59
}`
    // ------
    want := structs.ACLToken{
        AccessorID:  "ZkZiJHEevNXieErECcnDFpBKT",
        SecretID:    "HqfVWQENRLnBrgKnzfTgLGBRf",
        Description: "QGhpppbLTCtoXCRPoIKBBenrQ",
        Policies: []structs.ACLTokenPolicyLink{
            {
                ID:   "1eTEeYUsOmICjQCdvtdHJXHyVZ",
                Name: "1OohIEGpNzJRohkObYVdORTwsi",
            },
            {
                ID:   "1EpdmrVFuUFyJYdBblgSqBBbKk",
                Name: "1dVeOeiaWAievLrMNTTJgclWbJ",
            },
        },
        Roles: []structs.ACLTokenRoleLink{
            {
                ID:   "zrUjuFSRFxaOwgfnasCLEqQgq",
                Name: "YAtupcIOkpgxsZFwrbXqmZmxs",
            },
            {
                ID:   "zDqDnELtwWmtGqkznEgKUtRqF",
                Name: "VZgqisDbFqrlfAlOpTaubVNCO",
            },
            {
                ID:   "eTEeYUsOmICjQCdvtdHJXHyVZ",
                Name: "OohIEGpNzJRohkObYVdORTwsi",
            },
            {
                ID:   "EpdmrVFuUFyJYdBblgSqBBbKk",
                Name: "dVeOeiaWAievLrMNTTJgclWbJ",
            },
        },
        ServiceIdentities: []*structs.ACLServiceIdentity{
            {
                ServiceName: "1EpdmrVFuUFyJYdBblgSqBBbKk",
                Datacenters: []string{"1", "2"},
            },
        },
        Rules:          "DrLkIucfvnaKeKPaVTPNtBgxV",
        Local:          false,
        AuthMethod:     "lCXxfNKLRNorGKcJNqGRebLsJ",
        ExpirationTime: timePtr(time.Date(2124, 4, 17, 11, 3, 15, 0, time.UTC)),
        ExpirationTTL:  68,
        CreateTime:     time.Date(2308, 6, 04, 4, 53, 33, 0, time.UTC),
        Hash:           []byte("NA=="),
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACLToken{}
    if err := decodeBody(req, &out, fixTimeAndHashFields); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint.go:
//   555    }
//   556
//   557:   if err := decodeBody(req, &args.ACLToken, fixTimeAndHashFields); err != nil && err.Error() != "EOF" {
//   558        return nil, BadRequestError{Reason: fmt.Sprintf("Token decoding failed: %v", err)}
//   559    }
// ==================================
func TestDecodeSanityACLTokenClone(t *testing.T) {

    jsonBlob := `{
    "AccessorID": "cZPMuiWCiRKtLNbJIaxemIkKS",
    "SecretID": "mmOyWIkMsLyenYEzPgjWcemiC",
    "Description": "AIHZCBEaqNKhdrPMdiRBcYNzD",
    "Roles": [
        {
            "ID": "gZbJMUyIzZQGixugZgySuWxQX",
            "Name": "ItwDhZSFabKtVKnFTgzIGGJxs"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "BEcypZIbSCAvAFzIQpzgXqdKJ",
            "Datacenters": [
                "OayvVkTRGojgpIiRoBJefajMC"
            ]
        }
    ],
    "Rules": "NnXfbOWiLtBvhJSMZcvxStRmu",
    "Local": true,
    "AuthMethod": "AmdwKmsDaYTOIvdNMRWZkHcNu",
    "ExpirationTime": "2151-08-25T00:55:19.0Z",
    "ExpirationTTL": 41,
    "CreateTime": "2111-07-07T00:25:22.0Z",
    "Hash": "PzI=",
    "CreateIndex": 31,
    "ModifyIndex": 69
}`
    // ------
    want := structs.ACLToken{

        AccessorID:  "cZPMuiWCiRKtLNbJIaxemIkKS",
        SecretID:    "mmOyWIkMsLyenYEzPgjWcemiC",
        Description: "AIHZCBEaqNKhdrPMdiRBcYNzD",
        Roles: []structs.ACLTokenRoleLink{
            {
                ID:   "gZbJMUyIzZQGixugZgySuWxQX",
                Name: "ItwDhZSFabKtVKnFTgzIGGJxs",
            },
        },
        ServiceIdentities: []*structs.ACLServiceIdentity{
            {
                ServiceName: "BEcypZIbSCAvAFzIQpzgXqdKJ",
                Datacenters: []string{
                    "OayvVkTRGojgpIiRoBJefajMC",
                },
            },
        },
        Rules:          "NnXfbOWiLtBvhJSMZcvxStRmu",
        Local:          true,
        AuthMethod:     "AmdwKmsDaYTOIvdNMRWZkHcNu",
        ExpirationTime: timePtr(time.Date(2151, 8, 25, 0, 55, 19, 0, time.UTC)),
        ExpirationTTL:  41,
        CreateTime:     time.Date(2111, 7, 07, 0, 25, 22, 0, time.UTC),
        Hash:           []byte("PzI="),
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACLToken{}
    if err := decodeBody(req, &out, fixTimeAndHashFields); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint.go:
//   689    s.parseToken(req, &args.Token)
//   690
//   691:   if err := decodeBody(req, &args.Role, fixTimeAndHashFields); err != nil {
//   692        return nil, BadRequestError{Reason: fmt.Sprintf("Role decoding failed: %v", err)}
//   693    }
// ==================================

// ACLRoleSetRequest:
// Role structs.ACLRole
//     ID   string
//     Name string
//     Description  string
//     Policies []structs.ACLRolePolicyLink
//         ID   string
//         Name string
//     ServiceIdentities    []*structs.ACLServiceIdentity
//         ServiceName  string
//         Datacenters  []string
//     Hash []uint8
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// Datacenter   string
// WriteRequest structs.WriteRequest
//     Token    string

func TestDecodeSanityACLRoleWrite(t *testing.T) {

    jsonBlob := `{
    "ID": "TBYwpVJhVEgZhvSPLwRCFLwFR",
    "Name": "PzHsOCPNHSPpywxUtoywwgodb",
    "Description": "fLfpgmvaMzgdTKGAJOHHERqxw",
    "Policies": [
        {
            "ID": "QOThtPknsLJHstbVTvVEUHWqc",
            "Name": "whahuwCeeMIxJYuPHHlNWibCg"
        }
    ],
    "ServiceIdentities": [
        {
            "ServiceName": "dHDSZRNSZMSWeiLqJyzulrjHn"
        }
    ],
    "Hash": "TAY=",
    "CreateIndex": 74,
    "ModifyIndex": 92
}`
    // ------
    want := structs.ACLRole{
        ID:          "TBYwpVJhVEgZhvSPLwRCFLwFR",
        Name:        "PzHsOCPNHSPpywxUtoywwgodb",
        Description: "fLfpgmvaMzgdTKGAJOHHERqxw",
        Policies: []structs.ACLRolePolicyLink{
            {
                ID:   "QOThtPknsLJHstbVTvVEUHWqc",
                Name: "whahuwCeeMIxJYuPHHlNWibCg",
            },
        },
        ServiceIdentities: []*structs.ACLServiceIdentity{
            {
                ServiceName: "dHDSZRNSZMSWeiLqJyzulrjHn",
            },
        },
        Hash: []byte("TAY="),
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACLRole{}
    if err := decodeBody(req, &out, fixTimeAndHashFields); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint.go:
//   822    s.parseToken(req, &args.Token)
//   823
//   824:   if err := decodeBody(req, &args.BindingRule, fixTimeAndHashFields); err != nil {
//   825        return nil, BadRequestError{Reason: fmt.Sprintf("BindingRule decoding failed: %v", err)}
//   826    }
// ==================================
//
// ACLBindingRuleSetRequest:
// BindingRule  structs.ACLBindingRule
//     ID   string
//     Description  string
//     AuthMethod   string
//     Selector string
//     BindType string
//     BindName string
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// Datacenter   string
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityACLBindingRuleWrite(t *testing.T) {

    jsonBlob := `{
    "ID": "BCBUtfPTvtYirtgsQZCWxLJeh",
    "Description": "dAFxwbWIqFtfwwkYTuyWdIcrj",
    "AuthMethod": "RETfQfaqRPOhaSlULvyhnwZbZ",
    "Selector": "xwjvTkRbiqUzbsxwuuzSLFexr",
    "BindType": "SbIRVNEWjNCSGYipptJINQUiN",
    "BindName": "amyZGXAeVbXiFMxKKHEuEOYHP",
    "CreateIndex": 72,
    "ModifyIndex": 18
}`
    // ------
    want := structs.ACLBindingRule{
        ID:          "BCBUtfPTvtYirtgsQZCWxLJeh",
        Description: "dAFxwbWIqFtfwwkYTuyWdIcrj",
        AuthMethod:  "RETfQfaqRPOhaSlULvyhnwZbZ",
        Selector:    "xwjvTkRbiqUzbsxwuuzSLFexr",
        BindType:    "SbIRVNEWjNCSGYipptJINQUiN",
        BindName:    "amyZGXAeVbXiFMxKKHEuEOYHP",
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACLBindingRule{}
    if err := decodeBody(req, &out, fixTimeAndHashFields); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint.go:
//   954    s.parseToken(req, &args.Token)
//   955
//   956:   if err := decodeBody(req, &args.AuthMethod, fixTimeAndHashFields); err != nil {
//   957        return nil, BadRequestError{Reason: fmt.Sprintf("AuthMethod decoding failed: %v", err)}
//   958    }
// ==================================
// ACLAuthMethodSetRequest:
// AuthMethod   structs.ACLAuthMethod
//     Name string
//     Type string
//     Description  string
//     Config   map[string]interface {} `faker:"-"`
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// Datacenter   string
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityACLAuthMethodWrite(t *testing.T) {

    jsonBlob := `{
    "Name": "hoyvJdIUZrIFXRHDMXsAPDxVM",
    "Type": "NRbHrtHJrjqhqWBXcCnJHAPYn",
    "Description": "zvINFriAnLNKgZClUYJpoaqtx",
    "Config": {"key": ["a", "b"]},
    "CreateIndex": 78,
    "ModifyIndex": 48
}`
    // ------
    want := structs.ACLAuthMethod{
        Name:        "hoyvJdIUZrIFXRHDMXsAPDxVM",
        Type:        "NRbHrtHJrjqhqWBXcCnJHAPYn",
        Description: "zvINFriAnLNKgZClUYJpoaqtx",
        Config:      map[string]interface{}{"key": []interface{}{"a", "b"}},
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACLAuthMethod{}
    if err := decodeBody(req, &out, fixTimeAndHashFields); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint.go:
//  1000    s.parseDC(req, &args.Datacenter)
//  1001
//  1002:   if err := decodeBody(req, &args.Auth, nil); err != nil {
//  1003        return nil, BadRequestError{Reason: fmt.Sprintf("Failed to decode request body:: %v", err)}
//  1004    }
// ==================================
// ACLLoginRequest:
// Auth *structs.ACLLoginParams
//     AuthMethod   string
//     BearerToken  string
//     Meta map[string]string
// Datacenter   string
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityACLLogin(t *testing.T) {

    jsonBlob := `{
    "AuthMethod": "TkBNmSTcIzTGHxdGzLVHYvktQ",
    "BearerToken": "vAwUiPGBdrkJEcUvayYYcNlon",
    "Meta": {
        "BRMPOlWrKQHtgHimVhvbDDzQh": "MbsJZgSSBcgXwBtJGVoKeboRE",
        "pNwXEpxWElaSezYjjeHaTPKeV": "DkDRdfjBBOAdPpymMhTUTDsPC"
    }
}`
    // ------
    want := structs.ACLLoginParams{
        AuthMethod:  "TkBNmSTcIzTGHxdGzLVHYvktQ",
        BearerToken: "vAwUiPGBdrkJEcUvayYYcNlon",
        Meta: map[string]string{
            "BRMPOlWrKQHtgHimVhvbDDzQh": "MbsJZgSSBcgXwBtJGVoKeboRE",
            "pNwXEpxWElaSezYjjeHaTPKeV": "DkDRdfjBBOAdPpymMhTUTDsPC",
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACLLoginParams{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/acl_endpoint_legacy.go:
//    66    // Handle optional request body
//    67    if req.ContentLength > 0 {
//    68:       if err := decodeBody(req, &args.ACL, nil); err != nil {
//    69            resp.WriteHeader(http.StatusBadRequest)
//    70            fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
//
// ACLRequest:
// Datacenter   string
// Op   structs.ACLOp
// ACL  structs.ACL
//     ID   string
//     Name string
//     Type string
//     Rules    string
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityACLUpdate(t *testing.T) {

    jsonBlob := `{
    "ID": "rMdNoVfPTvhtKeQnUpOZVhfRo",
    "Name": "gIXhaQDvmFZAUWXVwZDeBYCMi",
    "Type": "UOnlImseToIhZhyTDagqidKfB",
    "Rules": "EjqhIyImTMcTZsMANCwzJpeFE",
    "CreateIndex": 43,
    "ModifyIndex": 49
}`
    // ------
    want := structs.ACL{
        ID:    "rMdNoVfPTvhtKeQnUpOZVhfRo",
        Name:  "gIXhaQDvmFZAUWXVwZDeBYCMi",
        Type:  "UOnlImseToIhZhyTDagqidKfB",
        Rules: "EjqhIyImTMcTZsMANCwzJpeFE",
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ACL{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/agent_endpoint.go:
//   461        return FixupCheckType(raw)
//   462    }
//   463:   if err := decodeBody(req, &args, decodeCB); err != nil {
//   464        resp.WriteHeader(http.StatusBadRequest)
//   465        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// CheckDefinition:
// ID   types.CheckID
// Name string
// Notes    string
// ServiceID    string
// Token    string
// Status   string
// ScriptArgs   []string
// HTTP string
// Header   map[string][]string
// Method   string
// TCP  string
// Interval time.Duration
// DockerContainerID    string
// Shell    string
// GRPC string
// GRPCUseTLS   bool
// TLSSkipVerify    bool
// AliasNode    string
// AliasService string
// Timeout  time.Duration
// TTL  time.Duration
// DeregisterCriticalServiceAfter   time.Duration
// OutputMaxSize    int
// ==========
// decodeCB == FixupCheckType
func TestDecodeSanityAgentRegisterCheck(t *testing.T) {

    jsonBlob := `{
    "ID": "ZeFDFJUBkizuvzwLtNaCYspVD",
    "Name": "vMbCbZjydNrRHZCIdRCTbCcoK",
    "Notes": "JSeziDFZWUvjYwbznywxQhYkv",
    "ServiceID": "RMEpDOCwWohxktrYGKslUaqwo",
    "Token": "KHqYcaPfZaPAAlvPppzcNfNEa",
    "Status": "EapNlvjdozKkkKIjLghsIWwII",
    "ScriptArgs": ["a"],
    "HTTP": "GHvRJwidcWzOrFzxyBYCThPfo",
    "Header": {
        "EUvAgWCXcbKpVetrNsIwQbUWJ": [
            "nCeqFVnXafCsCYOmepiArfAxN",
            "IIZbWUAESDeHRcmrflrhAzFWU"
        ],
        "YovhbzhZdrDAeQYyOWFrDwRfv": [
            "kueQfwmjhwRGElgnyrIVzwxem",
            "yvltyxkEQTBeBBtiPpjErAlZa",
            "RrmtTmylAQvMuMYxMozYqZCBC",
            "TByeqVojhAPlvCEISHTZbtyOX"
        ],
        "ajrIRnNfVhRiuIBJyobKHgiTW": [],
        "fRlEbpHWifoDERjfhUZVjfgNk": [
            "kCkbgYfZevLytxhMcliHmpXkF",
            "xvhdSkrxrYMAHVsLrzrEltYgp"
        ]
    },
    "Method": "DzFNUTbnuaIbhKrVQhDnCKxWT",
    "TCP": "iGszKOboHvhSiQUpBxePVPgKC",
    "Interval": 37,
    "DockerContainerID": "TXMHqeEgigtFLyOcROHhTrpSp",
    "Shell": "mvegRGXPbPypECAqxCnMURCLb",
    "GRPC": "rvrwBKEYFGKcNPTtoATHqeXzH",
    "GRPCUseTLS": true,
    "TLSSkipVerify": false,
    "AliasNode": "NOLRyjnUfTtpCcgHfZByQNGdB",
    "AliasService": "YqANEJoeWKlqsCDVVIIVwcWop",
    "Timeout": 8,
    "TTL": 0,
    "DeregisterCriticalServiceAfter": 53,
    "OutputMaxSize": 9
}`
    // ------
    want := structs.CheckDefinition{
        ID:         "ZeFDFJUBkizuvzwLtNaCYspVD",
        Name:       "vMbCbZjydNrRHZCIdRCTbCcoK",
        Notes:      "JSeziDFZWUvjYwbznywxQhYkv",
        ServiceID:  "RMEpDOCwWohxktrYGKslUaqwo",
        Token:      "KHqYcaPfZaPAAlvPppzcNfNEa",
        Status:     "EapNlvjdozKkkKIjLghsIWwII",
        ScriptArgs: []string{"a"},
        HTTP:       "GHvRJwidcWzOrFzxyBYCThPfo",
        Header: map[string][]string{
            "EUvAgWCXcbKpVetrNsIwQbUWJ": []string{
                "nCeqFVnXafCsCYOmepiArfAxN",
                "IIZbWUAESDeHRcmrflrhAzFWU",
            },
            "YovhbzhZdrDAeQYyOWFrDwRfv": []string{
                "kueQfwmjhwRGElgnyrIVzwxem",
                "yvltyxkEQTBeBBtiPpjErAlZa",
                "RrmtTmylAQvMuMYxMozYqZCBC",
                "TByeqVojhAPlvCEISHTZbtyOX",
            },
            "fRlEbpHWifoDERjfhUZVjfgNk": []string{
                "kCkbgYfZevLytxhMcliHmpXkF",
                "xvhdSkrxrYMAHVsLrzrEltYgp",
            },
        },
        Method:                         "DzFNUTbnuaIbhKrVQhDnCKxWT",
        TCP:                            "iGszKOboHvhSiQUpBxePVPgKC",
        Interval:                       37,
        DockerContainerID:              "TXMHqeEgigtFLyOcROHhTrpSp",
        Shell:                          "mvegRGXPbPypECAqxCnMURCLb",
        GRPC:                           "rvrwBKEYFGKcNPTtoATHqeXzH",
        GRPCUseTLS:                     true,
        TLSSkipVerify:                  false,
        AliasNode:                      "NOLRyjnUfTtpCcgHfZByQNGdB",
        AliasService:                   "YqANEJoeWKlqsCDVVIIVwcWop",
        Timeout:                        8,
        TTL:                            0,
        DeregisterCriticalServiceAfter: 53,
        OutputMaxSize:                  9,
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.CheckDefinition{}
    if err := decodeBody(req, &out, FixupCheckType); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/agent_endpoint.go:
//   603  func (s *HTTPServer) AgentCheckUpdate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
//   604    var update checkUpdate
//   605:   if err := decodeBody(req, &update, nil); err != nil {
//   606        resp.WriteHeader(http.StatusBadRequest)
//   607        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// type checkUpdate struct {
//  Status string
//  Output string
// }
func TestDecodeSanityAgentCheckUpdate(t *testing.T) {

    jsonBlob := `{
    "Status": "CvXtUzSapDGfFYpKICVsuSXtM",
    "Output": "PPoGMNURWfGwYAzxSrnPUsFkq"
}`
    // ------
    want := checkUpdate{
        Status: "CvXtUzSapDGfFYpKICVsuSXtM",
        Output: "PPoGMNURWfGwYAzxSrnPUsFkq",
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := checkUpdate{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/agent_endpoint.go:
//   822        return nil
//   823    }
//   824:   if err := decodeBody(req, &args, decodeCB); err != nil {
//   825        resp.WriteHeader(http.StatusBadRequest)
//   826        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
//
// decodeCB:
// -----------
// 1. lib.TranslateKeys()
// 2. FixupCheckType
//    a. lib.TranslateKeys()
//    b. parseDuration()
//    c. parseHeaderMap()
//
//
// Type fields:
// -----------
// ServiceDefinition:
// Kind structs.ServiceKind
// ID   string
// Name string
// Tags []string
// Address  string
// TaggedAddresses  map[string]structs.ServiceAddress
//     Address  string
//     Port int
// Meta map[string]string
// Port int
// Check    structs.CheckType
//     CheckID  types.CheckID
//     Name string
//     Status   string
//     Notes    string
//     ScriptArgs   []string
//     HTTP string
//     Header   map[string][]string
//     Method   string
//     TCP  string
//     Interval time.Duration
//     AliasNode    string
//     AliasService string
//     DockerContainerID    string
//     Shell    string
//     GRPC string
//     GRPCUseTLS   bool
//     TLSSkipVerify    bool
//     Timeout  time.Duration
//     TTL  time.Duration
//     ProxyHTTP    string
//     ProxyGRPC    string
//     DeregisterCriticalServiceAfter   time.Duration
//     OutputMaxSize    int
// Checks   structs.CheckTypes
// Weights  *structs.Weights
//     Passing  int
//     Warning  int
// Token    string
// EnableTagOverride    bool
// Proxy    *structs.ConnectProxyConfig
//     DestinationServiceName   string
//     DestinationServiceID string
//     LocalServiceAddress  string
//     LocalServicePort int
//     Config   map[string]interface {} `faker:"-"`
//     Upstreams    structs.Upstreams
//         DestinationType  string
//         DestinationNamespace string
//         DestinationName  string
//         Datacenter   string
//         LocalBindAddress string
//         LocalBindPort    int
//         Config   map[string]interface {} `faker:"-"`
//         MeshGateway  structs.MeshGatewayConfig
//             Mode structs.MeshGatewayMode
//     MeshGateway  structs.MeshGatewayConfig
//     Expose   structs.ExposeConfig
//         Checks   bool
//         Paths    []structs.ExposePath
//             ListenerPort int
//             Path string
//             LocalPathPort    int
//             Protocol string
//             ParsedFromCheck  bool
// Connect  *structs.ServiceConnect
//     Native   bool
//     SidecarService   *structs.ServiceDefinition `faker:"-"`
func TestDecodeSanityAgentRegisterService(t *testing.T) {
    //t.Skip("need decode callback from previous PR")
    jsonBlob := `{
        "Kind": "fAaUEQiqbqiCmCihBgdPPnhKm",
        "ID": "ItidJtWvgHLwAexuCODOkFMSX",
        "Name": "QkOOHwMapZGzrFtYnntkVodMK",
        "Tags": [
            "aVDuXWWtUlPcNbmtDpvZKyYjH",
            "jVbvxFTtSUhuWIaeyojtsBMcJ"
        ],
        "Address": "oMxcCabMpKmZLACtMMEOJddqc",
        "TaggedAddresses": {
            "bWqvZvHxHeNrjrPFNPhsdNVtM": {
                "Address": "pnAJuDcuqbjgRKCYojJpEOfxU",
                "Port": 42
            }
        },
        "Meta": {
            "GFYZPqpbaPRgivNTkXzKOQIfr": "ceTMIRiNZqhUYRsBwGjSJioIQ"
        },
        "Port": 84,
        "Check": {
            "CheckID": "TkKeoQZfvXDauRVcMHjXbWwGe",
            "Name": "ybaKpHMYkqhhoMDLrSxNdCsrR",
            "Status": "ymceFURxSxrgGsQuaXxSJedhf",
            "Notes": "SchHKnkdxamOGLbBRLCdrPgJb",
            "ScriptArgs": [
                "BoNOSsncjVhvfAWWgnhdngAdq"
            ],
            "HTTP": "eGyumMvRoMHMYVCSwiiOfhaGz",
            "Header": {
                "oJdkygHJmQfMNJXKwIWBTiXhF": [
                    "YAaruWBZCupkBtetaHddXaoLP"
                ]
            },
            "Method": "GbdzsGvwzuDwtFZJTXigNbKMM",
            "TCP": "jwfIpVvFcgDbkRwYPsuLxKRSi",
            "Interval": 59,
            "AliasNode": "fEyMwcvrngHTyBcaogBNfDRUS",
            "AliasService": "jLaRBwTlpCxWCefWXwDUyCdJp",
            "DockerContainerID": "rVvbqisfiYYIrVvxayrzBAdcC",
            "Shell": "KOifoowFfQWVdUjKtXKRuQOaL",
            "GRPC": "OSeIfmRLHNsMvwcBbwySeaFrb",
            "GRPCUseTLS": false,
            "TLSSkipVerify": true,
            "Timeout": 88,
            "TTL": 50,
            "ProxyHTTP": "CbRlADxDEaeiQyfRVKoDIpNEi",
            "ProxyGRPC": "rFPfmpYMBVPIhUieawlteVHAP",
            "DeregisterCriticalServiceAfter": 27,
            "OutputMaxSize": 90
        },
        "Checks": [
            {
                "CheckID": "TowMDpXpxwnWIwXcpoxsMtnJT",
                "Name": "XBphRhpGuBjSgwWxJRJnuIHGn",
                "Status": "ViLfRNuCklJIMsCyKPlAbNGjV",
                "Notes": "efDwBHrTgBhJcyncwUGOtMGRY",
                "ScriptArgs": [
                    "oGWEFfFThcliqITaDPNKCuDlo",
                    "mHkrSxSegsrlgRQBIJSbmAloC"
                ],
                "HTTP": "vhkUaJLVoUXUzaRAqUKxPDVaf",
                "Header": {
                    "DDcHUgbQHrUXCVegOJDaeuQDT": [],
                    "FcROALocdhuruMAmjhaBQCWBe": [
                        "xHGoHVTwEfRCIZqmknEfpNhkP",
                        "BgUfRtMZgIZKiaAFXgMvWtqRK"
                    ],
                    "ZScmpRADxrsTMmMpargiCQmyb": []
                },
                "Method": "WvvbNadtxfyipNCrgHloKKwhR",
                "TCP": "GNfwMBmEotccliygtiTRkhxKf",
                "Interval": 77,
                "AliasNode": "msrnnOhtxXMRyWeAjTHExKAHT",
                "AliasService": "PrwGzFoBCNTBNsghVdeoMnnuI",
                "DockerContainerID": "LlFTbiHJOCDdoubMEmNBxEANj",
                "Shell": "qSkAwQCrGuvWHFZSltAkMabSc",
                "GRPC": "LJALPHtophAsqhxZzQuiptVjF",
                "GRPCUseTLS": true,
                "TLSSkipVerify": true,
                "Timeout": 4,
                "TTL": 35,
                "ProxyHTTP": "ucNOLylZMGhDfqICzjZfNLxGz",
                "ProxyGRPC": "ozSbUVmaKDPqqMMbtjqEUystN",
                "DeregisterCriticalServiceAfter": 85,
                "OutputMaxSize": 59
            }
        ],
        "Weights": {
            "Passing": 99,
            "Warning": 45
        },
        "Token": "MQNPWxEVtOFyYdnmwrNxRMyYf",
        "EnableTagOverride": false,
        "Proxy": {
            "DestinationServiceName": "nxOeXwDxVVgKbpVFejlDEakWj",
            "DestinationServiceID": "LtSLPVqgLBbGmJtuJgEmbIryV",
            "LocalServiceAddress": "tZmPlZNAcQXaKQtrZGIgBSYRa",
            "LocalServicePort": 92,
            "Upstreams": [
                {
                    "DestinationType": "UwyGsLqsyvuYjxBGxHNaCnGeU",
                    "DestinationNamespace": "zcYRrRydaPXwfjpIfmrvhFZpb",
                    "DestinationName": "IhWiVtqxBGhQqNgnUNXgtAESt",
                    "Datacenter": "kvApuXgjIcLKSYgKSeJqTwGYs",
                    "LocalBindAddress": "PinqEHTSlDEBhTOCZxtvdgUxW",
                    "LocalBindPort": 84,
                    "Config": {"key": ["a", "b"]},
                    "MeshGateway": {
                        "Mode": "UYUDgaOxyWWhTaYbIjkYeBDOV"
                    }
                },
                {
                    "DestinationType": "pBwcaXbvZiwoAEMWeVCBEIpCh",
                    "DestinationNamespace": "QvzPyyQqLJGAMhEbybBquaLIt",
                    "DestinationName": "WasbwHybARSLBrfVuaZrqcJbY",
                    "Datacenter": "DdnGLHIPHzNJhoOJtHMnBKDVt",
                    "LocalBindAddress": "VshoGHygxPoqkhKjNnywyAElh",
                    "LocalBindPort": 16,
                    "Config": {"key": ["a", "b"]},
                    "MeshGateway": {
                        "Mode": "sEjKDchCNXArEzDYFSYuEknJs"
                    }
                },
                {
                    "DestinationType": "RrIVAbPfYCKegDAqTIdWWkYdm",
                    "DestinationNamespace": "AkwqgJmNQgmgkBtQyVkGzyBfR",
                    "DestinationName": "gBfQwQEOIUFHovDJRsfIbcsus",
                    "Datacenter": "rfvbMNnvLItrLcCPDIvJmZasR",
                    "LocalBindAddress": "iRtLtovNEwsJidfLDEEdXOJyM",
                    "LocalBindPort": 3,
                    "Config": {"key": ["a", "b"]},
                    "MeshGateway": {
                        "Mode": "ANnYYAVoCbjfzToudhmByDmMM"
                    }
                }
            ],
            "MeshGateway": {
                "Mode": "kDRyTHoNHeQGjAtcYEqdJWwyd"
            },
            "Expose": {
                "Checks": true,
                "Paths": [
                    {
                        "ListenerPort": 90,
                        "Path": "XQxXShMrORPkPSEcuHLoyvBAf",
                        "LocalPathPort": 2,
                        "Protocol": "LtHPVjWYbfWsjosCMmjqGHGXy",
                        "ParsedFromCheck": false
                    },
                    {
                        "ListenerPort": 28,
                        "Path": "cZbjhCqVgJrcYNdLKRkTebvoB",
                        "LocalPathPort": 49,
                        "Protocol": "hmQlDvwaHJMHvPCAUdzRRciWd",
                        "ParsedFromCheck": true
                    },
                    {
                        "ListenerPort": 48,
                        "Path": "lfDtrWjDFFEDyydlgMxWjPMkb",
                        "LocalPathPort": 57,
                        "Protocol": "mLIFraIZjbZWteXqqdWWDjylX",
                        "ParsedFromCheck": false
                    }
                ]
            }
        },
        "Connect": {
            "Native": true,
            "SidecarService": {
                "Kind": "1fAaUEQiqbqiCmCihBgdPPnhKm",
                "ID": "1ItidJtWvgHLwAexuCODOkFMSX",
                "Name": "1QkOOHwMapZGzrFtYnntkVodMK",
                "Tags": [
                    "1aVDuXWWtUlPcNbmtDpvZKyYjH",
                    "1jVbvxFTtSUhuWIaeyojtsBMcJ"
                ],
                "Address": "1oMxcCabMpKmZLACtMMEOJddqc",
                "TaggedAddresses": {
                    "1bWqvZvHxHeNrjrPFNPhsdNVtM": {
                        "Address": "1pnAJuDcuqbjgRKCYojJpEOfxU",
                        "Port": 142
                    }
                },
                "Meta": {
                    "1GFYZPqpbaPRgivNTkXzKOQIfr": "1ceTMIRiNZqhUYRsBwGjSJioIQ"
                },
                "Port": 184
            }
        }
    }`
    // ------
    want := structs.ServiceDefinition{
        Kind: "fAaUEQiqbqiCmCihBgdPPnhKm",
        ID:   "ItidJtWvgHLwAexuCODOkFMSX",
        Name: "QkOOHwMapZGzrFtYnntkVodMK",
        Tags: []string{
            "aVDuXWWtUlPcNbmtDpvZKyYjH",
            "jVbvxFTtSUhuWIaeyojtsBMcJ",
        },
        Address: "oMxcCabMpKmZLACtMMEOJddqc",
        TaggedAddresses: map[string]structs.ServiceAddress{
            "bWqvZvHxHeNrjrPFNPhsdNVtM": structs.ServiceAddress{
                Address: "pnAJuDcuqbjgRKCYojJpEOfxU",
                Port:    42,
            },
        },
        Meta: map[string]string{
            "GFYZPqpbaPRgivNTkXzKOQIfr": "ceTMIRiNZqhUYRsBwGjSJioIQ",
        },
        Port: 84,
        Check: structs.CheckType{
            CheckID: "TkKeoQZfvXDauRVcMHjXbWwGe",
            Name:    "ybaKpHMYkqhhoMDLrSxNdCsrR",
            Status:  "ymceFURxSxrgGsQuaXxSJedhf",
            Notes:   "SchHKnkdxamOGLbBRLCdrPgJb",
            ScriptArgs: []string{
                "BoNOSsncjVhvfAWWgnhdngAdq",
            },
            HTTP: "eGyumMvRoMHMYVCSwiiOfhaGz",
            Header: map[string][]string{
                "oJdkygHJmQfMNJXKwIWBTiXhF": []string{
                    "YAaruWBZCupkBtetaHddXaoLP",
                },
            },
            Method:                         "GbdzsGvwzuDwtFZJTXigNbKMM",
            TCP:                            "jwfIpVvFcgDbkRwYPsuLxKRSi",
            Interval:                       59,
            AliasNode:                      "fEyMwcvrngHTyBcaogBNfDRUS",
            AliasService:                   "jLaRBwTlpCxWCefWXwDUyCdJp",
            DockerContainerID:              "rVvbqisfiYYIrVvxayrzBAdcC",
            Shell:                          "KOifoowFfQWVdUjKtXKRuQOaL",
            GRPC:                           "OSeIfmRLHNsMvwcBbwySeaFrb",
            GRPCUseTLS:                     false,
            TLSSkipVerify:                  true,
            Timeout:                        88,
            TTL:                            50,
            ProxyHTTP:                      "CbRlADxDEaeiQyfRVKoDIpNEi",
            ProxyGRPC:                      "rFPfmpYMBVPIhUieawlteVHAP",
            DeregisterCriticalServiceAfter: 27,
            OutputMaxSize:                  90,
        },
        Checks: structs.CheckTypes{
            &structs.CheckType{
                CheckID: types.CheckID("TowMDpXpxwnWIwXcpoxsMtnJT"),
                Name:    "XBphRhpGuBjSgwWxJRJnuIHGn",
                Status:  "ViLfRNuCklJIMsCyKPlAbNGjV",
                Notes:   "efDwBHrTgBhJcyncwUGOtMGRY",
                ScriptArgs: []string{
                    "oGWEFfFThcliqITaDPNKCuDlo",
                    "mHkrSxSegsrlgRQBIJSbmAloC",
                },
                HTTP: "vhkUaJLVoUXUzaRAqUKxPDVaf",
                Header: map[string][]string{
                    "FcROALocdhuruMAmjhaBQCWBe": []string{
                        "xHGoHVTwEfRCIZqmknEfpNhkP",
                        "BgUfRtMZgIZKiaAFXgMvWtqRK",
                    },
                },
                Method:                         "WvvbNadtxfyipNCrgHloKKwhR",
                TCP:                            "GNfwMBmEotccliygtiTRkhxKf",
                Interval:                       77,
                AliasNode:                      "msrnnOhtxXMRyWeAjTHExKAHT",
                AliasService:                   "PrwGzFoBCNTBNsghVdeoMnnuI",
                DockerContainerID:              "LlFTbiHJOCDdoubMEmNBxEANj",
                Shell:                          "qSkAwQCrGuvWHFZSltAkMabSc",
                GRPC:                           "LJALPHtophAsqhxZzQuiptVjF",
                GRPCUseTLS:                     true,
                TLSSkipVerify:                  true,
                Timeout:                        4,
                TTL:                            35,
                ProxyHTTP:                      "ucNOLylZMGhDfqICzjZfNLxGz",
                ProxyGRPC:                      "ozSbUVmaKDPqqMMbtjqEUystN",
                DeregisterCriticalServiceAfter: 85,
                OutputMaxSize:                  59,
            },
        },
        Weights: &structs.Weights{
            Passing: 99,
            Warning: 45,
        },
        Token:             "MQNPWxEVtOFyYdnmwrNxRMyYf",
        EnableTagOverride: false,
        Proxy: &structs.ConnectProxyConfig{
            DestinationServiceName: "nxOeXwDxVVgKbpVFejlDEakWj",
            DestinationServiceID:   "LtSLPVqgLBbGmJtuJgEmbIryV",
            LocalServiceAddress:    "tZmPlZNAcQXaKQtrZGIgBSYRa",
            LocalServicePort:       92,
            Upstreams: structs.Upstreams{
                {
                    DestinationType:      "UwyGsLqsyvuYjxBGxHNaCnGeU",
                    DestinationNamespace: "zcYRrRydaPXwfjpIfmrvhFZpb",
                    DestinationName:      "IhWiVtqxBGhQqNgnUNXgtAESt",
                    Datacenter:           "kvApuXgjIcLKSYgKSeJqTwGYs",
                    LocalBindAddress:     "PinqEHTSlDEBhTOCZxtvdgUxW",
                    LocalBindPort:        84,
                    Config:               map[string]interface{}{"key": []interface{}{"a", "b"}},
                    MeshGateway: structs.MeshGatewayConfig{
                        Mode: "UYUDgaOxyWWhTaYbIjkYeBDOV",
                    },
                },
                {
                    DestinationType:      "pBwcaXbvZiwoAEMWeVCBEIpCh",
                    DestinationNamespace: "QvzPyyQqLJGAMhEbybBquaLIt",
                    DestinationName:      "WasbwHybARSLBrfVuaZrqcJbY",
                    Datacenter:           "DdnGLHIPHzNJhoOJtHMnBKDVt",
                    LocalBindAddress:     "VshoGHygxPoqkhKjNnywyAElh",
                    LocalBindPort:        16,
                    Config:               map[string]interface{}{"key": []interface{}{"a", "b"}},
                    MeshGateway: structs.MeshGatewayConfig{
                        Mode: "sEjKDchCNXArEzDYFSYuEknJs",
                    },
                },
                {
                    DestinationType:      "RrIVAbPfYCKegDAqTIdWWkYdm",
                    DestinationNamespace: "AkwqgJmNQgmgkBtQyVkGzyBfR",
                    DestinationName:      "gBfQwQEOIUFHovDJRsfIbcsus",
                    Datacenter:           "rfvbMNnvLItrLcCPDIvJmZasR",
                    LocalBindAddress:     "iRtLtovNEwsJidfLDEEdXOJyM",
                    LocalBindPort:        3,
                    Config:               map[string]interface{}{"key": []interface{}{"a", "b"}},
                    MeshGateway: structs.MeshGatewayConfig{
                        Mode: "ANnYYAVoCbjfzToudhmByDmMM",
                    },
                },
            },
            MeshGateway: structs.MeshGatewayConfig{
                Mode: "kDRyTHoNHeQGjAtcYEqdJWwyd",
            },
            Expose: structs.ExposeConfig{
                Checks: true,
                Paths: []structs.ExposePath{
                    {
                        ListenerPort:    90,
                        Path:            "XQxXShMrORPkPSEcuHLoyvBAf",
                        LocalPathPort:   2,
                        Protocol:        "LtHPVjWYbfWsjosCMmjqGHGXy",
                        ParsedFromCheck: false,
                    },
                    {
                        ListenerPort:    28,
                        Path:            "cZbjhCqVgJrcYNdLKRkTebvoB",
                        LocalPathPort:   49,
                        Protocol:        "hmQlDvwaHJMHvPCAUdzRRciWd",
                        ParsedFromCheck: true,
                    },
                    {
                        ListenerPort:    48,
                        Path:            "lfDtrWjDFFEDyydlgMxWjPMkb",
                        LocalPathPort:   57,
                        Protocol:        "mLIFraIZjbZWteXqqdWWDjylX",
                        ParsedFromCheck: false,
                    },
                },
            },
        },
        Connect: &structs.ServiceConnect{
            Native: true,
            SidecarService: &structs.ServiceDefinition{
                Kind: "1fAaUEQiqbqiCmCihBgdPPnhKm",
                ID:   "1ItidJtWvgHLwAexuCODOkFMSX",
                Name: "1QkOOHwMapZGzrFtYnntkVodMK",
                Tags: []string{
                    "1aVDuXWWtUlPcNbmtDpvZKyYjH",
                    "1jVbvxFTtSUhuWIaeyojtsBMcJ",
                },
                Address: "1oMxcCabMpKmZLACtMMEOJddqc",
                TaggedAddresses: map[string]structs.ServiceAddress{
                    "1bWqvZvHxHeNrjrPFNPhsdNVtM": structs.ServiceAddress{
                        Address: "1pnAJuDcuqbjgRKCYojJpEOfxU",
                        Port:    142,
                    },
                },
                Meta: map[string]string{
                    "1GFYZPqpbaPRgivNTkXzKOQIfr": "1ceTMIRiNZqhUYRsBwGjSJioIQ",
                },
                Port: 184,
            },
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ServiceDefinition{}
    if err := decodeBody(req, &out, registerServiceDecodeCB); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/agent_endpoint.go:
//  1173    // fields to this later if needed.
//  1174    var args api.AgentToken
//  1175:   if err := decodeBody(req, &args, nil); err != nil {
//  1176        resp.WriteHeader(http.StatusBadRequest)
//  1177        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// AgentToken:
// Token    string
func TestDecodeSanityAgentToken(t *testing.T) {

    jsonBlob := `{
    "Token": "WFeHWLrZXDSaaKMMXTHrBPvAA"
}`
    // ------
    want := api.AgentToken{
        Token: "WFeHWLrZXDSaaKMMXTHrBPvAA",
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := api.AgentToken{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/agent_endpoint.go:
//  1332    // Decode the request from the request body
//  1333    var authReq structs.ConnectAuthorizeRequest
//  1334:   if err := decodeBody(req, &authReq, nil); err != nil {
//  1335        return nil, BadRequestError{fmt.Sprintf("Request decode failed: %v", err)}
//  1336    }
// ==================================
// ConnectAuthorizeRequest:
// Target   string
// ClientCertURI    string
// ClientCertSerial string
func TestDecodeSanityAgentConnectAuthorize(t *testing.T) {

    jsonBlob := `{
    "Target": "cpEOaaIDNmxQuCuSoaRPrFkUP",
    "ClientCertURI": "ygTESMRLzblpiRLbWrLlctaIm",
    "ClientCertSerial": "mOgfsyUQztPGitSarYuxfBqiq"
}`
    // ------
    want := structs.ConnectAuthorizeRequest{
        Target:           "cpEOaaIDNmxQuCuSoaRPrFkUP",
        ClientCertURI:    "ygTESMRLzblpiRLbWrLlctaIm",
        ClientCertSerial: "mOgfsyUQztPGitSarYuxfBqiq",
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.ConnectAuthorizeRequest{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/catalog_endpoint.go:
//    18
//    19    var args structs.RegisterRequest
//    20:   if err := decodeBody(req, &args, durations.FixupDurations); err != nil {
//    21        resp.WriteHeader(http.StatusBadRequest)
//    22        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// RegisterRequest:
// Datacenter   string
// ID   types.NodeID
// Node string
// Address  string
// TaggedAddresses  map[string]string
// NodeMeta map[string]string
// Service  *structs.NodeService
//     Kind structs.ServiceKind
//     ID   string
//     Service  string
//     Tags []string
//     Address  string
//     TaggedAddresses  map[string]structs.ServiceAddress
//         Address  string
//         Port int
//     Meta map[string]string
//     Port int
//     Weights  *structs.Weights
//         Passing  int
//         Warning  int
//     EnableTagOverride    bool
//     Proxy    structs.ConnectProxyConfig
//         DestinationServiceName   string
//         DestinationServiceID string
//         LocalServiceAddress  string
//         LocalServicePort int
//         Config   map[string]interface {} `faker:"-"`
//         Upstreams    structs.Upstreams
//             DestinationType  string
//             DestinationNamespace string
//             DestinationName  string
//             Datacenter   string
//             LocalBindAddress string
//             LocalBindPort    int
//             Config   map[string]interface {} `faker:"-"`
//             MeshGateway  structs.MeshGatewayConfig
//                 Mode structs.MeshGatewayMode
//         MeshGateway  structs.MeshGatewayConfig
//         Expose   structs.ExposeConfig
//             Checks   bool
//             Paths    []structs.ExposePath
//                 ListenerPort int
//                 Path string
//                 LocalPathPort    int
//                 Protocol string
//                 ParsedFromCheck  bool
//     Connect  structs.ServiceConnect
//         Native   bool
//         SidecarService   *structs.ServiceDefinition
//             Kind structs.ServiceKind
//             ID   string
//             Name string
//             Tags []string
//             Address  string
//             TaggedAddresses  map[string]structs.ServiceAddress
//             Meta map[string]string
//             Port int
//             Check    structs.CheckType
//                 CheckID  types.CheckID
//                 Name string
//                 Status   string
//                 Notes    string
//                 ScriptArgs   []string
//                 HTTP string
//                 Header   map[string][]string
//                 Method   string
//                 TCP  string
//                 Interval time.Duration
//                 AliasNode    string
//                 AliasService string
//                 DockerContainerID    string
//                 Shell    string
//                 GRPC string
//                 GRPCUseTLS   bool
//                 TLSSkipVerify    bool
//                 Timeout  time.Duration
//                 TTL  time.Duration
//                 ProxyHTTP    string
//                 ProxyGRPC    string
//                 DeregisterCriticalServiceAfter   time.Duration
//                 OutputMaxSize    int
//             Checks   structs.CheckTypes
//             Weights  *structs.Weights
//             Token    string
//             EnableTagOverride    bool
//             Proxy    *structs.ConnectProxyConfig
//             Connect  *structs.ServiceConnect
//     LocallyRegisteredAsSidecar   bool
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// Check    *structs.HealthCheck
//     Node string
//     CheckID  types.CheckID
//     Name string
//     Status   string
//     Notes    string
//     Output   string
//     ServiceID    string
//     ServiceName  string
//     ServiceTags  []string
//     Definition   structs.HealthCheckDefinition
//         HTTP string
//         TLSSkipVerify    bool
//         Header   map[string][]string
//         Method   string
//         TCP  string
//         Interval time.Duration
//         OutputMaxSize    uint
//         Timeout  time.Duration
//         DeregisterCriticalServiceAfter   time.Duration
//         ScriptArgs   []string
//         DockerContainerID    string
//         Shell    string
//         GRPC string
//         GRPCUseTLS   bool
//         AliasNode    string
//         AliasService string
//         TTL  time.Duration
//     RaftIndex    structs.RaftIndex
// Checks   structs.HealthChecks
// SkipNodeUpdate   bool
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityCatalogRegister(t *testing.T) {
    // $$ HERE
    jsonBlob := `{
    "Datacenter": "TwCIbiaKgCDeInNdqSQJaBiFX",
    "ID": "uZYuorpgmOHYfJKJMTiSEOJoU",
    "Node": "fMKYZdQPAjHLwuuFKStHgsVrn",
    "Address": "aZYavzaecOGLNGucicUGCqfxT",
    "TaggedAddresses": {
        "RSrHiRIOjabDdBzULqslxItEt": "wvMHaLjcZlyeDXOJEZtDMHtun",
        "UVluuJRnbBQSLbQwrjIAUzvzM": "aEVrHWugtrEpDvqVdTveGeIIg"
    },
    "NodeMeta": {
        "QsZYZszITRmHVtrKxhUEEMfde": "RGbKdfHvkURDjPJElFVQrlZyj",
        "lLJBvAlAudNSUtpZoOhGDuOfx": "lljOYVQmKZfseWPZAkmxXhjCF",
        "sAomprDKabLKDFNyGwZvcfqRo": "HgtJQCQwsYKUlFMYmlJdWJYPI",
        "ywmTXWjsaeziXcJMIPGzaMBtu": "MuwUzkJpWhHzqwUEyrOrxdHna"
    },
    "Service": {
        "Kind": "OxbZkTjhRtCjYEKbfpbvqvyPw",
        "ID": "fcUESGLTWBhQSGqjaRpApiMVs",
        "Service": "daNdhPXppXrgLAgokDldibJdg",
        "Tags": [
            "MmTaqebtDGjSFqIUcJkuKcHGy"
        ],
        "Address": "SDvuqCEaSwalzGjOKFKeAKJOr",
        "TaggedAddresses": {
            "ApXlYSOYyfysayAGSOIzVUKXh": {
                "Address": "yFqJPbCspCzbDeSemOaekNXga",
                "Port": 65
            },
            "HzTxSdqaIyrHSBPDkUFcEdArF": {
                "Address": "FwxFGDJVVUWuatRUaJEZhTaUo",
                "Port": 52
            },
            "PrReAkbOvUmtJmsSrPsMurRrX": {
                "Address": "XBKYLllSeQZghHJbTbeNOYgAa",
                "Port": 16
            },
            "YjSalcxdlaRNhaKSWqhlIMTIw": {
                "Address": "PPSDxhXwCyEqAYVwLmYSOVhoX",
                "Port": 79
            }
        },
        "Meta": {"a":"a"},
        "Port": 76,
        "Weights": {
            "Passing": 44,
            "Warning": 47
        },
        "EnableTagOverride": false,
        "Proxy": {
            "DestinationServiceName": "vdfLkYpXoJwyUnIeTrBbNMHeP",
            "DestinationServiceID": "LyhWHADBhgDhwDDwEStowBHxm",
            "LocalServiceAddress": "DrKpMqOXwnMuSBENxpjoMYmVo",
            "LocalServicePort": 52,
            "MeshGateway": {
                "Mode": "wJbXQCvozjmtmdgStuvdYfVQF"
            },
            "Expose": {
                "Checks": true,
                "Paths": [
                    {
                        "ListenerPort": 41,
                        "Path": "TiVmLxBrrIpLupmMIhinPjmeM",
                        "LocalPathPort": 25,
                        "Protocol": "xTEabtqiFNFfrnyjIrbiZscWm",
                        "ParsedFromCheck": true
                    },
                    {
                        "ListenerPort": 67,
                        "Path": "LUsGkyjouSdniEoBaPIBvzSrt",
                        "LocalPathPort": 68,
                        "Protocol": "SUkukaNdFzFkroPbYaTNBQeYu",
                        "ParsedFromCheck": false
                    }
                ]
            }
        },
        "Connect": {
            "Native": true,
            "SidecarService": {
                "Kind": "1fAaUEQiqbqiCmCihBgdPPnhKm",
                "ID": "1ItidJtWvgHLwAexuCODOkFMSX",
                "Name": "1QkOOHwMapZGzrFtYnntkVodMK",
                "Tags": [
                    "1aVDuXWWtUlPcNbmtDpvZKyYjH",
                    "1jVbvxFTtSUhuWIaeyojtsBMcJ"
                ],
                "Address": "1oMxcCabMpKmZLACtMMEOJddqc",
                "TaggedAddresses": {
                    "1bWqvZvHxHeNrjrPFNPhsdNVtM": {
                        "Address": "1pnAJuDcuqbjgRKCYojJpEOfxU",
                        "Port": 142
                    }
                },
                "Meta": {
                    "1GFYZPqpbaPRgivNTkXzKOQIfr": "1ceTMIRiNZqhUYRsBwGjSJioIQ"
                },
                "Port": 184
            }
        },
        "CreateIndex": 8,
        "ModifyIndex": 21
    },
    "Check": {
        "Node": "wrTimIpLQBngcyCDLQdBoEqqT",
        "CheckID": "pPLCCUKawyuUPAvLyVncMKpGm",
        "Name": "hAQwrJdoeiUEPDSFYfaLvKJiR",
        "Status": "QYgIYHNFSQzRhYxCuXvdvtsEc",
        "Notes": "fsZRucAxRaDGcxUEsRQQnksUn",
        "Output": "gRvDWZBtPHsCRESnXvyLThTEz",
        "ServiceID": "QIddArLptfQuFnAsCEUfvyRol",
        "ServiceName": "qhkEDsoIeaRdFPugGElmnZqpS",
        "ServiceTags": [
            "nFphLIuTcPVRtfkhWUAqoULwE",
            "NvokaPbkQbTDwwAmmvshiYFNd"
        ],
        "Definition": {
            "Interval": "54ns",
            "OutputMaxSize": 62,
            "Timeout": "44ns",
            "DeregisterCriticalServiceAfter": "7ns",
            "HTTP": "IdGpDIMmIuCKbsPbnVMzPsvdp",
            "Header": {
                "DJdUOEsQufpPpVjEvhlXGslNr": [
                    "EKEvrvSwjfGthFGburpgihrKH",
                    "nJgRaNgaRjJsNNmdVGzDGjZrX",
                    "bMlgtZfyuaajeOvHXvUGWUItj",
                    "bqQRbVuNvOMmBzoHEFDwJZxbS"
                ],
                "NxffNfedeaknIElRvgqmBUDQi": [
                    "pwScAGqpcmwDZjEGdoYQWvSKd",
                    "EPXlqoySYSEgDIqlzopXaRcFq",
                    "ZQYZoETTzfLBwBHGcerqWQiBP"
                ],
                "aVIDUPCPApTeQEHojVuWhDRsP": [
                    "kLIRbJJuXtoZmOellTZnqYyDp"
                ]
            },
            "Method": "zqIGCnmuGhvgSgmPgJsXhwqnk",
            "TCP": "PMGZztYsQbLibeDFQneVaupwl",
            "ScriptArgs": [
                "EFGHsPrDhYlzdyeSDTUCxvtZw",
                "JhlEGmOfEVWcofKiJYWcJIYCI"
            ],
            "DockerContainerID": "vNOPDODyahBGCXTLwsussfUqP",
            "Shell": "jmEPQtwUwAAWPTzsDJGhUhynI",
            "GRPC": "CLXgGdBUvgaqLmFzmmyyKFdzl",
            "GRPCUseTLS": true,
            "AliasNode": "dyTLmRWYBEYtwYjPprjWsgdJg",
            "AliasService": "rICnARUCghApmyiLrQAbhvUYB",
            "TTL": 61
        },
        "CreateIndex": 57,
        "ModifyIndex": 50
    },
    "Checks": [
        {
            "Node": "uSoyxUHgmzRSnnTwyExTDEjvp",
            "CheckID": "hchQldFNdeaVTJPVTmxZRfUKw",
            "Name": "aksfEzjXJcljUcQuicNixoRYK",
            "Status": "KOwyFmSayFMQZyLprnrplnevC",
            "Notes": "gNDSkpnEovBCWWPnzUQyuiXGz",
            "Output": "SftcNdfHbCKmRLjORuYhLsKGW",
            "ServiceID": "gILbjhJUhwyGxNJwrMiMoJFco",
            "ServiceName": "HfKtngwCBaugvPSeLMiyefCbI",
            "ServiceTags": [
                "mvCzWwuqsLRtxnoOhkDAJXzub",
                "QGekCNXAYnHGMXDXPJyOhOkAy",
                "cEGzOhnaCObdtpmAjaHxXkLzU"
            ],
            "Definition": {
                "Interval": "65ns",
                "OutputMaxSize": 87,
                "Timeout": "92ns",
                "DeregisterCriticalServiceAfter": "75ns",
                "HTTP": "ztzHDQvJIMRVEiYXMHNsCRYxf",
                "TLSSkipVerify": true,
                "Header": {
                    "HZdgzkSczUmjlxKJHLWTPGZIC": [
                        "jQEXWvNHMLJZtVmodyyMRvOzM",
                        "WinTIbcRkXHlWyrcuaAioHdOh"
                    ],
                    "NxvMXncSrtSeapUsHweRBGCYx": [
                        "uEPwoKwvfkndjFHBzKlpUstMt",
                        "gouBrNeOtiEKnUObdZSDVShtm",
                        "sYIDVmBZfcRGFPVnTbKujcNCE",
                        "wkcwELlshRfpjqyNuanTTzUun"
                    ],
                    "iHvMqeWvscEiwLqPACJziDGHq": [
                        "FvwRUcIEQFTsGLKcBycByiOVv",
                        "IoxAlJoCNGhoAhyZNFpeVqRlW",
                        "iUyoegXugPolVRQUpdXacIhsI"
                    ],
                    "vhDqWwVgIxBFaJPVhSVPmtvwh": []
                },
                "Method": "oBoEPmlbeBhNwSGpomGTXHISP",
                "TCP": "ATWXqiFGzHtKNyiwedYsnfbcC",
                "ScriptArgs": [
                    "KHUqvEaFrPUiGgdCbhrgQoMBy",
                    "AnfojyhcCbULnhwpUmnaCMQdW",
                    "KpwZWnEGlamxPBztmmJXaEGOH",
                    "NaUEbAVhIcwxxugJGqbEgZnrE"
                ],
                "DockerContainerID": "jzOgUTSllSokuGCfsquWkHNmq",
                "Shell": "BTnLLENDPzTmOvpaJlDRHzZPr",
                "GRPC": "tpnLDmdLagtyrxjFkcPIAiDzB",
                "GRPCUseTLS": true,
                "AliasNode": "kEOVuRnVFlaOTEDaRJKiylpWi",
                "AliasService": "ZxQnulNkogJdvxEcmcMHvkaVx",
                "TTL": 53
            },
            "CreateIndex": 84,
            "ModifyIndex": 25
        },
        {
            "Node": "hkPBrmSsKTfvTKVAKcBPCKnMm",
            "CheckID": "XqSskkmtedkohRqNceWCUDULo",
            "Name": "LDTCMMGDDfshvxpkmtSyWnPib",
            "Status": "LQMCNSopIdhdyLcvXiywmMOhF",
            "Notes": "XxpgcnIXxcNWFDiigHHEUohBp",
            "Output": "DJjzWhgSqJYlBUrMvFkRjDYJv",
            "ServiceID": "EqqdFOApJRNVsvKSJwbfpQyOz",
            "ServiceName": "deJNKrjanwQusfWKfqYFfDkcW",
            "ServiceTags": [],
            "Definition": {
                "Interval": "87ns",
                "OutputMaxSize": 52,
                "Timeout": "95ns",
                "DeregisterCriticalServiceAfter": "21ns",
                "HTTP": "QdTAVaWLchwhegmOOHteSJcQT",
                "Header": {
                    "wOZnBwxpbxtCFGWCMNPQuDCDJ": [
                        "BrKrVwthovBmkykSeWlcPqtvt",
                        "zHGQkaDRUlAidMBaFdYPEIsfX"
                    ]
                },
                "Method": "ANkMdLIqsYUfkZmOuhpjMZxtg",
                "TCP": "HYYsropdZCyMYrjYMgDhRCDFu",
                "ScriptArgs": [
                    "hGVEQvRrTgmUkXUwVsqsumOvi",
                    "PxTQmSExzaxuRyliHzNpylBQx",
                    "lVAQRjqiTrYnPHDSaaPLoCqNM",
                    "ZZtnDcTHKEIsAPSVsAXAQVQsD"
                ],
                "DockerContainerID": "JUBTbCOnyyByNxCUkcmFnbJaL",
                "Shell": "nRvpXOuNfsGydISiJIBDjIxym",
                "GRPC": "klvUMOqRDCCHDfJnGontTOSrb",
                "GRPCUseTLS": true,
                "AliasNode": "ychfKkKLdcVdhEKzIQiOmWEvq",
                "AliasService": "yStNhDWIaaAzfvgIYiDnotXbD",
                "TTL": 84
            },
            "CreateIndex": 76,
            "ModifyIndex": 14
        }
    ],
    "SkipNodeUpdate": false,
    "Token": "NjWVDRQokTmDUzuIHTtoKYMsF"
}`
    // ------
    want := structs.RegisterRequest{
        Datacenter: "TwCIbiaKgCDeInNdqSQJaBiFX",
        ID:         "uZYuorpgmOHYfJKJMTiSEOJoU",
        Node:       "fMKYZdQPAjHLwuuFKStHgsVrn",
        Address:    "aZYavzaecOGLNGucicUGCqfxT",
        TaggedAddresses: map[string]string{
            "RSrHiRIOjabDdBzULqslxItEt": "wvMHaLjcZlyeDXOJEZtDMHtun",
            "UVluuJRnbBQSLbQwrjIAUzvzM": "aEVrHWugtrEpDvqVdTveGeIIg",
        },
        NodeMeta: map[string]string{
            "QsZYZszITRmHVtrKxhUEEMfde": "RGbKdfHvkURDjPJElFVQrlZyj",
            "lLJBvAlAudNSUtpZoOhGDuOfx": "lljOYVQmKZfseWPZAkmxXhjCF",
            "sAomprDKabLKDFNyGwZvcfqRo": "HgtJQCQwsYKUlFMYmlJdWJYPI",
            "ywmTXWjsaeziXcJMIPGzaMBtu": "MuwUzkJpWhHzqwUEyrOrxdHna",
        },
        Service: &structs.NodeService{
            Kind:    "OxbZkTjhRtCjYEKbfpbvqvyPw",
            ID:      "fcUESGLTWBhQSGqjaRpApiMVs",
            Service: "daNdhPXppXrgLAgokDldibJdg",
            Tags: []string{
                "MmTaqebtDGjSFqIUcJkuKcHGy",
            },
            Address: "SDvuqCEaSwalzGjOKFKeAKJOr",
            TaggedAddresses: map[string]structs.ServiceAddress{
                "ApXlYSOYyfysayAGSOIzVUKXh": structs.ServiceAddress{
                    Address: "yFqJPbCspCzbDeSemOaekNXga",
                    Port:    65,
                },
                "HzTxSdqaIyrHSBPDkUFcEdArF": structs.ServiceAddress{
                    Address: "FwxFGDJVVUWuatRUaJEZhTaUo",
                    Port:    52,
                },
                "PrReAkbOvUmtJmsSrPsMurRrX": structs.ServiceAddress{
                    Address: "XBKYLllSeQZghHJbTbeNOYgAa",
                    Port:    16,
                },
                "YjSalcxdlaRNhaKSWqhlIMTIw": structs.ServiceAddress{
                    Address: "PPSDxhXwCyEqAYVwLmYSOVhoX",
                    Port:    79,
                },
            },
            Meta: map[string]string{"a": "a"},
            Port: 76,
            Weights: &structs.Weights{
                Passing: 44,
                Warning: 47,
            },
            EnableTagOverride: false,
            Proxy: structs.ConnectProxyConfig{
                DestinationServiceName: "vdfLkYpXoJwyUnIeTrBbNMHeP",
                DestinationServiceID:   "LyhWHADBhgDhwDDwEStowBHxm",
                LocalServiceAddress:    "DrKpMqOXwnMuSBENxpjoMYmVo",
                LocalServicePort:       52,
                MeshGateway: structs.MeshGatewayConfig{
                    Mode: "wJbXQCvozjmtmdgStuvdYfVQF",
                },
                Expose: structs.ExposeConfig{
                    Checks: true,
                    Paths: []structs.ExposePath{
                        {
                            ListenerPort:    41,
                            Path:            "TiVmLxBrrIpLupmMIhinPjmeM",
                            LocalPathPort:   25,
                            Protocol:        "xTEabtqiFNFfrnyjIrbiZscWm",
                            ParsedFromCheck: true,
                        },
                        {
                            ListenerPort:    67,
                            Path:            "LUsGkyjouSdniEoBaPIBvzSrt",
                            LocalPathPort:   68,
                            Protocol:        "SUkukaNdFzFkroPbYaTNBQeYu",
                            ParsedFromCheck: false,
                        },
                    },
                },
            },
            Connect: structs.ServiceConnect{
                Native: true,
                SidecarService: &structs.ServiceDefinition{
                    Kind: "1fAaUEQiqbqiCmCihBgdPPnhKm",
                    ID:   "1ItidJtWvgHLwAexuCODOkFMSX",
                    Name: "1QkOOHwMapZGzrFtYnntkVodMK",
                    Tags: []string{
                        "1aVDuXWWtUlPcNbmtDpvZKyYjH",
                        "1jVbvxFTtSUhuWIaeyojtsBMcJ",
                    },
                    Address: "1oMxcCabMpKmZLACtMMEOJddqc",
                    TaggedAddresses: map[string]structs.ServiceAddress{
                        "1bWqvZvHxHeNrjrPFNPhsdNVtM": structs.ServiceAddress{
                            Address: "1pnAJuDcuqbjgRKCYojJpEOfxU",
                            Port:    142,
                        },
                    },
                    Meta: map[string]string{
                        "1GFYZPqpbaPRgivNTkXzKOQIfr": "1ceTMIRiNZqhUYRsBwGjSJioIQ",
                    },
                    Port: 184,
                },
            },
        },
        Check: &structs.HealthCheck{
            Node:        "wrTimIpLQBngcyCDLQdBoEqqT",
            CheckID:     "pPLCCUKawyuUPAvLyVncMKpGm",
            Name:        "hAQwrJdoeiUEPDSFYfaLvKJiR",
            Status:      "QYgIYHNFSQzRhYxCuXvdvtsEc",
            Notes:       "fsZRucAxRaDGcxUEsRQQnksUn",
            Output:      "gRvDWZBtPHsCRESnXvyLThTEz",
            ServiceID:   "QIddArLptfQuFnAsCEUfvyRol",
            ServiceName: "qhkEDsoIeaRdFPugGElmnZqpS",
            ServiceTags: []string{
                "nFphLIuTcPVRtfkhWUAqoULwE",
                "NvokaPbkQbTDwwAmmvshiYFNd",
            },
            Definition: structs.HealthCheckDefinition{
                Interval:                       duration("54ns"),
                OutputMaxSize:                  62,
                Timeout:                        duration("44ns"),
                DeregisterCriticalServiceAfter: duration("7ns"),
                HTTP:                           "IdGpDIMmIuCKbsPbnVMzPsvdp",
                Header: map[string][]string{
                    "DJdUOEsQufpPpVjEvhlXGslNr": []string{
                        "EKEvrvSwjfGthFGburpgihrKH",
                        "nJgRaNgaRjJsNNmdVGzDGjZrX",
                        "bMlgtZfyuaajeOvHXvUGWUItj",
                        "bqQRbVuNvOMmBzoHEFDwJZxbS",
                    },
                    "NxffNfedeaknIElRvgqmBUDQi": []string{
                        "pwScAGqpcmwDZjEGdoYQWvSKd",
                        "EPXlqoySYSEgDIqlzopXaRcFq",
                        "ZQYZoETTzfLBwBHGcerqWQiBP",
                    },
                    "aVIDUPCPApTeQEHojVuWhDRsP": []string{
                        "kLIRbJJuXtoZmOellTZnqYyDp",
                    },
                },
                Method: "zqIGCnmuGhvgSgmPgJsXhwqnk",
                TCP:    "PMGZztYsQbLibeDFQneVaupwl",
                ScriptArgs: []string{
                    "EFGHsPrDhYlzdyeSDTUCxvtZw",
                    "JhlEGmOfEVWcofKiJYWcJIYCI",
                },
                DockerContainerID: "vNOPDODyahBGCXTLwsussfUqP",
                Shell:             "jmEPQtwUwAAWPTzsDJGhUhynI",
                GRPC:              "CLXgGdBUvgaqLmFzmmyyKFdzl",
                GRPCUseTLS:        true,
                AliasNode:         "dyTLmRWYBEYtwYjPprjWsgdJg",
                AliasService:      "rICnARUCghApmyiLrQAbhvUYB",
                TTL:               61,
            },
        },
        Checks: structs.HealthChecks{
            {
                Node:        "uSoyxUHgmzRSnnTwyExTDEjvp",
                CheckID:     "hchQldFNdeaVTJPVTmxZRfUKw",
                Name:        "aksfEzjXJcljUcQuicNixoRYK",
                Status:      "KOwyFmSayFMQZyLprnrplnevC",
                Notes:       "gNDSkpnEovBCWWPnzUQyuiXGz",
                Output:      "SftcNdfHbCKmRLjORuYhLsKGW",
                ServiceID:   "gILbjhJUhwyGxNJwrMiMoJFco",
                ServiceName: "HfKtngwCBaugvPSeLMiyefCbI",
                ServiceTags: []string{
                    "mvCzWwuqsLRtxnoOhkDAJXzub",
                    "QGekCNXAYnHGMXDXPJyOhOkAy",
                    "cEGzOhnaCObdtpmAjaHxXkLzU",
                },
                Definition: structs.HealthCheckDefinition{
                    Interval:                       duration("65ns"),
                    OutputMaxSize:                  87,
                    Timeout:                        duration("92ns"),
                    DeregisterCriticalServiceAfter: duration("75ns"),
                    HTTP:                           "ztzHDQvJIMRVEiYXMHNsCRYxf",
                    TLSSkipVerify:                  true,
                    Header: map[string][]string{
                        "HZdgzkSczUmjlxKJHLWTPGZIC": []string{
                            "jQEXWvNHMLJZtVmodyyMRvOzM",
                            "WinTIbcRkXHlWyrcuaAioHdOh",
                        },
                        "NxvMXncSrtSeapUsHweRBGCYx": []string{
                            "uEPwoKwvfkndjFHBzKlpUstMt",
                            "gouBrNeOtiEKnUObdZSDVShtm",
                            "sYIDVmBZfcRGFPVnTbKujcNCE",
                            "wkcwELlshRfpjqyNuanTTzUun",
                        },
                        "iHvMqeWvscEiwLqPACJziDGHq": []string{
                            "FvwRUcIEQFTsGLKcBycByiOVv",
                            "IoxAlJoCNGhoAhyZNFpeVqRlW",
                            "iUyoegXugPolVRQUpdXacIhsI",
                        },
                        "vhDqWwVgIxBFaJPVhSVPmtvwh": nil,
                    },
                    Method: "oBoEPmlbeBhNwSGpomGTXHISP",
                    TCP:    "ATWXqiFGzHtKNyiwedYsnfbcC",
                    ScriptArgs: []string{
                        "KHUqvEaFrPUiGgdCbhrgQoMBy",
                        "AnfojyhcCbULnhwpUmnaCMQdW",
                        "KpwZWnEGlamxPBztmmJXaEGOH",
                        "NaUEbAVhIcwxxugJGqbEgZnrE",
                    },
                    DockerContainerID: "jzOgUTSllSokuGCfsquWkHNmq",
                    Shell:             "BTnLLENDPzTmOvpaJlDRHzZPr",
                    GRPC:              "tpnLDmdLagtyrxjFkcPIAiDzB",
                    GRPCUseTLS:        true,
                    AliasNode:         "kEOVuRnVFlaOTEDaRJKiylpWi",
                    AliasService:      "ZxQnulNkogJdvxEcmcMHvkaVx",
                    TTL:               53,
                },
            },
            {
                Node:        "hkPBrmSsKTfvTKVAKcBPCKnMm",
                CheckID:     "XqSskkmtedkohRqNceWCUDULo",
                Name:        "LDTCMMGDDfshvxpkmtSyWnPib",
                Status:      "LQMCNSopIdhdyLcvXiywmMOhF",
                Notes:       "XxpgcnIXxcNWFDiigHHEUohBp",
                Output:      "DJjzWhgSqJYlBUrMvFkRjDYJv",
                ServiceID:   "EqqdFOApJRNVsvKSJwbfpQyOz",
                ServiceName: "deJNKrjanwQusfWKfqYFfDkcW",
                ServiceTags: nil,
                Definition: structs.HealthCheckDefinition{
                    Interval:                       duration("87ns"),
                    OutputMaxSize:                  52,
                    Timeout:                        duration("95ns"),
                    DeregisterCriticalServiceAfter: duration("21ns"),
                    HTTP:                           "QdTAVaWLchwhegmOOHteSJcQT",
                    Header: map[string][]string{
                        "wOZnBwxpbxtCFGWCMNPQuDCDJ": []string{
                            "BrKrVwthovBmkykSeWlcPqtvt",
                            "zHGQkaDRUlAidMBaFdYPEIsfX",
                        },
                    },
                    Method: "ANkMdLIqsYUfkZmOuhpjMZxtg",
                    TCP:    "HYYsropdZCyMYrjYMgDhRCDFu",
                    ScriptArgs: []string{
                        "hGVEQvRrTgmUkXUwVsqsumOvi",
                        "PxTQmSExzaxuRyliHzNpylBQx",
                        "lVAQRjqiTrYnPHDSaaPLoCqNM",
                        "ZZtnDcTHKEIsAPSVsAXAQVQsD",
                    },
                    DockerContainerID: "JUBTbCOnyyByNxCUkcmFnbJaL",
                    Shell:             "nRvpXOuNfsGydISiJIBDjIxym",
                    GRPC:              "klvUMOqRDCCHDfJnGontTOSrb",
                    GRPCUseTLS:        true,
                    AliasNode:         "ychfKkKLdcVdhEKzIQiOmWEvq",
                    AliasService:      "yStNhDWIaaAzfvgIYiDnotXbD",
                    TTL:               84,
                },
            },
        },
        SkipNodeUpdate: false,
        WriteRequest: structs.WriteRequest{
            Token: "",
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.RegisterRequest{}
    if err := decodeBody(req, &out, durations.FixupDurations); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/catalog_endpoint.go:
//    47
//    48    var args structs.DeregisterRequest
//    49:   if err := decodeBody(req, &args, nil); err != nil {
//    50        resp.WriteHeader(http.StatusBadRequest)
//    51        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// DeregisterRequest:
// Datacenter   string
// Node string
// ServiceID    string
// CheckID  types.CheckID
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityCatalogDeregister(t *testing.T) {

    jsonBlob := `{
    "Datacenter": "bgOhTHVpHBPoJNSxwLBnefPGI",
    "Node": "jQQHcrohvqtFIcsVEgXlpvCwD",
    "ServiceID": "cONnTvfUVNxSEDcAxSmRHoNDA",
    "CheckID": "dtdzjxeGNNFSRKHXDsTyRpZxh",
    "Token": "QuduyIkPDzigbHrWFZPjgvzfh"
}`
    // ------
    want := structs.DeregisterRequest{
        Datacenter: "bgOhTHVpHBPoJNSxwLBnefPGI",
        Node:       "jQQHcrohvqtFIcsVEgXlpvCwD",
        ServiceID:  "cONnTvfUVNxSEDcAxSmRHoNDA",
        CheckID:    "dtdzjxeGNNFSRKHXDsTyRpZxh",
        WriteRequest: structs.WriteRequest{
            Token: "",
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.DeregisterRequest{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/config_endpoint.go:
//   104
//   105    var raw map[string]interface{}
//   106:   if err := decodeBody(req, &raw, nil); err != nil {
//   107        return nil, BadRequestError{Reason: fmt.Sprintf("Request decoding failed: %v", err)}
//   108    }
// ==================================
func TestDecodeSanityConfigApply(t *testing.T) {
    t.Skip("Leave this fn as-is? Decoding code should probably be the same for all config parsing.")

}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/connect_ca_endpoint.go:
//    63    s.parseDC(req, &args.Datacenter)
//    64    s.parseToken(req, &args.Token)
//    65:   if err := decodeBody(req, &args.Config, nil); err != nil {
//    66        resp.WriteHeader(http.StatusBadRequest)
//    67        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// CARequest:
// Config   *structs.CAConfiguration
//     ClusterID    string
//     Provider string
//     Config   map[string]interface {} `faker:"-"`
//     RaftIndex    structs.RaftIndex
func TestDecodeSanityConnectCAConfigurationSet(t *testing.T) {

    jsonBlob := `{
    "Provider": "ujUEWyMxXcbuIuNvUgXlsIgjQ",
    "Config": {"key": ["a", "b"]},
    "CreateIndex": 64,
    "ModifyIndex": 60
}`
    // ------
    want := structs.CAConfiguration{
        Provider: "ujUEWyMxXcbuIuNvUgXlsIgjQ",
        Config:   map[string]interface{}{"key": []interface{}{"a", "b"}},
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.CAConfiguration{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/coordinate_endpoint.go:
//   151
//   152    args := structs.CoordinateUpdateRequest{}
//   153:   if err := decodeBody(req, &args, nil); err != nil {
//   154        resp.WriteHeader(http.StatusBadRequest)
//   155        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// CoordinateUpdateRequest:
// Datacenter   string
// Node string
// Segment  string
// Coord    *coordinate.Coordinate
//     Vec  []float64
//     Error    float64
//     Adjustment   float64
//     Height   float64
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityCoordinateUpdate(t *testing.T) {

    jsonBlob := `{
    "Datacenter": "HcuGBrkVaBKrnPMCjJpAEqxxm",
    "Node": "qXaREPEBEyseSsyPeJEbBijlB",
    "Segment": "YGEXdNgmcImbQozlBxqPodRux",
    "Coord": {
        "Vec": [
            0.004086097968223041,
            0.5949880007019714
        ],
        "Error": 0.5006509010628553,
        "Adjustment": 0.1941770527003058,
        "Height": 0.9192864811556534
    },
    "Token": "xXUXPJRdGrRoilSlybjUwJglX"
}`
    // ------
    want := structs.CoordinateUpdateRequest{
        Datacenter: "HcuGBrkVaBKrnPMCjJpAEqxxm",
        Node:       "qXaREPEBEyseSsyPeJEbBijlB",
        Segment:    "YGEXdNgmcImbQozlBxqPodRux",
        Coord: &coordinate.Coordinate{
            Vec: []float64{
                0.004086097968223041,
                0.5949880007019714,
            },
            Error:      0.5006509010628553,
            Adjustment: 0.1941770527003058,
            Height:     0.9192864811556534,
        },
        WriteRequest: structs.WriteRequest{
            Token: "",
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.CoordinateUpdateRequest{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/discovery_chain_endpoint.go:
//    29    if req.Method == "POST" {
//    30        var raw map[string]interface{}
//    31:       if err := decodeBody(req, &raw, nil); err != nil {
//    32            return nil, BadRequestError{Reason: fmt.Sprintf("Request decoding failed: %v", err)}
//    33        }
// ==================================
// discoveryChainReadRequest:
// OverrideMeshGateway  structs.MeshGatewayConfig
//     Mode structs.MeshGatewayMode // string
// OverrideProtocol string
// OverrideConnectTimeout   time.Duration
func TestDecodeSanityDiscoveryChainRead(t *testing.T) {

    // Special Beast!

    // This decodeBody call is a special beast, in that it decodes with decodeBody
    // into a map[string]interface{} and runs subsequent decoding logic outside of
    // the call.

    // decode code copied from agent/discovery_chain_endpoint.go
    fullDecodeFn := func(req *http.Request, v *discoveryChainReadRequest) error {
        var raw map[string]interface{}
        if err := decodeBody(req, &raw, nil); err != nil {
            return fmt.Errorf("Request decoding failed: %v", err)
        }

        apiReq, err := decodeDiscoveryChainReadRequest(raw)
        if err != nil {
            return fmt.Errorf("Request decoding failed: %v", err)
        }

        *v = *apiReq

        return nil
    }

    //
    jsonBlob := `{
    "OverrideMeshGateway": {
        "Mode": "bHdiZtWDhiGOYIBfdrxKzIXsy"
    },
    "OverrideProtocol": "PHexHsZwgfcFMLgHsHrZypNsK",
    "OverrideConnectTimeout": 45
}`
    // ------
    want := discoveryChainReadRequest{
        OverrideMeshGateway: structs.MeshGatewayConfig{
            Mode: "bHdiZtWDhiGOYIBfdrxKzIXsy",
        },
        OverrideProtocol:       "PHexHsZwgfcFMLgHsHrZypNsK",
        OverrideConnectTimeout: 45,
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := discoveryChainReadRequest{}
    if err := fullDecodeFn(req, &out); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/intentions_endpoint.go:
//    66    s.parseDC(req, &args.Datacenter)
//    67    s.parseToken(req, &args.Token)
//    68:   if err := decodeBody(req, &args.Intention, fixHashField); err != nil {
//    69        return nil, fmt.Errorf("Failed to decode request body: %s", err)
//    70    }
// ==================================
// IntentionRequest:
// Datacenter   string
// Op   structs.IntentionOp
// Intention    *structs.Intention
//     ID   string
//     Description  string
//     SourceNS string
//     SourceName   string
//     DestinationNS    string
//     DestinationName  string
//     SourceType   structs.IntentionSourceType
//     Action   structs.IntentionAction
//     DefaultAddr  string
//     DefaultPort  int
//     Meta map[string]string
//     Precedence   int
//     CreatedAt    time.Time   mapstructure:'-'
//     UpdatedAt    time.Time   mapstructure:'-'
//     Hash []uint8
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityIntentionCreate(t *testing.T) {

    jsonBlob := `{
    "ID": "cclyOZaryJEEDsRINWTABQNCt",
    "Description": "fiPRYLZCWmZcBeFkShKeACkZz",
    "SourceNS": "wKFfeTDaLMKUvmghRBahdHEBw",
    "SourceName": "RhFcEHcZYyewhOUkovzrPkrEM",
    "DestinationNS": "QArYTchvjregvrbikVSGrDPlO",
    "DestinationName": "qyzPSdxJBtEzDIGUcDluBpvkw",
    "SourceType": "KRgrRLzFetQjepZAhUqnarNso",
    "Action": "szKplhNGiIGecPybJyNtuYpUu",
    "DefaultAddr": "mlgWmRegQAqCzJzHMIBtumkFS",
    "DefaultPort": 8,
    "Meta": {
        "DWOxmqlRVyeoxjvrmjYBVqvKB": "NTSYFrXglykXBJZExNcYEzxVZ",
        "RNOIXmMvZsnKEEYVvZqhBRkME": "AWPOSQlSgAqZwvKgIQIeoNAlJ",
        "tANyeRYjkGjldZBWfeRYlEqPi": "CAUBeouHyrNcLjrxpGszwtQhz"
    },
    "Precedence": 11,
    "CreatedAt": "2311-12-18T10:52:24.0Z",
    "UpdatedAt": "2170-11-23T21:50:27.0Z",
    "Hash": "QF8=",
    "CreateIndex": 83,
    "ModifyIndex": 97
}`
    // ------
    want := structs.Intention{
        ID:              "cclyOZaryJEEDsRINWTABQNCt",
        Description:     "fiPRYLZCWmZcBeFkShKeACkZz",
        SourceNS:        "wKFfeTDaLMKUvmghRBahdHEBw",
        SourceName:      "RhFcEHcZYyewhOUkovzrPkrEM",
        DestinationNS:   "QArYTchvjregvrbikVSGrDPlO",
        DestinationName: "qyzPSdxJBtEzDIGUcDluBpvkw",
        SourceType:      "KRgrRLzFetQjepZAhUqnarNso",
        Action:          "szKplhNGiIGecPybJyNtuYpUu",
        DefaultAddr:     "mlgWmRegQAqCzJzHMIBtumkFS",
        DefaultPort:     8,
        Meta: map[string]string{
            "DWOxmqlRVyeoxjvrmjYBVqvKB": "NTSYFrXglykXBJZExNcYEzxVZ",
            "RNOIXmMvZsnKEEYVvZqhBRkME": "AWPOSQlSgAqZwvKgIQIeoNAlJ",
            "tANyeRYjkGjldZBWfeRYlEqPi": "CAUBeouHyrNcLjrxpGszwtQhz",
        },
        Precedence: 11,
        Hash:       []byte("QF8="),
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.Intention{}
    if err := decodeBody(req, &out, fixHashField); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/intentions_endpoint.go:
//   259    s.parseDC(req, &args.Datacenter)
//   260    s.parseToken(req, &args.Token)
//   261:   if err := decodeBody(req, &args.Intention, fixHashField); err != nil {
//   262        return nil, BadRequestError{Reason: fmt.Sprintf("Request decode failed: %v", err)}
//   263    }
// ==================================
func TestDecodeSanityIntentionSpecificUpdate(t *testing.T) {

    jsonBlob := `{
    "ID": "aQXTjAjIXwJGHuXInOdzqUESf",
    "Description": "IHCJvQdKQpdsOaQwahDqGkEMD",
    "SourceNS": "mVdxtatmgzGJqUrcHcOpiDZVL",
    "SourceName": "wJhwPabQjfVfCJuwhsEwtCkrr",
    "DestinationNS": "jEkpilRrvJYxQJNfMumVPmOfl",
    "DestinationName": "CeWTJeGNTKZWlXXlPUuMfgSEM",
    "SourceType": "qycwrOsFbHonURTYNGKsZvYod",
    "Action": "NLkvIcVlidVQnWuCuZdNUGdEX",
    "DefaultAddr": "kvYVFwRIBPiyqzNNfVxkDPzuZ",
    "DefaultPort": 49,
    "Meta": {
        "BgoxihIlNrkiOdvQwpqrybMZs": "jDWiqKejIEkpOorEtJbyxtCNZ",
        "plhFkRrCxAXSmEXHZFsXHoudH": "yrVLSDNvBdPNrjtwxGLkVFMPy"
    },
    "Precedence": 83,
    "CreatedAt": "2268-07-01T23:23:13.0-00:00",
    "UpdatedAt": "2131-06-14T04:39:35.0-00:00",
    "Hash": "Xw==",
    "CreateIndex": 55,
    "ModifyIndex": 17
}`
    // ------
    want := structs.Intention{
        ID:              "aQXTjAjIXwJGHuXInOdzqUESf",
        Description:     "IHCJvQdKQpdsOaQwahDqGkEMD",
        SourceNS:        "mVdxtatmgzGJqUrcHcOpiDZVL",
        SourceName:      "wJhwPabQjfVfCJuwhsEwtCkrr",
        DestinationNS:   "jEkpilRrvJYxQJNfMumVPmOfl",
        DestinationName: "CeWTJeGNTKZWlXXlPUuMfgSEM",
        SourceType:      "qycwrOsFbHonURTYNGKsZvYod",
        Action:          "NLkvIcVlidVQnWuCuZdNUGdEX",
        DefaultAddr:     "kvYVFwRIBPiyqzNNfVxkDPzuZ",
        DefaultPort:     49,
        Meta: map[string]string{
            "BgoxihIlNrkiOdvQwpqrybMZs": "jDWiqKejIEkpOorEtJbyxtCNZ",
            "plhFkRrCxAXSmEXHZFsXHoudH": "yrVLSDNvBdPNrjtwxGLkVFMPy",
        },
        Precedence: 83,
        Hash:       []byte("Xw=="),
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.Intention{}
    if err := decodeBody(req, &out, fixHashField); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/operator_endpoint.go:
//    77    var args keyringArgs
//    78    if req.Method == "POST" || req.Method == "PUT" || req.Method == "DELETE" {
//    79:       if err := decodeBody(req, &args, nil); err != nil {
//    80            return nil, BadRequestError{Reason: fmt.Sprintf("Request decode failed: %v", err)}
//    81        }
// ==================================
// type keyringArgs struct {
//  Key         string
//  Token       string
//  RelayFactor uint8
//  LocalOnly   bool // ?local-only; only used for GET requests
// }
func TestDecodeSanityOperatorKeyringEndpoint(t *testing.T) {

    jsonBlob := `{
    "Key": "ekvQHUTlOCiqvCPFYEDpWxOrO",
    "Token": "jIeJeAavTaqAtofIeUDUAYnRo",
    "RelayFactor": 67,
    "LocalOnly": true
}`
    // ------
    want := keyringArgs{
        Key:         "ekvQHUTlOCiqvCPFYEDpWxOrO",
        Token:       "jIeJeAavTaqAtofIeUDUAYnRo",
        RelayFactor: 67,
        LocalOnly:   true,
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := keyringArgs{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/operator_endpoint.go:
//   219        var conf api.AutopilotConfiguration
//   220        durations := NewDurationFixer("lastcontactthreshold", "serverstabilizationtime")
//   221:       if err := decodeBody(req, &conf, durations.FixupDurations); err != nil {
//   222            return nil, BadRequestError{Reason: fmt.Sprintf("Error parsing autopilot config: %v", err)}
//   223        }
// ==================================
// AutopilotConfiguration:
// CleanupDeadServers   bool
// LastContactThreshold *api.ReadableDuration
// MaxTrailingLogs  uint64
// ServerStabilizationTime  *api.ReadableDuration
// RedundancyZoneTag    string
// DisableUpgradeMigration  bool
// UpgradeVersionTag    string
// CreateIndex  uint64
// ModifyIndex  uint64
func TestDecodeSanityOperatorAutopilotConfiguration(t *testing.T) {

    jsonBlob := `{
    "CleanupDeadServers": false,
    "LastContactThreshold": "32ns",
    "MaxTrailingLogs": 58,
    "ServerStabilizationTime": "81ns",
    "RedundancyZoneTag": "HZhgtUpJGXDRIogvdfIhQpNFM",
    "DisableUpgradeMigration": false,
    "UpgradeVersionTag": "NsWecyqYiGZCeFzcCeoUQZMdX",
    "CreateIndex": 18,
    "ModifyIndex": 34
}`
    // ------
    want := api.AutopilotConfiguration{
        CleanupDeadServers:      false,
        LastContactThreshold:    readableDurationPtr("32ns"),
        MaxTrailingLogs:         58,
        ServerStabilizationTime: readableDurationPtr("81ns"),
        RedundancyZoneTag:       "HZhgtUpJGXDRIogvdfIhQpNFM",
        DisableUpgradeMigration: false,
        UpgradeVersionTag:       "NsWecyqYiGZCeFzcCeoUQZMdX",
        CreateIndex:             18,
        ModifyIndex:             34,
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := api.AutopilotConfiguration{}
    if err := decodeBody(req, &out, durations.FixupDurations); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/prepared_query_endpoint.go:
//    24    s.parseDC(req, &args.Datacenter)
//    25    s.parseToken(req, &args.Token)
//    26:   if err := decodeBody(req, &args.Query, nil); err != nil {
//    27        resp.WriteHeader(http.StatusBadRequest)
//    28        fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// PreparedQueryRequest:
// Datacenter   string
// Op   structs.PreparedQueryOp
// Query    *structs.PreparedQuery
//     ID   string
//     Name string
//     Session  string
//     Token    string
//     Template structs.QueryTemplateOptions
//         Type string
//         Regexp   string
//         RemoveEmptyTags  bool
//     Service  structs.ServiceQuery
//         Service  string
//         Failover structs.QueryDatacenterOptions
//             NearestN int
//             Datacenters  []string
//         OnlyPassing  bool
//         IgnoreCheckIDs   []types.CheckID `faker:"-"`
//         Near string
//         Tags []string
//         NodeMeta map[string]string
//         ServiceMeta  map[string]string
//         Connect  bool
//     DNS  structs.QueryDNSOptions
//         TTL  string
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanityPreparedQueryGeneral_Create(t *testing.T) {

    jsonBlob := `{
    "ID": "szWtzIrQgmHOoBtONVVZAxXkb",
    "Name": "WXEkBjpzKBMDUMsARWjdLMAAb",
    "Session": "VMMrtChmZTZYMHWEsSArcKABL",
    "Token": "jCbSYHaAmzHIcpPYZkvXhQwTU",
    "Template": {
        "Type": "bLgEuzxLuuJdZTKosGrYyewPN",
        "Regexp": "XdhlqfSkWHxjUjwZbhdGHhcoE",
        "RemoveEmptyTags": true
    },
    "Service": {
        "Service": "LYeYcVXKwMHZvnUyGHgEToYki",
        "Failover": {
            "NearestN": 28,
            "Datacenters": [
                "CcvRknZwYJompYREeIiyIueRx",
                "tpyQUvZEhHWVUSdPCVXpqFrTP"
            ]
        },
        "OnlyPassing": true,
        "IgnoreCheckIDs": [
            "iIeqldhpjPasDLDyoeiIZfOcE"
        ],
        "Near": "wbXJMdogJNsPBYRJWwRQJanow",
        "Tags": [
            "kKCebMWHtOksNeIvxuzzvOpgz",
            "LOJFxxSeziuAqYkxwkCxpUqxL",
            "wwcYtvezJbNmNMeDysPanrGTD"
        ],
        "NodeMeta": {
            "LmswfngAYQRpQqbpEQLMDGzNb": "lAVpxHTMRXiLhzYBbBvifKicA",
            "VSzSdhcQHVhGtEpKmoTAcONAQ": "dkKDiZKOLWHuDzfoZAPezwcuy"
        },
        "ServiceMeta": {
            "EjxXpLQAgZOTJUeoWfiiPjQxs": "RpBCcxuPqBzXECpbgGAdVkYwi",
            "uxxXQFVhYhTUPtgEBeFDLZFfv": "iOSPQgmbWpKUBBGDmKyvdtBxU",
            "vWSgJeEcGISzYRPEXElOJgpZA": "kmfBOXAexQIaAbksRzUFgaKTO"
        },
        "Connect": false
    },
    "DNS": {
        "TTL": "ioxcZkJIRgsRxBTbfKuWeMlXd"
    },
    "CreateIndex": 70,
    "ModifyIndex": 31
}`
    // ------
    want := structs.PreparedQuery{

        ID:      "szWtzIrQgmHOoBtONVVZAxXkb",
        Name:    "WXEkBjpzKBMDUMsARWjdLMAAb",
        Session: "VMMrtChmZTZYMHWEsSArcKABL",
        Token:   "jCbSYHaAmzHIcpPYZkvXhQwTU",
        Template: structs.QueryTemplateOptions{
            Type:            "bLgEuzxLuuJdZTKosGrYyewPN",
            Regexp:          "XdhlqfSkWHxjUjwZbhdGHhcoE",
            RemoveEmptyTags: true,
        },
        Service: structs.ServiceQuery{
            Service: "LYeYcVXKwMHZvnUyGHgEToYki",
            Failover: structs.QueryDatacenterOptions{
                NearestN: 28,
                Datacenters: []string{
                    "CcvRknZwYJompYREeIiyIueRx",
                    "tpyQUvZEhHWVUSdPCVXpqFrTP",
                },
            },
            OnlyPassing: true,
            IgnoreCheckIDs: []types.CheckID{
                "iIeqldhpjPasDLDyoeiIZfOcE",
            },
            Near: "wbXJMdogJNsPBYRJWwRQJanow",
            Tags: []string{
                "kKCebMWHtOksNeIvxuzzvOpgz",
                "LOJFxxSeziuAqYkxwkCxpUqxL",
                "wwcYtvezJbNmNMeDysPanrGTD",
            },
            NodeMeta: map[string]string{
                "LmswfngAYQRpQqbpEQLMDGzNb": "lAVpxHTMRXiLhzYBbBvifKicA",
                "VSzSdhcQHVhGtEpKmoTAcONAQ": "dkKDiZKOLWHuDzfoZAPezwcuy",
            },
            ServiceMeta: map[string]string{
                "EjxXpLQAgZOTJUeoWfiiPjQxs": "RpBCcxuPqBzXECpbgGAdVkYwi",
                "uxxXQFVhYhTUPtgEBeFDLZFfv": "iOSPQgmbWpKUBBGDmKyvdtBxU",
                "vWSgJeEcGISzYRPEXElOJgpZA": "kmfBOXAexQIaAbksRzUFgaKTO",
            },
            Connect: false,
        },
        DNS: structs.QueryDNSOptions{
            TTL: "ioxcZkJIRgsRxBTbfKuWeMlXd",
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.PreparedQuery{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/prepared_query_endpoint.go:
//   254    s.parseToken(req, &args.Token)
//   255    if req.ContentLength > 0 {
//   256:       if err := decodeBody(req, &args.Query, nil); err != nil {
//   257            resp.WriteHeader(http.StatusBadRequest)
//   258            fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
func TestDecodeSanityPreparedQueryGeneral_Update(t *testing.T) {

    jsonBlob := `{
    "ID": "OiyRCBPtaiDcDnaxyuCkFoIcX",
    "Name": "GNVifqVJKnqYAKNMNrOtsBYkJ",
    "Session": "uQTnZraAYUmvsmAKxszXTWgJG",
    "Token": "UWBAqEBKJjcPmjacBFokAhObx",
    "Template": {
        "Type": "exVnteMQUQgevFJpYwrKcUCdQ",
        "Regexp": "AZsrllqdtIBQqQbGlmpnZkjsg",
        "RemoveEmptyTags": false
    },
    "Service": {
        "Service": "qwOXRmktaURQgySjbdXiUYoQm",
        "Failover": {
            "NearestN": 31,
            "Datacenters": [
                "KQGeNdOtcxOtIADfHHNinUxRI"
            ]
        },
        "OnlyPassing": false,
        "IgnoreCheckIDs": [
            "QuWEYZVYdDSnqgZGrabYWMnUd",
            "ItuepgqlquMPCNKIOsnLcpnlK",
            "vbCMohaNciokqPTCgmLBVJREa"
        ],
        "Near": "vDwdQYPBjCddGmXKoMdTzfNOa",
        "Tags": [
            "SVnYZUMNJkSSPKPLGXMlpXrMO",
            "adNAjbSHGKgOClrjzVJNbErYZ",
            "NPXseAbVDoQbRWUmqyFRFQlmN",
            "cIwQJmjszSxICUfsZTIPRRYeS"
        ],
        "NodeMeta": {
            "FbEXljobHRUKczZmgObflXOGH": "wsVWHKCGvmDEfJOupbnpPqlNC",
            "XXCZzrZAYDnkprYEsCuNzTVGU": "vRXQyMIOuvZmUnbijJhaiHMXf",
            "nCbmSuJRyFZnyiqxYmOoeFcYe": "ArQXKHpbrxuSkYphlLcztpylj"
        },
        "ServiceMeta": {
            "KZNxgOsmZdjkjevQcnIfkQvPW": "TKcoNeEYPpCMeCmKcOklHRwzB",
            "KuIzPBtVgLIoADoAjJMgqRJnD": "mdQcYgONNtDtCoJUkfVHZyqeN",
            "iTXzvfaSIgFaqPNKKemNHHoYa": "PAkwCzhCQWBPzzGSswoJLCxiR"
        },
        "Connect": true
    },
    "DNS": {
        "TTL": "ixunIqAgRRAREUrPuLpNNAyWK"
    },
    "CreateIndex": 28,
    "ModifyIndex": 20
}`
    // ------
    want := structs.PreparedQuery{
        ID:      "OiyRCBPtaiDcDnaxyuCkFoIcX",
        Name:    "GNVifqVJKnqYAKNMNrOtsBYkJ",
        Session: "uQTnZraAYUmvsmAKxszXTWgJG",
        Token:   "UWBAqEBKJjcPmjacBFokAhObx",
        Template: structs.QueryTemplateOptions{
            Type:            "exVnteMQUQgevFJpYwrKcUCdQ",
            Regexp:          "AZsrllqdtIBQqQbGlmpnZkjsg",
            RemoveEmptyTags: false,
        },
        Service: structs.ServiceQuery{
            Service: "qwOXRmktaURQgySjbdXiUYoQm",
            Failover: structs.QueryDatacenterOptions{
                NearestN: 31,
                Datacenters: []string{
                    "KQGeNdOtcxOtIADfHHNinUxRI",
                },
            },
            OnlyPassing: false,
            IgnoreCheckIDs: []types.CheckID{
                "QuWEYZVYdDSnqgZGrabYWMnUd",
                "ItuepgqlquMPCNKIOsnLcpnlK",
                "vbCMohaNciokqPTCgmLBVJREa",
            },
            Near: "vDwdQYPBjCddGmXKoMdTzfNOa",
            Tags: []string{
                "SVnYZUMNJkSSPKPLGXMlpXrMO",
                "adNAjbSHGKgOClrjzVJNbErYZ",
                "NPXseAbVDoQbRWUmqyFRFQlmN",
                "cIwQJmjszSxICUfsZTIPRRYeS",
            },
            NodeMeta: map[string]string{
                "FbEXljobHRUKczZmgObflXOGH": "wsVWHKCGvmDEfJOupbnpPqlNC",
                "XXCZzrZAYDnkprYEsCuNzTVGU": "vRXQyMIOuvZmUnbijJhaiHMXf",
                "nCbmSuJRyFZnyiqxYmOoeFcYe": "ArQXKHpbrxuSkYphlLcztpylj",
            },
            ServiceMeta: map[string]string{
                "KZNxgOsmZdjkjevQcnIfkQvPW": "TKcoNeEYPpCMeCmKcOklHRwzB",
                "KuIzPBtVgLIoADoAjJMgqRJnD": "mdQcYgONNtDtCoJUkfVHZyqeN",
                "iTXzvfaSIgFaqPNKKemNHHoYa": "PAkwCzhCQWBPzzGSswoJLCxiR",
            },
            Connect: true,
        },
        DNS: structs.QueryDNSOptions{
            TTL: "ixunIqAgRRAREUrPuLpNNAyWK",
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := structs.PreparedQuery{}
    if err := decodeBody(req, &out, nil); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/session_endpoint.go:
//    54            return nil
//    55        }
//    56:       if err := decodeBody(req, &args.Session, fixup); err != nil {
//    57            resp.WriteHeader(http.StatusBadRequest)
//    58            fmt.Fprintf(resp, "Request decode failed: %v", err)
// ==================================
// SessionRequest:
// Datacenter   string
// Op   structs.SessionOp
// Session  structs.Session
//     ID   string
//     Name string
//     Node string
//     Checks   []types.CheckID
//     LockDelay    time.Duration
//     Behavior structs.SessionBehavior
//     TTL  string
//     RaftIndex    structs.RaftIndex
//         CreateIndex  uint64
//         ModifyIndex  uint64
// WriteRequest structs.WriteRequest
//     Token    string
func TestDecodeSanitySessionCreate(t *testing.T) {

    jsonBlob := `{
    "ID": "kJLoWZVFxYVYqudQCOJyMXCLR",
    "Name": "LywYNmBHKjqEwWWRZNtWZCadx",
    "Node": "ZAOKlyQEhDByRqRgwjpwJPfun",
    "Checks": [
        "iiMTuLhLjsIBlKIrihuHvgkvX",
        "ItrpBRVAukkXIzDvGdJLqrAUu"
    ],
    "LockDelay": 45,
    "Behavior": "jZhPXULgGCnrGqydbVDcibBNa",
    "TTL": "iIoLjFGGMnuqdoPSUruypRRvH",
    "CreateIndex": 70,
    "ModifyIndex": 9
}`
    // ------
    want := structs.Session{
        ID:   "kJLoWZVFxYVYqudQCOJyMXCLR",
        Name: "LywYNmBHKjqEwWWRZNtWZCadx",
        Node: "ZAOKlyQEhDByRqRgwjpwJPfun",
        Checks: []types.CheckID{
            "iiMTuLhLjsIBlKIrihuHvgkvX",
            "ItrpBRVAukkXIzDvGdJLqrAUu",
        },
        LockDelay: 45 * time.Second,
        Behavior:  "jZhPXULgGCnrGqydbVDcibBNa",
        TTL:       "iIoLjFGGMnuqdoPSUruypRRvH",
    }

    out := structs.Session{}
    // copied from agent/session_endpoint.go
    fixupCB := func(raw interface{}) error {
        if err := FixupLockDelay(raw); err != nil {
            return err
        }
        if err := FixupChecks(raw, &out); err != nil {
            return err
        }
        return nil
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)

    if err := decodeBody(req, &out, fixupCB); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

// ==================================
// $GOPATH/github.com/hashicorp/consul/agent/txn_endpoint.go:
//   116    // associate the error with a given operation.
//   117    var ops api.TxnOps
//   118:   if err := decodeBody(req, &ops, fixupTxnOps); err != nil {
//   119        resp.WriteHeader(http.StatusBadRequest)
//   120        fmt.Fprintf(resp, "Failed to parse body: %v", err)
// ==================================
// TxnOps:
//         KV   *api.KVTxnOp
//             Verb api.KVOp
//             Key  string
//             Value    []uint8
//             Flags    uint64
//             Index    uint64
//             Session  string
//         Node *api.NodeTxnOp
//             Verb api.NodeOp
//             Node api.Node
//                 ID   string
//                 Node string
//                 Address  string
//                 Datacenter   string
//                 TaggedAddresses  map[string]string
//                 Meta map[string]string
//                 CreateIndex  uint64
//                 ModifyIndex  uint64
//         Service  *api.ServiceTxnOp
//             Verb api.ServiceOp
//             Node string
//             Service  api.AgentService
//                 Kind api.ServiceKind
//                 ID   string
//                 Service  string
//                 Tags []string
//                 Meta map[string]string
//                 Port int
//                 Address  string
//                 TaggedAddresses  map[string]api.ServiceAddress
//                     Address  string
//                     Port int
//                 Weights  api.AgentWeights
//                     Passing  int
//                     Warning  int
//                 EnableTagOverride    bool
//                 CreateIndex  uint64
//                 ModifyIndex  uint64
//                 ContentHash  string
//                 Proxy    *api.AgentServiceConnectProxyConfig
//                     DestinationServiceName   string
//                     DestinationServiceID string
//                     LocalServiceAddress  string
//                     LocalServicePort int
//                     Config   map[string]interface {} `faker:"-"`
//                     Upstreams    []api.Upstream
//                         DestinationType  api.UpstreamDestType
//                         DestinationNamespace string
//                         DestinationName  string
//                         Datacenter   string
//                         LocalBindAddress string
//                         LocalBindPort    int
//                         Config   map[string]interface {} `faker:"-"`
//                         MeshGateway  api.MeshGatewayConfig
//                             Mode api.MeshGatewayMode
//                     MeshGateway  api.MeshGatewayConfig
//                     Expose   api.ExposeConfig
//                         Checks   bool
//                         Paths    []api.ExposePath
//                             ListenerPort int
//                             Path string
//                             LocalPathPort    int
//                             Protocol string
//                             ParsedFromCheck  bool
//                 Connect  *api.AgentServiceConnect
//                     Native   bool
//                     SidecarService   *api.AgentServiceRegistration
//                         Kind api.ServiceKind
//                         ID   string
//                         Name string
//                         Tags []string
//                         Port int
//                         Address  string
//                         TaggedAddresses  map[string]api.ServiceAddress
//                         EnableTagOverride    bool
//                         Meta map[string]string
//                         Weights  *api.AgentWeights
//                         Check    *api.AgentServiceCheck
//                             CheckID  string
//                             Name string
//                             Args []string
//                             DockerContainerID    string
//                             Shell    string
//                             Interval string
//                             Timeout  string
//                             TTL  string
//                             HTTP string
//                             Header   map[string][]string
//                             Method   string
//                             TCP  string
//                             Status   string
//                             Notes    string
//                             TLSSkipVerify    bool
//                             GRPC string
//                             GRPCUseTLS   bool
//                             AliasNode    string
//                             AliasService string
//                             DeregisterCriticalServiceAfter   string
//                         Checks   api.AgentServiceChecks
//                         Proxy    *api.AgentServiceConnectProxyConfig
//                         Connect  *api.AgentServiceConnect
//         Check    *api.CheckTxnOp
//             Verb api.CheckOp
//             Check    api.HealthCheck
//                 Node string
//                 CheckID  string
//                 Name string
//                 Status   string
//                 Notes    string
//                 Output   string
//                 ServiceID    string
//                 ServiceName  string
//                 ServiceTags  []string
//                 Definition   api.HealthCheckDefinition
//                     HTTP string
//                     Header   map[string][]string
//                     Method   string
//                     TLSSkipVerify    bool
//                     TCP  string
//                     IntervalDuration time.Duration
//                     TimeoutDuration  time.Duration
//                     DeregisterCriticalServiceAfterDuration   time.Duration
//                     Interval api.ReadableDuration
//                     Timeout  api.ReadableDuration
//                     DeregisterCriticalServiceAfter   api.ReadableDuration
//                 CreateIndex  uint64
//                 ModifyIndex  uint64
func TestDecodeSanityTxnConvertOps(t *testing.T) {

    jsonBlob := `[{
    "KV": {
        "Verb": "aguwcHAiTZoSmVkcPgtFUDwcX",
        "Key": "zRizDYPePmZJllyoerFyOughD",
        "Value": "",
        "Flags": 21,
        "Index": 12,
        "Session": "ilRLmLzwGxBoBlGkcedILBFsy"
    },
    "Node": {
        "Verb": "TxNYyuZsQcPhOzMSkUyqqrXkp",
        "Node": {
            "ID": "JNbiKHnsaYawbwmuYmfWVxUCs",
            "Node": "ytijvGMzYbmRqodQjCmahTtkb",
            "Address": "eOBmzvFWUgcVfBuCUTQFcKlih",
            "Datacenter": "QWMePJNbwlpBEsCeGPvkoHnCg",
            "TaggedAddresses": {
                "GccNhyKKNPreqWiLbeosHnOYS": "ttMjjWJdbLRematNUHisANoRa",
                "KkaFklnbdSrrKpWDPYiKWJOEh": "FJVJYJcnbhSGAWUqsaLixeHPJ",
                "KyoHyKNHDULOkTjCMwLZPrCnq": "CZZIJilQyjtEXOXHsAUHfOdgx"
            },
            "Meta": {
                "KnAAsMQEscpTTAvsUCljSqIKk": "GqXkwUPcDLXftjHJRBclbEpil",
                "ulHLAbQPlJECGUwkvrDFdJigE": "eiHarMvnUYSQnqmmyfkAYZVsS",
                "ybEWIMcMiNrHpQQnZWhpCiwpL": "fYTJrhVgvwPlrBtVbfvhNfYln"
            },
            "CreateIndex": 96,
            "ModifyIndex": 58
        }
    },
    "Service": {
        "Verb": "TorUpGyEngNkxvgFeIjHgwyJi",
        "Node": "NEbdNybpLdBHBWklNBqrtEFaF",
        "Service": {
            "Kind": "zdjMDamJfJOQKpNEzxThGjrNd",
            "ID": "lKwMnuNvGuujPGGigUETRsibN",
            "Service": "hZoOoNRkySIvNMyXZMxqpvnBX",
            "Tags": [],
            "Meta": {
                "CScdKhmjojBiOUkksIyRsUezZ": "ORsdtERCqUfFlbIgxFiMmZwqe",
                "oBJzFaEhHlHZufjfLMzAZbmuB": "pUzaNjqvvGBDUctjoqeQyqMWx",
                "vOtvFeyZQzdtetgunusyfHhkV": "vNIHVqaTCIAEdTAhIpnaEzRTZ"
            },
            "Port": 31,
            "Address": "gVTaCcbehltNslOyoVKRPFykD",
            "TaggedAddresses": {
                "fSzoHGQjYtKYtlUatmMdCicCJ": {
                    "Address": "aWbVngjORsiveZuRtWvFRwruW",
                    "Port": 86
                }
            },
            "Weights": {
                "Passing": 46,
                "Warning": 40
            },
            "EnableTagOverride": false,
            "CreateIndex": 38,
            "ModifyIndex": 85,
            "ContentHash": "nNvQKctxiDVoWlxpAEWLszCQQ",
            "Proxy": {
                "DestinationServiceName": "SsUUucquwmmFtTBNODGVOxJhS",
                "DestinationServiceID": "BGkHcrIsfQOYVsoGBRpLrSRrN",
                "LocalServiceAddress": "cboecAdgiKSNaYpQxHTosjdrm",
                "LocalServicePort": 67,
                "Config": {"a":["a","b"]},
                "Upstreams": [{"DestinationNamespace": "a", "Datacenter": "b"}],
                "MeshGateway": {
                    "Mode": "XmdGptouMkWdXVfhsRynmrRyJ"
                },
                "Expose": {
                    "Checks": true,
                    "Paths": [
                        {
                            "ListenerPort": 1,
                            "Path": "zmvwmBwHmdZZvHoGwMLqhpWFQ",
                            "LocalPathPort": 76,
                            "Protocol": "XJBkrrlBhyaKJXQldeYQyQRlA",
                            "ParsedFromCheck": false
                        }
                    ]
                }
            },
            "Connect": {
                "Native": true,
                "SidecarService": {
                    "Kind": "uLNfcsNmxNJIlVbqdNkmTmSeO",
                    "ID": "kxRmkxZIwiPILaRPwoAUYZVGw",
                    "Name": "cfvuzKLFmtepVXtXNAupSoNRI",
                    "Tags": [
                        "nNABnFgSwGejWwjSVIbtZVHzX",
                        "ZbqnLSgRpacFBOEKYFwwmEkAP",
                        "XliPVOTPzbRSGSHQHsWhyEixB"
                    ],
                    "Port": 79,
                    "Address": "gWFGbPLhzJEfJyKLQvzhDLCVH",
                    "Meta": {
                        "QdsCYEZBVnEeSbQukezFiUzNW": "ubEywdLpXHzgqQKUDExwSEuaw",
                        "TcsPrVVzctJgTgPFMREmOZUVJ": "KyaOwEqjAgkfCQDNhERkimmEK",
                        "WtSAeQGJjUxetzzPXEgNWfoTX": "cnYZtDCuONEXeskhJaXuZoBfO"
                    },
                    "Weights": {
                        "Passing": 36,
                        "Warning": 67
                    },
                    "Check": {
                        "CheckID": "aPYronlUgogTpMgifXaSaDhjw",
                        "Name": "oRZACBwHqLnCiCKZQhAIsvtbU",
                        "ScriptArgs": [
                            "jFBpRHFVrbAeYvabadEbWuUuM"
                        ],
                        "DockerContainerID": "ksgmUjXaGnCBftnlIDQGrKXci",
                        "Shell": "jWjcxmSlbMhNphgvJOkdKFRfd",
                        "Interval": "ReGgAEThqNrDjPcBoXQhfbwLn",
                        "Timeout": "sQoBHFiHwCElObOkDxcLDfMwc",
                        "TTL": "YomkdRYzZSprGyKKacdbErZzY",
                        "HTTP": "PJdpozKoAhfLlghtWyiLQSksy",
                        "Header": {
                            "FsYFGasjjbsEQVvxHDzmiOFIT": [
                                "AueitZMnHgJhYARywoIyAgqBg"
                            ],
                            "VDiIlojOySPSwGmXhwAsOovAs": [],
                            "jIxgDRhzPiCKnnZyUmioUHiHz": [
                                "WeNLLcmjOYpxnWsEYMpnkknfy",
                                "CLCmXqVrmqpcvYZQTsIxIhPFE",
                                "ASpCFpEXySlZHlGfUUyZbUxWQ"
                            ],
                            "jTmskqkVFAOxyAfeEMduCPHiS": [
                                "AlBEBRPkmqdWSBLOhPzEgOBsb",
                                "LXVMNQffaJLTBPmyDcNGpmCnu",
                                "hPvbIKuQPyswFVUwlhLcRNktC",
                                "scCIVNxGbfZXScuaMRzrcOnjr"
                            ]
                        },
                        "Method": "MeROgsXljsTPkkeaScCPKoguj",
                        "TCP": "eUNcgTsmShhyOSyuLUJItzftk",
                        "Status": "TzFMpHZmvkYFteUOJneZhQTUX",
                        "Notes": "rlTBDijxJsafhgmUokqhstAGb",
                        "TLSSkipVerify": true,
                        "GRPC": "TsSXWkgmRkFRWBGCALAbbeYYU",
                        "GRPCUseTLS": true,
                        "AliasNode": "DhuHnFarJUVxKpmfnhOFerMUP",
                        "AliasService": "kpBBcEQEiukQMvWJhdevtstdL",
                        "DeregisterCriticalServiceAfter": "KiDfnfqPXwhWskyBsDtXWvEcw"
                    },
                    "Checks": [
                        {
                            "CheckID": "aSfjUkvTYrWNdnOiCoHVnwkPG",
                            "Name": "roECnIUGJeCisbuJbNFILUzzw",
                            "ScriptArgs": [
                                "mBWidaFkltcRYJNZFRNLQFrOw",
                                "JTQiksViBpxpeIVLrHVjmEbuR",
                                "xQVfCKwikDyqpYpMfnKGuuZDH",
                                "TXucxaboiMDQvWRbZlmdDlTWw"
                            ],
                            "DockerContainerID": "xPtcJlUDSrRphThTOaPLLtORM",
                            "Shell": "qNBIZZiAdKhpmjDhIKmQCXLJm",
                            "Interval": "owdXiJzMFqvysWaLxxkIJpLIo",
                            "Timeout": "YPzTyDgLwDbDDnavmlVpBhFyw",
                            "TTL": "GWHiGsaQRnjyAeOIBRbILlnNz",
                            "HTTP": "ZTyGRJnYqWXgYPCYlzsdOZZMs",
                            "Header": {
                                "NSBetqehGhCVKrSlehesskDPi": [
                                    "cMKNaJuiDSlmPkfRLMEZhLNce",
                                    "VEBbHHNnOUPDVezblyeDDPfsW"
                                ],
                                "jVslXkOALVmwIdMYZthbyXyqS": [
                                    "sUqKctQHHVlwmuXsaeXUUlXbl",
                                    "UQsVTxZQevjoRAgevsicvSgEI"
                                ],
                                "ocbAGkvLcjXEUlqPCmlJpBugO": [
                                    "cSpHcSJqbVagEkWBKffzdxCIg",
                                    "UsqLqFELsNtbBXWyuuEOqJQVK",
                                    "SLubRSRzpCnxemiOSpLFKcugM"
                                ],
                                "rxEwDAuZFLyPWeCXDIibwIJHf": []
                            },
                            "Method": "tgCBkeBLEDFIfKjuRnbSwKKiX",
                            "TCP": "ROdCCAfELQbTFeljyokiGZvmn",
                            "Status": "EVMxCalesoHfggMSRvQvMBnVB",
                            "Notes": "fXSCvqWgamqSHjSbhWWbMVqIN",
                            "TLSSkipVerify": true,
                            "GRPC": "WnyhRjycoVacILSiUkAmFdJZO",
                            "AliasNode": "PudhtUowROdGMtTVxGTLZmWVt",
                            "AliasService": "twiZosGDsJzTWGPeWzHtHSacs",
                            "DeregisterCriticalServiceAfter": "KPlyDPCtWNNNPzMwJmCsqYGdK"
                        },
                        {
                            "CheckID": "HkLHtkWLGDBBAPTCyfYBOkWQn",
                            "Name": "hNAdBkZSVDEonqbhavsSuDiBq",
                            "ScriptArgs": [
                                "sbElDaPWHjmQeFbLFOBQanynh",
                                "VMwQAweZFPQRAKmIayRsLDHrG",
                                "xIxqbrxkacQHgEbkvTbAgsKgu"
                            ],
                            "DockerContainerID": "TuqnIGeYsPrBUTaEiyHuvosxg",
                            "Shell": "SlwtPEBAqkZttHPDTjybhfSag",
                            "Interval": "uhGdARgJqkgszJhGdnqTsdsUY",
                            "Timeout": "FpHoBlkicccFMYdawJzpQtmRs",
                            "TTL": "ziQsYCCDICYSZTalHSriSBCTc",
                            "HTTP": "CUglrvYwveMkXyMTdIXyLSEDb",
                            "Header": {
                                "IOasgUiGKYGedGkqRKsMAEHDL": [],
                                "VLBppsXUtKKqbsGwPCLVkQVlu": [
                                    "aqfnCXHtdNCYztNobbDicUmtQ",
                                    "bpyAcNdhqPoyTiZDSGuXlWMUV",
                                    "hCIfUpRvZlMfEKDAFzWjtfZrN",
                                    "rVNcLUVHodcyHWHdDebMFKkiY"
                                ],
                                "ndVKJnrpWwvRCwBHQoTBdpNCJ": [
                                    "XhKKbQhncmEHrThPZhTJGbwJv",
                                    "BeLbhpekpnJHUgYcfMjtuPKfB",
                                    "dBqDevVgpxyIShBINOmiAMZDQ"
                                ]
                            },
                            "Method": "lHeOFWkNLDqmAQNknkEDLhOYs",
                            "TCP": "EFOUEKIUTnNCyAHxGsJBvafQK",
                            "Status": "taotLGLXPqROZBGCCphmfvyun",
                            "Notes": "uTHzhXYKcxDtpKOFvwJEKSvzJ",
                            "GRPC": "jdVLFicYqfyCjKhZEYPLkhzJO",
                            "AliasNode": "fUWGLRmauoBpgmfxpDqKfRSRr",
                            "AliasService": "YwNITUNCbRjOfjVAmXtVRfviL",
                            "DeregisterCriticalServiceAfter": "fjsvvogszaSbFenLvhwGPIazN"
                        }
                    ],
                    "Proxy": {
                        "DestinationServiceName": "zMZsYnugmtDqSmBWxKhakPkOI",
                        "DestinationServiceID": "SoCkzYSBnLeEvsNtaVIReCoZp",
                        "LocalServiceAddress": "FiyLHtNUqPDNrAktaMwQHCmZE",
                        "LocalServicePort": 35,
                        "Config": {"a": ["a","b"]},
                        "Upstreams": [
                            {
                                "DestinationType": "hpKYRZfqOZuvAMatGtRGFxVUY",
                                "DestinationNamespace": "FqdDCIhqLZpoQdXLYQzvoBBsG",
                                "DestinationName": "GXWNyJtmVvDrEgKiihfWQAvbN",
                                "Datacenter": "WyqVPmJfRmBcDPdclEVXoGWxV",
                                "LocalBindAddress": "kSKZMYZAOoLJDGBgngzSitAwW",
                                "LocalBindPort": 1,
                                "MeshGateway": {
                                    "Mode": "oVimspemmZpYVdZgJrvfjBify"
                                }
                            }
                        ],
                        "MeshGateway": {
                            "Mode": "dnFFBQWuYcrVjGUpuDarbUaDK"
                        },
                        "Expose": {
                            "Checks": true,
                            "Paths": [
                                {
                                    "ListenerPort": 63,
                                    "Path": "ppnYwsMIddkJIdZLgcRTTbhuj",
                                    "LocalPathPort": 64,
                                    "Protocol": "IHrdLEYtaVeAQOjOcHrGKvOyH",
                                    "ParsedFromCheck": true
                                },
                                {
                                    "ListenerPort": 78,
                                    "Path": "nCFFMqUtwiknPLzmjhnyJwuqD",
                                    "LocalPathPort": 90,
                                    "Protocol": "LYdjSiUdKCgHOhEdVdSQsVtNf",
                                    "ParsedFromCheck": true
                                }
                            ]
                        }
                    },
                    "Connect": {
                        "Native": true,
                        "SidecarService": {
                            "Kind": "1uLNfcsNmxNJIlVbqdNkmTmSeO",
                            "ID": "1kxRmkxZIwiPILaRPwoAUYZVGw",
                            "Name": "1cfvuzKLFmtepVXtXNAupSoNRI",
                            "Tags": [
                                "1nNABnFgSwGejWwjSVIbtZVHzX",
                                "1ZbqnLSgRpacFBOEKYFwwmEkAP",
                                "1XliPVOTPzbRSGSHQHsWhyEixB"
                            ],
                            "Port": 179
                        }
                    }
                }
            }
        }
    },
    "Check": {
        "Verb": "HmouRVeKEPKxiRCmSVdyEYOPE",
        "Check": {
            "Node": "lLvkgAQLFxgMXrDoqfqBswISn",
            "CheckID": "DoqXdDXKkXuUzfuJHlgsgKzyv",
            "Name": "ZdZltqOSHyASTHyraSDTmGjjp",
            "Status": "YBvDJcoQkphEGaQGDJqdKnREU",
            "Notes": "eluItMsDHhVPKymbrNijkBEFQ",
            "Output": "KENOcCQQakWfObmazgdYITpNp",
            "ServiceID": "bSsqqDSJJVhTlzCRzNywHLOlj",
            "ServiceName": "fcDUJDEVzFsqTLYXcMYXcuIeQ",
            "ServiceTags": [],
            "Definition": {
                "Interval": "45ns",
                "Timeout": "65ns",
                "DeregisterCriticalServiceAfter": "7ns",
                "HTTP": "vJYDirutpVomHpNYFHTQWxqHZ",
                "Header": {
                    "QGdOMTerHUjtrgwIaIYdtSepn": [
                        "lbmyGdqTIGzDTwTUTCyymHnfh",
                        "jDyFZXAcYTbLLHtqcqkKmvQEy",
                        "ysfTpggsaRuZXdJYbzBMNHXHb"
                    ],
                    "bfHwiOouuBltgTITnfpcFimyF": [
                        "hHvogvFspWHRAjqaYTOhScfls",
                        "uvwHmnzohChYLuRizskrRdbZc",
                        "wejmTCtzZLFfvHqDuRizcNEFU"
                    ]
                },
                "Method": "zDBrxVaFHZTpGpECXsdDderlq",
                "TLSSkipVerify": true,
                "TCP": "YgIFPmTyuhPKppqCznwIYShrU"
            },
            "CreateIndex": 63,
            "ModifyIndex": 73
        }
    }
}]`
    // ------
    want := api.TxnOps{
        {
            KV: &api.KVTxnOp{
                Verb:    "aguwcHAiTZoSmVkcPgtFUDwcX",
                Key:     "zRizDYPePmZJllyoerFyOughD",
                Value:   nil,
                Flags:   21,
                Index:   12,
                Session: "ilRLmLzwGxBoBlGkcedILBFsy",
            },
            Node: &api.NodeTxnOp{
                Verb: "TxNYyuZsQcPhOzMSkUyqqrXkp",
                Node: api.Node{
                    ID:         "JNbiKHnsaYawbwmuYmfWVxUCs",
                    Node:       "ytijvGMzYbmRqodQjCmahTtkb",
                    Address:    "eOBmzvFWUgcVfBuCUTQFcKlih",
                    Datacenter: "QWMePJNbwlpBEsCeGPvkoHnCg",
                    TaggedAddresses: map[string]string{
                        "GccNhyKKNPreqWiLbeosHnOYS": "ttMjjWJdbLRematNUHisANoRa",
                        "KkaFklnbdSrrKpWDPYiKWJOEh": "FJVJYJcnbhSGAWUqsaLixeHPJ",
                        "KyoHyKNHDULOkTjCMwLZPrCnq": "CZZIJilQyjtEXOXHsAUHfOdgx",
                    },
                    Meta: map[string]string{
                        "KnAAsMQEscpTTAvsUCljSqIKk": "GqXkwUPcDLXftjHJRBclbEpil",
                        "ulHLAbQPlJECGUwkvrDFdJigE": "eiHarMvnUYSQnqmmyfkAYZVsS",
                        "ybEWIMcMiNrHpQQnZWhpCiwpL": "fYTJrhVgvwPlrBtVbfvhNfYln",
                    },
                    CreateIndex: 96,
                    ModifyIndex: 58,
                },
            },
            Service: &api.ServiceTxnOp{
                Verb: "TorUpGyEngNkxvgFeIjHgwyJi",
                Node: "NEbdNybpLdBHBWklNBqrtEFaF",
                Service: api.AgentService{
                    Kind:    "zdjMDamJfJOQKpNEzxThGjrNd",
                    ID:      "lKwMnuNvGuujPGGigUETRsibN",
                    Service: "hZoOoNRkySIvNMyXZMxqpvnBX",
                    Tags:    nil,
                    Meta: map[string]string{
                        "CScdKhmjojBiOUkksIyRsUezZ": "ORsdtERCqUfFlbIgxFiMmZwqe",
                        "oBJzFaEhHlHZufjfLMzAZbmuB": "pUzaNjqvvGBDUctjoqeQyqMWx",
                        "vOtvFeyZQzdtetgunusyfHhkV": "vNIHVqaTCIAEdTAhIpnaEzRTZ",
                    },
                    Port:    31,
                    Address: "gVTaCcbehltNslOyoVKRPFykD",
                    TaggedAddresses: map[string]api.ServiceAddress{
                        "fSzoHGQjYtKYtlUatmMdCicCJ": api.ServiceAddress{
                            Address: "aWbVngjORsiveZuRtWvFRwruW",
                            Port:    86,
                        },
                    },
                    Weights: api.AgentWeights{
                        Passing: 46,
                        Warning: 40,
                    },
                    EnableTagOverride: false,
                    CreateIndex:       38,
                    ModifyIndex:       85,
                    ContentHash:       "nNvQKctxiDVoWlxpAEWLszCQQ",
                    Proxy: &api.AgentServiceConnectProxyConfig{
                        DestinationServiceName: "SsUUucquwmmFtTBNODGVOxJhS",
                        DestinationServiceID:   "BGkHcrIsfQOYVsoGBRpLrSRrN",
                        LocalServiceAddress:    "cboecAdgiKSNaYpQxHTosjdrm",
                        LocalServicePort:       67,
                        Config:                 map[string]interface{}{"a": []interface{}{"a", "b"}},
                        Upstreams:              []api.Upstream{{DestinationNamespace: "a", Datacenter: "b"}},
                        MeshGateway: api.MeshGatewayConfig{
                            Mode: "XmdGptouMkWdXVfhsRynmrRyJ",
                        },
                        Expose: api.ExposeConfig{
                            Checks: true,
                            Paths: []api.ExposePath{
                                {
                                    ListenerPort:    1,
                                    Path:            "zmvwmBwHmdZZvHoGwMLqhpWFQ",
                                    LocalPathPort:   76,
                                    Protocol:        "XJBkrrlBhyaKJXQldeYQyQRlA",
                                    ParsedFromCheck: false,
                                },
                            },
                        },
                    },
                    Connect: &api.AgentServiceConnect{
                        Native: true,
                        SidecarService: &api.AgentServiceRegistration{
                            Kind: "uLNfcsNmxNJIlVbqdNkmTmSeO",
                            ID:   "kxRmkxZIwiPILaRPwoAUYZVGw",
                            Name: "cfvuzKLFmtepVXtXNAupSoNRI",
                            Tags: []string{
                                "nNABnFgSwGejWwjSVIbtZVHzX",
                                "ZbqnLSgRpacFBOEKYFwwmEkAP",
                                "XliPVOTPzbRSGSHQHsWhyEixB",
                            },
                            Port:    79,
                            Address: "gWFGbPLhzJEfJyKLQvzhDLCVH",
                            Meta: map[string]string{
                                "QdsCYEZBVnEeSbQukezFiUzNW": "ubEywdLpXHzgqQKUDExwSEuaw",
                                "TcsPrVVzctJgTgPFMREmOZUVJ": "KyaOwEqjAgkfCQDNhERkimmEK",
                                "WtSAeQGJjUxetzzPXEgNWfoTX": "cnYZtDCuONEXeskhJaXuZoBfO",
                            },
                            Weights: &api.AgentWeights{
                                Passing: 36,
                                Warning: 67,
                            },
                            Check: &api.AgentServiceCheck{
                                CheckID:           "aPYronlUgogTpMgifXaSaDhjw",
                                Name:              "oRZACBwHqLnCiCKZQhAIsvtbU",
                                Args:              nil,
                                DockerContainerID: "ksgmUjXaGnCBftnlIDQGrKXci",
                                Shell:             "jWjcxmSlbMhNphgvJOkdKFRfd",
                                Interval:          "ReGgAEThqNrDjPcBoXQhfbwLn",
                                Timeout:           "sQoBHFiHwCElObOkDxcLDfMwc",
                                TTL:               "YomkdRYzZSprGyKKacdbErZzY",
                                HTTP:              "PJdpozKoAhfLlghtWyiLQSksy",
                                Header: map[string][]string{
                                    "FsYFGasjjbsEQVvxHDzmiOFIT": []string{
                                        "AueitZMnHgJhYARywoIyAgqBg",
                                    },
                                    "VDiIlojOySPSwGmXhwAsOovAs": nil,
                                    "jIxgDRhzPiCKnnZyUmioUHiHz": []string{
                                        "WeNLLcmjOYpxnWsEYMpnkknfy",
                                        "CLCmXqVrmqpcvYZQTsIxIhPFE",
                                        "ASpCFpEXySlZHlGfUUyZbUxWQ",
                                    },
                                    "jTmskqkVFAOxyAfeEMduCPHiS": []string{
                                        "AlBEBRPkmqdWSBLOhPzEgOBsb",
                                        "LXVMNQffaJLTBPmyDcNGpmCnu",
                                        "hPvbIKuQPyswFVUwlhLcRNktC",
                                        "scCIVNxGbfZXScuaMRzrcOnjr",
                                    },
                                },
                                Method:                         "MeROgsXljsTPkkeaScCPKoguj",
                                TCP:                            "eUNcgTsmShhyOSyuLUJItzftk",
                                Status:                         "TzFMpHZmvkYFteUOJneZhQTUX",
                                Notes:                          "rlTBDijxJsafhgmUokqhstAGb",
                                TLSSkipVerify:                  true,
                                GRPC:                           "TsSXWkgmRkFRWBGCALAbbeYYU",
                                GRPCUseTLS:                     true,
                                AliasNode:                      "DhuHnFarJUVxKpmfnhOFerMUP",
                                AliasService:                   "kpBBcEQEiukQMvWJhdevtstdL",
                                DeregisterCriticalServiceAfter: "KiDfnfqPXwhWskyBsDtXWvEcw",
                            },
                            Checks: api.AgentServiceChecks{
                                &api.AgentServiceCheck{
                                    CheckID:           "aSfjUkvTYrWNdnOiCoHVnwkPG",
                                    Name:              "roECnIUGJeCisbuJbNFILUzzw",
                                    Args:              nil,
                                    DockerContainerID: "xPtcJlUDSrRphThTOaPLLtORM",
                                    Shell:             "qNBIZZiAdKhpmjDhIKmQCXLJm",
                                    Interval:          "owdXiJzMFqvysWaLxxkIJpLIo",
                                    Timeout:           "YPzTyDgLwDbDDnavmlVpBhFyw",
                                    TTL:               "GWHiGsaQRnjyAeOIBRbILlnNz",
                                    HTTP:              "ZTyGRJnYqWXgYPCYlzsdOZZMs",
                                    Header: map[string][]string{
                                        "NSBetqehGhCVKrSlehesskDPi": []string{
                                            "cMKNaJuiDSlmPkfRLMEZhLNce",
                                            "VEBbHHNnOUPDVezblyeDDPfsW",
                                        },
                                        "jVslXkOALVmwIdMYZthbyXyqS": []string{
                                            "sUqKctQHHVlwmuXsaeXUUlXbl",
                                            "UQsVTxZQevjoRAgevsicvSgEI",
                                        },
                                        "ocbAGkvLcjXEUlqPCmlJpBugO": []string{
                                            "cSpHcSJqbVagEkWBKffzdxCIg",
                                            "UsqLqFELsNtbBXWyuuEOqJQVK",
                                            "SLubRSRzpCnxemiOSpLFKcugM",
                                        },
                                        "rxEwDAuZFLyPWeCXDIibwIJHf": nil,
                                    },
                                    Method:                         "tgCBkeBLEDFIfKjuRnbSwKKiX",
                                    TCP:                            "ROdCCAfELQbTFeljyokiGZvmn",
                                    Status:                         "EVMxCalesoHfggMSRvQvMBnVB",
                                    Notes:                          "fXSCvqWgamqSHjSbhWWbMVqIN",
                                    TLSSkipVerify:                  true,
                                    GRPC:                           "WnyhRjycoVacILSiUkAmFdJZO",
                                    AliasNode:                      "PudhtUowROdGMtTVxGTLZmWVt",
                                    AliasService:                   "twiZosGDsJzTWGPeWzHtHSacs",
                                    DeregisterCriticalServiceAfter: "KPlyDPCtWNNNPzMwJmCsqYGdK",
                                },
                                &api.AgentServiceCheck{
                                    CheckID:           "HkLHtkWLGDBBAPTCyfYBOkWQn",
                                    Name:              "hNAdBkZSVDEonqbhavsSuDiBq",
                                    Args:              nil,
                                    DockerContainerID: "TuqnIGeYsPrBUTaEiyHuvosxg",
                                    Shell:             "SlwtPEBAqkZttHPDTjybhfSag",
                                    Interval:          "uhGdARgJqkgszJhGdnqTsdsUY",
                                    Timeout:           "FpHoBlkicccFMYdawJzpQtmRs",
                                    TTL:               "ziQsYCCDICYSZTalHSriSBCTc",
                                    HTTP:              "CUglrvYwveMkXyMTdIXyLSEDb",
                                    Header: map[string][]string{
                                        "IOasgUiGKYGedGkqRKsMAEHDL": nil,
                                        "VLBppsXUtKKqbsGwPCLVkQVlu": []string{
                                            "aqfnCXHtdNCYztNobbDicUmtQ",
                                            "bpyAcNdhqPoyTiZDSGuXlWMUV",
                                            "hCIfUpRvZlMfEKDAFzWjtfZrN",
                                            "rVNcLUVHodcyHWHdDebMFKkiY",
                                        },
                                        "ndVKJnrpWwvRCwBHQoTBdpNCJ": []string{
                                            "XhKKbQhncmEHrThPZhTJGbwJv",
                                            "BeLbhpekpnJHUgYcfMjtuPKfB",
                                            "dBqDevVgpxyIShBINOmiAMZDQ",
                                        },
                                    },
                                    Method:                         "lHeOFWkNLDqmAQNknkEDLhOYs",
                                    TCP:                            "EFOUEKIUTnNCyAHxGsJBvafQK",
                                    Status:                         "taotLGLXPqROZBGCCphmfvyun",
                                    Notes:                          "uTHzhXYKcxDtpKOFvwJEKSvzJ",
                                    GRPC:                           "jdVLFicYqfyCjKhZEYPLkhzJO",
                                    AliasNode:                      "fUWGLRmauoBpgmfxpDqKfRSRr",
                                    AliasService:                   "YwNITUNCbRjOfjVAmXtVRfviL",
                                    DeregisterCriticalServiceAfter: "fjsvvogszaSbFenLvhwGPIazN",
                                },
                            },
                            Proxy: &api.AgentServiceConnectProxyConfig{
                                DestinationServiceName: "zMZsYnugmtDqSmBWxKhakPkOI",
                                DestinationServiceID:   "SoCkzYSBnLeEvsNtaVIReCoZp",
                                LocalServiceAddress:    "FiyLHtNUqPDNrAktaMwQHCmZE",
                                LocalServicePort:       35,
                                Config:                 map[string]interface{}{"a": []interface{}{"a", "b"}},
                                Upstreams: []api.Upstream{
                                    {
                                        DestinationType:      "hpKYRZfqOZuvAMatGtRGFxVUY",
                                        DestinationNamespace: "FqdDCIhqLZpoQdXLYQzvoBBsG",
                                        DestinationName:      "GXWNyJtmVvDrEgKiihfWQAvbN",
                                        Datacenter:           "WyqVPmJfRmBcDPdclEVXoGWxV",
                                        LocalBindAddress:     "kSKZMYZAOoLJDGBgngzSitAwW",
                                        LocalBindPort:        1,
                                        MeshGateway: api.MeshGatewayConfig{
                                            Mode: "oVimspemmZpYVdZgJrvfjBify",
                                        },
                                    },
                                },
                                MeshGateway: api.MeshGatewayConfig{
                                    Mode: "dnFFBQWuYcrVjGUpuDarbUaDK",
                                },
                                Expose: api.ExposeConfig{
                                    Checks: true,
                                    Paths: []api.ExposePath{
                                        {
                                            ListenerPort:    63,
                                            Path:            "ppnYwsMIddkJIdZLgcRTTbhuj",
                                            LocalPathPort:   64,
                                            Protocol:        "IHrdLEYtaVeAQOjOcHrGKvOyH",
                                            ParsedFromCheck: true,
                                        },
                                        {
                                            ListenerPort:    78,
                                            Path:            "nCFFMqUtwiknPLzmjhnyJwuqD",
                                            LocalPathPort:   90,
                                            Protocol:        "LYdjSiUdKCgHOhEdVdSQsVtNf",
                                            ParsedFromCheck: true,
                                        },
                                    },
                                },
                            },
                            Connect: &api.AgentServiceConnect{
                                Native: true,
                                SidecarService: &api.AgentServiceRegistration{
                                    Kind: "1uLNfcsNmxNJIlVbqdNkmTmSeO",
                                    ID:   "1kxRmkxZIwiPILaRPwoAUYZVGw",
                                    Name: "1cfvuzKLFmtepVXtXNAupSoNRI",
                                    Tags: []string{
                                        "1nNABnFgSwGejWwjSVIbtZVHzX",
                                        "1ZbqnLSgRpacFBOEKYFwwmEkAP",
                                        "1XliPVOTPzbRSGSHQHsWhyEixB",
                                    },
                                    Port: 179,
                                },
                            },
                        },
                    },
                },
            },
            Check: &api.CheckTxnOp{
                Verb: "HmouRVeKEPKxiRCmSVdyEYOPE",
                Check: api.HealthCheck{
                    Node:        "lLvkgAQLFxgMXrDoqfqBswISn",
                    CheckID:     "DoqXdDXKkXuUzfuJHlgsgKzyv",
                    Name:        "ZdZltqOSHyASTHyraSDTmGjjp",
                    Status:      "YBvDJcoQkphEGaQGDJqdKnREU",
                    Notes:       "eluItMsDHhVPKymbrNijkBEFQ",
                    Output:      "KENOcCQQakWfObmazgdYITpNp",
                    ServiceID:   "bSsqqDSJJVhTlzCRzNywHLOlj",
                    ServiceName: "fcDUJDEVzFsqTLYXcMYXcuIeQ",
                    ServiceTags: nil,
                    Definition: api.HealthCheckDefinition{
                        Interval:                       readableDuration("45ns"),
                        Timeout:                        readableDuration("65ns"),
                        DeregisterCriticalServiceAfter: readableDuration("7ns"),
                        HTTP:                           "vJYDirutpVomHpNYFHTQWxqHZ",
                        Header: map[string][]string{
                            "QGdOMTerHUjtrgwIaIYdtSepn": []string{
                                "lbmyGdqTIGzDTwTUTCyymHnfh",
                                "jDyFZXAcYTbLLHtqcqkKmvQEy",
                                "ysfTpggsaRuZXdJYbzBMNHXHb",
                            },
                            "bfHwiOouuBltgTITnfpcFimyF": []string{
                                "hHvogvFspWHRAjqaYTOhScfls",
                                "uvwHmnzohChYLuRizskrRdbZc",
                                "wejmTCtzZLFfvHqDuRizcNEFU",
                            },
                        },
                        Method:        "zDBrxVaFHZTpGpECXsdDderlq",
                        TLSSkipVerify: true,
                        TCP:           "YgIFPmTyuhPKppqCznwIYShrU",
                    },
                    CreateIndex: 63,
                    ModifyIndex: 73,
                },
            },
        },
    }

    // ---
    body := bytes.NewBuffer([]byte(jsonBlob))
    req := httptest.NewRequest("POST", "http://foo.com", body)
    out := api.TxnOps{}
    if err := decodeBody(req, &out, fixupTxnOps); err != nil {
        t.Fatal(err)
    }

    assert.Equal(t, want, out)
    // ---
}

func timePtr(t time.Time) *time.Time {
    return &t
}

func duration(s string) time.Duration {
    d, _ := time.ParseDuration(s)
    return d
}

func readableDuration(s string) api.ReadableDuration {
    d, _ := time.ParseDuration(s)
    return api.ReadableDuration(d)
}

func readableDurationPtr(s string) *api.ReadableDuration {
    d := readableDuration(s)
    return &d
}
