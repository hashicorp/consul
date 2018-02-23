package sarama

import (
	"testing"
)

var (
	alterResponseEmpty = []byte{
		0, 0, 0, 0, //throttle
		0, 0, 0, 0, // no configs
	}

	alterResponsePopulated = []byte{
		0, 0, 0, 0, //throttle
		0, 0, 0, 1, // response
		0, 0, //errorcode
		0, 0, //string
		2, // topic
		0, 3, 'f', 'o', 'o',
	}
)

func TestAlterConfigsResponse(t *testing.T) {
	var response *AlterConfigsResponse

	response = &AlterConfigsResponse{
		Resources: []*AlterConfigsResourceResponse{},
	}
	testVersionDecodable(t, "empty", response, alterResponseEmpty, 0)
	if len(response.Resources) != 0 {
		t.Error("Expected no groups")
	}

	response = &AlterConfigsResponse{
		Resources: []*AlterConfigsResourceResponse{
			&AlterConfigsResourceResponse{
				ErrorCode: 0,
				ErrorMsg:  "",
				Type:      TopicResource,
				Name:      "foo",
			},
		},
	}
	testResponse(t, "response with error", response, alterResponsePopulated)
}
