package finder

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

// IDFromArgs returns the intention ID for the given CLI args. An error is returned
// if args is not 1 or 2 elements.
func IDFromArgs(client *api.Client, args []string) (string, error) {
	switch len(args) {
	case 1:
		return args[0], nil

	case 2:
		ixn, _, err := client.Connect().IntentionGetExact(
			args[0], args[1], nil,
		)
		if err != nil {
			return "", err
		}
		if ixn == nil {
			return "", fmt.Errorf(
				"Intention with source %q and destination %q not found.",
				args[0], args[1])
		}

		return ixn.ID, nil

	default:
		return "", fmt.Errorf("command requires exactly 1 or 2 arguments")
	}
}
