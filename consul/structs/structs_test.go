package structs

import (
	"reflect"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	arg := &RegisterRequest{
		Datacenter: "foo",
		Node:       "bar",
		Address:    "baz",
		Service: &NodeService{
			Service: "test",
			Address: "127.0.0.2",
		},
	}
	buf, err := Encode(RegisterRequestType, arg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out RegisterRequest
	err = Decode(buf[1:], &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(arg.Service, out.Service) {
		t.Fatalf("bad: %#v %#v", arg.Service, out.Service)
	}
	if !reflect.DeepEqual(arg, &out) {
		t.Fatalf("bad: %#v %#v", arg, out)
	}
}

func TestStructs_Implements(t *testing.T) {
	var (
		_ RPCInfo          = &RegisterRequest{}
		_ RPCInfo          = &DeregisterRequest{}
		_ RPCInfo          = &DCSpecificRequest{}
		_ RPCInfo          = &ServiceSpecificRequest{}
		_ RPCInfo          = &NodeSpecificRequest{}
		_ RPCInfo          = &ChecksInStateRequest{}
		_ RPCInfo          = &KVSRequest{}
		_ RPCInfo          = &KeyRequest{}
		_ RPCInfo          = &KeyListRequest{}
		_ RPCInfo          = &SessionRequest{}
		_ RPCInfo          = &SessionSpecificRequest{}
		_ RPCInfo          = &EventFireRequest{}
		_ RPCInfo          = &ACLPolicyRequest{}
		_ RPCInfo          = &KeyringRequest{}
		_ CompoundResponse = &KeyringResponses{}
	)
}

func TestStructs_ServiceNode_Clone(t *testing.T) {
	sn := &ServiceNode{
		Node:           "node1",
		Address:        "127.0.0.1",
		ServiceID:      "service1",
		ServiceName:    "dogs",
		ServiceTags:    []string{"prod", "v1"},
		ServiceAddress: "127.0.0.2",
		ServicePort:    8080,
		RaftIndex:      RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}

	clone := sn.Clone()
	if !reflect.DeepEqual(sn, clone) {
		t.Fatalf("bad: %v", clone)
	}

	sn.ServiceTags = append(sn.ServiceTags, "hello")
	if reflect.DeepEqual(sn, clone) {
		t.Fatalf("clone wasn't independent of the original")
	}
}

func TestStructs_DirEntry_Clone(t *testing.T) {
	e := &DirEntry{
		LockIndex: 5,
		Key: "hello",
		Flags: 23,
		Value: []byte("this is a test"),
		Session: "session1",
		RaftIndex:      RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}

	clone := e.Clone()
	if !reflect.DeepEqual(e, clone) {
		t.Fatalf("bad: %v", clone)
	}

	e.Value = []byte("a new value")
	if reflect.DeepEqual(e, clone) {
		t.Fatalf("clone wasn't independent of the original")
	}
}
