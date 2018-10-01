package agent

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// aclCreateResponse is used to wrap the ACL ID
type aclBootstrapResponse struct {
	ID string
	structs.ACLToken
}

type aclTokenResponse struct {
	Legacy bool `json:",omitempty"`
	structs.ACLToken
}

func convertTokenForResponse(in *structs.ACLToken) *aclTokenResponse {
	out := &aclTokenResponse{
		ACLToken: *in,
	}

	if in.Rules != "" || in.Type == structs.ACLTokenTypeClient || in.AccessorID == "" {
		out.Legacy = true
	}

	return out
}

// checkACLDisabled will return a standard response if ACLs are disabled. This
// returns true if they are disabled and we should not continue.
func (s *HTTPServer) checkACLDisabled(resp http.ResponseWriter, req *http.Request) bool {
	if s.agent.delegate.ACLsEnabled() {
		return false
	}

	resp.WriteHeader(http.StatusUnauthorized)
	fmt.Fprint(resp, "ACL support disabled")
	return true
}

// ACLBootstrap is used to perform a one-time ACL bootstrap operation on
// a cluster to get the first management token.
func (s *HTTPServer) ACLBootstrap(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	args := structs.DCSpecificRequest{
		Datacenter: s.agent.config.Datacenter,
	}

	var out structs.ACLToken
	err := s.agent.RPC("ACL.Bootstrap", &args, &out)
	if err != nil {
		if strings.Contains(err.Error(), structs.ACLBootstrapNotAllowedErr.Error()) {
			resp.WriteHeader(http.StatusForbidden)
			fmt.Fprint(resp, acl.PermissionDeniedError{Cause: err.Error()}.Error())
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &aclBootstrapResponse{ID: out.SecretID, ACLToken: out}, nil
}

func (s *HTTPServer) ACLReplicationStatus(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	// Note that we do not forward to the ACL DC here. This is a query for
	// any DC that's doing replication.
	args := structs.DCSpecificRequest{}
	s.parseSource(req, &args.Source)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Make the request.
	var out structs.ACLReplicationStatus
	if err := s.agent.RPC("ACL.ReplicationStatus", &args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPServer) ACLPolicyTranslate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	policyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Failed to read body: %v", err)}
	}

	translated, err := acl.TranslateLegacyRules(policyBytes)
	if err != nil {
		return nil, BadRequestError{Reason: err.Error()}
	}

	resp.Write(translated)
	return nil, nil
}

func (s *HTTPServer) ACLPolicyList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	var out structs.ACLPolicyMultiResponse
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.PolicyList", &args, &out); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	if out.Policies == nil {
		out.Policies = make(structs.ACLPolicies, 0)
	}

	return out.Policies, nil
}

func (s *HTTPServer) ACLPolicyCRUD(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	var fn func(resp http.ResponseWriter, req *http.Request, policyID string) (interface{}, error)

	switch req.Method {
	case "GET":
		fn = s.ACLPolicyRead

	case "PUT":
		fn = s.ACLPolicyWrite

	case "DELETE":
		fn = s.ACLPolicyDelete

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "PUT", "DELETE"}}
	}

	policyID := strings.TrimPrefix(req.URL.Path, "/v1/acl/policy/")
	if policyID == "" && req.Method != "PUT" {
		return nil, BadRequestError{Reason: "Missing policy ID"}
	}

	return fn(resp, req, policyID)
}

func (s *HTTPServer) ACLPolicyRead(resp http.ResponseWriter, req *http.Request, policyID string) (interface{}, error) {
	args := structs.ACLPolicyReadRequest{
		Datacenter: s.agent.config.Datacenter,
		ID:         policyID,
		IDType:     structs.ACLPolicyID,
	}

	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	var out structs.ACLPolicyResponse
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.PolicyRead", &args, &out); err != nil {
		return nil, err
	}

	if out.Policy == nil {
		return nil, acl.ErrNotFound
	}

	return out.Policy, nil
}

func (s *HTTPServer) ACLPolicyCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.ACLPolicyWrite(resp, req, "")
}

func (s *HTTPServer) ACLPolicyWrite(resp http.ResponseWriter, req *http.Request, policyID string) (interface{}, error) {
	args := structs.ACLPolicyWriteRequest{
		Datacenter: s.agent.config.Datacenter,
		Op:         structs.ACLSet,
	}
	s.parseToken(req, &args.Token)

	if err := decodeBody(req, &args.Policy, nil); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Policy decoding failed: %v", err)}
	}

	args.Policy.Syntax = acl.SyntaxCurrent

	// TODO (ACL-V2) - Should we allow not specifying the ID in the payload when its specified in the URL
	if policyID != "" && args.Policy.ID != policyID {
		return nil, BadRequestError{Reason: "Policy ID in URL and payload do not match"}
	}

	var out structs.ACLPolicy
	if err := s.agent.RPC("ACL.PolicyWrite", args, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (s *HTTPServer) ACLPolicyDelete(resp http.ResponseWriter, req *http.Request, policyID string) (interface{}, error) {
	args := structs.ACLPolicyWriteRequest{
		Datacenter: s.agent.config.Datacenter,
		Op:         structs.ACLDelete,
		Policy: structs.ACLPolicy{
			ID: policyID,
		},
	}
	s.parseToken(req, &args.Token)

	var out structs.ACLPolicy
	if err := s.agent.RPC("ACL.PolicyWrite", args, &out); err != nil {
		return nil, err
	}

	return true, nil
}

func (s *HTTPServer) ACLTokenList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	var out structs.ACLTokensResponse
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.TokenList", &args, &out); err != nil {
		return nil, err
	}

	tokens := make([]*aclTokenResponse, 0, len(out.Tokens))
	for _, token := range out.Tokens {
		tokens = append(tokens, convertTokenForResponse(token))
	}

	return tokens, nil
}

