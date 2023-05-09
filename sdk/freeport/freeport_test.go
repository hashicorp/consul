package freeport

import (
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestTakeReturn(t *testing.T) {
	// NOTE: for global var reasons this cannot execute in parallel
	// t.Parallel()

	// Since this test is destructive (i.e. it leaks all ports) it means that
	// any other test cases in this package will not function after it runs. To
	// help out we reset the global state after we run this test.
	defer reset()

	// OK: do a simple take/return cycle to trigger the package initialization
	func() {
		ports, err := Take(1)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		defer Return(ports)

		if len(ports) != 1 {
			t.Fatalf("expected %d but got %d ports", 1, len(ports))
		}
	}()

	waitForStatsReset := func() (numTotal int) {
		t.Helper()
		numTotal, numPending, numFree := stats()
		if numTotal != numFree+numPending {
			t.Fatalf("expected total (%d) and free+pending (%d) ports to match", numTotal, numFree+numPending)
		}
		retry.Run(t, func(r *retry.R) {
			numTotal, numPending, numFree = stats()
			if numPending != 0 {
				r.Fatalf("pending is still non zero: %d", numPending)
			}
			if numTotal != numFree {
				r.Fatalf("total (%d) does not equal free (%d)", numTotal, numFree)
			}
		})
		return numTotal
	}

	// Reset
	numTotal := waitForStatsReset()

	// --------------------
	// OK: take the max
	func() {
		ports, err := Take(numTotal)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		defer Return(ports)

		if len(ports) != numTotal {
			t.Fatalf("expected %d but got %d ports", numTotal, len(ports))
		}
	}()

	// Reset
	numTotal = waitForStatsReset()

	expectError := func(expected string, got error) {
		t.Helper()
		if got == nil {
			t.Fatalf("expected error but was nil")
		}
		if got.Error() != expected {
			t.Fatalf("expected error %q but got %q", expected, got.Error())
		}
	}

	// --------------------
	// ERROR: take too many ports
	func() {
		ports, err := Take(numTotal + 1)
		defer Return(ports)
		expectError("freeport: block size too small", err)
	}()

	// --------------------
	// ERROR: invalid ports request (negative)
	func() {
		_, err := Take(-1)
		expectError("freeport: cannot take -1 ports", err)
	}()

	// --------------------
	// ERROR: invalid ports request (zero)
	func() {
		_, err := Take(0)
		expectError("freeport: cannot take 0 ports", err)
	}()

	// --------------------
	// OK: Steal a port under the covers and let freeport detect the theft and compensate
	leakedPort := peekFree()
	func() {
		leakyListener, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", leakedPort))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		defer leakyListener.Close()

		func() {
			ports, err := Take(3)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			defer Return(ports)

			if len(ports) != 3 {
				t.Fatalf("expected %d but got %d ports", 3, len(ports))
			}

			for _, port := range ports {
				if port == leakedPort {
					t.Fatalf("did not expect for Take to return the leaked port")
				}
			}
		}()

		newNumTotal := waitForStatsReset()
		if newNumTotal != numTotal-1 {
			t.Fatalf("expected total to drop to %d but got %d", numTotal-1, newNumTotal)
		}
		numTotal = newNumTotal // update outer variable for later tests
	}()

	// --------------------
	// OK: sequence it so that one Take must wait on another Take to Return.
	func() {
		mostPorts, err := Take(numTotal - 5)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		type reply struct {
			ports []int
			err   error
		}
		ch := make(chan reply, 1)
		go func() {
			ports, err := Take(10)
			ch <- reply{ports: ports, err: err}
		}()

		Return(mostPorts)

		r := <-ch
		if r.err != nil {
			t.Fatalf("err: %v", r.err)
		}
		defer Return(r.ports)

		if len(r.ports) != 10 {
			t.Fatalf("expected %d ports but got %d", 10, len(r.ports))
		}
	}()

	// Reset
	numTotal = waitForStatsReset()

	// --------------------
	// ERROR: Now we end on the crazy "Ocean's 11" level port theft where we
	// orchestrate a situation where all ports are stolen and we don't find out
	// until Take.
	func() {
		// 1. Grab all of the ports.
		allPorts := peekAllFree()

		// 2. Leak all of the ports
		leaked := make([]io.Closer, 0, len(allPorts))
		defer func() {
			for _, c := range leaked {
				c.Close()
			}
		}()
		for i, port := range allPorts {
			ln, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", port))
			if err != nil {
				t.Fatalf("%d err: %v", i, err)
			}
			leaked = append(leaked, ln)
		}

		// 3. Request 1 port which will detect the leaked ports and fail.
		_, err := Take(1)
		expectError("freeport: impossible to satisfy request; there are no actual free ports in the block anymore", err)

		// 4. Wait for the block to zero out.
		newNumTotal := waitForStatsReset()
		if newNumTotal != 0 {
			t.Fatalf("expected total to drop to %d but got %d", 0, newNumTotal)
		}
	}()
}

func TestIntervalOverlap(t *testing.T) {
	cases := []struct {
		min1, max1, min2, max2 int
		overlap                bool
	}{
		{0, 0, 0, 0, true},
		{1, 1, 1, 1, true},
		{1, 3, 1, 3, true},  // same
		{1, 3, 4, 6, false}, // serial
		{1, 4, 3, 6, true},  // inner overlap
		{1, 6, 3, 4, true},  // nest
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d:%d vs %d:%d", tc.min1, tc.max1, tc.min2, tc.max2), func(t *testing.T) {
			if tc.overlap != intervalOverlap(tc.min1, tc.max1, tc.min2, tc.max2) { // 1 vs 2
				t.Fatalf("expected %v but got %v", tc.overlap, !tc.overlap)
			}
			if tc.overlap != intervalOverlap(tc.min2, tc.max2, tc.min1, tc.max1) { // 2 vs 1
				t.Fatalf("expected %v but got %v", tc.overlap, !tc.overlap)
			}
		})
	}
}
