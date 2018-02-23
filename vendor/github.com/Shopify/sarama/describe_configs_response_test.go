package sarama

import (
	"testing"
)

var (
	describeConfigsResponseEmpty = []byte{
		0, 0, 0, 0, //throttle
		0, 0, 0, 0, // no configs
	}

	describeConfigsResponsePopulated = []byte{
		0, 0, 0, 0, //throttle
		0, 0, 0, 1, // response
		0, 0, //errorcode
		0, 0, //string
		2, // topic
		0, 3, 'f', 'o', 'o',
		0, 0, 0, 1, //configs
		0, 10, 's', 'e', 'g', 'm', 'e', 'n', 't', '.', 'm', 's',
		0, 4, '1', '0', '0', '0',
		0, // ReadOnly
		0, // Default
		0, // Sensitive
	}
)

func TestDescribeConfigsResponse(t *testing.T) {
	var response *DescribeConfigsResponse

	response = &DescribeConfigsResponse{
		Resources: []*ResourceResponse{},
	}
	testVersionDecodable(t, "empty", response, describeConfigsResponseEmpty, 0)
	if len(response.Resources) != 0 {
		t.Error("Expected no groups")
	}

	response = &DescribeConfigsResponse{
		Resources: []*ResourceResponse{
			&ResourceResponse{
				ErrorCode: 0,
				ErrorMsg:  "",
				Type:      TopicResource,
				Name:      "foo",
				Configs: []*ConfigEntry{
					&ConfigEntry{
						Name:      "segment.ms",
						Value:     "1000",
						ReadOnly:  false,
						Default:   false,
						Sensitive: false,
					},
				},
			},
		},
	}
	testResponse(t, "response with error", response, describeConfigsResponsePopulated)
}