func (s *HTTPServer) ACLTokenCRUD(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	var fn func(resp http.ResponseWriter, req *http.Request, tokenID string) (interface{}, error)

	switch req.Method {
	case "GET":
		fn = s.ACLTokenRead

	case "PUT":
		fn = s.ACLTokenWrite

	case "DELETE":
		fn = s.ACLTokenDelete

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "PUT", "DELETE"}}
	}

	tokenID := strings.TrimPrefix(req.URL.Path, "/v1/acl/token/")
	if tokenID == "" && req.Method != "PUT" {
		return nil, BadRequestError{Reason: "Missing token ID"}
	}

	return fn(resp, req, tokenID)
}

func (s *HTTPServer) ACLTokenSelf(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ACLTokenReadRequest{
		IDType: structs.ACLTokenSecret,
	}

	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// copy the token parameter to the ID
	args.ID = args.Token

	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	var out structs.ACLTokenResponse
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.TokenRead", &args, &out); err != nil {
		return nil, err
	}

	if out.Token == nil {
		return nil, acl.ErrNotFound
	}

	return convertTokenForResponse(out.Token), nil
}

func (s *HTTPServer) ACLTokenCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	return s.ACLTokenWrite(resp, req, "")
}

func (s *HTTPServer) ACLTokenRead(resp http.ResponseWriter, req *http.Request, tokenID string) (interface{}, error) {
	args := structs.ACLTokenReadRequest{
		Datacenter: s.agent.config.Datacenter,
		ID:         tokenID,
		IDType:     structs.ACLTokenAccessor,
	}

	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	var out structs.ACLTokenResponse
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.TokenRead", &args, &out); err != nil {
		return nil, err
	}

	if out.Token == nil {
		return nil, acl.ErrNotFound
	}

	return convertTokenForResponse(out.Token), nil
}

func (s *HTTPServer) ACLTokenWrite(resp http.ResponseWriter, req *http.Request, tokenID string) (interface{}, error) {
	args := structs.ACLTokenWriteRequest{
		Datacenter: s.agent.config.Datacenter,
		Op:         structs.ACLSet,
	}
	s.parseToken(req, &args.Token)

	if err := decodeBody(req, &args.ACLToken, nil); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Token decoding failed: %v", err)}
	}

	// TODO (ACL-V2) - should we do more validation here or just defer to the RPC layer

	// TODO (ACL-V2) - Should we allow not specifying the ID in the payload when its specified in the URL
	if tokenID != "" && args.ACLToken.AccessorID != tokenID {
		return nil, BadRequestError{Reason: "Token Accessor ID in URL and payload do not match"}
	}

	var out structs.ACLToken
	if err := s.agent.RPC("ACL.TokenWrite", args, &out); err != nil {
		return nil, err
	}

	return convertTokenForResponse(&out), nil
}

func (s *HTTPServer) ACLTokenDelete(resp http.ResponseWriter, req *http.Request, tokenID string) (interface{}, error) {
	args := structs.ACLTokenWriteRequest{
		Datacenter: s.agent.config.Datacenter,
		Op:         structs.ACLDelete,
		ACLToken: structs.ACLToken{
			AccessorID: tokenID,
		},
	}
	s.parseToken(req, &args.Token)

	var out structs.ACLToken
	if err := s.agent.RPC("ACL.TokenWrite", args, &out); err != nil {
		return nil, err
	}

	return true, nil
}

func (s *HTTPServer) ACLTokenClone(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	tokenID := strings.TrimPrefix(req.URL.Path, "/v1/acl/token/clone/")
	if tokenID == "" {
		return nil, BadRequestError{Reason: "Missing token ID"}
	}

	args := structs.ACLTokenWriteRequest{
		Datacenter: s.agent.config.Datacenter,
		Op:         structs.ACLSet,
	}

	if req.Body != nil {
		if err := decodeBody(req, &args.ACLToken, nil); err != nil {
			return nil, BadRequestError{Reason: fmt.Sprintf("Token decoding failed: %v", err)}
		}
	}
	s.parseToken(req, &args.Token)

	// Set this for the ID to clone
	args.ACLToken.AccessorID = tokenID

	var out structs.ACLToken
	if err := s.agent.RPC("ACL.TokenClone", args, &out); err != nil {
		return nil, err
	}

	return convertTokenForResponse(&out), nil
}

func (s *HTTPServer) ACLTokenUpgrade(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}
	return nil, fmt.Errorf("// TODO (ACL-V2) - Implement token upgrade once I have other backwards compat worked out")
}
