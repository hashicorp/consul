package sarama

import (
	"testing"
	"time"
)

var aclDescribeResponseError = []byte{
	0, 0, 0, 100,
	0, 8, // error
	0, 5, 'e', 'r', 'r', 'o', 'r',
	0, 0, 0, 1, // 1 resource
	2, // cluster type
	0, 5, 't', 'o', 'p', 'i', 'c',
	0, 0, 0, 1, // 1 acl
	0, 9, 'p', 'r', 'i', 'n', 'c', 'i', 'p', 'a', 'l',
	0, 4, 'h', 'o', 's', 't',
	4, // write
	3, // allow
}

func TestAclDescribeResponse(t *testing.T) {
	errmsg := "error"
	resp := &DescribeAclsResponse{
		ThrottleTime: 100 * time.Millisecond,
		Err:          ErrBrokerNotAvailable,
		ErrMsg:       &errmsg,
		ResourceAcls: []*ResourceAcls{{
			Resource: Resource{
				ResourceName: "topic",
				ResourceType: AclResourceTopic,
			},
			Acls: []*Acl{
				{
					Principal:      "principal",
					Host:           "host",
					Operation:      AclOperationWrite,
					PermissionType: AclPermissionAllow,
				},
			},
		}},
	}

	testResponse(t, "describe", resp, aclDescribeResponseError)
}
