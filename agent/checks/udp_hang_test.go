package checks

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// silentUDPServer receives datagrams but never replies, which is what a
// service that is up but not answering looks like on the wire.
func silentUDPServer(ctx context.Context, port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return
	}
	ser, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return
	}
	defer ser.Close()

	b := make([]byte, 1024)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			// Read the datagram and deliberately do not answer it.
			_, _, err := ser.ReadFromUDP(b)
			if err != nil {
				return
			}
		}
	}()
	<-ctx.Done()
}

// TestCheckUDPDoesNotHangForever reproduces #22068.
//
// A UDP check sends a datagram and then waits for a reply. The wait has no
// deadline of its own: net.Dialer's Timeout only bounds establishing the
// connection, which for UDP is a no-op, so a peer that receives the datagram
// but never answers leaves the check blocked in Read forever.
//
// A check that never completes never reports a new status, so the service
// stays in whatever state it had - "passing", from a previous check - even
// though the peer is no longer answering. The check then stops running for
// good: the status is never updated again.
func TestCheckUDPDoesNotHangForever(t *testing.T) {
	port := freeport.GetOne(t)
	serverURL := "127.0.0.1:" + strconv.Itoa(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go silentUDPServer(ctx, port)

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckUDP{
		CheckID:       cid,
		UDP:           serverURL,
		Interval:      10 * time.Millisecond,
		Timeout:       100 * time.Millisecond,
		Message:       "hello",
		Logger:        logger,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()

	// The check must report a status within a bounded time even though the
	// peer never answers. Without a read deadline the check blocks in Read
	// and no status is ever reported.
	retry.Run(t, func(r *retry.R) {
		if got := notif.Updates(cid); got == 0 {
			r.Fatalf("no status was reported; the check is stuck waiting for a reply")
		}
	})

	// And the check must keep running: it has to report again on the next
	// interval rather than staying wedged on the first one.
	first := notif.Updates(cid)
	retry.Run(t, func(r *retry.R) {
		if got := notif.Updates(cid); got <= first {
			r.Fatalf("the check stopped reporting after %d updates", got)
		}
	})
}

var _ = require.New
