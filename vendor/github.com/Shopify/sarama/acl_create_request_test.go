package sarama

import "testing"

var (
	aclCreateRequest = []byte{
		0, 0, 0, 1,
		3, // resource type = group
		0, 5, 'g', 'r', 'o', 'u', 'p',
		0, 9, 'p', 'r', 'i', 'n', 'c', 'i', 'p', 'a', 'l',
		0, 4, 'h', 'o', 's', 't',
		2, // all
		2, // deny
	}
)

func TestCreateAclsRequest(t *testing.T) {
	req := &CreateAclsRequest{
		AclCreations: []*AclCreation{{
			Resource: Resource{
				ResourceType: AclResourceGroup,
				ResourceName: "group",
			},
			Acl: Acl{
				Principal:      "principal",
				Host:           "host",
				Operation:      AclOperationAll,
				PermissionType: AclPermissionDeny,
			}},
		},
	}

	testRequest(t, "create request", req, aclCreateRequest)
}
