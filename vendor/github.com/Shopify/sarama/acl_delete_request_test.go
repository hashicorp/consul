package sarama

import "testing"

var (
	aclDeleteRequestNulls = []byte{
		0, 0, 0, 1,
		1,
		255, 255,
		255, 255,
		255, 255,
		11,
		3,
	}

	aclDeleteRequest = []byte{
		0, 0, 0, 1,
		1, // any
		0, 6, 'f', 'i', 'l', 't', 'e', 'r',
		0, 9, 'p', 'r', 'i', 'n', 'c', 'i', 'p', 'a', 'l',
		0, 4, 'h', 'o', 's', 't',
		4, // write
		3, // allow
	}

	aclDeleteRequestArray = []byte{
		0, 0, 0, 2,
		1,
		0, 6, 'f', 'i', 'l', 't', 'e', 'r',
		0, 9, 'p', 'r', 'i', 'n', 'c', 'i', 'p', 'a', 'l',
		0, 4, 'h', 'o', 's', 't',
		4, // write
		3, // allow
		2,
		0, 5, 't', 'o', 'p', 'i', 'c',
		255, 255,
		255, 255,
		6,
		2,
	}
)

func TestDeleteAclsRequest(t *testing.T) {
	req := &DeleteAclsRequest{
		Filters: []*AclFilter{{
			ResourceType:   AclResourceAny,
			Operation:      AclOperationAlterConfigs,
			PermissionType: AclPermissionAllow,
		}},
	}

	testRequest(t, "delete request nulls", req, aclDeleteRequestNulls)

	req.Filters[0].ResourceName = nullString("filter")
	req.Filters[0].Principal = nullString("principal")
	req.Filters[0].Host = nullString("host")
	req.Filters[0].Operation = AclOperationWrite

	testRequest(t, "delete request", req, aclDeleteRequest)

	req.Filters = append(req.Filters, &AclFilter{
		ResourceType:   AclResourceTopic,
		ResourceName:   nullString("topic"),
		Operation:      AclOperationDelete,
		PermissionType: AclPermissionDeny,
	})

	testRequest(t, "delete request array", req, aclDeleteRequestArray)
}
