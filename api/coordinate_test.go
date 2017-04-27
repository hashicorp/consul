package api

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testutil/retry"
)

func TestCoordinate_Datacenters(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	coordinate := c.Coordinate()

	retry.Fatal(t, func() error {
		datacenters, err := coordinate.Datacenters()
		if err != nil {
			return err
		}
		if len(datacenters) == 0 {
			return fmt.Errorf("Bad: %v", datacenters)
		}
		return nil
	})
}

func TestCoordinate_Nodes(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	coordinate := c.Coordinate()

	retry.Fatal(t, func() error {
		// There's not a good way to populate coordinates without
		// waiting for them to calculate and update, so the best
		// we can do is call the endpoint and make sure we don't
		// get an error.
		_, _, err := coordinate.Nodes(nil)
		return err
	})
}
