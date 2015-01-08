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
