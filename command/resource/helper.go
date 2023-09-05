package resource

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

func GetTypeAndResourceName(args []string) (gvk *api.GVK, resourceName string, e error) {
	// it has to be resource name after the type
	if strings.HasPrefix(args[1], "-") {
		return nil, "", fmt.Errorf("Must provide resource name right after type")
	}

	s := strings.Split(args[0], ".")
	gvk = &api.GVK{
		Group:   s[0],
		Version: s[1],
		Kind:    s[2],
	}

	resourceName = args[1]
	return
}
