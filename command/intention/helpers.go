package intention

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

// ParseIntentionTarget parses a target of the form <namespace>/<name> and returns
// the two distinct parts. In some cases the namespace may be elided and this function
// will return the empty string for the namespace then.
func ParseIntentionTarget(input string) (name string, namespace string, err error) {
	// Get the index to the '/'. If it doesn't exist, we have just a name
	// so just set that and return.
	idx := strings.IndexByte(input, '/')
	if idx == -1 {
		// let the agent do token based defaulting of the namespace
		return input, "", nil
	}

	namespace = input[:idx]
	name = input[idx+1:]
	if strings.IndexByte(name, '/') != -1 {
		return "", "", fmt.Errorf("target can contain at most one '/'")
	}

	return name, namespace, nil
}

func GetFromArgs(client *api.Client, args []string) (*api.Intention, error) {
	switch len(args) {
	case 1:
		id := args[0]

		//nolint:staticcheck
		ixn, _, err := client.Connect().IntentionGet(id, nil)
		if err != nil {
			return nil, fmt.Errorf("Error reading the intention: %s", err)
		} else if ixn == nil {
			return nil, fmt.Errorf("Intention not found with ID %q", id)
		}

		return ixn, nil

	case 2:
		source, destination := args[0], args[1]

		ixn, _, err := client.Connect().IntentionGetExact(source, destination, nil)
		if err != nil {
			return nil, fmt.Errorf("Error reading the intention: %s", err)
		} else if ixn == nil {
			return nil, fmt.Errorf("Intention not found with source %q and destination %q", source, destination)
		}

		return ixn, nil

	default:
		return nil, fmt.Errorf("command requires exactly 1 or 2 arguments")
	}
}
