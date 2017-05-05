package api

import (
	"testing"

	"github.com/hashicorp/consul/testutil/retry"
)

func TestCoordinate_Datacenters(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	coordinate := c.Coordinate()
	retry.Run(t, func(r *retry.R) {
		datacenters, err := coordinate.Datacenters()
		if err != nil {
			r.Fatal(err)
		}

		if len(datacenters) == 0 {
			r.Fatalf("Bad: %v", datacenters)
		}
	})
}

func TestCoordinate_Nodes(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	coordinate := c.Coordinate()
	retry.Run(t, func(r *retry.R) {
		_, _, err := coordinate.Nodes(nil)
		if err != nil {
			r.Fatal(err)
		}

		// There's not a good way to populate coordinates without
		// waiting for them to calculate and update, so the best
		// we can do is call the endpoint and make sure we don't
		// get an error.
	})
}
