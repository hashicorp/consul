package intention

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

// ParseIntentionTarget parses a target of the form <partition>/<namespace>/<name> and returns
// the distinct parts. In some cases the partition and namespace may be elided and this function
// will return the empty string for them then.
// If two parts are present, it is assumed they are namespace/name and not partition/name.
func ParseIntentionTarget(input string) (name string, ns string, partition string, err error) {
	ss := strings.Split(input, "/")
	switch len(ss) {
	case 1: // Name only
		name = ss[0]
		return
	case 2: // namespace/name
		ns = ss[0]
		name = ss[1]
		return
	case 3: // partition/namespace/name
		partition = ss[0]
		ns = ss[1]
		name = ss[2]
		return
	default:
		return "", "", "", fmt.Errorf("input can contain at most two '/'")
	}
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
